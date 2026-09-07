package core

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"net/url"
	"res-downloader/core/plugins"
	"res-downloader/core/shared"
	"strings"
	"time"

	"github.com/elazarl/goproxy"
	"golang.org/x/net/proxy"
)

type Proxy struct {
	ctx   context.Context
	Proxy *goproxy.ProxyHttpServer
	Is    bool
}

var pluginRegistry = make(map[string]shared.Plugin)

func init() {
	ps := []shared.Plugin{
		&plugins.QqPlugin{},
		&plugins.DouyinPlugin{},
		&plugins.BilibiliPlugin{},
		&plugins.KuaishouPlugin{},
		&plugins.DefaultPlugin{},
	}

	bridge := &shared.Bridge{
		GetVersion: func() string {
			return appOnce.Version
		},
		GetResType: func(key string) (bool, bool) {
			return resourceOnce.getResType(key)
		},
		TypeSuffix: func(mine string) (string, string) {
			return globalConfig.typeSuffix(mine)
		},
		MediaIsMarked: func(key string) bool {
			return resourceOnce.mediaIsMarked(key)
		},
		MarkMedia: func(key string) {
			resourceOnce.markMedia(key)
		},
		GetConfig: func(key string) interface{} {
			return globalConfig.getConfig(key)
		},
		Send: func(t string, data interface{}) {
			httpServerOnce.send(t, data)
		},
		IsProxy: func() bool {
			return appOnce != nil && appOnce.IsProxy
		},
		RecordHtmlTitle: func(resp *http.Response) {
			RecordHtmlTitle(resp)
		},
		GetTitle: func(req *http.Request) string {
			return GetTitleForMediaRequest(req)
		},
	}

	for _, p := range ps {
		p.SetBridge(bridge)
		for _, domain := range p.Domains() {
			pluginRegistry[domain] = p
		}
	}
}

func initProxy() *Proxy {
	if proxyOnce == nil {
		proxyOnce = &Proxy{}
		proxyOnce.Startup()
	}
	return proxyOnce
}

func (p *Proxy) Startup() {
	err := p.setCa()
	if err != nil {
		DialogErr("Failed to start proxy service：" + err.Error())
		return
	}

	p.Proxy = goproxy.NewProxyHttpServer()
	//p.Proxy.KeepDestinationHeaders = true
	//p.Proxy.Verbose = false
	p.setTransport()
	//p.Proxy.OnRequest().HandleConnect(goproxy.AlwaysMitm)
	p.Proxy.OnRequest().HandleConnectFunc(func(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
		if ruleOnce.shouldMitm(host) {
			return goproxy.MitmConnect, host
		}
		return goproxy.OkConnect, host
	})

	p.Proxy.OnRequest().DoFunc(p.httpRequestEvent)
	p.Proxy.OnResponse().DoFunc(p.httpResponseEvent)
}

func (p *Proxy) setCa() error {
	ca, err := tls.X509KeyPair(appOnce.PublicCrt, appOnce.PrivateKey)
	if err != nil {
		return err
	}
	if ca.Leaf, err = x509.ParseCertificate(ca.Certificate[0]); err != nil {
		return err
	}
	goproxy.GoproxyCa = ca
	goproxy.OkConnect = &goproxy.ConnectAction{Action: goproxy.ConnectAccept, TLSConfig: goproxy.TLSConfigFromCA(&ca)}
	goproxy.MitmConnect = &goproxy.ConnectAction{Action: goproxy.ConnectMitm, TLSConfig: goproxy.TLSConfigFromCA(&ca)}
	goproxy.HTTPMitmConnect = &goproxy.ConnectAction{Action: goproxy.ConnectHTTPMitm, TLSConfig: goproxy.TLSConfigFromCA(&ca)}
	goproxy.RejectConnect = &goproxy.ConnectAction{Action: goproxy.ConnectReject, TLSConfig: goproxy.TLSConfigFromCA(&ca)}
	return nil
}

func BuildUpstreamTransport() *http.Transport {
	upstream := strings.TrimSpace(globalConfig.UpstreamProxy)
	transport := &http.Transport{
		DisableKeepAlives: false,
		DialContext: (&net.Dialer{
			Timeout: 60 * time.Second,
		}).DialContext,
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
		TLSHandshakeTimeout:   60 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second,
		IdleConnTimeout:       30 * time.Second,
	}

	if upstream != "" && globalConfig.DownloadProxy && !strings.Contains(upstream, globalConfig.Port) {
		proxyURL, err := url.Parse(upstream)
		if err == nil {
			transport.Proxy = http.ProxyURL(proxyURL)
			if proxyURL.Scheme == "socks5" || proxyURL.Scheme == "socks5h" {
				dialer, dialErr := proxy.FromURL(proxyURL, proxy.Direct)
				if dialErr == nil {
					transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
						return dialer.Dial(network, addr)
					}
				}
			}
		}
	}
	return transport
}

func (p *Proxy) setTransport() {
	upstream := strings.TrimSpace(globalConfig.UpstreamProxy)
	transport := &http.Transport{
		DisableKeepAlives: false,
		// MaxIdleConnsPerHost: 10,
		DialContext: (&net.Dialer{
			Timeout: 60 * time.Second,
		}).DialContext,
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
		TLSHandshakeTimeout:   60 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second,
		IdleConnTimeout:       30 * time.Second,
	}

	p.Proxy.ConnectDial = nil
	p.Proxy.ConnectDialWithReq = nil

	if upstream != "" && globalConfig.OpenProxy && !strings.Contains(upstream, globalConfig.Port) {
		proxyURL, err := url.Parse(upstream)
		if err == nil {
			transport.Proxy = http.ProxyURL(proxyURL)
			if proxyURL.Scheme == "socks5" || proxyURL.Scheme == "socks5h" {
				dialer, dialErr := proxy.FromURL(proxyURL, proxy.Direct)
				if dialErr == nil {
					transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
						return dialer.Dial(network, addr)
					}
					p.Proxy.ConnectDial = func(network, addr string) (net.Conn, error) {
						return dialer.Dial(network, addr)
					}
				}
			} else {
				p.Proxy.ConnectDial = p.Proxy.NewConnectDialToProxy(upstream)
			}
		}
	}
	p.Proxy.Tr = transport
}

func (p *Proxy) matchPlugin(host string) shared.Plugin {
	domain := shared.GetTopLevelDomain(host)
	if plugin, ok := pluginRegistry[domain]; ok {
		return plugin
	}
	return nil
}

func (p *Proxy) httpRequestEvent(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
	plugin := p.matchPlugin(r.Host)
	if plugin != nil {
		newReq, newResp := plugin.OnRequest(r, ctx)
		if newResp != nil {
			return newReq, newResp
		}

		if newReq != nil {
			return newReq, nil
		}
	}
	return pluginRegistry["default"].OnRequest(r, ctx)
}

func (p *Proxy) httpResponseEvent(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
	if resp == nil || resp.Request == nil {
		return resp
	}

	plugin := p.matchPlugin(resp.Request.Host)
	if plugin != nil {
		newResp := plugin.OnResponse(resp, ctx)
		if newResp != nil {
			return newResp
		}
	}

	return pluginRegistry["default"].OnResponse(resp, ctx)
}
