package plugins

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"res-downloader/core/shared"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/elazarl/goproxy"
	gonanoid "github.com/matoous/go-nanoid/v2"
)

type KsMeta struct {
	PhotoId  string
	Caption  string
	CoverUrl string
	UserName string
	PlayUrl  string
}

type KsRecentItem struct {
	Meta      KsMeta
	Timestamp time.Time
}

type KuaishouPlugin struct {
	bridge      *shared.Bridge
	metaCache   sync.Map // key (url / cleanUrl / photoId / filename / path / cck) -> KsMeta
	recentMu    sync.RWMutex
	recentMetas []KsRecentItem
}

var (
	ksPhotoIdRegex        = regexp.MustCompile(`(?:short-video|photo|fw/photo|new-reco|f|video|work)/([a-zA-Z0-9_-]+)`)
	ks3xIdRegex           = regexp.MustCompile(`\b(3x[a-zA-Z0-9_-]{8,})`)
	ksFilenameSuffixRegex = regexp.MustCompile(`(?:_[a-zA-Z0-9]+)?(\.[a-zA-Z0-9]+)$`)
	ksHttpUrlRegex        = regexp.MustCompile(`https?://[a-zA-Z0-9][-a-zA-Z0-9.]*(?:kwaicdn|yximgs|ks-cdn|oskwai|infinitedispatch|ksyungslb|bsgslb|ourdvs)[^\s"'\\]+`)
	ksApolloRegex         = regexp.MustCompile(`window\.__APOLLO_STATE__\s*=\s*(\{.+?\});`)
	ksNextDataRegex       = regexp.MustCompile(`<script\s+id="__NEXT_DATA__"[^>]*>(\{.+?\})</script>`)
	ksInitStateRegex      = regexp.MustCompile(`window\.INIT_STATE\s*=\s*(\{.+?\});`)
)

func (p *KuaishouPlugin) SetBridge(bridge *shared.Bridge) {
	p.bridge = bridge
}

func (p *KuaishouPlugin) Domains() []string {
	return []string{
		"kuaishou.com",
		"gifshow.com",
		"yximgs.com",
		"kwimgs.com",
		"kspkg.com",
		"ksapisrv.com",
		"kuaishouzt.com",
		"kwai.com",
		"kwaicdn.com",
		"kuaishoupay.com",
		"oskwai.com",
		"kwai.net",
		"kwaixiaodian.com",
		"ks-cdn.com",
		"chenzhongtech.com",
		"kwaizt.com",
		"aikan-tv.com",
		"kuaishou.cn",
		"kwai.cn",
		"infinitedispatch.com",
		"ksyungslb.com",
		"bsgslb.cn",
		"kpkcloud.com",
		"kwaimsg.com",
		"kwaishop.com",
		"kwai.pro",
		"bsclink.cn",
		"ourdvs.com",
		"wsdvs.com",
	}
}

func (p *KuaishouPlugin) pushRecentMeta(meta KsMeta) {
	if meta.Caption == "" && meta.PhotoId == "" {
		return
	}
	p.recentMu.Lock()
	defer p.recentMu.Unlock()
	if len(p.recentMetas) > 0 && p.recentMetas[len(p.recentMetas)-1].Meta.PhotoId == meta.PhotoId && meta.PhotoId != "" {
		p.recentMetas[len(p.recentMetas)-1] = KsRecentItem{Meta: meta, Timestamp: time.Now()}
		return
	}
	p.recentMetas = append(p.recentMetas, KsRecentItem{Meta: meta, Timestamp: time.Now()})
	if len(p.recentMetas) > 60 {
		p.recentMetas = p.recentMetas[len(p.recentMetas)-60:]
	}
}

func (p *KuaishouPlugin) getLatestRecentMeta(maxAge time.Duration) (KsMeta, bool) {
	p.recentMu.RLock()
	defer p.recentMu.RUnlock()
	now := time.Now()
	for i := len(p.recentMetas) - 1; i >= 0; i-- {
		item := p.recentMetas[i]
		if now.Sub(item.Timestamp) <= maxAge {
			return item.Meta, true
		}
	}
	return KsMeta{}, false
}

