package runtime

import (
	"testing"

	"github.com/yttydcs/myflowhub-core/header"
	protovar "github.com/yttydcs/myflowhub-proto/protocol/varstore"

	"github.com/yttydcs/myflowhub-metricsnode/core/metrics"
)

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
		in     string
		want   bool
		wantOK bool
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

func TestHandleVarStoreNotifySet_BrightnessPercentClampAndEnqueue(t *testing.T) {
	r := &Runtime{controlQ: newActionQueue()}
	r.auth = AuthSnapshot{LoggedIn: true, NodeID: 7, HubID: 1}
	r.cfg = runtimeConfig{
		Bindings: []Binding{{Metric: metrics.MetricBrightnessPercent, VarName: "sys_brightness_percent"}},
	}
	hdr := (&header.HeaderTcp{}).WithMajor(header.MajorMsg).WithSubProto(0).WithSourceID(1).WithTargetID(1)

	r.handleVarStoreNotifySet(hdr, protovar.VarResp{
		Name:       "sys_brightness_percent",
		Value:      "200",
		Owner:      7,
		Visibility: protovar.VisibilityPublic,
	})

	actions := r.DequeueActions()
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Metric != metrics.MetricBrightnessPercent || actions[0].Value != "100" {
		t.Fatalf("expected brightness_percent=100, got (%q,%q)", actions[0].Metric, actions[0].Value)
	}
}

func TestHandleVarStoreNotifySet_BrightnessPercentInvalidIgnored(t *testing.T) {
	r := &Runtime{controlQ: newActionQueue()}
	r.auth = AuthSnapshot{LoggedIn: true, NodeID: 7, HubID: 1}
	r.cfg = runtimeConfig{
		Bindings: []Binding{{Metric: metrics.MetricBrightnessPercent, VarName: "sys_brightness_percent"}},
	}
	hdr := (&header.HeaderTcp{}).WithMajor(header.MajorMsg).WithSubProto(0).WithSourceID(1).WithTargetID(1)

	r.handleVarStoreNotifySet(hdr, protovar.VarResp{
		Name:       "sys_brightness_percent",
		Value:      "abc",
		Owner:      7,
		Visibility: protovar.VisibilityPublic,
	})

	if actions := r.DequeueActions(); len(actions) != 0 {
		t.Fatalf("expected 0 actions, got %d", len(actions))
	}
}

func TestHandleVarStoreNotifySet_BrightnessPercentOwnerMismatchIgnored(t *testing.T) {
	r := &Runtime{controlQ: newActionQueue()}
	r.auth = AuthSnapshot{LoggedIn: true, NodeID: 7, HubID: 1}
	r.cfg = runtimeConfig{
		Bindings: []Binding{{Metric: metrics.MetricBrightnessPercent, VarName: "sys_brightness_percent"}},
	}
	hdr := (&header.HeaderTcp{}).WithMajor(header.MajorMsg).WithSubProto(0).WithSourceID(1).WithTargetID(1)

	r.handleVarStoreNotifySet(hdr, protovar.VarResp{
		Name:       "sys_brightness_percent",
		Value:      "50",
		Owner:      8,
		Visibility: protovar.VisibilityPublic,
	})

	if actions := r.DequeueActions(); len(actions) != 0 {
		t.Fatalf("expected 0 actions, got %d", len(actions))
	}
}

func TestHandleVarStoreNotifySet_FlashlightEnabledEnqueue(t *testing.T) {
	r := &Runtime{controlQ: newActionQueue()}
	r.auth = AuthSnapshot{LoggedIn: true, NodeID: 7, HubID: 1}
	r.cfg = runtimeConfig{
		Bindings: []Binding{{Metric: metrics.MetricFlashlightEnabled, VarName: "sys_flashlight_enabled"}},
	}
	hdr := (&header.HeaderTcp{}).WithMajor(header.MajorMsg).WithSubProto(0).WithSourceID(1).WithTargetID(1)

	r.handleVarStoreNotifySet(hdr, protovar.VarResp{
		Name:       "sys_flashlight_enabled",
		Value:      "1",
		Owner:      7,
		Visibility: protovar.VisibilityPublic,
	})

	actions := r.DequeueActions()
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Metric != metrics.MetricFlashlightEnabled || actions[0].Value != "1" {
		t.Fatalf("expected flashlight_enabled=1, got (%q,%q)", actions[0].Metric, actions[0].Value)
	}
}

func TestHandleVarStoreNotifySet_ReadOnlyCorrection(t *testing.T) {
	r := &Runtime{controlQ: newActionQueue()}
	r.auth = AuthSnapshot{LoggedIn: true, NodeID: 7, HubID: 1}
	r.cfg = runtimeConfig{
		Bindings:          []Binding{{Metric: metrics.MetricNetType, VarName: "sys_net_type"}},
		VisibilityDefault: protovar.VisibilityPublic,
	}
	r.lastMetrics = map[string]string{metrics.MetricNetType: "wifi"}
	hdr := (&header.HeaderTcp{}).WithMajor(header.MajorMsg).WithSubProto(0).WithSourceID(1).WithTargetID(1)

	r.handleVarStoreNotifySet(hdr, protovar.VarResp{
		Name:       "sys_net_type",
		Value:      "ethernet",
		Owner:      7,
		Visibility: protovar.VisibilityPublic,
	})

	if actions := r.DequeueActions(); len(actions) != 0 {
		t.Fatalf("expected 0 actions, got %d", len(actions))
	}

	r.reportMu.Lock()
	pv := r.lastPublished["sys_net_type"]
	r.reportMu.Unlock()
	if pv.Value != "wifi" {
		t.Fatalf("expected corrected sys_net_type=wifi, got %q", pv.Value)
	}
}

func TestHandleVarStoreNotifySet_ReadOnlyCorrectionUnavailable(t *testing.T) {
	r := &Runtime{controlQ: newActionQueue()}
	r.auth = AuthSnapshot{LoggedIn: true, NodeID: 7, HubID: 1}
	r.cfg = runtimeConfig{
		Bindings:          []Binding{{Metric: metrics.MetricNetType, VarName: "sys_net_type"}},
		VisibilityDefault: protovar.VisibilityPublic,
	}
	hdr := (&header.HeaderTcp{}).WithMajor(header.MajorMsg).WithSubProto(0).WithSourceID(1).WithTargetID(1)

	r.handleVarStoreNotifySet(hdr, protovar.VarResp{
		Name:       "sys_net_type",
		Value:      "ethernet",
		Owner:      7,
		Visibility: protovar.VisibilityPublic,
	})

	if actions := r.DequeueActions(); len(actions) != 0 {
		t.Fatalf("expected 0 actions, got %d", len(actions))
	}

	r.reportMu.Lock()
	pv := r.lastPublished["sys_net_type"]
	r.reportMu.Unlock()
	if pv.Value != "ethernet" {
		t.Fatalf("expected sys_net_type preserved, got %q", pv.Value)
	}
}
