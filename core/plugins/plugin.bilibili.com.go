package plugins

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/elazarl/goproxy"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"res-downloader/core/shared"
)

type BiliMeta struct {
	Title string
	Desc  string
	Pic   string
	Up    string
}

type BilibiliPlugin struct {
	bridge    *shared.Bridge
	metaCache sync.Map // key: bvid / cid / avid (string) -> BiliMeta
}

func (p *BilibiliPlugin) SetBridge(bridge *shared.Bridge) {
	p.bridge = bridge
}

func (p *BilibiliPlugin) Domains() []string {
	return []string{
		"bilibili.com",
		"bilivideo.com",
		"bilivideo.cn",
		"hdslb.com",
		"biliapi.net",
		"biliapi.com",
	}
}

func (p *BilibiliPlugin) OnRequest(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
	return r, nil
}

func (p *BilibiliPlugin) OnResponse(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
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

	// 1. 过滤掉无用的数据埋点、心跳统计、弹幕流等垃圾请求
	if strings.Contains(host, "data.bilibili.com") ||
		strings.Contains(lowerUrl, "/log/web") ||
		strings.Contains(lowerUrl, "seg.so") ||
		strings.Contains(lowerUrl, "/x/v2/dm/") ||
		strings.Contains(lowerUrl, "heartbeat") ||
		strings.Contains(lowerUrl, "buvid") ||
		strings.Contains(lowerUrl, "fingerprint") ||
		strings.Contains(host, "cm.bilibili.com") {
		return resp
	}

	// 2. 拦截并解析 B站 视频信息与详情 API（提取真实标题、UP主、封面并缓存）
	if strings.Contains(lowerUrl, "/x/web-interface/view") ||
		strings.Contains(lowerUrl, "/x/web-interface/wbi/view") ||
		strings.Contains(lowerUrl, "/x/player/v2") {
		body, err := io.ReadAll(resp.Body)
		if err == nil {
			resp.Body = io.NopCloser(bytes.NewBuffer(body))
			go p.extractVideoDetail(body)
		}
		return resp
	}

	// 3. 拦截并解析 B站 播放地址 API（playurl）返回的高清 DASH / DURL 视频流与音频流
	if strings.Contains(lowerUrl, "/x/player/wbi/playurl") ||
		strings.Contains(lowerUrl, "/x/player/playurl") ||
		strings.Contains(lowerUrl, "/pgc/player/web/v2/playurl") ||
		strings.Contains(lowerUrl, "/pgc/player/web/playurl") {
		body, err := io.ReadAll(resp.Body)
		if err == nil {
			resp.Body = io.NopCloser(bytes.NewBuffer(body))
			go p.extractPlayUrl(resp.Request.URL, body)
		}
		return resp
	}

	// 4. 拦截网页 HTML（提取 SSR 内嵌的 window.__playinfo__ 与 window.__INITIAL_STATE__）
	if strings.Contains(contentType, "text/html") &&
		(strings.Contains(lowerUrl, "bilibili.com/video/") || strings.Contains(lowerUrl, "bilibili.com/bangumi/play/")) {
		body, err := io.ReadAll(resp.Body)
		if err == nil {
			resp.Body = io.NopCloser(bytes.NewBuffer(body))
			go p.extractFromHTML(resp.Request.URL, body)
		}
		return resp
	}

	// 5. 拦截 bilivideo.com / bilivideo.cn 直接媒体流（m4s / mp4 / upgcxcode）
	if strings.Contains(host, "bilivideo.com") || strings.Contains(host, "bilivideo.cn") {
		p.handleDirectMedia(resp)
		return resp
	}

	return nil
}