func (p *KuaishouPlugin) OnRequest(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
	lowerUrl := strings.ToLower(r.URL.String())
	host := strings.ToLower(r.Host)
	if strings.Contains(lowerUrl, "/graphql") ||
		strings.Contains(lowerUrl, "/rest/") ||
		strings.Contains(host, "kuaishou.com") ||
		strings.Contains(host, "gifshow.com") ||
		strings.Contains(host, "ksapisrv.com") {
		// 避免浏览器与快手服务端协商 Brotli (br) 或 zstd 压缩，优先使用 gzip 或 deflate，确保代理端能准确解包 JSON / HTML 提取作品真实标题标签
		r.Header.Set("Accept-Encoding", "gzip, deflate")
	}
	return r, nil
}

func (p *KuaishouPlugin) OnResponse(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
	if p.bridge.IsProxy != nil && !p.bridge.IsProxy() {
		return nil
	}

	if resp == nil || resp.Request == nil || (resp.StatusCode != 200 && resp.StatusCode != 206) {
		return nil
	}

	rawUrl := resp.Request.URL.String()
	lowerUrl := strings.ToLower(rawUrl)
	host := strings.ToLower(resp.Request.Host)
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))

	// 1. 拦截并解析快手 API、GraphQL 及 JSON 接口响应
	if strings.Contains(contentType, "json") ||
		strings.Contains(lowerUrl, "/graphql") ||
		strings.Contains(lowerUrl, "/rest/") ||
		strings.Contains(host, "ksapisrv.com") {

		body, err := io.ReadAll(resp.Body)
		if err == nil {
			resp.Body = io.NopCloser(bytes.NewBuffer(body))
			encoding := resp.Header.Get("Content-Encoding")
			go p.extractKuaishouJson(body, encoding)
		}
		return resp
	}

	// 2. 拦截并解析快手 SSR HTML 页面中的预注状态（__APOLLO_STATE__ / __NEXT_DATA__ / INIT_STATE）
	if strings.Contains(contentType, "html") && (strings.Contains(host, "kuaishou.com") || strings.Contains(host, "gifshow.com")) {
		body, err := io.ReadAll(resp.Body)
		if err == nil {
			resp.Body = io.NopCloser(bytes.NewBuffer(body))
			encoding := resp.Header.Get("Content-Encoding")
			go p.extractKuaishouHtml(body, encoding)
		}
		return resp
	}

	// 3. 拦截并捕获快手媒体 CDN（视频、图片、音频等）
	classify, suffix := p.bridge.TypeSuffix(contentType)
	if classify == "" || classify == "stream" {
		lowerPath := strings.ToLower(resp.Request.URL.Path)
		if strings.HasSuffix(lowerPath, ".mp4") || strings.Contains(lowerPath, ".mp4") || strings.Contains(lowerPath, "/upic/") || strings.Contains(lowerPath, "/bs2/gslb/") || strings.Contains(contentType, "video") {
			classify = "video"
			suffix = ".mp4"
		} else if strings.HasSuffix(lowerPath, ".jpg") || strings.HasSuffix(lowerPath, ".jpeg") {
			classify = "image"
			suffix = ".jpg"
		} else if strings.HasSuffix(lowerPath, ".png") {
			classify = "image"
			suffix = ".png"
		} else if strings.HasSuffix(lowerPath, ".webp") {
			classify = "image"
			suffix = ".webp"
		} else if strings.HasSuffix(lowerPath, ".m3u8") || strings.Contains(lowerPath, ".m3u8") {
			classify = "m3u8"
			suffix = ".m3u8"
		} else if strings.HasSuffix(lowerPath, ".flv") {
			classify = "live"
			suffix = ".flv"
		}
	}

	if classify == "" {
		return resp
	}

	isAll, _ := p.bridge.GetResType("all")
	isClassify, _ := p.bridge.GetResType(classify)
	if !isAll && !isClassify {
		return resp
	}

	p.processMediaStream(resp, rawUrl, classify, suffix)
	return resp
}

