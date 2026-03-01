//go:build !windows

package actuator

import "errors"

var ErrUnsupported = errors.New("actuator unsupported on this platform")

type EndpointVolume struct{}

func OpenDefaultEndpointVolume() (*EndpointVolume, error) { return nil, ErrUnsupported }

func (v *EndpointVolume) Release() {}

func (v *EndpointVolume) SetPercent(_ int) error { return ErrUnsupported }

func (v *EndpointVolume) SetMuted(_ bool) error { return ErrUnsupported }

