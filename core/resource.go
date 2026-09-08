package core

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"res-downloader/core/shared"
	"strconv"
	"strings"
	"sync"
)

type WxFileDecodeResult struct {
	SavePath string
	Message  string
}

type Resource struct {
	mediaMark  sync.Map
	tasks      sync.Map
	resType    map[string]bool
	resTypeMux sync.RWMutex
}

func initResource() *Resource {
	if resourceOnce == nil {
		resourceOnce = &Resource{}
		resourceOnce.resType = resourceOnce.buildResType(globalConfig.MimeMap)
	}
	return resourceOnce
}

func (r *Resource) buildResType(mime map[string]MimeInfo) map[string]bool {
	t := map[string]bool{
		"all": true,
	}

	for _, item := range mime {
		if _, ok := t[item.Type]; !ok {
			t[item.Type] = true
		}
	}

	return t
}

func (r *Resource) mediaIsMarked(key string) bool {
	_, loaded := r.mediaMark.Load(key)
	return loaded
}

func (r *Resource) markMedia(key string) {
	r.mediaMark.Store(key, true)
}

func (r *Resource) getResType(key string) (bool, bool) {
	r.resTypeMux.RLock()
	value, ok := r.resType[key]
	r.resTypeMux.RUnlock()
	return value, ok
}

func (r *Resource) setResType(n []string) {
	r.resTypeMux.Lock()
	for key := range r.resType {
		r.resType[key] = false
	}

	for _, value := range n {
		if _, ok := r.resType[value]; ok {
			r.resType[value] = true
		}
	}
	r.resTypeMux.Unlock()
}

func (r *Resource) clear() {
	r.mediaMark.Clear()
}

func (r *Resource) delete(sign string) {
	r.mediaMark.Delete(sign)
}

func (r *Resource) cancel(id string) error {
	if v, ok := r.tasks.Load(id); ok {
		if d, ok := v.(*FileDownloader); ok {
			d.Cancel()
		} else if cancel, ok := v.(context.CancelFunc); ok {
			cancel()
		}
		r.tasks.Delete(id)
		return nil
	}
	return errors.New("task not found")
}

