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
	rtDir   string
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

	if _, err := Init(workDir); err != nil {
		return "", err
	}
	if _, err := Connect(addr); err != nil {
		return "", err
	}
	if _, err := EnsureKeys(); err != nil {
		return "", err
	}
	auth := StatusAuthSnapshot()
	if auth.NodeID != 0 {
		if _, err := Login(deviceID, int64(auth.NodeID)); err != nil {
			return "", err
		}
	} else {
		if _, err := Register(deviceID); err != nil {
			return "", err
		}
	}
	return StartReporting()
}

func Stop() (string, error) {
	mu.Lock()
	r := rt
	rt = nil
	rtDir = ""
	mu.Unlock()

	if r == nil {
		return marshalStatus(nil), nil
	}
	r.StopReporting()
	r.Close()
	return marshalStatus(nil), nil
}

func Init(workDir string) (string, error) {
	workDir = strings.TrimSpace(workDir)
	if workDir == "" {
		return "", errors.New("workDir is required")
	}

	mu.Lock()
	r := rt
	mu.Unlock()

	if r == nil {
		created, err := runtime.New(workDir, slog.Default())
		if err != nil {
			mu.Lock()
			lastErr = err.Error()
			mu.Unlock()
			return "", err
		}
		mu.Lock()
		rt = created
		rtDir = workDir
		r = created
		mu.Unlock()
	} else {
		mu.Lock()
		rtDir = workDir
		mu.Unlock()
	}

	return marshalStatus(r), nil
}

func Connect(addr string) (string, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return "", errors.New("addr is required")
	}
	mu.Lock()
	r := rt
	mu.Unlock()
	if r == nil {
		return "", errors.New("runtime not started")
	}
	if err := r.Connect(addr); err != nil {
		mu.Lock()
		lastErr = err.Error()
		mu.Unlock()
		return "", err
	}
	return marshalStatus(r), nil
}

func Disconnect() (string, error) {
	mu.Lock()
	r := rt
	mu.Unlock()
	if r == nil {
		return marshalStatus(nil), nil
	}
	r.Close()
	return marshalStatus(r), nil
}

func EnsureKeys() (string, error) {
	mu.Lock()
	r := rt
	mu.Unlock()
	if r == nil {
		return "", errors.New("runtime not started")
	}
	if _, err := r.EnsureKeys(); err != nil {
		mu.Lock()
		lastErr = err.Error()
		mu.Unlock()
		return "", err
	}
	return marshalStatus(r), nil
}

func Register(deviceID string) (string, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return "", errors.New("device_id is required")
	}
	mu.Lock()
	r := rt
	mu.Unlock()
	if r == nil {
		return "", errors.New("runtime not started")
	}
	if _, err := r.EnsureKeys(); err != nil {
		mu.Lock()
		lastErr = err.Error()
		mu.Unlock()
		return "", err
	}
	if _, err := r.Register(deviceID); err != nil {
		mu.Lock()
		lastErr = err.Error()
		mu.Unlock()
		return "", err
	}
	return marshalStatus(r), nil
}

func Login(deviceID string, nodeID int64) (string, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return "", errors.New("device_id is required")
	}
	mu.Lock()
	r := rt
	mu.Unlock()
	if r == nil {
		return "", errors.New("runtime not started")
	}
	if _, err := r.EnsureKeys(); err != nil {
		mu.Lock()
		lastErr = err.Error()
		mu.Unlock()
		return "", err
	}
	if nodeID < 0 {
		nodeID = 0
	}
	if nodeID == 0 {
		if st := r.AuthState(); st.NodeID != 0 {
			nodeID = int64(st.NodeID)
		}
	}
	if nodeID == 0 {
		return "", errors.New("node_id is required")
	}
	const maxUint32 = int64(^uint32(0))
	if nodeID > maxUint32 {
		return "", fmt.Errorf("node_id out of range: %d", nodeID)
	}

	if _, err := r.Login(deviceID, uint32(nodeID)); err != nil {
		mu.Lock()
		lastErr = err.Error()
		mu.Unlock()
		return "", err
	}
	return marshalStatus(r), nil
}

func StartReporting() (string, error) {
	mu.Lock()
	r := rt
	mu.Unlock()
	if r == nil {
		return "", errors.New("runtime not started")
	}
	if err := r.StartReporting(); err != nil {
		mu.Lock()
		lastErr = err.Error()
		mu.Unlock()
		return "", err
	}
	return marshalStatus(r), nil
}

func StopReporting() (string, error) {
	mu.Lock()
	r := rt
	mu.Unlock()
	if r == nil {
		return marshalStatus(nil), nil
	}
	r.StopReporting()
	return marshalStatus(r), nil
}

func StopAll() (string, error) {
	return Stop()
}

