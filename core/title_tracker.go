package core

import (
	"bytes"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"res-downloader/core/shared"
	"strings"
	"sync"
	"time"
)

var (
	titleTagRegex = regexp.MustCompile(`(?i)<title[^>]*>([^<]+)</title>`)
	ogTitleTag    = regexp.MustCompile(`(?i)<meta[^>]+(?:property|name)=["']og:title["'][^>]+content=["']([^"']+)["']`)
	ogTitleTag2   = regexp.MustCompile(`(?i)<meta[^>]+content=["']([^"']+)["'][^>]+(?:property|name)=["']og:title["']`)
	h1TagRegex    = regexp.MustCompile(`(?i)<h1[^>]*>([^<]+)</h1>`)

	pageTitleLock sync.RWMutex
	pageTitleMap  = make(map[string]string)

	lastTitleMux      sync.RWMutex
	lastSeenTitle     string
	lastSeenHost      string
	lastSeenTopDomain string
	lastSeenURL       string
	lastSeenTime      time.Time
)

// CleanArticleTitle 清洗网页标题，去掉站点后缀及特殊空白字符
func CleanArticleTitle(raw string) string {
	t := html.UnescapeString(raw)
	t = strings.ReplaceAll(t, "\r", " ")
	t = strings.ReplaceAll(t, "\n", " ")
	t = strings.ReplaceAll(t, "\t", " ")
	for strings.Contains(t, "  ") {
		t = strings.ReplaceAll(t, "  ", " ")
	}
	t = strings.TrimSpace(t)

	// 过滤站点常见尾缀（保留文章真实核心标题）
	siteSuffixes := []string{
		" - 51吃瓜",
		"_51吃瓜",
		"- 51吃瓜",
		"- 51cg",
		"_51cg",
		" - 海角社区",
		"_海角社区",
		" - 每日吃瓜",
		"_每日吃瓜",
	}
	for _, s := range siteSuffixes {
		if idx := strings.Index(strings.ToLower(t), strings.ToLower(s)); idx > 0 {
			t = strings.TrimSpace(t[:idx])
			break
		}
	}

	return t
}

// RecordHtmlTitle 从代理流经的 HTML 页面响应中解析出网页标题并缓存
func RecordHtmlTitle(resp *http.Response) {
	if resp == nil || resp.Request == nil || resp.Body == nil {
		return
	}

	// 仅读取前 128KB 即可提取 <title> 和 <meta>，无需占用大量内存
	buf := make([]byte, 131072)
	n, err := io.ReadFull(resp.Body, buf)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return
	}
	readBytes := buf[:n]

	// 使用 MultiReader 无损恢复 resp.Body，下游浏览器正常接收完整网页
	resp.Body = io.NopCloser(io.MultiReader(bytes.NewReader(readBytes), resp.Body))

	var rawTitle string
	if m := ogTitleTag.FindSubmatch(readBytes); len(m) > 1 {
		rawTitle = string(m[1])
	} else if m := ogTitleTag2.FindSubmatch(readBytes); len(m) > 1 {
		rawTitle = string(m[1])
	} else if m := titleTagRegex.FindSubmatch(readBytes); len(m) > 1 {
		rawTitle = string(m[1])
	} else if m := h1TagRegex.FindSubmatch(readBytes); len(m) > 1 {
		rawTitle = string(m[1])
	}

	title := CleanArticleTitle(rawTitle)
	if title == "" || title == "404" || title == "Not Found" || strings.Contains(title, "Attention Required") {
		return
	}

	req := resp.Request
	fullURL := req.URL.String()
	host := strings.ToLower(req.Host)
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	pathURL := req.URL.Scheme + "://" + host + req.URL.Path
	topDomain := shared.GetTopLevelDomain(host)

	pageTitleLock.Lock()
	if len(pageTitleMap) > 1000 {
		pageTitleMap = make(map[string]string)
	}

	pageTitleMap[fullURL] = title
	pageTitleMap[strings.TrimRight(fullURL, "/")] = title
	pageTitleMap[pathURL] = title
	pageTitleMap[strings.TrimRight(pathURL, "/")] = title
	// 作用域限定在 host 域名下的路径，绝不存全局裸路径，避免不同站点同名路径（如 / 或 /index.html）相互干扰
	pageTitleMap[host+req.URL.Path] = title
	pageTitleMap[strings.TrimRight(host+req.URL.Path, "/")] = title
	pageTitleMap[host] = title
	if topDomain != "" {
		pageTitleMap[topDomain] = title
	}
	pageTitleLock.Unlock()

	lastTitleMux.Lock()
	lastSeenTitle = title
	lastSeenHost = host
	lastSeenTopDomain = topDomain
	lastSeenURL = fullURL
	lastSeenTime = time.Now()
	lastTitleMux.Unlock()
}