// 提取视频详情
func (p *BilibiliPlugin) extractVideoDetail(body []byte) {
	var res map[string]interface{}
	if err := json.Unmarshal(body, &res); err != nil {
		return
	}

	data, ok := res["data"].(map[string]interface{})
	if !ok {
		return
	}

	title, _ := data["title"].(string)
	desc, _ := data["desc"].(string)
	pic, _ := data["pic"].(string)
	bvid, _ := data["bvid"].(string)

	up := ""
	if owner, ok := data["owner"].(map[string]interface{}); ok {
		up, _ = owner["name"].(string)
	}

	meta := BiliMeta{
		Title: strings.TrimSpace(title),
		Desc:  strings.TrimSpace(desc),
		Pic:   pic,
		Up:    strings.TrimSpace(up),
	}

	if bvid != "" {
		p.metaCache.Store(bvid, meta)
	}

	if cid, ok := data["cid"].(float64); ok && cid > 0 {
		p.metaCache.Store(fmt.Sprintf("%.0f", cid), meta)
	}

	if aid, ok := data["aid"].(float64); ok && aid > 0 {
		p.metaCache.Store(fmt.Sprintf("%.0f", aid), meta)
	}
}

// 从 HTML 中提取内嵌的 playinfo 和 INITIAL_STATE
func (p *BilibiliPlugin) extractFromHTML(reqUrl *url.URL, body []byte) {
	htmlStr := string(body)

	// 提取 INITIAL_STATE (获取标题/封面/UP主)
	meta := BiliMeta{}
	stateRe := regexp.MustCompile(`window\.__INITIAL_STATE__\s*=\s*(\{.*?\});`)
	if match := stateRe.FindStringSubmatch(htmlStr); len(match) > 1 {
		var state map[string]interface{}
		if err := json.Unmarshal([]byte(match[1]), &state); err == nil {
			if vData, ok := state["videoData"].(map[string]interface{}); ok {
				meta.Title, _ = vData["title"].(string)
				meta.Desc, _ = vData["desc"].(string)
				meta.Pic, _ = vData["pic"].(string)
				if owner, ok := vData["owner"].(map[string]interface{}); ok {
					meta.Up, _ = owner["name"].(string)
				}
				if bvid, ok := vData["bvid"].(string); ok && bvid != "" {
					p.metaCache.Store(bvid, meta)
				}
			}
		}
	}

	// 提取 playinfo (获取 DASH 播放地址)
	playRe := regexp.MustCompile(`window\.__playinfo__\s*=\s*(\{.*?\})</script>`)
	if match := playRe.FindStringSubmatch(htmlStr); len(match) > 1 {
		p.extractPlayUrl(reqUrl, []byte(match[1]))
	}
}