func RuntimeConfigGet(key string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", errors.New("key is required")
	}
	mu.Lock()
	r := rt
	mu.Unlock()
	if r == nil {
		return "", errors.New("runtime not started")
	}
	if v, ok := r.RuntimeConfigGet(key); ok {
		return v, nil
	}
	return "", errors.New("key not found")
}

func RuntimeConfigSet(key, value string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", errors.New("key is required")
	}
	mu.Lock()
	r := rt
	mu.Unlock()
	if r == nil {
		return "", errors.New("runtime not started")
	}
	if err := r.RuntimeConfigSet(key, value, 0); err != nil {
		mu.Lock()
		lastErr = err.Error()
		mu.Unlock()
		return "", err
	}
	return marshalStatus(r), nil
}

func MetricsSettingsGet() (string, error) {
	return RuntimeConfigGet(runtime.KeyMetricsSettingsJSON)
}

func MetricsSettingsSet(value string) (string, error) {
	return RuntimeConfigSet(runtime.KeyMetricsSettingsJSON, value)
}

func StatusAuthSnapshot() runtime.AuthSnapshot {
	mu.Lock()
	r := rt
	mu.Unlock()
	if r == nil {
		return runtime.AuthSnapshot{}
	}
	return r.AuthState()
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

func UpdateBatteryCharging(charging string) {
	mu.Lock()
	r := rt
	mu.Unlock()
	if r == nil {
		return
	}
	v := strings.TrimSpace(charging)
	if v == "" {
		return
	}
	if v == "-1" {
		r.UpdateMetric(metrics.MetricBatteryCharging, "-1")
		return
	}
	if isTruthy(v) {
		r.UpdateMetric(metrics.MetricBatteryCharging, "1")
		return
	}
	r.UpdateMetric(metrics.MetricBatteryCharging, "0")
}

func UpdateBatteryOnAC(onAC string) {
	mu.Lock()
	r := rt
	mu.Unlock()
	if r == nil {
		return
	}
	v := strings.TrimSpace(onAC)
	if v == "" {
		return
	}
	if v == "-1" {
		r.UpdateMetric(metrics.MetricBatteryOnAC, "-1")
		return
	}
	if isTruthy(v) {
		r.UpdateMetric(metrics.MetricBatteryOnAC, "1")
		return
	}
	r.UpdateMetric(metrics.MetricBatteryOnAC, "0")
}

func UpdateNetOnline(online string) {
	mu.Lock()
	r := rt
	mu.Unlock()
	if r == nil {
		return
	}
	v := strings.TrimSpace(online)
	if v == "" {
		return
	}
	if v == "-1" {
		r.UpdateMetric(metrics.MetricNetOnline, "-1")
		return
	}
	if isTruthy(v) {
		r.UpdateMetric(metrics.MetricNetOnline, "1")
		return
	}
	r.UpdateMetric(metrics.MetricNetOnline, "0")
}

func UpdateNetType(netType string) {
	mu.Lock()
	r := rt
	mu.Unlock()
	if r == nil {
		return
	}
	v := strings.ToLower(strings.TrimSpace(netType))
	if v == "" {
		return
	}
	if v == "-1" {
		r.UpdateMetric(metrics.MetricNetType, "-1")
		return
	}
	switch v {
	case "none", "wifi", "ethernet", "cellular", "unknown":
		// ok
	default:
		v = "unknown"
	}
	r.UpdateMetric(metrics.MetricNetType, v)
}

func UpdateCPUPercent(percent string) {
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
		r.UpdateMetric(metrics.MetricCPUPercent, "-1")
		return
	}
	if n > 100 {
		n = 100
	}
	r.UpdateMetric(metrics.MetricCPUPercent, fmt.Sprintf("%d", n))
}

func UpdateMemPercent(percent string) {
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
		r.UpdateMetric(metrics.MetricMemPercent, "-1")
		return
	}
	if n > 100 {
		n = 100
	}
	r.UpdateMetric(metrics.MetricMemPercent, fmt.Sprintf("%d", n))
}

func UpdateFlashlightEnabled(enabled string) {
	mu.Lock()
	r := rt
	mu.Unlock()
	if r == nil {
		return
	}
	v := strings.TrimSpace(enabled)
	if v == "" {
		return
	}
	if v == "-1" {
		r.UpdateMetric(metrics.MetricFlashlightEnabled, "-1")
		return
	}
	if isTruthy(v) {
		r.UpdateMetric(metrics.MetricFlashlightEnabled, "1")
		return
	}
	r.UpdateMetric(metrics.MetricFlashlightEnabled, "0")
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

func isTruthy(text string) bool {
	v := strings.ToLower(strings.TrimSpace(text))
	switch v {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
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
