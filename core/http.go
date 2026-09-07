package core

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"res-downloader/core/shared"
	"strconv"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type respData map[string]interface{}

type ResponseData struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

type HttpServer struct{}

func initHttpServer() *HttpServer {
	if httpServerOnce == nil {
		httpServerOnce = &HttpServer{}
	}
	return httpServerOnce
}

func (h *HttpServer) run() {
	listener, err := net.Listen("tcp", globalConfig.Host+":"+globalConfig.Port)
	if err != nil {
		globalLogger.Err(err)
		log.Fatalf("Service cannot start: %v", err)
	}
	fmt.Println("Service started, listening http://" + globalConfig.Host + ":" + globalConfig.Port)
	if err1 := http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host == "127.0.0.1:"+globalConfig.Port && HandleApi(w, r) {

		} else {
			proxyOnce.Proxy.ServeHTTP(w, r) // 代理
		}
	})); err1 != nil {
		globalLogger.Err(err1)
		fmt.Printf("Service startup exception: %v", err1)
	}
}

func (h *HttpServer) downCert(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/x-x509-ca-data")
	w.Header().Set("Content-Disposition", "attachment;filename=res-downloader-public.crt")
	w.Header().Set("Content-Transfer-Encoding", "binary")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(appOnce.PublicCrt)))
	w.WriteHeader(http.StatusOK)
	io.Copy(w, io.NopCloser(bytes.NewReader(appOnce.PublicCrt)))
}

func resolveURL(base *url.URL, target string) string {
	target = strings.TrimSpace(target)
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		return target
	}
	if base == nil {
		return target
	}
	u, err := url.Parse(target)
	if err != nil {
		return target
	}
	return base.ResolveReference(u).String()
}

