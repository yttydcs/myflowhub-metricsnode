package metrics

const (
	MetricBatteryPercent = "battery_percent"
	MetricVolumePercent  = "volume_percent"
	MetricVolumeMuted    = "volume_muted"
	MetricBrightnessPercent = "brightness_percent"
)

type EmitFunc func(metric string, value string)