// decompressBody 自动识别并解压 gzip / brotli / zlib / deflate 压缩的响应内容
func decompressBody(body []byte, encoding string) []byte {
	if len(body) == 0 {
		return body
	}

	encoding = strings.ToLower(strings.TrimSpace(encoding))

	// 1. 检查 gzip 魔数 (0x1f, 0x8b) 或 Content-Encoding: gzip
	if (len(body) >= 2 && body[0] == 0x1f && body[1] == 0x8b) || strings.Contains(encoding, "gzip") {
		gr, err := gzip.NewReader(bytes.NewReader(body))
		if err == nil {
			defer gr.Close()
			if data, err := io.ReadAll(gr); err == nil {
				return data
			}
		}
	}

	// 2. 检查 brotli (br)
	if strings.Contains(encoding, "br") {
		br := brotli.NewReader(bytes.NewReader(body))
		if data, err := io.ReadAll(br); err == nil {
			return data
		}
	}

	// 3. 检查 zlib / deflate 魔数 (0x78) 或 Content-Encoding: deflate
	if (len(body) >= 2 && body[0] == 0x78) || strings.Contains(encoding, "deflate") {
		zr, err := zlib.NewReader(bytes.NewReader(body))
		if err == nil {
			defer zr.Close()
			if data, err := io.ReadAll(zr); err == nil {
				return data
			}
		}
		fr := flate.NewReader(bytes.NewReader(body))
		defer fr.Close()
		if data, err := io.ReadAll(fr); err == nil {
			return data
		}
	}

	// 兜底：若前置未命中且数据看起来是压缩格式，尝试使用 brotli 解包
	if len(body) > 4 && body[0] != '{' && body[0] != '<' && body[0] != '[' {
		br := brotli.NewReader(bytes.NewReader(body))
		if data, err := io.ReadAll(br); err == nil && len(data) > len(body) {
			return data
		}
	}

	return body
}

func (p *KuaishouPlugin) extractKuaishouJson(body []byte, encoding string) {
	decompressed := decompressBody(body, encoding)
	var root interface{}
	if err := json.Unmarshal(decompressed, &root); err != nil {
		return
	}
	p.findPhotosRecursively(root)
}

func (p *KuaishouPlugin) extractKuaishouHtml(body []byte, encoding string) {
	decompressed := decompressBody(body, encoding)
	// 尝试匹配 __APOLLO_STATE__
	if match := ksApolloRegex.FindSubmatch(decompressed); len(match) > 1 {
		var apollo map[string]interface{}
		if err := json.Unmarshal(match[1], &apollo); err == nil {
			p.findPhotosRecursively(apollo)
		}
	}
	// 尝试匹配 __NEXT_DATA__
	if match := ksNextDataRegex.FindSubmatch(decompressed); len(match) > 1 {
		var nextData map[string]interface{}
		if err := json.Unmarshal(match[1], &nextData); err == nil {
			p.findPhotosRecursively(nextData)
		}
	}
	// 尝试匹配 INIT_STATE
	if match := ksInitStateRegex.FindSubmatch(decompressed); len(match) > 1 {
		var initState map[string]interface{}
		if err := json.Unmarshal(match[1], &initState); err == nil {
			p.findPhotosRecursively(initState)
		}
	}
}

// 递归遍历 JSON 结构，提取所有快手视频作品对象并建立元数据缓存
func (p *KuaishouPlugin) findPhotosRecursively(node interface{}) {
	switch val := node.(type) {
	case map[string]interface{}:
		if photo, ok := val["photo"].(map[string]interface{}); ok {
			// 将外层 author 信息同步注入 photo 对象中，防止作者名丢失
			if _, hasAuth := photo["author"]; !hasAuth {
				if auth, hasValAuth := val["author"]; hasValAuth {
					photo["author"] = auth
				}
			}
			if _, hasUser := photo["userName"]; !hasUser {
				if auth, hasValAuth := val["author"].(map[string]interface{}); hasValAuth {
					if name, ok := auth["name"].(string); ok {
						photo["userName"] = name
					}
				}
			}
			p.processPhotoItem(photo)
		} else if isPhotoMap(val) {
			p.processPhotoItem(val)
		}

		for _, v := range val {
			p.findPhotosRecursively(v)
		}
	case []interface{}:
		for _, item := range val {
			p.findPhotosRecursively(item)
		}
	}
}

