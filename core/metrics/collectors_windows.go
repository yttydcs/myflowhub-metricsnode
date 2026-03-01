//go:build windows

package metrics

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	goruntime "runtime"
	"syscall"
	"time"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/moutend/go-wca/pkg/wca"
)

const (
	defaultBatteryPollInterval = 30 * time.Second
	defaultVolumePollInterval  = 1 * time.Second
)

type systemPowerStatus struct {
	ACLineStatus        byte
	BatteryFlag         byte
	BatteryLifePercent  byte
	SystemStatusFlag    byte
	BatteryLifeTime     uint32
	BatteryFullLifeTime uint32
}

var (
	modKernel32             = syscall.NewLazyDLL("kernel32.dll")
	procGetSystemPowerStatus = modKernel32.NewProc("GetSystemPowerStatus")
)

func StartPlatformCollectors(ctx context.Context, log *slog.Logger, emit EmitFunc) {
	if ctx == nil || emit == nil {
		return
	}
	go batteryLoop(ctx, log, emit)
	go volumeLoop(ctx, log, emit)
}

func batteryLoop(ctx context.Context, log *slog.Logger, emit EmitFunc) {
	ticker := time.NewTicker(defaultBatteryPollInterval)
	defer ticker.Stop()

	push := func() {
		percent, hasBattery, err := readBatteryPercent()
		if err != nil {
			if log != nil {
				log.Warn("read battery failed", "err", err.Error())
			}
			return
		}
		if !hasBattery {
			emit(MetricBatteryPercent, "-1")
			return
		}
		emit(MetricBatteryPercent, fmt.Sprintf("%d", percent))
	}

	push()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			push()
		}
	}
}

func volumeLoop(ctx context.Context, log *slog.Logger, emit EmitFunc) {
	// COM must be initialized per-thread; keep all calls inside this goroutine.
	goruntime.LockOSThread()
	defer goruntime.UnlockOSThread()
	if err := ole.CoInitialize(0); err != nil {
		if log != nil {
			log.Warn("ole init failed", "err", err.Error())
		}
		return
	}
	defer ole.CoUninitialize()

	endpoint, release, err := openDefaultEndpointVolume()
	if err != nil {
		if log != nil {
			log.Warn("open endpoint volume failed", "err", err.Error())
		}
		return
	}
	defer release()

	ticker := time.NewTicker(defaultVolumePollInterval)
	defer ticker.Stop()

	push := func() {
		volPercent, muted, err := readEndpointVolume(endpoint)
		if err != nil {
			if log != nil {
				log.Warn("read volume failed", "err", err.Error())
			}
			return
		}
		emit(MetricVolumePercent, fmt.Sprintf("%d", volPercent))
		if muted {
			emit(MetricVolumeMuted, "1")
		} else {
			emit(MetricVolumeMuted, "0")
		}
	}

	push()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			push()
		}
	}
}

func readBatteryPercent() (percent int, hasBattery bool, _ error) {
	var st systemPowerStatus
	r1, _, err := procGetSystemPowerStatus.Call(uintptr(unsafe.Pointer(&st)))
	if r1 == 0 {
		if err != nil && !errors.Is(err, syscall.Errno(0)) {
			return 0, false, err
		}
		return 0, false, errors.New("GetSystemPowerStatus failed")
	}

	// Docs:
	// - BatteryFlag 128 means "No system battery".
	// - BatteryLifePercent 255 means unknown.
	if st.BatteryFlag == 128 || st.BatteryLifePercent == 255 {
		return 0, false, nil
	}
	if st.BatteryLifePercent > 100 {
		return 0, false, fmt.Errorf("battery percent out of range: %d", st.BatteryLifePercent)
	}
	return int(st.BatteryLifePercent), true, nil
}

func openDefaultEndpointVolume() (*wca.IAudioEndpointVolume, func(), error) {
	var enumerator *wca.IMMDeviceEnumerator
	if err := wca.CoCreateInstance(
		wca.CLSID_MMDeviceEnumerator,
		0,
		wca.CLSCTX_ALL,
		wca.IID_IMMDeviceEnumerator,
		&enumerator,
	); err != nil {
		return nil, nil, err
	}
	if enumerator == nil {
		return nil, nil, errors.New("device enumerator nil")
	}

	var device *wca.IMMDevice
	if err := enumerator.GetDefaultAudioEndpoint(wca.ERender, wca.EConsole, &device); err != nil {
		enumerator.Release()
		return nil, nil, err
	}
	if device == nil {
		enumerator.Release()
		return nil, nil, errors.New("default audio device nil")
	}

	var endpoint *wca.IAudioEndpointVolume
	if err := device.Activate(wca.IID_IAudioEndpointVolume, wca.CLSCTX_ALL, nil, &endpoint); err != nil {
		device.Release()
		enumerator.Release()
		return nil, nil, err
	}
	if endpoint == nil {
		device.Release()
		enumerator.Release()
		return nil, nil, errors.New("endpoint volume nil")
	}

	release := func() {
		endpoint.Release()
		device.Release()
		enumerator.Release()
	}
	return endpoint, release, nil
}

func readEndpointVolume(endpoint *wca.IAudioEndpointVolume) (percent int, muted bool, _ error) {
	if endpoint == nil {
		return 0, false, errors.New("endpoint nil")
	}
	var level float32
	if err := endpoint.GetMasterVolumeLevelScalar(&level); err != nil {
		return 0, false, err
	}
	if level < 0 {
		level = 0
	}
	if level > 1 {
		level = 1
	}
	percent = int(math.Round(float64(level * 100)))

	var m bool
	if err := endpoint.GetMute(&m); err != nil {
		return 0, false, err
	}
	return percent, m, nil
}