func (r *Resource) download(mediaInfo shared.MediaInfo, decodeStr string) {
	if globalConfig.SaveDirectory == "" {
		return
	}
	go func(mediaInfo shared.MediaInfo) {
		rawUrl := mediaInfo.Url
		fileName := shared.Md5(rawUrl)

		if v := shared.GetFileNameFromURL(rawUrl); v != "" {
			fileName = v
		}

		if mediaInfo.Description != "" {
			fileName = shared.SanitizeFileName(mediaInfo.Description)
			if globalConfig.FilenameLen > 0 {
				runes := []rune(fileName)
				if len(runes) > globalConfig.FilenameLen {
					fileName = string(runes[:globalConfig.FilenameLen])
				}
			} else {
				runes := []rune(fileName)
				if len(runes) > 180 {
					fileName = string(runes[:180])
				}
			}
		}

		if globalConfig.FilenameTime {
			mediaInfo.SavePath = filepath.Join(globalConfig.SaveDirectory, fileName+"_"+shared.GetCurrentDateTimeFormatted())
		} else {
			mediaInfo.SavePath = filepath.Join(globalConfig.SaveDirectory, fileName)
		}

		suffix := mediaInfo.Suffix
		if suffix == "" || suffix == "default" || suffix == ".default" {
			if mediaInfo.Classify == "video" || strings.Contains(rawUrl, "douyin") || strings.Contains(rawUrl, "tos") {
				suffix = ".mp4"
			} else if mediaInfo.Classify == "audio" {
				suffix = ".mp3"
			} else if mediaInfo.Classify == "image" {
				suffix = ".png"
			}
		}
		if suffix != "" && !strings.HasPrefix(suffix, ".") {
			suffix = "." + suffix
		}
		mediaInfo.Suffix = suffix

		if suffix != "" && !strings.HasSuffix(strings.ToLower(mediaInfo.SavePath), strings.ToLower(suffix)) {
			mediaInfo.SavePath = mediaInfo.SavePath + suffix
		}

		if strings.Contains(rawUrl, "qq.com") {
			if globalConfig.Quality == 1 &&
				strings.Contains(rawUrl, "encfilekey=") &&
				strings.Contains(rawUrl, "token=") {
				parseUrl, err := url.Parse(rawUrl)
				queryParams := parseUrl.Query()
				if err == nil && queryParams.Has("encfilekey") && queryParams.Has("token") {
					rawUrl = parseUrl.Scheme + "://" + parseUrl.Host + "/" + parseUrl.Path +
						"?encfilekey=" + queryParams.Get("encfilekey") +
						"&token=" + queryParams.Get("token")
				}
			} else if globalConfig.Quality > 1 && mediaInfo.OtherData["wx_file_formats"] != "" {
				format := strings.Split(mediaInfo.OtherData["wx_file_formats"], "#")
				qualityMap := []string{
					format[0],
					format[len(format)/2],
					format[len(format)-1],
				}
				rawUrl += "&X-snsvideoflag=" + qualityMap[globalConfig.Quality-2]
			}
		}

		headers, _ := r.parseHeaders(mediaInfo)

		// 检查是否为 M3U8 视频：如果系统存在 ffmpeg，通过内置代理流直接下载并无损合并为标准 MP4
		isM3U8 := mediaInfo.Classify == "m3u8" || strings.Contains(strings.ToLower(rawUrl), ".m3u8")
		ffmpegPath := shared.FindFFmpegPath()
		if isM3U8 && ffmpegPath != "" {
			r.progressEventsEmit(mediaInfo, "正在下载M3U8视频流...", shared.DownloadStatusRunning)
			if strings.HasSuffix(strings.ToLower(mediaInfo.SavePath), ".m3u8") || mediaInfo.Suffix == ".m3u8" || mediaInfo.Suffix == "" {
				mediaInfo.SavePath = strings.TrimSuffix(mediaInfo.SavePath, filepath.Ext(mediaInfo.SavePath)) + ".mp4"
			}
			mediaInfo.SavePath = shared.GetUniqueFileName(mediaInfo.SavePath)

			previewURL := fmt.Sprintf("http://127.0.0.1:%s/api/preview/playlist.m3u8?url=%s", globalConfig.Port, url.QueryEscape(rawUrl))
			ctx, cancel := context.WithCancel(context.Background())
			r.tasks.Store(mediaInfo.Id, cancel)

			cmd := exec.CommandContext(ctx, ffmpegPath, "-allowed_extensions", "ALL", "-protocol_whitelist", "file,http,https,tcp,tls,crypto", "-y", "-i", previewURL, "-c", "copy", "-bsf:a", "aac_adtstoasc", mediaInfo.SavePath)
			output, err := cmd.CombinedOutput()
			// 若因 fMP4/H.265 已是 ASC 格式不需要 aac_adtstoasc 导致失败，自动回退纯 copy 模式
			if err != nil && ctx.Err() == nil {
				cmdFallback := exec.CommandContext(ctx, ffmpegPath, "-allowed_extensions", "ALL", "-protocol_whitelist", "file,http,https,tcp,tls,crypto", "-y", "-i", previewURL, "-c", "copy", mediaInfo.SavePath)
				outputFallback, errFallback := cmdFallback.CombinedOutput()
				if errFallback == nil {
					err = nil
					output = outputFallback
				}
			}
			r.tasks.Delete(mediaInfo.Id)

			if err != nil {
				if ctx.Err() != nil {
					_ = os.Remove(mediaInfo.SavePath)
					return
				}
				globalLogger.Warn().Msgf("ffmpeg m3u8 download error: %v, output: %s", err, string(output))
				downloader := NewFileDownloader(rawUrl, mediaInfo.SavePath, globalConfig.TaskNumber, headers)
				_ = downloader.Start()
			}
			if fi, err := os.Stat(mediaInfo.SavePath); err == nil {
				mediaInfo.Size = float64(fi.Size())
			}
			r.progressEventsEmit(mediaInfo, "complete", shared.DownloadStatusDone)
			return
		}

		downloader := NewFileDownloader(rawUrl, mediaInfo.SavePath, globalConfig.TaskNumber, headers)
		downloader.progressCallback = func(totalDownloaded, totalSize float64, taskID int, taskProgress float64) {
			r.progressEventsEmit(mediaInfo, strconv.Itoa(int(totalDownloaded*100/totalSize))+"%", shared.DownloadStatusRunning)
		}
		r.tasks.Store(mediaInfo.Id, downloader)
		err := downloader.Start()
		mediaInfo.SavePath = downloader.FileName
		if err != nil {
			if !strings.Contains(err.Error(), "cancelled") {
				r.progressEventsEmit(mediaInfo, err.Error())
			}
			return
		}

		// 检查是否有伴音音频轨（如 B站 DASH 音画分离流）
		if audioUrl, ok := mediaInfo.OtherData["audio_url"]; ok && audioUrl != "" && mediaInfo.Classify == "video" {
			r.progressEventsEmit(mediaInfo, "正在下载音频伴音...", shared.DownloadStatusRunning)
			tempAudioPath := mediaInfo.SavePath + ".audio.tmp"
			audioDownloader := NewFileDownloader(audioUrl, tempAudioPath, globalConfig.TaskNumber, headers)
			if audioErr := audioDownloader.Start(); audioErr == nil {
				r.progressEventsEmit(mediaInfo, "正在合成音视频(FFmpeg)...", shared.DownloadStatusRunning)
				tempVideoPath := mediaInfo.SavePath + ".video.tmp"
				if renameErr := os.Rename(mediaInfo.SavePath, tempVideoPath); renameErr == nil {
					mergeErr := shared.MergeMediaWithFFmpeg(tempVideoPath, tempAudioPath, mediaInfo.SavePath)
					if mergeErr == nil {
						_ = os.Remove(tempVideoPath)
						_ = os.Remove(tempAudioPath)
					} else {
						// 若无 ffmpeg，还原主视频并将音频独立保存
						_ = os.Rename(tempVideoPath, mediaInfo.SavePath)
						audioDest := strings.TrimSuffix(mediaInfo.SavePath, filepath.Ext(mediaInfo.SavePath)) + "_音频.mp3"
						_ = os.Rename(tempAudioPath, audioDest)
					}
				}
			}
		}

		if decodeStr != "" {
			r.progressEventsEmit(mediaInfo, "decrypting in progress", shared.DownloadStatusRunning)
			if err := r.decodeWxFile(mediaInfo.SavePath, decodeStr); err != nil {
				r.progressEventsEmit(mediaInfo, "decryption error: "+err.Error())
				return
			}
		} else {
			// 自动解密内置 AES 加密图片（例如 51cg1 等站点的图片）
			if newPath, err := DecryptFileOnDisk(mediaInfo.SavePath); err == nil && newPath != "" {
				mediaInfo.SavePath = newPath
			}
		}
		if fi, err := os.Stat(mediaInfo.SavePath); err == nil {
			mediaInfo.Size = float64(fi.Size())
		}
		r.progressEventsEmit(mediaInfo, "complete", shared.DownloadStatusDone)
	}(mediaInfo)
}