func isPhotoMap(m map[string]interface{}) bool {
	caption := getCaptionFromMap(m)
	_, hasId := m["id"]
	if !hasId {
		_, hasId = m["photoId"]
	}
	if !hasId {
		_, hasId = m["photo_id"]
	}

	_, hasPhotoUrl := m["photoUrl"]
	_, hasPhotoUrls := m["photoUrls"]
	_, hasMainMvUrls := m["mainMvUrls"]
	_, hasCoverUrl := m["coverUrl"]
	_, hasCoverUrls := m["coverUrls"]
	_, hasManifest := m["manifest"]
	_, hasManifestStr := m["manifestStr"]
	_, hasVideoRes := m["videoResource"]

	hasMedia := hasPhotoUrl || hasPhotoUrls || hasMainMvUrls || hasCoverUrl || hasCoverUrls || hasManifest || hasManifestStr || hasVideoRes

	return (caption != "" || hasId) && hasMedia
}

func getCaptionFromMap(m map[string]interface{}) string {
	for _, k := range []string{"caption", "title", "desc", "name", "text"} {
		if v, ok := m[k].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func (p *KuaishouPlugin) processPhotoItem(photo map[string]interface{}) {
	caption := getCaptionFromMap(photo)

	// 提取并补齐可能独立存在的标签 (tags / tagList / topics)
	for _, tagKey := range []string{"tags", "tagList", "tag_list", "topics"} {
		if tags, ok := photo[tagKey].([]interface{}); ok {
			for _, t := range tags {
				tagName := ""
				if tMap, ok := t.(map[string]interface{}); ok {
					if n, ok := tMap["name"].(string); ok && n != "" {
						tagName = n
					} else if n, ok := tMap["tagName"].(string); ok && n != "" {
						tagName = n
					}
				} else if tStr, ok := t.(string); ok && tStr != "" {
					tagName = tStr
				}
				tagName = strings.TrimSpace(tagName)
				if tagName != "" && !strings.Contains(caption, "#"+tagName) {
					if caption == "" {
						caption = "#" + tagName
					} else {
						caption += " #" + tagName
					}
				}
			}
		}
	}

	userName, _ := photo["userName"].(string)
	if userName == "" {
		if author, ok := photo["author"].(map[string]interface{}); ok {
			userName, _ = author["name"].(string)
		}
	}
	if userName == "" {
		if user, ok := photo["user"].(map[string]interface{}); ok {
			userName, _ = user["name"].(string)
		}
	}

	var photoId string
	for _, idKey := range []string{"id", "photo_id", "photoId", "fid"} {
		if idVal, ok := photo[idKey]; ok && idVal != nil {
			strVal := fmt.Sprintf("%v", idVal)
			if strVal != "" && strVal != "0" {
				photoId = strVal
				break
			}
		}
	}

	// 提取封面地址
	var coverUrl string
	var allCoverUrls []string
	if c, ok := photo["coverUrl"].(string); ok && c != "" {
		coverUrl = c
		allCoverUrls = append(allCoverUrls, c)
	}
	for _, cKey := range []string{"coverUrls", "animatedCoverUrls"} {
		if urls, ok := photo[cKey].([]interface{}); ok {
			for _, uItem := range urls {
				if uMap, ok := uItem.(map[string]interface{}); ok {
					if u, ok := uMap["url"].(string); ok && u != "" {
						if coverUrl == "" {
							coverUrl = u
						}
						allCoverUrls = append(allCoverUrls, u)
					}
				} else if uStr, ok := uItem.(string); ok && uStr != "" {
					if coverUrl == "" {
						coverUrl = uStr
					}
					allCoverUrls = append(allCoverUrls, uStr)
				}
			}
		}
	}

	// 收集所有候选播放地址（深入解析 photoUrl, mainMvUrls, manifest, videoResource 等全部 CDN 线路）
	var allPlayUrls []string
	addPlayUrl := func(u string) {
		u = strings.TrimSpace(u)
		if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
			allPlayUrls = append(allPlayUrls, u)
		}
	}

	// 1. 常规直接字段
	for _, uKey := range []string{"photoUrl", "h265PhotoUrl", "url", "playUrl"} {
		if u, ok := photo[uKey].(string); ok {
			addPlayUrl(u)
		}
	}

	// 2. 常规列表字段
	for _, listKey := range []string{"photoUrls", "mainMvUrls", "h265PhotoUrls", "playUrls"} {
		if urls, ok := photo[listKey].([]interface{}); ok {
			for _, uItem := range urls {
				if uMap, ok := uItem.(map[string]interface{}); ok {
					if u, ok := uMap["url"].(string); ok {
						addPlayUrl(u)
					}
				} else if uStr, ok := uItem.(string); ok {
					addPlayUrl(uStr)
				}
			}
		}
	}

	// 3. 递归解析 manifest / manifestStr / videoResource 中深嵌的视频流 URL（现代快手 Web 端标准结构）
	var extractUrlsFromNode func(node interface{})
	extractUrlsFromNode = func(node interface{}) {
		switch v := node.(type) {
		case string:
			if strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://") {
				addPlayUrl(v)
			} else if strings.Contains(v, "http") && (strings.Contains(v, "kwaicdn") || strings.Contains(v, "yximgs") || strings.Contains(v, ".mp4") || strings.Contains(v, ".m3u8")) {
				urls := ksHttpUrlRegex.FindAllString(v, -1)
				for _, u := range urls {
					addPlayUrl(u)
				}
			}
		case map[string]interface{}:
			for _, item := range v {
				extractUrlsFromNode(item)
			}
		case []interface{}:
			for _, item := range v {
				extractUrlsFromNode(item)
			}
		}
	}

	for _, mKey := range []string{"manifest", "manifestStr", "videoResource"} {
		if mVal, ok := photo[mKey]; ok && mVal != nil {
			if mStr, ok := mVal.(string); ok && strings.TrimSpace(mStr) != "" {
				var parsed interface{}
				if err := json.Unmarshal([]byte(mStr), &parsed); err == nil {
					extractUrlsFromNode(parsed)
				} else {
					extractUrlsFromNode(mStr)
				}
			} else {
				extractUrlsFromNode(mVal)
			}
		}
	}

	var playUrl string
	if len(allPlayUrls) > 0 {
		playUrl = allPlayUrls[0]
	}

	// 格式化描述（带有完整标签，若无则采用作者名）
	displayDesc := caption
	if displayDesc == "" && userName != "" {
		displayDesc = "@" + userName + " 的快手作品"
	}
	if displayDesc == "" && photoId != "" {
		displayDesc = "快手视频_" + photoId
	}

	meta := KsMeta{
		PhotoId:  photoId,
		Caption:  displayDesc,
		CoverUrl: coverUrl,
		UserName: userName,
		PlayUrl:  playUrl,
	}

	// 记录到最近作品缓存队列（供后续流拦截精准回退）
	p.pushRecentMeta(meta)

	// 将元数据存入多级索引缓存池供流拦截时精准匹配
	if photoId != "" {
		p.metaCache.Store("id:"+photoId, meta)
	}
	for _, u := range allPlayUrls {
		p.cacheMetaByUrl(u, meta)
	}
	for _, c := range allCoverUrls {
		p.cacheMetaByUrl(c, meta)
	}
}

