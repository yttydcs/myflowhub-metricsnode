package nodemobile

// 本文件承载 MetricsNode `nodemobile` 桥接中与 `nodemobile` 相关的逻辑。

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

// Start 按移动端宿主期望串起 init、connect、key 准备与 register/login，再启动上报。
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

// Stop 完整关闭 runtime 与上报循环，并清空进程内保存的 bridge 状态。
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

// Init 确保单例 runtime 已创建，并把最新 workDir 记录到 bridge 全局状态里。
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

// Connect 让当前 runtime 连接远端 Hub；runtime 尚未初始化时直接返回显式错误。
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

// Disconnect 主动断开当前 runtime 的连接，但保留 runtime 结构供后续重连复用。
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

// EnsureKeys 确保本地节点密钥存在，供后续 register/login 复用同一身份。
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

// Register 在移动端侧执行首次注册；这里会先补齐密钥，避免发出无签名身份请求。
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

// Login 复用已保存或显式传入的 node_id 进行登录，并拦截超出 uint32 范围的输入。
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

// StartReporting 打开平台指标上报循环，让 MetricsNode 开始持续推送采集值。
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

// StopReporting 只停止指标上报，不销毁 runtime 本身，便于 UI 单独控制采集状态。
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

// StopAll 保留给宿主侧的兼容入口，当前直接复用完整 Stop 流程。
func StopAll() (string, error) {
	return Stop()
}

// RuntimeConfigGet 读取 runtime 的单个配置键，找不到时返回显式 not found。
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

// RuntimeConfigSet 写入 runtime 配置，并把失败原因同步到 bridge 的 lastErr。
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

// MetricsSettingsGet 是 metrics settings JSON 的便捷读取入口。
func MetricsSettingsGet() (string, error) {
	return RuntimeConfigGet(runtime.KeyMetricsSettingsJSON)
}

// MetricsSettingsSet 是 metrics settings JSON 的便捷写入入口。
func MetricsSettingsSet(value string) (string, error) {
	return RuntimeConfigSet(runtime.KeyMetricsSettingsJSON, value)
}

// StatusAuthSnapshot 导出当前登录态快照，便于宿主判断是否已有 node_id。
func StatusAuthSnapshot() runtime.AuthSnapshot {
	mu.Lock()
	r := rt
	mu.Unlock()
	if r == nil {
		return runtime.AuthSnapshot{}
	}
	return r.AuthState()
}

// Status 汇总 bridge 当前可见的 runtime、连接、鉴权与 metrics 快照，供宿主直接渲染。
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

// UpdateBatteryPercent 把宿主输入归一化到协议约定的百分比或未知值，再写入 runtime。
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

// UpdateVolumePercent 约束音量百分比在 0~100 之间，屏蔽宿主侧异常输入。
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

// UpdateVolumeMuted 把宿主传来的多种布尔文本折叠成协议层统一的 0/1 字符串。
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

// UpdateBrightnessPercent 与电量类似，允许 -1 表示未知，其余值钳制到 0~100。
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

// UpdateBatteryCharging 兼容未知态与多种 truthy 文本，输出稳定的线协议值。
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

// UpdateBatteryOnAC 把宿主充电来源状态转换成 runtime 统一上报格式。
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

// UpdateNetOnline 归一化联网状态，支持未知态透传为 -1。
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

// UpdateNetType 只接受协议枚举内的网络类型，异常值统一回落到 unknown。
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

// UpdateCPUPercent 规范化 CPU 百分比，越界值按协议上限/未知态处理。
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

// UpdateMemPercent 与 CPU 上报保持同一套百分比归一化规则。
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

// UpdateFlashlightEnabled 把手电筒开关态折叠成 1/0/-1，减少宿主侧分支差异。
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

// GetLastError 读取 bridge 最近一次记录的错误文本，供宿主做轻量提示。
func GetLastError() string {
	mu.Lock()
	defer mu.Unlock()
	return strings.TrimSpace(lastErr)
}

// DequeueActions 取走等待宿主执行的控制动作，并序列化成稳定 JSON 数组。
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

// isTruthy 统一识别 bridge 里多处复用的布尔文本输入。
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

// marshalStatus 把 runtime 当前状态压平成 bridge 可直接返回给宿主的 JSON 结构。
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
