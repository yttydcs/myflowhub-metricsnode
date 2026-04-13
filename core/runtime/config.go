package runtime

// Context: This file belongs to the MetricsNode application layer around config.

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	goruntime "runtime"
	"strings"

	protovar "github.com/yttydcs/myflowhub-proto/protocol/varstore"

	"github.com/yttydcs/myflowhub-metricsnode/core/configstore"
	"github.com/yttydcs/myflowhub-metricsnode/core/metrics"
	rtvar "github.com/yttydcs/myflowhub-metricsnode/core/varstore"
)

const (
	KeyMetricsBindingsJSON      = "metrics.bindings_json"
	KeyMetricsSettingsJSON      = "metrics.settings_json"
	KeyMetricsVisibilityDefault = "metrics.visibility_default"
	KeyMetricsBatteryNoBattery  = "metrics.battery.no_battery_value"

	defaultNoBatteryValue = "-1"
)

type Binding struct {
	Metric  string `json:"metric"`
	VarName string `json:"var_name"`
}

type MetricSetting struct {
	Metric   string `json:"metric"`
	VarName  string `json:"var_name"`
	Enabled  bool   `json:"enabled"`
	Writable bool   `json:"writable"`
}

type metricSettingJSON struct {
	Metric   string `json:"metric"`
	VarName  string `json:"var_name"`
	Enabled  *bool  `json:"enabled,omitempty"`
	Writable *bool  `json:"writable,omitempty"`
}

type varBinding struct {
	Metric   string
	Writable bool
}

type runtimeConfig struct {
	Bindings          []Binding
	Settings          []MetricSetting
	EnabledByMetric   map[string]bool
	BindingByVarName  map[string]varBinding
	VisibilityDefault string
	NoBatteryValue    string
}

func defaultBindings() []Binding {
	return defaultBindingsForOS(goruntime.GOOS)
}

func defaultBindingsForOS(goos string) []Binding {
	list := []Binding{
		{Metric: metrics.MetricBatteryPercent, VarName: "sys_battery_percent"},
		{Metric: metrics.MetricBatteryCharging, VarName: "sys_battery_charging"},
		{Metric: metrics.MetricBatteryOnAC, VarName: "sys_battery_on_ac"},
		{Metric: metrics.MetricVolumePercent, VarName: "sys_volume_percent"},
		{Metric: metrics.MetricVolumeMuted, VarName: "sys_volume_muted"},
		{Metric: metrics.MetricBrightnessPercent, VarName: "sys_brightness_percent"},
		{Metric: metrics.MetricNetOnline, VarName: "sys_net_online"},
		{Metric: metrics.MetricNetType, VarName: "sys_net_type"},
		{Metric: metrics.MetricCPUPercent, VarName: "sys_cpu_percent"},
		{Metric: metrics.MetricMemPercent, VarName: "sys_mem_percent"},
	}
	if goos == "android" {
		list = append(list, Binding{Metric: metrics.MetricFlashlightEnabled, VarName: "sys_flashlight_enabled"})
	}
	return list
}

func legacyDefaultBindingsV0() []Binding {
	return []Binding{
		{Metric: metrics.MetricBatteryPercent, VarName: "sys_battery_percent"},
		{Metric: metrics.MetricVolumePercent, VarName: "sys_volume_percent"},
		{Metric: metrics.MetricVolumeMuted, VarName: "sys_volume_muted"},
	}
}

func legacyDefaultBindingsV1() []Binding {
	return []Binding{
		{Metric: metrics.MetricBatteryPercent, VarName: "sys_battery_percent"},
		{Metric: metrics.MetricVolumePercent, VarName: "sys_volume_percent"},
		{Metric: metrics.MetricVolumeMuted, VarName: "sys_volume_muted"},
		{Metric: metrics.MetricBrightnessPercent, VarName: "sys_brightness_percent"},
	}
}