func (p *KuaishouPlugin) cacheMetaByUrl(rawUrl string, meta KsMeta) {
	urlSign := shared.Md5(rawUrl)
	p.metaCache.Store("sign:"+urlSign, meta)

	cleanUrl := strings.Split(rawUrl, "?")[0]
	p.metaCache.Store("clean:"+cleanUrl, meta)

	filename := path.Base(cleanUrl)
	if filename != "" && filename != "." && filename != "/" {
		p.metaCache.Store("file:"+filename, meta)
		// 剥离清晰度或编码后缀，例如 3xabc_b.mp4 -> 3xabc.mp4
		cleanFilename := ksFilenameSuffixRegex.ReplaceAllString(filename, "$1")
		if cleanFilename != filename {
			p.metaCache.Store("file:"+cleanFilename, meta)
		}
	}

	if u, err := url.Parse(rawUrl); err == nil {
		if u.Path != "" && u.Path != "/" {
			p.metaCache.Store("path:"+u.Path, meta)
		}
		// 缓存 client_cache_key 及其剥离后缀形式
		if cck := u.Query().Get("client_cache_key"); cck != "" {
			p.metaCache.Store("cck:"+cck, meta)
			cleanKey := strings.TrimSuffix(cck, path.Ext(cck))
			if idx := strings.Index(cleanKey, "_"); idx > 0 {
				cleanKey = cleanKey[:idx]
			}
			p.metaCache.Store("id:"+cleanKey, meta)
			if match := ks3xIdRegex.FindStringSubmatch(cck); len(match) > 1 {
				p.metaCache.Store("id:"+match[1], meta)
			}
		}
	}

	// 匹配 URL 中含有的 3x 格式 photoId
	if match := ks3xIdRegex.FindStringSubmatch(rawUrl); len(match) > 1 {
		p.metaCache.Store("id:"+match[1], meta)
	}
}

