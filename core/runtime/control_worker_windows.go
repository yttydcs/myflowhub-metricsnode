//go:build windows

package runtime

// Context: This file belongs to the MetricsNode application layer around control_worker_windows.

import (
	"context"
	goruntime "runtime"

	"github.com/go-ole/go-ole"

	"github.com/yttydcs/myflowhub-metricsnode/core/actuator"
	"github.com/yttydcs/myflowhub-metricsnode/core/metrics"
)

func (r *Runtime) startControlWorker(ctx context.Context) {
	if r == nil || ctx == nil || r.controlQ == nil {
		return
	}
	go r.controlWorker(ctx)
}

func (r *Runtime) controlWorker(ctx context.Context) {
	goruntime.LockOSThread()
	defer goruntime.UnlockOSThread()

	if err := ole.CoInitialize(0); err != nil {
		if r.log != nil {
			r.log.Warn("ole init failed (control)", "err", err.Error())
		}
		return
	}
	defer ole.CoUninitialize()

	var endpoint *actuator.EndpointVolume
	defer func() { endpoint.Release() }()

	ensureEndpoint := func() bool {
		if endpoint != nil {
			return true
		}
		ep, err := actuator.OpenDefaultEndpointVolume()
		if err != nil {
			if r.log != nil {
				r.log.Warn("open endpoint volume failed (control)", "err", err.Error())
			}
			return false
		}
		endpoint = ep
		return true
	}

	for {
		if ok := r.controlQ.Wait(ctx); !ok {
			return
		}
		actions := r.controlQ.DequeueAll()
		if len(actions) == 0 {
			continue
		}

		volPercentValue, haveVolPercent := "", false
		mutedValue, haveMuted := "", false
		brightnessValue, haveBrightness := "", false
		for _, a := range actions {
			switch a.Metric {
			case metrics.MetricVolumePercent:
				volPercentValue = a.Value
				haveVolPercent = true
			case metrics.MetricVolumeMuted:
				mutedValue = a.Value
				haveMuted = true
			case metrics.MetricBrightnessPercent:
				brightnessValue = a.Value
				haveBrightness = true
			default:
				// ignore unknown actions
			}
		}

		volumeReady := true
		if haveVolPercent || haveMuted {
			if !ensureEndpoint() {
				volumeReady = false
			}
		}
		if haveVolPercent && volumeReady {
			percent, ok := parseInt(volPercentValue)
			if ok {
				percent = clampInt(percent, 0, 100)
				if err := endpoint.SetPercent(percent); err != nil {
					if r.log != nil {
						r.log.Warn("set volume failed", "percent", percent, "err", err.Error())
					}
					endpoint.Release()
					endpoint = nil
				}
			}
		}
		if haveMuted && volumeReady {
			m, ok := parseBoolish(mutedValue)
			if ok {
				if err := endpoint.SetMuted(m); err != nil {
					if r.log != nil {
						r.log.Warn("set mute failed", "muted", m, "err", err.Error())
					}
					endpoint.Release()
					endpoint = nil
				}
			}
		}
		if haveBrightness {
			percent, ok := parseInt(brightnessValue)
			if ok {
				percent = clampInt(percent, 0, 100)
				if err := actuator.SetPrimaryMonitorBrightnessPercent(percent); err != nil {
					if r.log != nil {
						r.log.Warn("set brightness failed", "percent", percent, "err", err.Error())
					}
				}
			}
		}
	}
}