func (r *Resource) parseHeaders(mediaInfo shared.MediaInfo) (map[string]string, error) {
	headers := make(map[string]string)

	if hh, ok := mediaInfo.OtherData["headers"]; ok {
		var tempHeaders map[string][]string
		if err := json.Unmarshal([]byte(hh), &tempHeaders); err != nil {
			return headers, fmt.Errorf("parse headers JSON err: %v", err)
		}

		for key, values := range tempHeaders {
			if len(values) > 0 {
				headers[key] = values[0]
			}
		}
	}

	return headers, nil
}

func (r *Resource) wxFileDecode(mediaInfo shared.MediaInfo, fileName, decodeStr string) (string, error) {
	sourceFile, err := os.Open(fileName)
	if err != nil {
		return "", err
	}
	defer sourceFile.Close()
	mediaInfo.SavePath = strings.ReplaceAll(fileName, ".mp4", "_decrypt.mp4")

	destinationFile, err := os.Create(mediaInfo.SavePath)
	if err != nil {
		return "", err
	}
	defer destinationFile.Close()

	_, err = io.Copy(destinationFile, sourceFile)
	if err != nil {
		return "", err
	}
	err = r.decodeWxFile(mediaInfo.SavePath, decodeStr)
	if err != nil {
		return "", err
	}
	return mediaInfo.SavePath, nil
}

func (r *Resource) progressEventsEmit(mediaInfo shared.MediaInfo, args ...string) {
	Status := shared.DownloadStatusError
	Message := "ok"

	if len(args) > 0 {
		Message = args[0]
	}
	if len(args) > 1 {
		Status = args[1]
	}

	httpServerOnce.send("downloadProgress", map[string]interface{}{
		"Id":       mediaInfo.Id,
		"Status":   Status,
		"SavePath": mediaInfo.SavePath,
		"Message":  Message,
		"Size":     mediaInfo.Size,
	})
	return
}

func (r *Resource) decodeWxFile(fileName, decodeStr string) error {
	decodedBytes, err := base64.StdEncoding.DecodeString(decodeStr)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(fileName, os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	byteCount := len(decodedBytes)
	fileBytes := make([]byte, byteCount)
	n, err := file.Read(fileBytes)
	if err != nil && err != io.EOF {
		return err
	}

	if n < byteCount {
		byteCount = n
	}

	xorResult := make([]byte, byteCount)
	for i := 0; i < byteCount; i++ {
		xorResult[i] = decodedBytes[i] ^ fileBytes[i]
	}
	_, err = file.Seek(0, 0)
	if err != nil {
		return err
	}

	_, err = file.Write(xorResult)
	if err != nil {
		return err
	}
	return nil
}
