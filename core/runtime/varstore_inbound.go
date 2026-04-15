package runtime

// 本文件承载 MetricsNode 应用层中与 `varstore_inbound` 相关的逻辑。

import (
	"encoding/json"
	"strconv"
	"strings"

	core "github.com/yttydcs/myflowhub-core"
	"github.com/yttydcs/myflowhub-core/header"
	protovar "github.com/yttydcs/myflowhub-proto/protocol/varstore"
	"github.com/yttydcs/myflowhub-sdk/transport"

	"github.com/yttydcs/myflowhub-metricsnode/core/metrics"
)

func (r *Runtime) tryHandleVarStoreFrame(hdr core.IHeader, payload []byte) bool {
	if r == nil || hdr == nil || len(payload) == 0 {
		return false
	}
	if hdr.SubProto() != protovar.SubProtoVarStore {
		return false
	}
	switch hdr.Major() {
	case header.MajorCmd, header.MajorMsg:
		// ok
	default:
		return false
	}
	r.handleVarStoreFrame(hdr, payload)
	return true
}

func (r *Runtime) handleVarStoreFrame(hdr core.IHeader, payload []byte) {
	if r == nil || hdr == nil || len(payload) == 0 {
		return
	}

	msg, err := transport.DecodeMessage(payload)
	if err != nil {
		if r.log != nil {
			r.log.Warn("varstore decode failed", "err", err.Error())
		}
		return
	}

	switch msg.Action {
	case protovar.ActionNotifySet:
		var resp protovar.VarResp
		if err := json.Unmarshal(msg.Data, &resp); err != nil {
			if r.log != nil {
				r.log.Warn("varstore notify_set invalid data", "err", err.Error())
			}
			return
		}
		r.handleVarStoreNotifySet(hdr, resp)
	default:
		// ignore
	}
}

func (r *Runtime) handleVarStoreNotifySet(hdr core.IHeader, resp protovar.VarResp) {
	if r == nil || hdr == nil {
		return
	}

	auth := r.AuthState()
	if !auth.LoggedIn || auth.NodeID == 0 || auth.HubID == 0 {
		return
	}

	name := strings.TrimSpace(resp.Name)
	value := strings.TrimSpace(resp.Value)
	if name == "" || value == "" {
		return
	}
	if resp.Owner != 0 && resp.Owner != auth.NodeID {
		return
	}

	cfg := r.configSnapshot()
	binding, ok := bindingByVarName(cfg, name)
	if !ok {
		return
	}
	metric := binding.Metric

	r.updatePublishedShadow(name, value, resp.Visibility)

	if metrics.IsControllable(metric) && !binding.Writable {
		correct, ok := r.currentPublishedMetricValue(metric, cfg)
		if !ok || correct == "" {
			if r.log != nil {
				r.log.Warn("writable disabled; correction unavailable", "metric", metric, "var", name)
			}
			return
		}
		if value == correct {
			return
		}
		r.publishVar(auth.NodeID, auth.HubID, name, correct, cfg.VisibilityDefault)
		return
	}

	switch metric {
	case metrics.MetricVolumePercent:
		percent, ok := parseInt(value)
		if !ok {
			if r.log != nil {
				r.log.Warn("volume_percent command invalid", "var", name, "value", value)
			}
			return
		}
		percent = clampInt(percent, 0, 100)
		r.enqueueControlAction(metric, strconv.Itoa(percent))
	case metrics.MetricVolumeMuted:
		muted, ok := parseBoolish(value)
		if !ok {
			if r.log != nil {
				r.log.Warn("volume_muted command invalid", "var", name, "value", value)
			}
			return
		}
		if muted {
			r.enqueueControlAction(metric, "1")
		} else {
			r.enqueueControlAction(metric, "0")
		}
	case metrics.MetricBrightnessPercent:
		percent, ok := parseInt(value)
		if !ok {
			if r.log != nil {
				r.log.Warn("brightness_percent command invalid", "var", name, "value", value)
			}
			return
		}
		percent = clampInt(percent, 0, 100)
		r.enqueueControlAction(metric, strconv.Itoa(percent))
	case metrics.MetricFlashlightEnabled:
		enabled, ok := parseBoolish(value)
		if !ok {
			if r.log != nil {
				r.log.Warn("flashlight_enabled command invalid", "var", name, "value", value)
			}
			return
		}
		if enabled {
			r.enqueueControlAction(metric, "1")
		} else {
			r.enqueueControlAction(metric, "0")
		}
	default:
		if !metrics.IsReadOnly(metric) {
			return
		}
		correct, ok := r.currentPublishedMetricValue(metric, cfg)
		if !ok || correct == "" {
			if r.log != nil {
				r.log.Warn("readonly metric correction unavailable", "metric", metric, "var", name)
			}
			return
		}
		if value == correct {
			return
		}
		r.publishVar(auth.NodeID, auth.HubID, name, correct, cfg.VisibilityDefault)
	}
}

func metricByVarName(cfg runtimeConfig, varName string) (string, bool) {
	b, ok := bindingByVarName(cfg, varName)
	if !ok {
		return "", false
	}
	return b.Metric, true
}

func bindingByVarName(cfg runtimeConfig, varName string) (varBinding, bool) {
	varName = strings.TrimSpace(varName)
	if varName == "" {
		return varBinding{}, false
	}
	if len(cfg.BindingByVarName) > 0 {
		if b, ok := cfg.BindingByVarName[varName]; ok && strings.TrimSpace(b.Metric) != "" {
			return b, true
		}
	}
	if len(cfg.Bindings) == 0 {
		return varBinding{}, false
	}
	for _, b := range cfg.Bindings {
		if b.VarName == varName {
			return varBinding{Metric: b.Metric, Writable: metrics.IsControllable(b.Metric)}, true
		}
	}
	return varBinding{}, false
}

func (r *Runtime) updatePublishedShadow(varName, value, visibility string) {
	if r == nil {
		return
	}
	varName = strings.TrimSpace(varName)
	value = strings.TrimSpace(value)
	if varName == "" || value == "" {
		return
	}
	visibility = strings.ToLower(strings.TrimSpace(visibility))
	if visibility == "" {
		visibility = protovar.VisibilityPublic
	}

	r.reportMu.Lock()
	if r.lastPublished == nil {
		r.lastPublished = make(map[string]publishedVar)
	}
	r.lastPublished[varName] = publishedVar{Value: value, Visibility: visibility}
	r.reportMu.Unlock()
}

func (r *Runtime) enqueueControlAction(metric, value string) {
	if r == nil || r.controlQ == nil {
		return
	}
	r.controlQ.Enqueue(metric, value)
}

func (r *Runtime) currentPublishedMetricValue(metric string, cfg runtimeConfig) (string, bool) {
	if r == nil {
		return "", false
	}
	r.reportMu.Lock()
	raw, ok := r.lastMetrics[metric]
	r.reportMu.Unlock()
	if !ok {
		return "", false
	}
	val := transformMetricValue(metric, raw, cfg)
	if strings.TrimSpace(val) == "" {
		return "", false
	}
	return val, true
}

func parseInt(text string) (int, bool) {
	n, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil {
		return 0, false
	}
	return n, true
}

func parseBoolish(text string) (bool, bool) {
	v := strings.ToLower(strings.TrimSpace(text))
	switch v {
	case "1", "true", "yes", "y", "on":
		return true, true
	case "0", "false", "no", "n", "off":
		return false, true
	default:
		return false, false
	}
}

func clampInt(n, min, max int) int {
	if n < min {
		return min
	}
	if n > max {
		return max
	}
	return n
}