func defaultSettingsForOS(goos string) []MetricSetting {
	bindings := defaultBindingsForOS(goos)
	out := make([]MetricSetting, 0, len(bindings))
	for _, b := range bindings {
		writable := metrics.IsControllable(strings.TrimSpace(b.Metric))
		out = append(out, MetricSetting{
			Metric:   strings.TrimSpace(b.Metric),
			VarName:  strings.TrimSpace(b.VarName),
			Enabled:  true,
			Writable: writable,
		})
	}
	return out
}

func defaultSettings() []MetricSetting {
	return defaultSettingsForOS(goruntime.GOOS)
}

func equalBindings(a, b []Binding) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if strings.TrimSpace(a[i].Metric) != strings.TrimSpace(b[i].Metric) {
			return false
		}
		if strings.TrimSpace(a[i].VarName) != strings.TrimSpace(b[i].VarName) {
			return false
		}
	}
	return true
}

func defaultBindingsJSON() string {
	raw, _ := json.Marshal(defaultBindings())
	return string(raw)
}

func settingsToBindings(settings []MetricSetting) []Binding {
	out := make([]Binding, 0, len(settings))
	for _, s := range settings {
		if !s.Enabled {
			continue
		}
		out = append(out, Binding{
			Metric:  strings.TrimSpace(s.Metric),
			VarName: strings.TrimSpace(s.VarName),
		})
	}
	return out
}

func settingsFromBindings(bindings []Binding, supported []Binding) []MetricSetting {
	byMetric := make(map[string]string, len(bindings))
	for _, b := range bindings {
		metric := strings.TrimSpace(b.Metric)
		byMetric[metric] = strings.TrimSpace(b.VarName) // last wins
	}

	out := make([]MetricSetting, 0, len(supported))
	for _, b := range supported {
		metric := strings.TrimSpace(b.Metric)
		defaultVar := strings.TrimSpace(b.VarName)
		varName, ok := byMetric[metric]
		if !ok || strings.TrimSpace(varName) == "" {
			varName = defaultVar
		}
		enabled := ok
		out = append(out, MetricSetting{
			Metric:   metric,
			VarName:  varName,
			Enabled:  enabled,
			Writable: metrics.IsControllable(metric),
		})
	}
	return out
}

func defaultSettingsJSON() string {
	raw, _ := json.Marshal(defaultSettings())
	return string(raw)
}

func normalizeVisibility(text string) (string, error) {
	v := strings.ToLower(strings.TrimSpace(text))
	if v == "" {
		return protovar.VisibilityPublic, nil
	}
	switch v {
	case protovar.VisibilityPublic, protovar.VisibilityPrivate:
		return v, nil
	default:
		return "", fmt.Errorf("invalid visibility: %q", text)
	}
}

func normalizeNoBatteryValue(text string) (string, error) {
	v := strings.TrimSpace(text)
	if v == "" {
		return "", errors.New("no_battery_value is required")
	}
	return v, nil
}

func parseBindingsJSON(text string) ([]Binding, error) {
	raw := strings.TrimSpace(text)
	if raw == "" {
		return defaultBindings(), nil
	}
	var list []Binding
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		return nil, err
	}
	return validateBindings(list)
}

func validateBindings(list []Binding) ([]Binding, error) {
	// Allow empty bindings to support "disable all metrics".
	if len(list) == 0 {
		return []Binding{}, nil
	}
	out := make([]Binding, 0, len(list))
	seenVar := make(map[string]struct{}, len(list))
	for i := range list {
		b := list[i]
		b.Metric = strings.TrimSpace(b.Metric)
		b.VarName = strings.TrimSpace(b.VarName)
		if !supportedMetricForOS(goruntime.GOOS, b.Metric) {
			return nil, fmt.Errorf("unsupported metric: %q", b.Metric)
		}
		if !rtvar.ValidVarName(b.VarName) {
			return nil, fmt.Errorf("invalid var_name: %q", b.VarName)
		}
		if _, ok := seenVar[b.VarName]; ok {
			return nil, fmt.Errorf("duplicate var_name: %q", b.VarName)
		}
		seenVar[b.VarName] = struct{}{}
		out = append(out, b)
	}
	return out, nil
}

