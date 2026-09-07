package plugins

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"res-downloader/core/shared"
	"strconv"
	"strings"
	"sync"

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

type KuaishouPlugin struct {
	bridge    *shared.Bridge
	metaCache sync.Map // key (url / cleanUrl / photoId / filename) -> KsMeta
}

var (
	ksPhotoIdRegex  = regexp.MustCompile(`(?:short-video|photo|fw/photo)/([a-zA-Z0-9_-]+)`)
	ksApolloRegex   = regexp.MustCompile(`window\.__APOLLO_STATE__\s*=\s*(\{.+?\});`)
	ksNextDataRegex = regexp.MustCompile(`<script\s+id="__NEXT_DATA__"[^>]*>(\{.+?\})</script>`)
	ksInitStateRegex = regexp.MustCompile(`window\.INIT_STATE\s*=\s*(\{.+?\});`)
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
	}
}

func (p *KuaishouPlugin) OnRequest(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
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
			go p.extractKuaishouJson(body)
		}
		return resp
	}

	// 2. 拦截并解析快手 SSR HTML 页面中的预注状态（__APOLLO_STATE__ / __NEXT_DATA__）
	if strings.Contains(contentType, "html") && (strings.Contains(host, "kuaishou.com") || strings.Contains(host, "gifshow.com")) {
		body, err := io.ReadAll(resp.Body)
		if err == nil {
			resp.Body = io.NopCloser(bytes.NewBuffer(body))
			go p.extractKuaishouHtml(body)
		}
		return resp
	}

	// 3. 拦截并捕获快手媒体 CDN（视频、图片、音频等）
	classify, suffix := p.bridge.TypeSuffix(contentType)
	if classify == "" || classify == "stream" {
		lowerPath := strings.ToLower(resp.Request.URL.Path)
		if strings.HasSuffix(lowerPath, ".mp4") || strings.Contains(lowerPath, "/upic/") || strings.Contains(lowerPath, "/bs2/gslb/") {
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
		} else if strings.HasSuffix(lowerPath, ".m3u8") {
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

func (p *KuaishouPlugin) extractKuaishouJson(body []byte) {
	var root interface{}
	if err := json.Unmarshal(body, &root); err != nil {
		return
	}
	p.findPhotosRecursively(root)
}

func (p *KuaishouPlugin) extractKuaishouHtml(body []byte) {
	// 尝试匹配 __APOLLO_STATE__
	if match := ksApolloRegex.FindSubmatch(body); len(match) > 1 {
		var apollo map[string]interface{}
		if err := json.Unmarshal(match[1], &apollo); err == nil {
			p.findPhotosRecursively(apollo)
		}
	}
	// 尝试匹配 __NEXT_DATA__
	if match := ksNextDataRegex.FindSubmatch(body); len(match) > 1 {
		var nextData map[string]interface{}
		if err := json.Unmarshal(match[1], &nextData); err == nil {
			p.findPhotosRecursively(nextData)
		}
	}
	// 尝试匹配 INIT_STATE
	if match := ksInitStateRegex.FindSubmatch(body); len(match) > 1 {
		var initState map[string]interface{}
		if err := json.Unmarshal(match[1], &initState); err == nil {
			p.findPhotosRecursively(initState)
		}
	}
}

// 递归遍历 JSON 结构，提取所有快手视频对象并建立元数据缓存
func (p *KuaishouPlugin) findPhotosRecursively(node interface{}) {
	switch val := node.(type) {
	case map[string]interface{}:
		if isPhotoMap(val) {
			p.processPhotoItem(val)
		} else if photo, ok := val["photo"].(map[string]interface{}); ok {
			p.processPhotoItem(photo)
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
	_, hasCaption := m["caption"]
	_, hasPhotoUrl := m["photoUrl"]
	_, hasPhotoUrls := m["photoUrls"]
	_, hasMainMvUrls := m["mainMvUrls"]
	_, hasCoverUrl := m["coverUrl"]
	_, hasCoverUrls := m["coverUrls"]

	return (hasCaption || hasPhotoUrl || hasPhotoUrls || hasMainMvUrls) &&
		(hasPhotoUrl || hasPhotoUrls || hasMainMvUrls || hasCoverUrl || hasCoverUrls)
}

func (p *KuaishouPlugin) processPhotoItem(photo map[string]interface{}) {
	caption, _ := photo["caption"].(string)
	if caption == "" {
		caption, _ = photo["title"].(string)
	}
	if caption == "" {
		caption, _ = photo["desc"].(string)
	}
	caption = strings.TrimSpace(caption)

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
	if idVal, ok := photo["id"]; ok {
		photoId = fmt.Sprintf("%v", idVal)
	} else if idVal, ok := photo["photo_id"]; ok {
		photoId = fmt.Sprintf("%v", idVal)
	} else if idVal, ok := photo["photoId"]; ok {
		photoId = fmt.Sprintf("%v", idVal)
	}

	// 提取封面地址
	var coverUrl string
	if c, ok := photo["coverUrl"].(string); ok && c != "" {
		coverUrl = c
	} else if urls, ok := photo["coverUrls"].([]interface{}); ok && len(urls) > 0 {
		if uMap, ok := urls[0].(map[string]interface{}); ok {
			coverUrl, _ = uMap["url"].(string)
		} else if uStr, ok := urls[0].(string); ok {
			coverUrl = uStr
		}
	}

	// 提取播放地址
	var playUrl string

	// 1. 直接取 photoUrl
	if pUrl, ok := photo["photoUrl"].(string); ok && pUrl != "" && strings.HasPrefix(pUrl, "http") {
		playUrl = pUrl
	}

	// 2. 取 photoUrls 列表
	if playUrl == "" {
		if urls, ok := photo["photoUrls"].([]interface{}); ok && len(urls) > 0 {
			for _, uItem := range urls {
				if uMap, ok := uItem.(map[string]interface{}); ok {
					if u, ok := uMap["url"].(string); ok && strings.HasPrefix(u, "http") {
						playUrl = u
						break
					}
				} else if uStr, ok := uItem.(string); ok && strings.HasPrefix(uStr, "http") {
					playUrl = uStr
					break
				}
			}
		}
	}

	// 3. 取 mainMvUrls 列表
	if playUrl == "" {
		if urls, ok := photo["mainMvUrls"].([]interface{}); ok && len(urls) > 0 {
			for _, uItem := range urls {
				if uMap, ok := uItem.(map[string]interface{}); ok {
					if u, ok := uMap["url"].(string); ok && strings.HasPrefix(u, "http") {
						playUrl = u
						break
					}
				} else if uStr, ok := uItem.(string); ok && strings.HasPrefix(uStr, "http") {
					playUrl = uStr
					break
				}
			}
		}
	}

	// 4. 取 h265PhotoUrl
	if playUrl == "" {
		if h265, ok := photo["h265PhotoUrl"].(string); ok && strings.HasPrefix(h265, "http") {
			playUrl = h265
		}
	}

	// 格式化描述（若标题为空则采用作者名）
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

	// 将元数据存入多级索引缓存池供流拦截时精准匹配
	if photoId != "" {
		p.metaCache.Store("id:"+photoId, meta)
	}
	if playUrl != "" {
		p.cacheMetaByUrl(playUrl, meta)
	}
	if coverUrl != "" {
		p.cacheMetaByUrl(coverUrl, meta)
	}
}

func (p *KuaishouPlugin) cacheMetaByUrl(rawUrl string, meta KsMeta) {
	urlSign := shared.Md5(rawUrl)
	p.metaCache.Store("sign:"+urlSign, meta)

	cleanUrl := strings.Split(rawUrl, "?")[0]
	p.metaCache.Store("clean:"+cleanUrl, meta)

	filename := filepath.Base(cleanUrl)
	if filename != "" && filename != "." && filename != "/" {
		p.metaCache.Store("file:"+filename, meta)
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

	// 3. 通过文件名匹配
	filename := filepath.Base(cleanUrl)
	if filename != "" && filename != "." && filename != "/" {
		if v, ok := p.metaCache.Load("file:" + filename); ok {
			return v.(KsMeta), true
		}
	}

	// 4. 通过 Referer 中携带的 photoId 匹配
	if referer != "" {
		if match := ksPhotoIdRegex.FindStringSubmatch(referer); len(match) > 1 {
			if v, ok := p.metaCache.Load("id:" + match[1]); ok {
				return v.(KsMeta), true
			}
		}
	}

	// 5. 通过 URL 中的 photoId 参数匹配
	if u, err := url.Parse(rawUrl); err == nil {
		if photoId := u.Query().Get("photoId"); photoId != "" {
			if v, ok := p.metaCache.Load("id:" + photoId); ok {
				return v.(KsMeta), true
			}
		}
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

	if description == "" {
		description = "快手资源"
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
