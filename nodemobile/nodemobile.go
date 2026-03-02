package nodemobile

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"

	"github.com/yttydcs/myflowhub-metricsnode/core/metrics"
	"github.com/yttydcs/myflowhub-metricsnode/core/runtime"
)

var (
	mu      sync.Mutex
	rt      *runtime.Runtime
	lastErr string
)

type statusDTO struct {
	Running   bool                 `json:"running"`
	Connected bool                 `json:"connected"`
	Addr      string               `json:"addr,omitempty"`
	WorkDir   string               `json:"workdir,omitempty"`
	Auth      runtime.AuthSnapshot `json:"auth,omitempty"`
	Reporting bool                 `json:"reporting"`
	Metrics   map[string]string    `json:"metrics,omitempty"`
	LastError string               `json:"last_error,omitempty"`
}

func Start(addr, deviceID, workDir string) (string, error) {
	mu.Lock()
	defer mu.Unlock()

	addr = strings.TrimSpace(addr)
	deviceID = strings.TrimSpace(deviceID)
	workDir = strings.TrimSpace(workDir)
	if addr == "" {
		return "", errors.New("addr is required")
	}
	if deviceID == "" {
		return "", errors.New("device_id is required")
	}
	if workDir == "" {
		return "", errors.New("workDir is required")
	}

	if rt == nil {
		r, err := runtime.New(workDir, slog.Default())
		if err != nil {
			lastErr = err.Error()
			return "", err
		}
		rt = r
	}

	if err := rt.Connect(addr); err != nil {
		lastErr = err.Error()
		return "", err
	}

	if _, err := rt.EnsureKeys(); err != nil {
		lastErr = err.Error()
		return "", err
	}

	auth := rt.AuthState()
	if auth.NodeID != 0 {
		if _, err := rt.Login(deviceID, auth.NodeID); err != nil {
			lastErr = err.Error()
			return "", err
		}
	} else {
		if _, err := rt.Register(deviceID); err != nil {
			lastErr = err.Error()
			return "", err
		}
	}

	if err := rt.StartReporting(); err != nil {
		lastErr = err.Error()
		return "", err
	}

	return marshalStatus(rt), nil
}

func Stop() (string, error) {
	mu.Lock()
	r := rt
	rt = nil
	mu.Unlock()

	if r == nil {
		return marshalStatus(nil), nil
	}
	r.StopReporting()
	r.Close()
	return marshalStatus(nil), nil
}

func Status() string {
	mu.Lock()
	r := rt
	errText := lastErr
	mu.Unlock()

	dto := statusDTO{
		Running:   r != nil,
		Connected: r != nil && r.IsConnected(),
		Addr:      "",
		WorkDir:   "",
		Auth:      runtime.AuthSnapshot{},
		Reporting: r != nil && r.IsReporting(),
		Metrics:   map[string]string{},
		LastError: strings.TrimSpace(errText),
	}
	if r != nil {
		dto.Addr = r.LastAddr()
		dto.WorkDir = r.WorkDir()
		dto.Auth = r.AuthState()
		dto.Metrics = r.MetricsSnapshot()
		dto.LastError = strings.TrimSpace(r.LastError())
	}
	raw, _ := json.Marshal(dto)
	return string(raw)
}

func UpdateBatteryPercent(percent string) {
	mu.Lock()
	r := rt
	mu.Unlock()
	if r == nil {
		return
	}
	p := strings.TrimSpace(percent)
	if p == "" {
		return
	}
	n, err := strconv.Atoi(p)
	if err != nil {
		return
	}
	if n < 0 {
		r.UpdateMetric(metrics.MetricBatteryPercent, "-1")
		return
	}
	if n > 100 {
		n = 100
	}
	r.UpdateMetric(metrics.MetricBatteryPercent, fmt.Sprintf("%d", n))
}

func UpdateVolumePercent(percent string) {
	mu.Lock()
	r := rt
	mu.Unlock()
	if r == nil {
		return
	}
	p := strings.TrimSpace(percent)
	if p == "" {
		return
	}
	n, err := strconv.Atoi(p)
	if err != nil {
		return
	}
	if n < 0 {
		n = 0
	}
	if n > 100 {
		n = 100
	}
	r.UpdateMetric(metrics.MetricVolumePercent, fmt.Sprintf("%d", n))
}

func UpdateVolumeMuted(muted string) {
	mu.Lock()
	r := rt
	mu.Unlock()
	if r == nil {
		return
	}
	m := strings.ToLower(strings.TrimSpace(muted))
	switch m {
	case "1", "true", "yes", "y", "on":
		r.UpdateMetric(metrics.MetricVolumeMuted, "1")
	default:
		r.UpdateMetric(metrics.MetricVolumeMuted, "0")
	}
}

func UpdateBrightnessPercent(percent string) {
	mu.Lock()
	r := rt
	mu.Unlock()
	if r == nil {
		return
	}
	p := strings.TrimSpace(percent)
	if p == "" {
		return
	}
	n, err := strconv.Atoi(p)
	if err != nil {
		return
	}
	if n < 0 {
		r.UpdateMetric(metrics.MetricBrightnessPercent, "-1")
		return
	}
	if n > 100 {
		n = 100
	}
	r.UpdateMetric(metrics.MetricBrightnessPercent, fmt.Sprintf("%d", n))
}

func GetLastError() string {
	mu.Lock()
	defer mu.Unlock()
	return strings.TrimSpace(lastErr)
}

func DequeueActions() string {
	mu.Lock()
	r := rt
	mu.Unlock()

	if r == nil {
		return "[]"
	}
	actions := r.DequeueActions()
	raw, _ := json.Marshal(actions)
	return string(raw)
}

// EnsureLinked is a no-op function to make it obvious in Java/Kotlin that the AAR is present.
func EnsureLinked() error {
	mu.Lock()
	defer mu.Unlock()
	if rt == nil {
		err := errors.New("runtime not started")
		lastErr = err.Error()
		return err
	}
	return nil
}

func marshalStatus(r *runtime.Runtime) string {
	if r == nil {
		raw, _ := json.Marshal(statusDTO{Running: false})
		return string(raw)
	}
	dto := statusDTO{
		Running:   true,
		Connected: r.IsConnected(),
		Addr:      r.LastAddr(),
		WorkDir:   r.WorkDir(),
		Auth:      r.AuthState(),
		Reporting: r.IsReporting(),
		Metrics:   r.MetricsSnapshot(),
		LastError: strings.TrimSpace(r.LastError()),
	}
	raw, _ := json.Marshal(dto)
	return string(raw)
}