func supportedMetricForOS(goos, metric string) bool {
	metric = strings.TrimSpace(metric)
	if goos != "android" && metric == metrics.MetricFlashlightEnabled {
		return false
	}
	switch metric {
	case metrics.MetricBatteryPercent, metrics.MetricBatteryCharging, metrics.MetricBatteryOnAC:
		return true
	case metrics.MetricNetOnline, metrics.MetricNetType:
		return true
	case metrics.MetricCPUPercent, metrics.MetricMemPercent:
		return true
	case metrics.MetricVolumePercent, metrics.MetricVolumeMuted, metrics.MetricBrightnessPercent, metrics.MetricFlashlightEnabled:
		return true
	default:
		return false
	}
}

func parseSettingsJSON(text string, goos string) ([]MetricSetting, error) {
	raw := strings.TrimSpace(text)
	if raw == "" {
		return defaultSettingsForOS(goos), nil
	}
	var list []metricSettingJSON
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		return nil, err
	}
	return validateSettings(list, goos)
}

func validateSettings(list []metricSettingJSON, goos string) ([]MetricSetting, error) {
	supported := defaultBindingsForOS(goos)
	defaultVarByMetric := make(map[string]string, len(supported))
	ordered := make([]string, 0, len(supported))
	for _, b := range supported {
		metric := strings.TrimSpace(b.Metric)
		ordered = append(ordered, metric)
		defaultVarByMetric[metric] = strings.TrimSpace(b.VarName)
	}

	byMetric := make(map[string]MetricSetting, len(list))
	for i := range list {
		in := list[i]
		metric := strings.TrimSpace(in.Metric)
		if !supportedMetricForOS(goos, metric) {
			return nil, fmt.Errorf("unsupported metric: %q", in.Metric)
		}
		varName := strings.TrimSpace(in.VarName)
		if !rtvar.ValidVarName(varName) {
			return nil, fmt.Errorf("invalid var_name: %q", in.VarName)
		}

		enabled := true
		if in.Enabled != nil {
			enabled = *in.Enabled
		}
		writable := metrics.IsControllable(metric)
		if in.Writable != nil {
			writable = *in.Writable
		}
		if !metrics.IsControllable(metric) {
			writable = false
		}
		byMetric[metric] = MetricSetting{
			Metric:   metric,
			VarName:  varName,
			Enabled:  enabled,
			Writable: writable,
		}
	}

	out := make([]MetricSetting, 0, len(ordered))
	for _, metric := range ordered {
		if s, ok := byMetric[metric]; ok {
			out = append(out, s)
			continue
		}
		out = append(out, MetricSetting{
			Metric:   metric,
			VarName:  defaultVarByMetric[metric],
			Enabled:  false,
			Writable: metrics.IsControllable(metric),
		})
	}

	seenEnabledVar := make(map[string]struct{}, len(out))
	for _, s := range out {
		if !s.Enabled {
			continue
		}
		if _, ok := seenEnabledVar[s.VarName]; ok {
			return nil, fmt.Errorf("duplicate enabled var_name: %q", s.VarName)
		}
		seenEnabledVar[s.VarName] = struct{}{}
	}

	return out, nil
}

