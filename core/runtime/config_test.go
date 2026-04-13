package runtime

// Context: This file belongs to the MetricsNode application layer around config_test.

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	goruntime "runtime"
	"testing"
)

func TestParseBindingsJSON_Defaults(t *testing.T) {
	list, err := parseBindingsJSON("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list) != len(defaultBindings()) {
		t.Fatalf("expected %d defaults, got %d", len(defaultBindings()), len(list))
	}
}

func TestParseBindingsJSON_EmptyListAllowed(t *testing.T) {
	list, err := parseBindingsJSON(`[]`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected empty list, got %d", len(list))
	}
}

func TestParseBindingsJSON_InvalidJSON(t *testing.T) {
	_, err := parseBindingsJSON("{")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestParseBindingsJSON_UnsupportedMetric(t *testing.T) {
	_, err := parseBindingsJSON(`[{"metric":"nope","var_name":"sys_a"}]`)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestParseBindingsJSON_InvalidVarName(t *testing.T) {
	_, err := parseBindingsJSON(`[{"metric":"battery_percent","var_name":"sys.battery"}]`)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestParseBindingsJSON_DuplicateVarName(t *testing.T) {
	_, err := parseBindingsJSON(`[
  {"metric":"battery_percent","var_name":"sys_a"},
  {"metric":"volume_percent","var_name":"sys_a"}
]`)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestParseSettingsJSON_Defaults(t *testing.T) {
	list, err := parseSettingsJSON("", "windows")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list) != len(defaultBindingsForOS("windows")) {
		t.Fatalf("expected %d defaults, got %d", len(defaultBindingsForOS("windows")), len(list))
	}
}

func TestParseSettingsJSON_EmptyListIsAllDisabled(t *testing.T) {
	list, err := parseSettingsJSON("[]", "windows")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list) != len(defaultBindingsForOS("windows")) {
		t.Fatalf("expected %d defaults, got %d", len(defaultBindingsForOS("windows")), len(list))
	}
	for _, s := range list {
		if s.Enabled {
			t.Fatalf("expected all disabled, got enabled metric=%q", s.Metric)
		}
	}
}

func TestNormalizeVisibility(t *testing.T) {
	v, err := normalizeVisibility("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "public" {
		t.Fatalf("expected public, got %q", v)
	}

	v, err = normalizeVisibility("PRIVATE")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "private" {
		t.Fatalf("expected private, got %q", v)
	}

	if _, err := normalizeVisibility("x"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestTransformMetricValue_NoBatteryValue(t *testing.T) {
	cfg := runtimeConfig{NoBatteryValue: "N/A"}
	got := transformMetricValue("battery_percent", "-1", cfg)
	if got != "N/A" {
		t.Fatalf("expected N/A, got %q", got)
	}
}

func TestInitRuntimeConfig_MigrateLegacyBindings(t *testing.T) {
	tests := []struct {
		name     string
		bindings []Binding
	}{
		{name: "v0", bindings: legacyDefaultBindingsV0()},
		{name: "v1", bindings: legacyDefaultBindingsV1()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			legacyRaw, err := json.Marshal(tt.bindings)
			if err != nil {
				t.Fatalf("unexpected marshal error: %v", err)
			}

			path := filepath.Join(dir, "runtime_config.json")
			initial := map[string]string{
				KeyMetricsBindingsJSON:      string(legacyRaw),
				KeyMetricsVisibilityDefault: "public",
				KeyMetricsBatteryNoBattery:  "-1",
			}
			raw, _ := json.MarshalIndent(initial, "", "  ")
			if err := os.WriteFile(path, raw, 0o600); err != nil {
				t.Fatalf("write runtime_config.json failed: %v", err)
			}

			rt, err := New(dir, slog.Default())
			if err != nil {
				t.Fatalf("runtime init failed: %v", err)
			}

			got, ok := rt.RuntimeConfigGet(KeyMetricsBindingsJSON)
			if !ok {
				t.Fatalf("expected bindings_json present")
			}
			if got != defaultBindingsJSON() {
				t.Fatalf("expected migrated bindings_json, got %q", got)
			}

			// settings_json should exist and be valid.
			if raw, ok := rt.RuntimeConfigGet(KeyMetricsSettingsJSON); !ok || raw == "" {
				t.Fatalf("expected settings_json present")
			}
		})
	}
}

func TestInitRuntimeConfig_NoMigrateCustomBindings(t *testing.T) {
	dir := t.TempDir()
	custom := []Binding{
		{Metric: "battery_percent", VarName: "sys_battery_percent_custom"},
		{Metric: "volume_percent", VarName: "sys_volume_percent"},
		{Metric: "volume_muted", VarName: "sys_volume_muted"},
	}
	customRaw, err := json.Marshal(custom)
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}

	path := filepath.Join(dir, "runtime_config.json")
	initial := map[string]string{
		KeyMetricsBindingsJSON:      string(customRaw),
		KeyMetricsVisibilityDefault: "public",
		KeyMetricsBatteryNoBattery:  "-1",
	}
	raw, _ := json.MarshalIndent(initial, "", "  ")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write runtime_config.json failed: %v", err)
	}

	rt, err := New(dir, slog.Default())
	if err != nil {
		t.Fatalf("runtime init failed: %v", err)
	}

	got, ok := rt.RuntimeConfigGet(KeyMetricsBindingsJSON)
	if !ok {
		t.Fatalf("expected bindings_json present")
	}
	if got != string(customRaw) {
		t.Fatalf("expected custom bindings_json preserved, got %q", got)
	}

	if raw, ok := rt.RuntimeConfigGet(KeyMetricsSettingsJSON); !ok || raw == "" {
		t.Fatalf("expected settings_json present")
	}
}

func TestRuntimeConfigSet_BindingsJSON_DisableAll(t *testing.T) {
	dir := t.TempDir()
	rt, err := New(dir, slog.Default())
	if err != nil {
		t.Fatalf("runtime init failed: %v", err)
	}
	if err := rt.RuntimeConfigSet(KeyMetricsBindingsJSON, "[]", 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, ok := rt.RuntimeConfigGet(KeyMetricsBindingsJSON); !ok || got != "[]" {
		t.Fatalf("expected bindings_json=[], got (%q,%v)", got, ok)
	}
	cfg := rt.configSnapshot()
	if len(cfg.Bindings) != 0 {
		t.Fatalf("expected 0 derived bindings, got %d", len(cfg.Bindings))
	}
	raw, ok := rt.RuntimeConfigGet(KeyMetricsSettingsJSON)
	if !ok || raw == "" {
		t.Fatalf("expected settings_json present")
	}
	settings, err := parseSettingsJSON(raw, goruntime.GOOS)
	if err != nil {
		t.Fatalf("settings_json invalid: %v", err)
	}
	for _, s := range settings {
		if s.Enabled {
			t.Fatalf("expected all disabled, got enabled metric=%q", s.Metric)
		}
	}
}

func TestRuntimeConfigSet_SettingsJSON_DisableAll(t *testing.T) {
	dir := t.TempDir()
	rt, err := New(dir, slog.Default())
	if err != nil {
		t.Fatalf("runtime init failed: %v", err)
	}
	if err := rt.RuntimeConfigSet(KeyMetricsSettingsJSON, "[]", 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, ok := rt.RuntimeConfigGet(KeyMetricsBindingsJSON); !ok || got != "[]" {
		t.Fatalf("expected bindings_json=[], got (%q,%v)", got, ok)
	}
	cfg := rt.configSnapshot()
	if len(cfg.Bindings) != 0 {
		t.Fatalf("expected 0 derived bindings, got %d", len(cfg.Bindings))
	}
	raw, ok := rt.RuntimeConfigGet(KeyMetricsSettingsJSON)
	if !ok || raw == "" {
		t.Fatalf("expected settings_json present")
	}
	settings, err := parseSettingsJSON(raw, goruntime.GOOS)
	if err != nil {
		t.Fatalf("settings_json invalid: %v", err)
	}
	for _, s := range settings {
		if s.Enabled {
			t.Fatalf("expected all disabled, got enabled metric=%q", s.Metric)
		}
	}
}
