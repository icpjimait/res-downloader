package plugins

import (
	"encoding/json"
	"github.com/elazarl/goproxy"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"net/http"
	"path/filepath"
	"res-downloader/core/shared"
	"strconv"
	"strings"
)

type DefaultPlugin struct {
	bridge *shared.Bridge
}

func (p *DefaultPlugin) SetBridge(bridge *shared.Bridge) {
	p.bridge = bridge
}

func (p *DefaultPlugin) Domains() []string {
	return []string{"default"}
}

func (p *DefaultPlugin) OnRequest(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
	return r, nil
}

func (p *DefaultPlugin) OnResponse(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
	if p.bridge.IsProxy != nil && !p.bridge.IsProxy() {
		return resp
	}

	if resp == nil || resp.Request == nil || (resp.StatusCode != 200 && resp.StatusCode != 206 && resp.StatusCode != 304) {
		return resp
	}

	contentType := resp.Header.Get("Content-Type")
	classify, suffix := p.bridge.TypeSuffix(contentType)

	rawUrl := resp.Request.URL.String()
	lowerUrl := strings.ToLower(rawUrl)
	host := strings.ToLower(resp.Request.Host)

	// 特别针对抖音 / 字节跳动 / 西瓜视频媒体 CDN 进行精准视频特征识别（排除常规 API 与上报请求）
	isByteDanceVideo := strings.Contains(host, "douyinvod.com") ||
		strings.Contains(lowerUrl, "/video/tos/") ||
		strings.Contains(lowerUrl, "/tos-cn-") ||
		strings.Contains(lowerUrl, "aweme/v1/play") ||
		((strings.Contains(host, "snssdk.com") || strings.Contains(host, "amemv.com") || strings.Contains(host, "douyin.com") || strings.Contains(host, "ixigua.com")) &&
			(strings.Contains(lowerUrl, ".mp4") || strings.Contains(contentType, "video") || strings.Contains(contentType, "tos")))

	if isByteDanceVideo {
		if classify == "" || classify == "stream" {
			classify = "video"
			suffix = ".mp4"
			contentType = "video/mp4"
		}
	} else if classify == "" || classify == "stream" {
		// URL 路径含有常规音视频后缀但 Header 为 application/octet-stream 的智能识别
		cleanPath := strings.Split(strings.Split(lowerUrl, "?")[0], "#")[0]
		if strings.HasSuffix(cleanPath, ".mp4") || strings.Contains(cleanPath, ".mp4") {
			classify = "video"
			suffix = ".mp4"
			contentType = "video/mp4"
		} else if strings.HasSuffix(cleanPath, ".flv") {
			classify = "live"
			suffix = ".flv"
		} else if strings.HasSuffix(cleanPath, ".m3u8") {
			classify = "m3u8"
			suffix = ".m3u8"
		} else if strings.HasSuffix(cleanPath, ".mp3") || strings.HasSuffix(cleanPath, ".m4a") {
			classify = "audio"
			suffix = ".mp3"
		}
	}

	if classify == "" {
		return resp
	}

	isAll, _ := p.bridge.GetResType("all")
	isClassify, _ := p.bridge.GetResType(classify)

	if suffix == "default" || suffix == "" {
		ext := filepath.Ext(filepath.Base(strings.Split(strings.Split(rawUrl, "?")[0], "#")[0]))
		if ext != "" {
			suffix = ext
		} else if classify == "video" {
			suffix = ".mp4"
		}
	}

	urlSign := shared.Md5(rawUrl)
	if ok := p.bridge.MediaIsMarked(urlSign); !ok && (isAll || isClassify) {
		var value float64
		// 优先从 Content-Range（如 bytes 0-32767/15482931）中获取真实文件总大小
		if contentRange := resp.Header.Get("Content-Range"); contentRange != "" {
			parts := strings.Split(contentRange, "/")
			if len(parts) == 2 && parts[1] != "*" {
				if totalSize, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64); err == nil && totalSize > 0 {
					value = totalSize
				}
			}
		}
		if value == 0 {
			value, _ = strconv.ParseFloat(resp.Header.Get("content-length"), 64)
		}

		if classify == "image" {
			if minSize, ok := p.bridge.GetConfig("MinImageSize").(int); ok && minSize > 0 {
				if value < float64(minSize*1024) {
					return resp
				}
			}
		} else if classify == "video" || classify == "m3u8" {
			if minSize, ok := p.bridge.GetConfig("MinVideoSize").(int); ok && minSize > 0 {
				if value < float64(minSize*1024) {
					return resp
				}
			}
		}
		id, err := gonanoid.New()
		if err != nil {
			id = urlSign
		}
		res := shared.MediaInfo{
			Id:          id,
			Url:         rawUrl,
			UrlSign:     urlSign,
			CoverUrl:    "",
			Size:        value,
			Domain:      shared.GetTopLevelDomain(rawUrl),
			Classify:    classify,
			Suffix:      suffix,
			Status:      shared.DownloadStatusReady,
			SavePath:    "",
			DecodeKey:   "",
			OtherData:   map[string]string{},
			Description: "",
			ContentType: contentType,
		}

		// Store entire request headers as JSON
		reqHeaders := make(http.Header)
		for k, v := range resp.Request.Header {
			reqHeaders[k] = v
		}
		if isByteDanceVideo && reqHeaders.Get("Referer") == "" {
			reqHeaders.Set("Referer", "https://www.douyin.com/")
		}

		if headers, err := json.Marshal(reqHeaders); err == nil {
			res.OtherData["headers"] = string(headers)
		}

		p.bridge.MarkMedia(urlSign)
		go func(res shared.MediaInfo) {
			p.bridge.Send("newResources", res)
		}(res)
	}

	return resp
}