func (r *Runtime) initRuntimeConfig() error {
	if r == nil {
		return errors.New("runtime not initialized")
	}

	path := filepath.Join(r.workDir, "runtime_config.json")
	defaults := map[string]string{
		KeyMetricsBindingsJSON:      defaultBindingsJSON(),
		KeyMetricsVisibilityDefault: protovar.VisibilityPublic,
		KeyMetricsBatteryNoBattery:  defaultNoBatteryValue,
	}
	store, err := configstore.New(path, defaults, r.log)
	if err != nil {
		return err
	}

	// Safe migration: if the user still has legacy default bindings, automatically upgrade to
	// include P0 metrics (and flashlight on Android). Never overwrite custom bindings.
	if raw, ok := store.Get(KeyMetricsBindingsJSON); ok {
		if list, err := parseBindingsJSON(raw); err == nil && (equalBindings(list, legacyDefaultBindingsV0()) || equalBindings(list, legacyDefaultBindingsV1())) {
			_ = store.Set(KeyMetricsBindingsJSON, defaultBindingsJSON())
			if r.log != nil {
				r.log.Info("migrated metrics.bindings_json to include P0 metrics")
			}
		}
	}

	// Create settings_json if missing. It is derived from bindings_json to preserve user custom bindings.
	if _, ok := store.Get(KeyMetricsSettingsJSON); !ok {
		raw, _ := store.Get(KeyMetricsBindingsJSON)
		bindings, err := parseBindingsJSON(raw)
		if err != nil {
			bindings = defaultBindings()
		}
		settings := settingsFromBindings(bindings, defaultBindingsForOS(goruntime.GOOS))
		normalized, _ := json.Marshal(settings)
		_ = store.Set(KeyMetricsSettingsJSON, string(normalized))
	}

	r.cfgStore = store
	r.reloadRuntimeConfig("init", "")
	return nil
}

func (r *Runtime) RuntimeConfigKeys() []string {
	if r == nil || r.cfgStore == nil {
		return nil
	}
	return r.cfgStore.Keys()
}

func (r *Runtime) RuntimeConfigGet(key string) (string, bool) {
	if r == nil || r.cfgStore == nil {
		return "", false
	}
	return r.cfgStore.Get(key)
}

func (r *Runtime) RuntimeConfigSet(key, value string, sourceNode uint32) error {
	if r == nil || r.cfgStore == nil {
		return errors.New("runtime not initialized")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return errors.New("key is required")
	}

	// Validate known keys; allow arbitrary keys by default.
	switch key {
	case KeyMetricsSettingsJSON:
		settings, err := parseSettingsJSON(value, goruntime.GOOS)
		if err != nil {
			return err
		}
		normalized, _ := json.Marshal(settings)
		value = string(normalized)
	case KeyMetricsBindingsJSON:
		if _, err := parseBindingsJSON(value); err != nil {
			return err
		}
	case KeyMetricsVisibilityDefault:
		if _, err := normalizeVisibility(value); err != nil {
			return err
		}
	case KeyMetricsBatteryNoBattery:
		if _, err := normalizeNoBatteryValue(value); err != nil {
			return err
		}
	}

	if err := r.cfgStore.Set(key, value); err != nil {
		return err
	}
	if r.log != nil {
		r.log.Info("runtime config updated", "key", key, "source", sourceNode)
	}
	r.reloadRuntimeConfig(key, value)
	return nil
}

func (r *Runtime) storeSetIfChanged(key, value string) {
	if r == nil || r.cfgStore == nil {
		return
	}
	cur, ok := r.cfgStore.Get(key)
	if ok && cur == value {
		return
	}
	if err := r.cfgStore.Set(key, value); err != nil && r.log != nil {
		r.log.Warn("runtime config save failed", "key", key, "err", err.Error())
	}
}

