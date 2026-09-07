package core

import (
	"os"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type Bind struct {
}

func NewBind() *Bind {
	return &Bind{}
}

func (b *Bind) Config() *ResponseData {
	return httpServerOnce.buildResp(1, "ok", globalConfig)
}

func (b *Bind) AppInfo() *ResponseData {
	return httpServerOnce.buildResp(1, "ok", appOnce)
}

func (b *Bind) ResetApp() {
	appOnce.IsReset = true
	runtime.Quit(appOnce.ctx)
}

func (b *Bind) FileExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func (b *Bind) GetFileSize(path string) int64 {
	if path == "" {
		return 0
	}
	fi, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return fi.Size()
}

