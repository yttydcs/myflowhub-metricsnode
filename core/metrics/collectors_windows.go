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
	"github.com/go-ole/go-ole/oleutil"
	"github.com/moutend/go-wca/pkg/wca"
)

const (
	defaultBatteryPollInterval    = 30 * time.Second
	defaultVolumePollInterval     = 1 * time.Second
	defaultBrightnessPollInterval = 2 * time.Second
	brightnessErrLogMinInterval   = 60 * time.Second
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
	modKernel32              = syscall.NewLazyDLL("kernel32.dll")
	procGetSystemPowerStatus = modKernel32.NewProc("GetSystemPowerStatus")

	modUser32             = syscall.NewLazyDLL("user32.dll")
	procGetDesktopWindow  = modUser32.NewProc("GetDesktopWindow")
	procMonitorFromWindow = modUser32.NewProc("MonitorFromWindow")

	modDxva2                                    = syscall.NewLazyDLL("dxva2.dll")
	procGetNumberOfPhysicalMonitorsFromHMONITOR = modDxva2.NewProc("GetNumberOfPhysicalMonitorsFromHMONITOR")
	procGetPhysicalMonitorsFromHMONITOR         = modDxva2.NewProc("GetPhysicalMonitorsFromHMONITOR")
	procDestroyPhysicalMonitor                  = modDxva2.NewProc("DestroyPhysicalMonitor")
	procGetMonitorBrightness                    = modDxva2.NewProc("GetMonitorBrightness")
)

