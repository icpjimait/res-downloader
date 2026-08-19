package plugins

import (
	"bytes"
	"encoding/json"
	"github.com/elazarl/goproxy"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"io"
	"net/http"
	"res-downloader/core/shared"
	"strings"
)

type DouyinPlugin struct {
	bridge *shared.Bridge
}

func (p *DouyinPlugin) SetBridge(bridge *shared.Bridge) {
	p.bridge = bridge
}

func (p *DouyinPlugin) Domains() []string {
	return []string{"douyin.com", "iesdouyin.com", "amemv.com", "snssdk.com", "douyincdn.com", "douyinvod.com"}
}

func (p *DouyinPlugin) OnRequest(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
	return r, nil
}

func (p *DouyinPlugin) OnResponse(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
	if p.bridge.IsProxy != nil && !p.bridge.IsProxy() {
		return nil
	}

	if resp == nil || resp.Request == nil || (resp.StatusCode != 200 && resp.StatusCode != 206) {
		return nil
	}

	rawUrl := resp.Request.URL.String()
	lowerUrl := strings.ToLower(rawUrl)
	contentType := resp.Header.Get("Content-Type")

	// 1. 过滤掉 DASH / MSE 分离式的音视频裸分片（如 media-video-avc1.mp4, media-audio-mp4a.mp4）
	// 这些裸分片缺少音轨或缺少完整 moov 索引头，下载后无法独立播放
	if strings.Contains(lowerUrl, "media-video-") || strings.Contains(lowerUrl, "media-audio-") {
		return resp
	}

	// 2. 拦截并解析抖音 Web API JSON 响应，直接提取带标题的音视频合一完整高画质 MP4
	if strings.Contains(contentType, "json") ||
		strings.Contains(lowerUrl, "/aweme/v1/web/") ||
		strings.Contains(lowerUrl, "/aweme/v1/feed/") ||
		strings.Contains(lowerUrl, "/aweme/v1/play/") ||
		strings.Contains(lowerUrl, "/iteminfo/") {

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return resp
		}
		// 还原 Body 供后续正常消费
		resp.Body = io.NopCloser(bytes.NewBuffer(body))

		go p.extractAwemeVideos(body)
		return resp
	}

	return nil
}

func (p *DouyinPlugin) extractAwemeVideos(body []byte) {
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return
	}

	isAll, _ := p.bridge.GetResType("all")
	isVideo, _ := p.bridge.GetResType("video")
	if !isAll && !isVideo {
		return
	}

	// 提取单个或列表中的所有 aweme 视频对象
	var awemeList []map[string]interface{}

	if list, ok := data["aweme_list"].([]interface{}); ok {
		for _, item := range list {
			if m, ok := item.(map[string]interface{}); ok {
				awemeList = append(awemeList, m)
			}
		}
	} else if item, ok := data["aweme_detail"].(map[string]interface{}); ok {
		awemeList = append(awemeList, item)
	} else if list, ok := data["item_list"].([]interface{}); ok {
		for _, item := range list {
			if m, ok := item.(map[string]interface{}); ok {
				awemeList = append(awemeList, m)
			}
		}
	} else if item, ok := data["item_info"].(map[string]interface{}); ok {
		awemeList = append(awemeList, item)
	}

	for _, aweme := range awemeList {
		p.processAwemeItem(aweme)
	}
}

func (p *DouyinPlugin) processAwemeItem(aweme map[string]interface{}) {
	videoObj, ok := aweme["video"].(map[string]interface{})
	if !ok {
		return
	}

	desc, _ := aweme["desc"].(string)
	if desc == "" {
		desc, _ = aweme["title"].(string)
	}
	desc = strings.TrimSpace(desc)

	// 优先提取最高画质的播放地址（play_addr / play_addr_h264）
	var playUrl string
	var dataSize float64

	// 1. 尝试从 play_addr_h264 或 play_addr 获取
	for _, key := range []string{"play_addr_h264", "play_addr", "download_addr"} {
		if addr, ok := videoObj[key].(map[string]interface{}); ok {
			if size, ok := addr["data_size"].(float64); ok && size > 0 {
				dataSize = size
			}
			if urlList, ok := addr["url_list"].([]interface{}); ok && len(urlList) > 0 {
				for _, u := range urlList {
					if str, ok := u.(string); ok && str != "" && strings.HasPrefix(str, "http") {
						playUrl = str
						break
					}
				}
			}
		}
		if playUrl != "" {
			break
		}
	}

	// 2. 尝试从 bit_rate 列表中获取
	if playUrl == "" {
		if bitRates, ok := videoObj["bit_rate"].([]interface{}); ok && len(bitRates) > 0 {
			for _, br := range bitRates {
				if brMap, ok := br.(map[string]interface{}); ok {
					if addr, ok := brMap["play_addr"].(map[string]interface{}); ok {
						if size, ok := addr["data_size"].(float64); ok && size > 0 {
							dataSize = size
						}
						if urlList, ok := addr["url_list"].([]interface{}); ok && len(urlList) > 0 {
							for _, u := range urlList {
								if str, ok := u.(string); ok && str != "" && strings.HasPrefix(str, "http") {
									playUrl = str
									break
								}
							}
						}
					}
				}
				if playUrl != "" {
					break
				}
			}
		}
	}

	if playUrl == "" {
		return
	}

	// 大小过滤
	if minSize, ok := p.bridge.GetConfig("MinVideoSize").(int); ok && minSize > 0 {
		if dataSize > 0 && dataSize < float64(minSize*1024) {
			return
		}
	}

	// 封面提取
	var coverUrl string
	if cover, ok := videoObj["cover"].(map[string]interface{}); ok {
		if urlList, ok := cover["url_list"].([]interface{}); ok && len(urlList) > 0 {
			if str, ok := urlList[0].(string); ok {
				coverUrl = str
			}
		}
	}

	urlSign := shared.Md5(playUrl)
	if p.bridge.MediaIsMarked(urlSign) {
		return
	}

	id, err := gonanoid.New()
	if err != nil {
		id = urlSign
	}

	reqHeaders := make(http.Header)
	reqHeaders.Set("Referer", "https://www.douyin.com/")
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
		Size:        dataSize,
		Domain:      shared.GetTopLevelDomain(playUrl),
		Classify:    "video",
		Suffix:      ".mp4",
		Status:      shared.DownloadStatusReady,
		SavePath:    "",
		DecodeKey:   "",
		OtherData:   otherData,
		Description: desc,
		ContentType: "video/mp4",
	}

	p.bridge.MarkMedia(urlSign)
	p.bridge.Send("newResources", res)
}
