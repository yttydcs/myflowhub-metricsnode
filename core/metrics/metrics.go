package metrics

const (
	MetricBatteryPercent = "battery_percent"
	MetricVolumePercent  = "volume_percent"
	MetricVolumeMuted    = "volume_muted"
)

type EmitFunc func(metric string, value string)