func (r *Runtime) reloadRuntimeConfig(reasonKey, _ string) {
	if r == nil || r.cfgStore == nil {
		return
	}

	goos := goruntime.GOOS

	var settings []MetricSetting
	switch reasonKey {
	case KeyMetricsBindingsJSON:
		// User updated legacy bindings_json: convert to settings_json to keep configs in sync.
		bindingsRaw, _ := r.cfgStore.Get(KeyMetricsBindingsJSON)
		bindings, err := parseBindingsJSON(bindingsRaw)
		if err != nil {
			if r.log != nil {
				r.log.Warn("invalid bindings_json; fallback to defaults", "err", err.Error())
			}
			bindings = defaultBindings()
		}
		settings = settingsFromBindings(bindings, defaultBindingsForOS(goos))
		normalized, _ := json.Marshal(settings)
		r.storeSetIfChanged(KeyMetricsSettingsJSON, string(normalized))
	default:
		settingsRaw, ok := r.cfgStore.Get(KeyMetricsSettingsJSON)
		if ok {
			list, err := parseSettingsJSON(settingsRaw, goos)
			if err == nil {
				settings = list
				break
			}
			if r.log != nil {
				r.log.Warn("invalid settings_json; fallback to defaults", "err", err.Error())
			}
		}
		settings = defaultSettingsForOS(goos)
		normalized, _ := json.Marshal(settings)
		r.storeSetIfChanged(KeyMetricsSettingsJSON, string(normalized))
	}

	bindings := validateDerivedBindings(settings)
	derivedBindingsRaw, _ := json.Marshal(bindings)
	r.storeSetIfChanged(KeyMetricsBindingsJSON, string(derivedBindingsRaw))

	visRaw, _ := r.cfgStore.Get(KeyMetricsVisibilityDefault)
	vis, err := normalizeVisibility(visRaw)
	if err != nil {
		if r.log != nil {
			r.log.Warn("invalid visibility_default; fallback", "err", err.Error())
		}
		vis = protovar.VisibilityPublic
	}
	noBatRaw, _ := r.cfgStore.Get(KeyMetricsBatteryNoBattery)
	noBat, err := normalizeNoBatteryValue(noBatRaw)
	if err != nil {
		noBat = defaultNoBatteryValue
	}

	enabledByMetric := make(map[string]bool, len(settings))
	bindingByVarName := make(map[string]varBinding, len(bindings))
	for _, s := range settings {
		if !s.Enabled {
			continue
		}
		enabledByMetric[s.Metric] = true
		bindingByVarName[s.VarName] = varBinding{Metric: s.Metric, Writable: s.Writable}
	}

	r.cfgMu.Lock()
	r.cfg = runtimeConfig{
		Bindings:          bindings,
		Settings:          settings,
		EnabledByMetric:   enabledByMetric,
		BindingByVarName:  bindingByVarName,
		VisibilityDefault: vis,
		NoBatteryValue:    noBat,
	}
	r.cfgMu.Unlock()
	r.signalConfigChanged()

	if r.log != nil {
		r.log.Debug("runtime config applied", "reason", reasonKey, "bindings", len(bindings), "visibility", vis)
	}
	r.republishFromConfig()
}

func validateDerivedBindings(settings []MetricSetting) []Binding {
	return settingsToBindings(settings)
}

func (r *Runtime) configSnapshot() runtimeConfig {
	if r == nil {
		return runtimeConfig{}
	}
	r.cfgMu.RLock()
	cfg := r.cfg
	// shallow-copy slice to keep it immutable to callers
	if len(cfg.Bindings) > 0 {
		copied := make([]Binding, len(cfg.Bindings))
		copy(copied, cfg.Bindings)
		cfg.Bindings = copied
	}
	r.cfgMu.RUnlock()
	return cfg
}

func (r *Runtime) republishFromConfig() {
	if r == nil {
		return
	}
	if !r.IsReporting() {
		return
	}
	auth := r.AuthState()
	if !auth.LoggedIn || auth.NodeID == 0 || auth.HubID == 0 {
		return
	}

	cfg := r.configSnapshot()

	r.reportMu.Lock()
	rawMetrics := make(map[string]string, len(r.lastMetrics))
	for k, v := range r.lastMetrics {
		rawMetrics[k] = v
	}
	r.reportMu.Unlock()

	for _, b := range cfg.Bindings {
		raw, ok := rawMetrics[b.Metric]
		if !ok || strings.TrimSpace(raw) == "" {
			continue
		}
		val := transformMetricValue(b.Metric, raw, cfg)
		r.publishVar(auth.NodeID, auth.HubID, b.VarName, val, cfg.VisibilityDefault)
	}
}

func transformMetricValue(metric string, raw string, cfg runtimeConfig) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if metric == metrics.MetricBatteryPercent && raw == "-1" {
		v := strings.TrimSpace(cfg.NoBatteryValue)
		if v == "" {
			return defaultNoBatteryValue
		}
		return v
	}
	return raw
}
