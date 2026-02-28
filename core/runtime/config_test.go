package runtime

import "testing"

func TestParseBindingsJSON_Defaults(t *testing.T) {
	list, err := parseBindingsJSON("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("expected 3 defaults, got %d", len(list))
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

