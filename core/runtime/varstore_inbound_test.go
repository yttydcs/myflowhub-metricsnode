package runtime

import "testing"

func TestParseInt(t *testing.T) {
	n, ok := parseInt(" 10 ")
	if !ok || n != 10 {
		t.Fatalf("parseInt expected (10,true), got (%d,%v)", n, ok)
	}
	if _, ok := parseInt("x"); ok {
		t.Fatalf("parseInt expected ok=false")
	}
}

func TestParseBoolish(t *testing.T) {
	cases := []struct {
		in      string
		want    bool
		wantOK  bool
	}{
		{"1", true, true},
		{"true", true, true},
		{"YES", true, true},
		{"on", true, true},
		{"0", false, true},
		{"false", false, true},
		{"No", false, true},
		{"off", false, true},
		{"x", false, false},
	}
	for _, tc := range cases {
		got, ok := parseBoolish(tc.in)
		if ok != tc.wantOK || (ok && got != tc.want) {
			t.Fatalf("parseBoolish(%q)=(%v,%v) want (%v,%v)", tc.in, got, ok, tc.want, tc.wantOK)
		}
	}
}

func TestClampInt(t *testing.T) {
	if got := clampInt(-1, 0, 100); got != 0 {
		t.Fatalf("clampInt(-1,0,100)=%d want 0", got)
	}
	if got := clampInt(200, 0, 100); got != 100 {
		t.Fatalf("clampInt(200,0,100)=%d want 100", got)
	}
	if got := clampInt(50, 0, 100); got != 50 {
		t.Fatalf("clampInt(50,0,100)=%d want 50", got)
	}
}

func TestMetricByVarName(t *testing.T) {
	cfg := runtimeConfig{
		Bindings: []Binding{
			{Metric: "battery_percent", VarName: "sys_battery_percent"},
			{Metric: "volume_percent", VarName: "sys_volume_percent"},
		},
	}
	metric, ok := metricByVarName(cfg, "sys_volume_percent")
	if !ok || metric != "volume_percent" {
		t.Fatalf("metricByVarName expected volume_percent, got (%q,%v)", metric, ok)
	}
	if _, ok := metricByVarName(cfg, "missing"); ok {
		t.Fatalf("metricByVarName expected ok=false")
	}
}