func (h *HttpServer) preview(w http.ResponseWriter, r *http.Request) {
	// 允许跨域请求与预检
	origin := r.Header.Get("Origin")
	if origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	} else {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	}
	w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "*")
	w.Header().Set("Access-Control-Expose-Headers", "Content-Range, Content-Length, Accept-Ranges")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	realURL := r.URL.Query().Get("url")
	if realURL == "" {
		http.Error(w, "Missing 'url' parameter", http.StatusBadRequest)
		return
	}
	realURL, _ = url.QueryUnescape(realURL)
	parsedURL, err := url.Parse(realURL)
	if err != nil {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}
	request, err := http.NewRequest("GET", parsedURL.String(), nil)
	if err != nil {
		http.Error(w, "Failed to fetch the resource", http.StatusInternalServerError)
		return
	}

	// 1. 设置真实有效的 User-Agent
	ua := globalConfig.UserAgent
	if ua == "" {
		ua = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36"
	}
	request.Header.Set("User-Agent", ua)

	// 2. 根据域名智能设置 Referer 与防盗链头
	host := strings.ToLower(parsedURL.Host)
	if strings.Contains(host, "douyinvod.com") || strings.Contains(host, "douyin.com") ||
		strings.Contains(host, "snssdk.com") || strings.Contains(host, "amemv.com") ||
		strings.Contains(host, "bytegoofy.com") || strings.Contains(host, "ixigua.com") {
		request.Header.Set("Referer", "https://www.douyin.com/")
	} else if strings.Contains(host, "kuaishou.com") || strings.Contains(host, "yximgs.com") || strings.Contains(host, "kwimgs.com") {
		request.Header.Set("Referer", "https://www.kuaishou.com/")
	} else if strings.Contains(host, "xiaohongshu.com") || strings.Contains(host, "xhscdn.com") {
		request.Header.Set("Referer", "https://www.xiaohongshu.com/")
	} else if strings.Contains(host, "bilibili.com") || strings.Contains(host, "bilivideo.com") || strings.Contains(host, "bilivideo.cn") || strings.Contains(host, "hdslb.com") || strings.Contains(host, "biliapi.net") {
		request.Header.Set("Referer", "https://www.bilibili.com/")
	} else if parsedURL.Scheme != "" && parsedURL.Host != "" {
		request.Header.Set("Referer", parsedURL.Scheme+"://"+parsedURL.Host+"/")
	}

	// 3. 支持前端音视频播放器 Range 请求分段播放
	if rangeHeader := r.Header.Get("Range"); rangeHeader != "" {
		request.Header.Set("Range", rangeHeader)
	}

	client := &http.Client{
		Transport: BuildUpstreamTransport(),
		Timeout:   60 * time.Second,
	}

	resp, err := client.Do(request)
	if err != nil {
		http.Error(w, "Failed to fetch the resource: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	peekBytes, _ := reader.Peek(512)

	// A. 检测是否为 M3U8 播放列表
	isM3U8 := strings.Contains(strings.ToLower(realURL), ".m3u8") ||
		strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "mpegurl") ||
		bytes.HasPrefix(bytes.TrimSpace(peekBytes), []byte("#EXTM3U"))

	if isM3U8 {
		m3u8Data, err := io.ReadAll(reader)
		if err != nil {
			http.Error(w, "Failed to read m3u8", http.StatusInternalServerError)
			return
		}

		baseHost := "http://127.0.0.1:" + globalConfig.Port
		if r.Host != "" {
			scheme := "http"
			if r.TLS != nil {
				scheme = "https"
			}
			baseHost = scheme + "://" + r.Host
		}
		keyPrefix := baseHost + "/api/preview/key.key?url="
		segPrefix := baseHost + "/api/preview/segment.ts?url="
		m3u8Prefix := baseHost + "/api/preview/playlist.m3u8?url="

		scanner := bufio.NewScanner(bytes.NewReader(m3u8Data))
		var rewrittenLines []string
		reKey := regexp.MustCompile(`URI="([^"]+)"`)

		for scanner.Scan() {
			line := scanner.Text()
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				rewrittenLines = append(rewrittenLines, line)
				continue
			}

			if strings.HasPrefix(trimmed, "#EXT-X-KEY:") {
				line = reKey.ReplaceAllStringFunc(line, func(match string) string {
					sub := reKey.FindStringSubmatch(match)
					if len(sub) >= 2 {
						keyURL := resolveURL(parsedURL, sub[1])
						return fmt.Sprintf(`URI="%s%s"`, keyPrefix, url.QueryEscape(keyURL))
					}
					return match
				})
				rewrittenLines = append(rewrittenLines, line)
			} else if strings.HasPrefix(trimmed, "#") {
				if reKey.MatchString(line) {
					line = reKey.ReplaceAllStringFunc(line, func(match string) string {
						sub := reKey.FindStringSubmatch(match)
						if len(sub) >= 2 {
							keyURL := resolveURL(parsedURL, sub[1])
							return fmt.Sprintf(`URI="%s%s"`, keyPrefix, url.QueryEscape(keyURL))
						}
						return match
					})
				}
				rewrittenLines = append(rewrittenLines, line)
			} else {
				segURL := resolveURL(parsedURL, trimmed)
				if strings.Contains(strings.ToLower(segURL), ".m3u8") {
					rewrittenLines = append(rewrittenLines, m3u8Prefix+url.QueryEscape(segURL))
				} else {
					rewrittenLines = append(rewrittenLines, segPrefix+url.QueryEscape(segURL))
				}
			}
		}

		output := strings.Join(rewrittenLines, "\n")
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl; charset=utf-8")
		w.Header().Set("Content-Length", strconv.Itoa(len(output)))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(output))
		return
	}

	// B. 检测是否为 AES 加密图片（以 {s}g=- 开头）
	if bytes.HasPrefix(peekBytes, EncryptedImgPrefix) {
		allData, err := io.ReadAll(reader)
		if err == nil {
			decrypted, isEnc, mimeType, _, decErr := DecryptImageBytes(allData)
			if isEnc && decErr == nil {
				w.Header().Set("Content-Type", mimeType)
				w.Header().Set("Content-Length", strconv.Itoa(len(decrypted)))
				w.WriteHeader(http.StatusOK)
				w.Write(decrypted)
				return
			}
		}
	}

	// C. 常规媒体或分段切片（TS、MP4、加密 Key 等）
	for k, v := range resp.Header {
		lk := strings.ToLower(k)
		if strings.HasPrefix(lk, "access-control-") {
			continue
		}
		for _, vv := range v {
			w.Header().Add(k, vv)
		}
	}

	// 确保切片和密钥的 Content-Type 能够被 HLS 播放器正确解析
	if strings.Contains(strings.ToLower(realURL), ".ts") {
		w.Header().Set("Content-Type", "video/mp2t")
	} else if strings.Contains(strings.ToLower(realURL), ".key") {
		w.Header().Set("Content-Type", "application/octet-stream")
	}

	w.WriteHeader(resp.StatusCode)
	_, err = io.Copy(w, reader)
	if err != nil {
		globalLogger.Warn().Msgf("preview stream write error: %v", err)
	}
}