func StartPlatformCollectors(ctx context.Context, log *slog.Logger, emit EmitFunc) {
	if ctx == nil || emit == nil {
		return
	}
	go batteryLoop(ctx, log, emit)
	go volumeLoop(ctx, log, emit)
	go brightnessLoop(ctx, log, emit)
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

type physicalMonitor struct {
	Handle syscall.Handle
	Desc   [128]uint16
}

func brightnessLoop(ctx context.Context, log *slog.Logger, emit EmitFunc) {
	// WMI relies on COM; keep WMI calls inside this goroutine and pinned to one OS thread.
	goruntime.LockOSThread()
	defer goruntime.UnlockOSThread()

	wmiEnabled := false
	if err := ole.CoInitialize(0); err != nil {
		// Treat S_FALSE (already initialized) as success; it's safe and still requires CoUninitialize.
		if oe, ok := err.(*ole.OleError); !ok || oe.Code() != 1 {
			if log != nil {
				log.Warn("ole init failed (brightness)", "err", err.Error())
			}
		} else {
			wmiEnabled = true
		}
	} else {
		wmiEnabled = true
	}
	if wmiEnabled {
		defer ole.CoUninitialize()
	}

	ticker := time.NewTicker(defaultBrightnessPollInterval)
	defer ticker.Stop()

	var lastErrText string
	var lastErrAt time.Time
	var dxva2DisabledUntil time.Time

	var wmiSvc *ole.IDispatch
	if wmiEnabled {
		if svc, err := connectWMIRootWMI(); err == nil {
			wmiSvc = svc
		} else if log != nil {
			log.Warn("wmi connect failed (brightness); fallback to dxva2 only", "err", err.Error())
		}
	}
	defer func() {
		if wmiSvc != nil {
			wmiSvc.Release()
		}
	}()

	push := func() {
		now := time.Now()

		var dxva2Err error
		var wmiErr error

		// 1) Try DXVA2 first, unless temporarily disabled due to repeated failures.
		if now.After(dxva2DisabledUntil) {
			if percent, ok, err := readPrimaryMonitorBrightnessPercentDXVA2(); ok {
				emit(MetricBrightnessPercent, fmt.Sprintf("%d", percent))
				return
			} else if err != nil {
				dxva2Err = err
			}
		}

		// 2) WMI fallback (common for laptop internal panels).
		if wmiSvc != nil {
			if percent, ok, err := readBrightnessPercentWMI(wmiSvc); ok {
				if dxva2Err != nil {
					// Avoid hammering I2C when WMI is available.
					dxva2DisabledUntil = now.Add(30 * time.Second)
				}
				emit(MetricBrightnessPercent, fmt.Sprintf("%d", percent))
				return
			} else if err != nil {
				wmiErr = err
			}
		}

		// 3) If DXVA2 was skipped due to backoff, try it as a last resort.
		if now.Before(dxva2DisabledUntil) {
			if percent, ok, err := readPrimaryMonitorBrightnessPercentDXVA2(); ok {
				emit(MetricBrightnessPercent, fmt.Sprintf("%d", percent))
				return
			} else if err != nil && dxva2Err == nil {
				dxva2Err = err
			}
		}

		if dxva2Err != nil || wmiErr != nil {
			errText := ""
			if wmiErr != nil {
				errText = "wmi: " + wmiErr.Error()
			}
			if dxva2Err != nil {
				if errText != "" {
					errText += "; "
				}
				errText += "dxva2: " + dxva2Err.Error()
			}
			if errText != "" && (errText != lastErrText || time.Since(lastErrAt) >= brightnessErrLogMinInterval) {
				lastErrText = errText
				lastErrAt = now
				if log != nil {
					log.Warn("read brightness failed", "err", errText)
				}
			}
		}

		emit(MetricBrightnessPercent, "-1")
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

const monitorDefaultToPrimary = 1

func readPrimaryMonitorBrightnessPercentDXVA2() (percent int, ok bool, _ error) {
	hwnd, _, err := procGetDesktopWindow.Call()
	if hwnd == 0 {
		if err != nil && !errors.Is(err, syscall.Errno(0)) {
			return 0, false, err
		}
		return 0, false, errors.New("GetDesktopWindow failed")
	}

	hmon, _, err := procMonitorFromWindow.Call(hwnd, monitorDefaultToPrimary)
	if hmon == 0 {
		if err != nil && !errors.Is(err, syscall.Errno(0)) {
			return 0, false, err
		}
		return 0, false, errors.New("MonitorFromWindow failed")
	}

	var count uint32
	r1, _, err := procGetNumberOfPhysicalMonitorsFromHMONITOR.Call(hmon, uintptr(unsafe.Pointer(&count)))
	if r1 == 0 {
		if err != nil && !errors.Is(err, syscall.Errno(0)) {
			return 0, false, err
		}
		return 0, false, errors.New("GetNumberOfPhysicalMonitorsFromHMONITOR failed")
	}
	if count == 0 {
		return 0, false, errors.New("no physical monitors")
	}

	monitors := make([]physicalMonitor, int(count))
	r1, _, err = procGetPhysicalMonitorsFromHMONITOR.Call(hmon, uintptr(count), uintptr(unsafe.Pointer(&monitors[0])))
	if r1 == 0 {
		if err != nil && !errors.Is(err, syscall.Errno(0)) {
			return 0, false, err
		}
		return 0, false, errors.New("GetPhysicalMonitorsFromHMONITOR failed")
	}

	defer func() {
		for i := range monitors {
			h := monitors[i].Handle
			if h == 0 {
				continue
			}
			// Best-effort cleanup; ignore destroy errors.
			_, _, _ = procDestroyPhysicalMonitor.Call(uintptr(h))
		}
	}()

	h := monitors[0].Handle
	if h == 0 {
		return 0, false, errors.New("physical monitor handle is 0")
	}

	var min, cur, max uint32
	r1, _, err = procGetMonitorBrightness.Call(
		uintptr(h),
		uintptr(unsafe.Pointer(&min)),
		uintptr(unsafe.Pointer(&cur)),
		uintptr(unsafe.Pointer(&max)),
	)
	if r1 == 0 {
		if err != nil && !errors.Is(err, syscall.Errno(0)) {
			return 0, false, err
		}
		return 0, false, errors.New("GetMonitorBrightness failed")
	}
	if max <= min {
		return 0, false, fmt.Errorf("brightness range invalid: min=%d max=%d", min, max)
	}
	if cur < min {
		cur = min
	}
	if cur > max {
		cur = max
	}
	percent = int(((cur - min) * 100) / (max - min))
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	return percent, true, nil
}

var errWMIFound = errors.New("wmi: found")

func connectWMIRootWMI() (*ole.IDispatch, error) {
	unknown, err := oleutil.CreateObject("WbemScripting.SWbemLocator")
	if err != nil {
		return nil, err
	}
	defer unknown.Release()

	locator, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return nil, err
	}
	defer locator.Release()

	// ConnectServer optional args: (server, namespace, ...)
	svcVar, err := oleutil.CallMethod(locator, "ConnectServer", nil, "root\\wmi")
	if err != nil {
		svcVar, err = oleutil.CallMethod(locator, "ConnectServer", ".", "root\\wmi")
		if err != nil {
			return nil, err
		}
	}
	if svcVar == nil {
		return nil, errors.New("wmi service is nil")
	}
	svc := svcVar.ToIDispatch()
	if svc == nil {
		_ = svcVar.Clear()
		return nil, errors.New("wmi service is nil")
	}
	// `ToIDispatch` does not AddRef. Keep the service alive after clearing the VARIANT.
	svc.AddRef()
	_ = svcVar.Clear()

	// Best-effort: set impersonation level to impersonate (3).
	secVar, err := oleutil.GetProperty(svc, "Security_")
	if secVar != nil {
		if err == nil {
			sec := secVar.ToIDispatch()
			if sec != nil {
				_, _ = oleutil.PutProperty(sec, "ImpersonationLevel", 3)
			}
		}
		_ = secVar.Clear()
	}
	return svc, nil
}

func readBrightnessPercentWMI(svc *ole.IDispatch) (percent int, ok bool, _ error) {
	if svc == nil {
		return 0, false, nil
	}
	query := "SELECT CurrentBrightness FROM WmiMonitorBrightness WHERE Active=TRUE"
	setVar, err := oleutil.CallMethod(svc, "ExecQuery", query)
	if err != nil {
		return 0, false, err
	}
	if setVar == nil {
		return 0, false, errors.New("wmi query result is nil")
	}
	defer func() { _ = setVar.Clear() }()
	set := setVar.ToIDispatch()
	if set == nil {
		return 0, false, errors.New("wmi query result is nil")
	}

	found := false
	var out int
	err = oleutil.ForEach(set, func(v *ole.VARIANT) error {
		defer func() { _ = v.Clear() }()
		item := v.ToIDispatch()
		if item == nil {
			return nil
		}

		curVar, err := oleutil.GetProperty(item, "CurrentBrightness")
		if err != nil {
			if curVar != nil {
				_ = curVar.Clear()
			}
			return err
		}
		if curVar == nil {
			return nil
		}
		val := curVar.Value()
		_ = curVar.Clear()

		n, ok := variantNumberToInt(val)
		if !ok {
			return nil
		}
		if n < 0 {
			n = 0
		}
		if n > 100 {
			n = 100
		}
		out = n
		found = true
		return errWMIFound
	})
	if err != nil && !errors.Is(err, errWMIFound) {
		return 0, false, err
	}
	if !found {
		return 0, false, nil
	}
	return out, true, nil
}

func variantNumberToInt(v interface{}) (int, bool) {
	switch n := v.(type) {
	case int8:
		return int(n), true
	case uint8:
		return int(n), true
	case int16:
		return int(n), true
	case uint16:
		return int(n), true
	case int32:
		return int(n), true
	case uint32:
		return int(n), true
	case int64:
		return int(n), true
	case uint64:
		if n > uint64(^uint(0)) {
			return 0, false
		}
		return int(n), true
	case int:
		return n, true
	case uint:
		if n > uint(^uint(0)>>1) {
			return 0, false
		}
		return int(n), true
	default:
		return 0, false
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