// 提取播放地址并推送
func (p *BilibiliPlugin) extractPlayUrl(reqUrl *url.URL, body []byte) {
	var res map[string]interface{}
	if err := json.Unmarshal(body, &res); err != nil {
		return
	}

	data, ok := res["data"].(map[string]interface{})
	if !ok {
		// PGC / 动漫返回格式在 result 字段中
		data, ok = res["result"].(map[string]interface{})
		if !ok {
			return
		}
	}

	// 匹配关联的视频元信息
	meta := BiliMeta{}
	if reqUrl != nil {
		q := reqUrl.Query()
		if bvid := q.Get("bvid"); bvid != "" {
			if val, ok := p.metaCache.Load(bvid); ok {
				meta = val.(BiliMeta)
			}
		}
		if meta.Title == "" {
			if cid := q.Get("cid"); cid != "" {
				if val, ok := p.metaCache.Load(cid); ok {
					meta = val.(BiliMeta)
				}
			}
		}
		if meta.Title == "" {
			if avid := q.Get("avid"); avid != "" {
				if val, ok := p.metaCache.Load(avid); ok {
					meta = val.(BiliMeta)
				}
			}
		}
	}

	if meta.Title == "" {
		meta.Title = "B站视频"
	}

	isAll, _ := p.bridge.GetResType("all")
	isVideo, _ := p.bridge.GetResType("video")
	isAudio, _ := p.bridge.GetResType("audio")

	// 1. 解析 DASH 格式（包含独立的最高画质视频流与音频流）
	if dash, ok := data["dash"].(map[string]interface{}); ok {
		duration, _ := dash["duration"].(float64)

		// 视频轨
		if isAll || isVideo {
			if videoList, ok := dash["video"].([]interface{}); ok && len(videoList) > 0 {
				// 寻找最高画质的视频轨
				var bestVideo map[string]interface{}
				var bestQuality float64 = -1

				for _, item := range videoList {
					if vMap, ok := item.(map[string]interface{}); ok {
						q, _ := vMap["id"].(float64)
						if q > bestQuality {
							bestQuality = q
							bestVideo = vMap
						}
					}
				}

				if bestVideo != nil {
					playUrl := p.extractUrlFromMediaMap(bestVideo)
					if playUrl != "" {
						qualityName := p.getQualityLabel(bestQuality)
						bandwidth, _ := bestVideo["bandwidth"].(float64)
						estimatedSize := float64(0)
						if bandwidth > 0 && duration > 0 {
							estimatedSize = (bandwidth * duration) / 8
						}

						videoDesc := fmt.Sprintf("[%s] %s", qualityName, meta.Title)
						if meta.Up != "" {
							videoDesc = fmt.Sprintf("[%s] %s - %s", qualityName, meta.Title, meta.Up)
						}

						p.sendMedia(playUrl, "video", ".mp4", "video/mp4", videoDesc, meta.Pic, estimatedSize)
					}
				}
			}
		}

		// 音频轨
		if isAll || isAudio {
			if audioList, ok := dash["audio"].([]interface{}); ok && len(audioList) > 0 {
				var bestAudio map[string]interface{}
				var maxBandwidth float64 = -1

				for _, item := range audioList {
					if aMap, ok := item.(map[string]interface{}); ok {
						bw, _ := aMap["bandwidth"].(float64)
						if bw > maxBandwidth {
							maxBandwidth = bw
							bestAudio = aMap
						}
					}
				}

				if bestAudio != nil {
					playUrl := p.extractUrlFromMediaMap(bestAudio)
					if playUrl != "" {
						audioDesc := fmt.Sprintf("[音频轨] %s", meta.Title)
						if meta.Up != "" {
							audioDesc = fmt.Sprintf("[音频轨] %s - %s", meta.Title, meta.Up)
						}

						p.sendMedia(playUrl, "audio", ".mp3", "audio/mp4", audioDesc, meta.Pic, 0)
					}
				}
			}
		}
	}

	// 2. 解析传统 DURL 格式（单段/多段完整 MP4/FLV）
	if isAll || isVideo {
		if durlList, ok := data["durl"].([]interface{}); ok && len(durlList) > 0 {
			for idx, item := range durlList {
				if dMap, ok := item.(map[string]interface{}); ok {
					playUrl, _ := dMap["url"].(string)
					size, _ := dMap["size"].(float64)
					if playUrl != "" {
						desc := meta.Title
						if len(durlList) > 1 {
							desc = fmt.Sprintf("%s (分段%d)", meta.Title, idx+1)
						}
						suffix := ".mp4"
						if strings.Contains(playUrl, ".flv") {
							suffix = ".flv"
						}
						p.sendMedia(playUrl, "video", suffix, "video/mp4", desc, meta.Pic, size)
					}
				}
			}
		}
	}
}