func (h *HttpServer) send(t string, data interface{}) {
	jsonData, err := json.Marshal(map[string]interface{}{
		"type": t,
		"data": data,
	})
	if err != nil {
		fmt.Println("Error converting map to JSON:", err)
		return
	}
	runtime.EventsEmit(appOnce.ctx, "event", string(jsonData))
}

func (h *HttpServer) writeJson(w http.ResponseWriter, data *ResponseData) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(200)
	err := json.NewEncoder(w).Encode(data)
	if err != nil {
		globalLogger.Err(err)
	}
}

func (h *HttpServer) error(w http.ResponseWriter, args ...interface{}) {
	message := "ok"
	var data interface{}

	if len(args) > 0 {
		message = args[0].(string)
	}
	if len(args) > 1 {
		data = args[1]
	}
	h.writeJson(w, h.buildResp(0, message, data))
}

func (h *HttpServer) success(w http.ResponseWriter, args ...interface{}) {
	message := "ok"
	var data interface{}

	if len(args) > 0 {
		data = args[0]
	}

	if len(args) > 1 {
		message = args[1].(string)
	}
	h.writeJson(w, h.buildResp(1, message, data))
}

func (h *HttpServer) buildResp(code int, message string, data interface{}) *ResponseData {
	return &ResponseData{
		Code:    code,
		Message: message,
		Data:    data,
	}
}

func (h *HttpServer) openDirectoryDialog(w http.ResponseWriter, r *http.Request) {
	folder, err := runtime.OpenDirectoryDialog(appOnce.ctx, runtime.OpenDialogOptions{
		DefaultDirectory: "",
		Title:            "Select a folder",
	})
	if err != nil {
		h.error(w, err.Error())
		return
	}
	h.success(w, respData{
		"folder": folder,
	})
}

func (h *HttpServer) openFileDialog(w http.ResponseWriter, r *http.Request) {
	filePath, err := runtime.OpenFileDialog(appOnce.ctx, runtime.OpenDialogOptions{
		Filters: []runtime.FileFilter{
			{
				DisplayName: "Videos (*.mov;*.mp4)",
				Pattern:     "*.mp4",
			},
		},
		Title: "Select a file",
	})
	if err != nil {
		h.error(w, err.Error())
		return
	}
	h.success(w, respData{
		"file": filePath,
	})
}

func (h *HttpServer) openFolder(w http.ResponseWriter, r *http.Request) {
	var data struct {
		FilePath string `json:"filePath"`
	}
	err := json.NewDecoder(r.Body).Decode(&data)
	if err == nil && data.FilePath == "" {
		return
	}

	err = shared.OpenFolder(data.FilePath)
	if err != nil {
		globalLogger.Err(err)
		h.error(w, err.Error())
		return
	}
	h.success(w)
	return
}

func (h *HttpServer) install(w http.ResponseWriter, r *http.Request) {
	if appOnce.isInstall() {
		h.success(w, respData{
			"isPass": systemOnce.Password == "",
		})
		return
	}

	out, err := appOnce.installCert()
	if err != nil {
		h.error(w, err.Error()+"\n"+out, respData{
			"isPass": systemOnce.Password == "",
		})
		return
	}

	h.success(w, respData{
		"isPass": systemOnce.Password == "",
	})
}