// GetTitleForMediaRequest 根据嗅探到的音视频或图片请求关联最匹配的网页文章标题
func GetTitleForMediaRequest(req *http.Request) string {
	if req == nil {
		return ""
	}

	reqHost := strings.ToLower(req.Host)
	if h, _, err := net.SplitHostPort(reqHost); err == nil {
		reqHost = h
	}
	reqTopDomain := shared.GetTopLevelDomain(reqHost)

	referer := req.Header.Get("Referer")
	var refHost, refTopDomain string

	if referer != "" {
		if u, err := url.Parse(referer); err == nil && u.Host != "" {
			refHost = strings.ToLower(u.Host)
			if h, _, err := net.SplitHostPort(refHost); err == nil {
				refHost = h
			}
			refTopDomain = shared.GetTopLevelDomain(refHost)
		}

		pageTitleLock.RLock()
		if t, ok := pageTitleMap[referer]; ok && t != "" {
			pageTitleLock.RUnlock()
			return t
		}
		if t, ok := pageTitleMap[strings.TrimRight(referer, "/")]; ok && t != "" {
			pageTitleLock.RUnlock()
			return t
		}
		if u, err := url.Parse(referer); err == nil {
			pathURL := u.Scheme + "://" + refHost + u.Path
			if t, ok := pageTitleMap[pathURL]; ok && t != "" {
				pageTitleLock.RUnlock()
				return t
			}
			if t, ok := pageTitleMap[strings.TrimRight(pathURL, "/")]; ok && t != "" {
				pageTitleLock.RUnlock()
				return t
			}
			hostPath := refHost + u.Path
			if t, ok := pageTitleMap[hostPath]; ok && t != "" {
				pageTitleLock.RUnlock()
				return t
			}
			if t, ok := pageTitleMap[strings.TrimRight(hostPath, "/")]; ok && t != "" {
				pageTitleLock.RUnlock()
				return t
			}
			if t, ok := pageTitleMap[refHost]; ok && t != "" {
				pageTitleLock.RUnlock()
				return t
			}
			if refTopDomain != "" {
				if t, ok := pageTitleMap[refTopDomain]; ok && t != "" {
					pageTitleLock.RUnlock()
					return t
				}
			}
		}
		pageTitleLock.RUnlock()
	}

	// 严格防“串台”回退逻辑：
	// 仅当请求来源（Referer）主域名或媒体服务主域名与最近访问的 HTML 网页在同一主域名下时，才允许继承标题
	lastTitleMux.RLock()
	defer lastTitleMux.RUnlock()
	if lastSeenTitle != "" && time.Since(lastSeenTime) < 3*time.Minute {
		isSameSite := false
		if refTopDomain != "" && lastSeenTopDomain != "" && refTopDomain == lastSeenTopDomain {
			isSameSite = true
		} else if refHost != "" && lastSeenHost != "" && refHost == lastSeenHost {
			isSameSite = true
		} else if reqTopDomain != "" && lastSeenTopDomain != "" && reqTopDomain == lastSeenTopDomain {
			isSameSite = true
		} else if reqHost != "" && lastSeenHost != "" && reqHost == lastSeenHost {
			isSameSite = true
		}

		if isSameSite {
			return lastSeenTitle
		}
	}

	return ""
}

// GetLastSeenPageURL 获取最近 10 分钟内流经代理的网页文章 URL
func GetLastSeenPageURL() string {
	lastTitleMux.RLock()
	defer lastTitleMux.RUnlock()
	if lastSeenURL != "" && time.Since(lastSeenTime) < 10*time.Minute {
		return lastSeenURL
	}
	return ""
}
