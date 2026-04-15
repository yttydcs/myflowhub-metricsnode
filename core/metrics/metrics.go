package metrics

// 本文件承载 MetricsNode 应用层中与 `metrics` 相关的逻辑。

const (
	MetricBatteryPercent    = "battery_percent"
	MetricBatteryCharging   = "battery_charging"
	MetricBatteryOnAC       = "battery_on_ac"
	MetricNetOnline         = "net_online"
	MetricNetType           = "net_type"
	MetricCPUPercent        = "cpu_percent"
	MetricMemPercent        = "mem_percent"
	MetricVolumePercent     = "volume_percent"
	MetricVolumeMuted       = "volume_muted"
	MetricBrightnessPercent = "brightness_percent"
	MetricFlashlightEnabled = "flashlight_enabled"
)

type EmitFunc func(metric string, value string)

func IsControllable(metric string) bool {
	switch metric {
	case MetricVolumePercent, MetricVolumeMuted, MetricBrightnessPercent, MetricFlashlightEnabled:
		return true
	default:
		return false
	}
}

func IsReadOnly(metric string) bool {
	switch metric {
	case MetricBatteryPercent, MetricBatteryCharging, MetricBatteryOnAC, MetricNetOnline, MetricNetType, MetricCPUPercent, MetricMemPercent:
		return true
	default:
		return false
	}
}