func (h *HttpServer) setSystemPassword(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Password string `json:"password"`
		IsCache  bool   `json:"isCache"`
	}
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		h.error(w, err.Error())
		return
	}
	systemOnce.SetPassword(data.Password, data.IsCache)
	h.success(w)
}

func (h *HttpServer) openSystemProxy(w http.ResponseWriter, r *http.Request) {
	err := appOnce.OpenSystemProxy()
	if err != nil {
		h.error(w, err.Error(), respData{
			"value": appOnce.IsProxy,
		})
		return
	}
	h.success(w, respData{
		"value": appOnce.IsProxy,
	})
}

func (h *HttpServer) unsetSystemProxy(w http.ResponseWriter, r *http.Request) {
	err := appOnce.UnsetSystemProxy()
	if err != nil {
		h.error(w, err.Error(), respData{
			"value": appOnce.IsProxy,
		})
		return
	}
	h.success(w, respData{
		"value": appOnce.IsProxy,
	})
}

func (h *HttpServer) isProxy(w http.ResponseWriter, r *http.Request) {
	h.success(w, respData{
		"value": appOnce.IsProxy,
	})
}

func (h *HttpServer) appInfo(w http.ResponseWriter, r *http.Request) {
	h.success(w, appOnce)
}

func (h *HttpServer) getConfig(w http.ResponseWriter, r *http.Request) {
	h.success(w, globalConfig)
}

func (h *HttpServer) setConfig(w http.ResponseWriter, r *http.Request) {
	var data Config
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		h.error(w, err.Error())
		return
	}
	globalConfig.setConfig(data)
	h.success(w)
}

func (h *HttpServer) setType(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Type string `json:"type"`
	}
	err := json.NewDecoder(r.Body).Decode(&data)
	if err == nil {
		if data.Type != "" {
			resourceOnce.setResType(strings.Split(data.Type, ","))
		} else {
			resourceOnce.setResType([]string{})
		}
	}

	h.success(w)
}

func (h *HttpServer) clear(w http.ResponseWriter, r *http.Request) {
	resourceOnce.clear()
	h.success(w)
}

func (h *HttpServer) delete(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Sign []string `json:"sign"`
	}
	err := json.NewDecoder(r.Body).Decode(&data)
	if err == nil && len(data.Sign) > 0 {
		for _, v := range data.Sign {
			resourceOnce.delete(v)
		}
	}
	h.success(w)
}

func (h *HttpServer) download(w http.ResponseWriter, r *http.Request) {
	var data struct {
		shared.MediaInfo
		DecodeStr string `json:"decodeStr"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		h.error(w, err.Error())
		return
	}
	resourceOnce.download(data.MediaInfo, data.DecodeStr)
	h.success(w)
}

func (h *HttpServer) cancel(w http.ResponseWriter, r *http.Request) {
	var data struct {
		shared.MediaInfo
	}

	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		h.error(w, err.Error())
		return
	}

	err := resourceOnce.cancel(data.Id)
	if err != nil {
		h.error(w, err.Error())
		return
	}
	h.success(w)
}

func (h *HttpServer) wxFileDecode(w http.ResponseWriter, r *http.Request) {
	var data struct {
		shared.MediaInfo
		Filename  string `json:"filename"`
		DecodeStr string `json:"decodeStr"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		h.error(w, err.Error())
		return
	}
	savePath, err := resourceOnce.wxFileDecode(data.MediaInfo, data.Filename, data.DecodeStr)
	if err != nil {
		h.error(w, err.Error())
		return
	}
	h.success(w, respData{
		"save_path": savePath,
	})
}

func (h *HttpServer) batchExport(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		h.error(w, err.Error())
		return
	}
	fileName := filepath.Join(globalConfig.SaveDirectory, "res-downloader-"+shared.GetCurrentDateTimeFormatted()+".txt")
	err := os.WriteFile(fileName, []byte(data.Content), 0644)
	if err != nil {
		h.error(w, err.Error())
		return
	}

	_ = shared.OpenFolder(fileName)
	h.success(w, respData{
		"file_name": fileName,
	})
}
