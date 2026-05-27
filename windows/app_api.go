package main

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/yttydcs/myflowhub-metricsnode/core/notify"
	"github.com/yttydcs/myflowhub-metricsnode/core/runtime"
)

type BootstrapDTO struct {
	Addr     string `json:"addr"`
	DeviceID string `json:"device_id"`
}

type StatusDTO struct {
	WorkDir   string               `json:"work_dir"`
	Connected bool                 `json:"connected"`
	Addr      string               `json:"addr"`
	Reporting bool                 `json:"reporting"`
	Notify    bool                 `json:"notify"`
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
		Notify:    rt.IsNotifyRunning(),
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

func (a *App) NotifySettingsGet() ([]notify.TopicSetting, error) {
	a.mu.Lock()
	rt := a.rt
	a.mu.Unlock()
	if rt == nil {
		return nil, errors.New("runtime not initialized")
	}
	return rt.NotifySettingsGet(), nil
}

func (a *App) NotifySettingsSet(settings []notify.TopicSetting) error {
	a.mu.Lock()
	rt := a.rt
	a.mu.Unlock()
	if rt == nil {
		return errors.New("runtime not initialized")
	}
	return rt.NotifySettingsSet(settings)
}

func (a *App) StartNotify() error {
	a.mu.Lock()
	rt := a.rt
	a.mu.Unlock()
	if rt == nil {
		return errors.New("runtime not initialized")
	}
	return rt.StartNotify()
}

func (a *App) StopNotify() {
	a.mu.Lock()
	rt := a.rt
	a.mu.Unlock()
	if rt != nil {
		rt.StopNotify()
	}
}

func (a *App) DequeueNotifications() ([]notify.Event, error) {
	a.mu.Lock()
	rt := a.rt
	a.mu.Unlock()
	if rt == nil {
		return nil, errors.New("runtime not initialized")
	}
	return rt.DequeueNotifications(), nil
}