func (p *KuaishouPlugin) lookupMeta(rawUrl string, referer string) (KsMeta, bool) {
	// 1. 通过完整 URL sign 匹配
	urlSign := shared.Md5(rawUrl)
	if v, ok := p.metaCache.Load("sign:" + urlSign); ok {
		return v.(KsMeta), true
	}

	// 2. 通过干净 URL 路径匹配
	cleanUrl := strings.Split(rawUrl, "?")[0]
	if v, ok := p.metaCache.Load("clean:" + cleanUrl); ok {
		return v.(KsMeta), true
	}

	// 3. 通过 URL Path 匹配（跨 CDN 域名统一路径匹配）
	if u, err := url.Parse(rawUrl); err == nil && u.Path != "" && u.Path != "/" {
		if v, ok := p.metaCache.Load("path:" + u.Path); ok {
			return v.(KsMeta), true
		}
	}

	// 4. 通过文件名匹配
	filename := path.Base(cleanUrl)
	if filename != "" && filename != "." && filename != "/" {
		if v, ok := p.metaCache.Load("file:" + filename); ok {
			return v.(KsMeta), true
		}
		cleanFilename := ksFilenameSuffixRegex.ReplaceAllString(filename, "$1")
		if cleanFilename != filename {
			if v, ok := p.metaCache.Load("file:" + cleanFilename); ok {
				return v.(KsMeta), true
			}
		}
	}

	// 5. 通过 URL 中的 client_cache_key 参数匹配 (快手核心缓存标识)
	if u, err := url.Parse(rawUrl); err == nil {
		if cck := u.Query().Get("client_cache_key"); cck != "" {
			if v, ok := p.metaCache.Load("cck:" + cck); ok {
				return v.(KsMeta), true
			}
			cleanKey := strings.TrimSuffix(cck, path.Ext(cck))
			if idx := strings.Index(cleanKey, "_"); idx > 0 {
				cleanKey = cleanKey[:idx]
			}
			if v, ok := p.metaCache.Load("id:" + cleanKey); ok {
				return v.(KsMeta), true
			}
			if match := ks3xIdRegex.FindStringSubmatch(cck); len(match) > 1 {
				if v, ok := p.metaCache.Load("id:" + match[1]); ok {
					return v.(KsMeta), true
				}
			}
		}

		// 6. 通过 URL 中的 photoId 等参数匹配
		for _, qKey := range []string{"photoId", "photo_id", "id", "fid", "shareObjectId"} {
			if photoId := u.Query().Get(qKey); photoId != "" {
				if v, ok := p.metaCache.Load("id:" + photoId); ok {
					return v.(KsMeta), true
				}
			}
		}
	}

	// 7. 通过 URL 路径中携带的 3x 开头 photoId 匹配
	if match := ks3xIdRegex.FindStringSubmatch(rawUrl); len(match) > 1 {
		if v, ok := p.metaCache.Load("id:" + match[1]); ok {
			return v.(KsMeta), true
		}
	}

	// 8. 通过 Referer 中携带的 photoId 匹配
	if referer != "" {
		if match := ksPhotoIdRegex.FindStringSubmatch(referer); len(match) > 1 {
			if v, ok := p.metaCache.Load("id:" + match[1]); ok {
				return v.(KsMeta), true
			}
		}
		if match := ks3xIdRegex.FindStringSubmatch(referer); len(match) > 1 {
			if v, ok := p.metaCache.Load("id:" + match[1]); ok {
				return v.(KsMeta), true
			}
		}
	}

	// 9. 时序就近兜底匹配：若是快手媒体流，直接关联最近 120 秒内流经的快手视频作品信息
	if latest, ok := p.getLatestRecentMeta(120 * time.Second); ok && latest.Caption != "" {
		return latest, true
	}

	return KsMeta{}, false
}

