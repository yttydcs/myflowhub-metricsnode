//go:build windows

package actuator

import (
	"errors"

	"github.com/moutend/go-wca/pkg/wca"
)

type EndpointVolume struct {
	enumerator *wca.IMMDeviceEnumerator
	device     *wca.IMMDevice
	endpoint   *wca.IAudioEndpointVolume
}

func OpenDefaultEndpointVolume() (*EndpointVolume, error) {
	var enumerator *wca.IMMDeviceEnumerator
	if err := wca.CoCreateInstance(
		wca.CLSID_MMDeviceEnumerator,
		0,
		wca.CLSCTX_ALL,
		wca.IID_IMMDeviceEnumerator,
		&enumerator,
	); err != nil {
		return nil, err
	}
	if enumerator == nil {
		return nil, errors.New("device enumerator nil")
	}

	var device *wca.IMMDevice
	if err := enumerator.GetDefaultAudioEndpoint(wca.ERender, wca.EConsole, &device); err != nil {
		enumerator.Release()
		return nil, err
	}
	if device == nil {
		enumerator.Release()
		return nil, errors.New("default audio device nil")
	}

	var endpoint *wca.IAudioEndpointVolume
	if err := device.Activate(wca.IID_IAudioEndpointVolume, wca.CLSCTX_ALL, nil, &endpoint); err != nil {
		device.Release()
		enumerator.Release()
		return nil, err
	}
	if endpoint == nil {
		device.Release()
		enumerator.Release()
		return nil, errors.New("endpoint volume nil")
	}

	return &EndpointVolume{
		enumerator: enumerator,
		device:     device,
		endpoint:   endpoint,
	}, nil
}

func (v *EndpointVolume) Release() {
	if v == nil {
		return
	}
	if v.endpoint != nil {
		v.endpoint.Release()
		v.endpoint = nil
	}
	if v.device != nil {
		v.device.Release()
		v.device = nil
	}
	if v.enumerator != nil {
		v.enumerator.Release()
		v.enumerator = nil
	}
}

func (v *EndpointVolume) SetPercent(percent int) error {
	if v == nil || v.endpoint == nil {
		return errors.New("endpoint nil")
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	level := float32(float64(percent) / 100.0)
	return v.endpoint.SetMasterVolumeLevelScalar(level, nil)
}

func (v *EndpointVolume) SetMuted(muted bool) error {
	if v == nil || v.endpoint == nil {
		return errors.New("endpoint nil")
	}
	return v.endpoint.SetMute(muted, nil)
}