// 处理直接通过 CDN 请求的媒体流
func (p *BilibiliPlugin) handleDirectMedia(resp *http.Response) {
	rawUrl := resp.Request.URL.String()
	lowerUrl := strings.ToLower(rawUrl)

	// 只处理含有媒体特征的链接
	if !strings.Contains(lowerUrl, ".m4s") &&
		!strings.Contains(lowerUrl, ".mp4") &&
		!strings.Contains(lowerUrl, "/upgcxcode/") {
		return
	}

	var totalSize float64
	// 优先从 Content-Range 中获取完整大小
	if contentRange := resp.Header.Get("Content-Range"); contentRange != "" {
		parts := strings.Split(contentRange, "/")
		if len(parts) == 2 && parts[1] != "*" {
			if val, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64); err == nil && val > 0 {
				totalSize = val
			}
		}
	}
	if totalSize == 0 {
		totalSize, _ = strconv.ParseFloat(resp.Header.Get("Content-Length"), 64)
	}

	// 过滤掉小于 100KB 的初始化头部探测微碎片（防止出现 1.23KB 等碎流）
	if totalSize > 0 && totalSize < 102400 {
		return
	}

	// 判断是视频还是音频
	classify := "video"
	suffix := ".mp4"
	contentType := "video/mp4"

	if strings.Contains(lowerUrl, "30280") ||
		strings.Contains(lowerUrl, "30232") ||
		strings.Contains(lowerUrl, "30216") ||
		strings.Contains(lowerUrl, "audio") {
		classify = "audio"
		suffix = ".mp3"
		contentType = "audio/mp4"
	}

	isAll, _ := p.bridge.GetResType("all")
	isMatch, _ := p.bridge.GetResType(classify)
	if !isAll && !isMatch {
		return
	}

	// 大小过滤
	if classify == "video" {
		if minSize, ok := p.bridge.GetConfig("MinVideoSize").(int); ok && minSize > 0 {
			if totalSize > 0 && totalSize < float64(minSize*1024) {
				return
			}
		}
	}

	urlSign := shared.Md5(rawUrl)
	if p.bridge.MediaIsMarked(urlSign) {
		return
	}

	desc := "B站媒体流"
	if classify == "audio" {
		desc = "B站音频流"
	}

	p.sendMedia(rawUrl, classify, suffix, contentType, desc, "", totalSize)
}

func (p *BilibiliPlugin) sendMedia(playUrl, classify, suffix, contentType, desc, coverUrl string, size float64) {
	urlSign := shared.Md5(playUrl)
	if p.bridge.MediaIsMarked(urlSign) {
		return
	}

	// 大小过滤
	if classify == "video" {
		if minSize, ok := p.bridge.GetConfig("MinVideoSize").(int); ok && minSize > 0 {
			if size > 0 && size < float64(minSize*1024) {
				return
			}
		}
	}

	id, err := gonanoid.New()
	if err != nil {
		id = urlSign
	}

	reqHeaders := make(http.Header)
	reqHeaders.Set("Referer", "https://www.bilibili.com/")
	reqHeaders.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36")

	otherData := map[string]string{}
	if hJson, err := json.Marshal(reqHeaders); err == nil {
		otherData["headers"] = string(hJson)
	}

	res := shared.MediaInfo{
		Id:          id,
		Url:         playUrl,
		UrlSign:     urlSign,
		CoverUrl:    coverUrl,
		Size:        size,
		Domain:      shared.GetTopLevelDomain(playUrl),
		Classify:    classify,
		Suffix:      suffix,
		Status:      shared.DownloadStatusReady,
		SavePath:    "",
		DecodeKey:   "",
		OtherData:   otherData,
		Description: desc,
		ContentType: contentType,
	}

	p.bridge.MarkMedia(urlSign)
	p.bridge.Send("newResources", res)
}

func (p *BilibiliPlugin) extractUrlFromMediaMap(m map[string]interface{}) string {
	if baseUrl, ok := m["baseUrl"].(string); ok && baseUrl != "" {
		return baseUrl
	}
	if baseUrl, ok := m["base_url"].(string); ok && baseUrl != "" {
		return baseUrl
	}
	if backupUrl, ok := m["backupUrl"].([]interface{}); ok && len(backupUrl) > 0 {
		if u, ok := backupUrl[0].(string); ok && u != "" {
			return u
		}
	}
	if backupUrl, ok := m["backup_url"].([]interface{}); ok && len(backupUrl) > 0 {
		if u, ok := backupUrl[0].(string); ok && u != "" {
			return u
		}
	}
	if u, ok := m["url"].(string); ok && u != "" {
		return u
	}
	return ""
}

func (p *BilibiliPlugin) getQualityLabel(id float64) string {
	switch int(id) {
	case 127:
		return "8K超高清"
	case 126:
		return "杜比视界"
	case 125:
		return "HDR真彩"
	case 120:
		return "4K超清"
	case 116:
		return "1080P60帧"
	case 112:
		return "1080P高码率"
	case 80:
		return "1080P高清"
	case 74:
		return "720P60帧"
	case 64:
		return "720P高清"
	case 32:
		return "480P清晰"
	case 16:
		return "360P流畅"
	default:
		return "高清视频"
	}
}
