package main

// Context: This file belongs to the MetricsNode application layer around app.

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
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
		"hub.addr":       "127.0.0.1:9000",
		"auth.device_id": defaultDeviceID,
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

type BootstrapDTO struct {
	Addr     string `json:"addr"`
	DeviceID string `json:"device_id"`
}

type StatusDTO struct {
	WorkDir   string               `json:"work_dir"`
	Connected bool                 `json:"connected"`
	Addr      string               `json:"addr"`
	Reporting bool                 `json:"reporting"`
	Auth      runtime.AuthSnapshot `json:"auth"`
	Metrics   map[string]string    `json:"metrics"`
	LastError string               `json:"last_error"`
}

func (a *App) Status() StatusDTO {
	a.mu.Lock()
	rt := a.rt
	boot := a.boot
	a.mu.Unlock()

	if rt == nil {
		return StatusDTO{LastError: "runtime not initialized"}
	}

	addr := rt.LastAddr()
	if strings.TrimSpace(addr) == "" && boot != nil {
		if v, ok := boot.Get("hub.addr"); ok {
			addr = v
		}
	}

	return StatusDTO{
		WorkDir:   rt.WorkDir(),
		Connected: rt.IsConnected(),
		Addr:      strings.TrimSpace(addr),
		Reporting: rt.IsReporting(),
		Auth:      rt.AuthState(),
		Metrics:   rt.MetricsSnapshot(),
		LastError: rt.LastError(),
	}
}

func (a *App) BootstrapGet() BootstrapDTO {
	a.mu.Lock()
	boot := a.boot
	a.mu.Unlock()
	if boot == nil {
		return BootstrapDTO{}
	}
	addr, _ := boot.Get("hub.addr")
	deviceID, _ := boot.Get("auth.device_id")
	return BootstrapDTO{Addr: addr, DeviceID: deviceID}
}

func (a *App) BootstrapSet(input BootstrapDTO) error {
	a.mu.Lock()
	boot := a.boot
	a.mu.Unlock()
	if boot == nil {
		return errors.New("bootstrap config not initialized")
	}
	if strings.TrimSpace(input.Addr) != "" {
		if err := boot.Set("hub.addr", strings.TrimSpace(input.Addr)); err != nil {
			return err
		}
	}
	if strings.TrimSpace(input.DeviceID) != "" {
		if err := boot.Set("auth.device_id", strings.TrimSpace(input.DeviceID)); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) Connect(addr string) error {
	a.mu.Lock()
	rt := a.rt
	boot := a.boot
	a.mu.Unlock()
	if rt == nil {
		return errors.New("runtime not initialized")
	}
	addr = strings.TrimSpace(addr)
	if addr == "" && boot != nil {
		if v, ok := boot.Get("hub.addr"); ok {
			addr = strings.TrimSpace(v)
		}
	}
	if addr == "" {
		return errors.New("addr is required")
	}
	if err := rt.Connect(addr); err != nil {
		return err
	}
	if boot != nil {
		_ = boot.Set("hub.addr", addr)
	}
	return nil
}

func (a *App) Disconnect() {
	a.mu.Lock()
	rt := a.rt
	a.mu.Unlock()
	if rt != nil {
		rt.Close()
	}
}

func (a *App) EnsureKeys() (string, error) {
	a.mu.Lock()
	rt := a.rt
	a.mu.Unlock()
	if rt == nil {
		return "", errors.New("runtime not initialized")
	}
	return rt.EnsureKeys()
}

func (a *App) ClearAuth() error {
	a.mu.Lock()
	rt := a.rt
	a.mu.Unlock()
	if rt == nil {
		return errors.New("runtime not initialized")
	}
	return rt.ClearAuth()
}

func (a *App) Register(deviceID string) (runtime.AuthSnapshot, error) {
	a.mu.Lock()
	rt := a.rt
	boot := a.boot
	a.mu.Unlock()
	if rt == nil {
		return runtime.AuthSnapshot{}, errors.New("runtime not initialized")
	}
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" && boot != nil {
		if v, ok := boot.Get("auth.device_id"); ok {
			deviceID = strings.TrimSpace(v)
		}
	}
	if deviceID == "" {
		return runtime.AuthSnapshot{}, errors.New("device_id is required")
	}
	if _, err := rt.Register(deviceID); err != nil {
		return runtime.AuthSnapshot{}, err
	}
	if boot != nil {
		_ = boot.Set("auth.device_id", deviceID)
	}
	return rt.AuthState(), nil
}

func (a *App) Login(deviceID string, nodeID uint32) (runtime.AuthSnapshot, error) {
	a.mu.Lock()
	rt := a.rt
	boot := a.boot
	a.mu.Unlock()
	if rt == nil {
		return runtime.AuthSnapshot{}, errors.New("runtime not initialized")
	}
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" && boot != nil {
		if v, ok := boot.Get("auth.device_id"); ok {
			deviceID = strings.TrimSpace(v)
		}
	}
	if deviceID == "" {
		return runtime.AuthSnapshot{}, errors.New("device_id is required")
	}
	if nodeID == 0 {
		if st := rt.AuthState(); st.NodeID != 0 {
			nodeID = st.NodeID
		}
	}
	if nodeID == 0 {
		return runtime.AuthSnapshot{}, errors.New("node_id is required")
	}
	if _, err := rt.Login(deviceID, nodeID); err != nil {
		return runtime.AuthSnapshot{}, err
	}
	if boot != nil {
		_ = boot.Set("auth.device_id", deviceID)
	}
	return rt.AuthState(), nil
}

func (a *App) StartReporting() error {
	a.mu.Lock()
	rt := a.rt
	a.mu.Unlock()
	if rt == nil {
		return errors.New("runtime not initialized")
	}
	return rt.StartReporting()
}

func (a *App) StopReporting() {
	a.mu.Lock()
	rt := a.rt
	a.mu.Unlock()
	if rt != nil {
		rt.StopReporting()
	}
}

func (a *App) MetricsSettingsGet() ([]runtime.MetricSetting, error) {
	a.mu.Lock()
	rt := a.rt
	a.mu.Unlock()
	if rt == nil {
		return nil, errors.New("runtime not initialized")
	}
	raw, ok := rt.RuntimeConfigGet(runtime.KeyMetricsSettingsJSON)
	if !ok || strings.TrimSpace(raw) == "" {
		return []runtime.MetricSetting{}, nil
	}
	var out []runtime.MetricSetting
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (a *App) MetricsSettingsSet(settings []runtime.MetricSetting) error {
	a.mu.Lock()
	rt := a.rt
	a.mu.Unlock()
	if rt == nil {
		return errors.New("runtime not initialized")
	}
	encoded, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	return rt.RuntimeConfigSet(runtime.KeyMetricsSettingsJSON, string(encoded), 0)
}