func (p *KuaishouPlugin) processMediaStream(resp *http.Response, rawUrl string, classify string, suffix string) {
	urlSign := shared.Md5(rawUrl)
	if p.bridge.MediaIsMarked(urlSign) {
		return
	}

	var size float64
	if contentRange := resp.Header.Get("Content-Range"); contentRange != "" {
		parts := strings.Split(contentRange, "/")
		if len(parts) == 2 && parts[1] != "*" {
			if totalSize, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64); err == nil && totalSize > 0 {
				size = totalSize
			}
		}
	}
	if size == 0 {
		if resp.ContentLength > 0 {
			size = float64(resp.ContentLength)
		} else {
			size, _ = strconv.ParseFloat(resp.Header.Get("Content-Length"), 64)
		}
	}

	// 严格执行图片和视频大小过滤规则
	if classify == "image" {
		if minSize, ok := p.bridge.GetConfig("MinImageSize").(int); ok && minSize > 0 {
			if size < float64(minSize*1024) {
				return
			}
		}
	} else if classify == "video" {
		if minSize, ok := p.bridge.GetConfig("MinVideoSize").(int); ok && minSize > 0 {
			if size < float64(minSize*1024) {
				return
			}
		}
	}

	referer := resp.Request.Header.Get("Referer")
	meta, found := p.lookupMeta(rawUrl, referer)

	description := ""
	coverUrl := ""
	if found {
		description = meta.Caption
		coverUrl = meta.CoverUrl
	} else if referer != "" {
		if match := ksPhotoIdRegex.FindStringSubmatch(referer); len(match) > 1 {
			description = "快手视频_" + match[1]
		}
	}

	if description == "" && p.bridge.GetTitle != nil {
		description = p.bridge.GetTitle(resp.Request)
	}

	if description == "" {
		description = "快手视频"
	}

	id, err := gonanoid.New()
	if err != nil {
		id = urlSign
	}

	reqHeaders := make(http.Header)
	reqHeaders.Set("Referer", "https://www.kuaishou.com/")
	reqHeaders.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36")

	otherData := map[string]string{}
	if hJson, err := json.Marshal(reqHeaders); err == nil {
		otherData["headers"] = string(hJson)
	}

	res := shared.MediaInfo{
		Id:          id,
		Url:         rawUrl,
		UrlSign:     urlSign,
		CoverUrl:    coverUrl,
		Size:        size,
		Domain:      "kuaishou.com",
		Classify:    classify,
		Suffix:      suffix,
		Status:      shared.DownloadStatusReady,
		SavePath:    "",
		DecodeKey:   "",
		OtherData:   otherData,
		Description: description,
		ContentType: resp.Header.Get("Content-Type"),
	}

	p.bridge.MarkMedia(urlSign)
	p.bridge.Send("newResources", res)
}
