//go:build !windows

package actuator

// 本文件承载 MetricsNode 应用层中与 `volume_other` 相关的逻辑。

import "errors"

var ErrUnsupported = errors.New("actuator unsupported on this platform")

type EndpointVolume struct{}

func OpenDefaultEndpointVolume() (*EndpointVolume, error) { return nil, ErrUnsupported }

func (v *EndpointVolume) Release() {}

func (v *EndpointVolume) SetPercent(_ int) error { return ErrUnsupported }

func (v *EndpointVolume) SetMuted(_ bool) error { return ErrUnsupported }
