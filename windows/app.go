package main

// 本文件承载 MetricsNode Windows 宿主中与 `app` 相关的逻辑。

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"

	"github.com/yttydcs/myflowhub-metricsnode/core/configstore"
	"github.com/yttydcs/myflowhub-metricsnode/core/runtime"
)

// App struct
type App struct {
	ctx context.Context

	mu   sync.Mutex
	log  *slog.Logger
	rt   *runtime.Runtime
	boot *configstore.Store

	notifyIconPath string
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{log: slog.Default()}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.rt != nil {
		return
	}

	// Store everything in a local config folder (per user confirmation).
	rt, err := runtime.New("config", a.log)
	if err != nil {
		a.log.Error("runtime init failed", "err", err.Error())
		return
	}
	a.rt = rt

	defaultDeviceID, err := generateDeviceID("win")
	if err != nil && a.log != nil {
		a.log.Warn("generate default device_id failed", "err", err.Error())
	}

	bootPath := filepath.Join(rt.WorkDir(), "bootstrap.json")
	boot, err := configstore.New(bootPath, map[string]string{
		"hub.addr":         "127.0.0.1:9000",
		"auth.device_id":   defaultDeviceID,
		keyNotifyPresenter: notifyPresenterScript,
	}, a.log)
	if err != nil {
		a.log.Error("bootstrap config init failed", "err", err.Error())
		return
	}

	// Ensure device_id is non-empty. This prevents Auth actions from failing due to missing input.
	if cur, _ := boot.Get("auth.device_id"); strings.TrimSpace(cur) == "" {
		id, err := generateDeviceID("win")
		if err != nil {
			a.log.Warn("generate device_id failed", "err", err.Error())
		} else {
			_ = boot.Set("auth.device_id", id)
		}
	}
	a.boot = boot

	notifyIconPath, err := ensureNotifyIcon(rt.WorkDir())
	if err != nil {
		a.log.Warn("notify icon init failed", "err", err.Error())
	} else {
		a.notifyIconPath = notifyIconPath
	}
}

func generateDeviceID(prefix string) (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	id := hex.EncodeToString(b)
	p := strings.TrimSpace(prefix)
	if p == "" {
		return id, nil
	}
	return p + "-" + id, nil
}
