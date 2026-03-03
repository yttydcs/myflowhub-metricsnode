//go:build windows

package metrics

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	goruntime "runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"github.com/moutend/go-wca/pkg/wca"
	"golang.org/x/sys/windows"
)

const (
	defaultBatteryPollInterval    = 30 * time.Second
	defaultVolumePollInterval     = 1 * time.Second
	defaultBrightnessPollInterval = 2 * time.Second
	defaultCPUPollInterval        = 2 * time.Second
	defaultMemPollInterval        = 2 * time.Second
	defaultNetPollInterval        = 5 * time.Second
	brightnessErrLogMinInterval   = 60 * time.Second
	cpuErrLogMinInterval          = 60 * time.Second
	memErrLogMinInterval          = 60 * time.Second
	netErrLogMinInterval          = 60 * time.Second
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
	procGetSystemTimes       = modKernel32.NewProc("GetSystemTimes")
	procGlobalMemoryStatusEx = modKernel32.NewProc("GlobalMemoryStatusEx")

	modUser32             = syscall.NewLazyDLL("user32.dll")
	procGetDesktopWindow  = modUser32.NewProc("GetDesktopWindow")
	procMonitorFromWindow = modUser32.NewProc("MonitorFromWindow")

	modDxva2                                    = syscall.NewLazyDLL("dxva2.dll")
	procGetNumberOfPhysicalMonitorsFromHMONITOR = modDxva2.NewProc("GetNumberOfPhysicalMonitorsFromHMONITOR")
	procGetPhysicalMonitorsFromHMONITOR         = modDxva2.NewProc("GetPhysicalMonitorsFromHMONITOR")
	procDestroyPhysicalMonitor                  = modDxva2.NewProc("DestroyPhysicalMonitor")
	procGetMonitorBrightness                    = modDxva2.NewProc("GetMonitorBrightness")
)

type EnabledFunc func(metric string) bool
type ConfigChangedFunc func() <-chan struct{}

func anyMetricEnabled(enabled EnabledFunc, metrics ...string) bool {
	if enabled == nil {
		return true
	}
	for _, m := range metrics {
		if enabled(m) {
			return true
		}
	}
	return false
}

func configChangedChan(configChanged ConfigChangedFunc) <-chan struct{} {
	if configChanged == nil {
		return nil
	}
	return configChanged()
}

func waitUntilEnabled(ctx context.Context, enabled EnabledFunc, configChanged ConfigChangedFunc, metrics ...string) bool {
	if ctx == nil {
		return false
	}
	if anyMetricEnabled(enabled, metrics...) {
		return true
	}
	ch := configChangedChan(configChanged)
	for !anyMetricEnabled(enabled, metrics...) {
		if ch == nil {
			select {
			case <-ctx.Done():
				return false
			}
		} else {
			select {
			case <-ctx.Done():
				return false
			case <-ch:
			}
		}
		ch = configChangedChan(configChanged)
	}
	return true
}

func StartPlatformCollectors(ctx context.Context, log *slog.Logger, emit EmitFunc, enabled EnabledFunc, configChanged ConfigChangedFunc) {
	if ctx == nil || emit == nil {
		return
	}
	go batteryLoop(ctx, log, emit, enabled, configChanged)
	go volumeLoop(ctx, log, emit, enabled, configChanged)
	go brightnessLoop(ctx, log, emit, enabled, configChanged)
	go cpuLoop(ctx, log, emit, enabled, configChanged)
	go memLoop(ctx, log, emit, enabled, configChanged)
	go netLoop(ctx, log, emit, enabled, configChanged)
}

func batteryLoop(ctx context.Context, log *slog.Logger, emit EmitFunc, enabled EnabledFunc, configChanged ConfigChangedFunc) {
	var ticker *time.Ticker
	defer func() {
		if ticker != nil {
			ticker.Stop()
		}
	}()

	push := func() {
		st, err := readSystemPowerStatus()
		if err != nil {
			if log != nil {
				log.Warn("read battery failed", "err", err.Error())
			}
			emit(MetricBatteryPercent, "-1")
			emit(MetricBatteryOnAC, "-1")
			emit(MetricBatteryCharging, "-1")
			return
		}

		percent, hasBattery, err := batteryPercentFromStatus(st)
		if err != nil {
			if log != nil {
				log.Warn("read battery percent failed", "err", err.Error())
			}
			emit(MetricBatteryPercent, "-1")
		} else if !hasBattery {
			emit(MetricBatteryPercent, "-1")
		} else {
			emit(MetricBatteryPercent, fmt.Sprintf("%d", percent))
		}

		acValue := acLineStatusBoolish(st.ACLineStatus)
		emit(MetricBatteryOnAC, acValue)
		emit(MetricBatteryCharging, acValue)
	}

	for {
		if !waitUntilEnabled(ctx, enabled, configChanged, MetricBatteryPercent, MetricBatteryOnAC, MetricBatteryCharging) {
			return
		}
		if ticker == nil {
			ticker = time.NewTicker(defaultBatteryPollInterval)
			push()
		}

		ch := configChangedChan(configChanged)
		select {
		case <-ctx.Done():
			return
		case <-ch:
			if !anyMetricEnabled(enabled, MetricBatteryPercent, MetricBatteryOnAC, MetricBatteryCharging) {
				ticker.Stop()
				ticker = nil
			}
		case <-ticker.C:
			if !anyMetricEnabled(enabled, MetricBatteryPercent, MetricBatteryOnAC, MetricBatteryCharging) {
				ticker.Stop()
				ticker = nil
				continue
			}
			push()
		}
	}
}

type fileTime struct {
	LowDateTime  uint32
	HighDateTime uint32
}

func (t fileTime) uint64() uint64 {
	return (uint64(t.HighDateTime) << 32) | uint64(t.LowDateTime)
}

func cpuLoop(ctx context.Context, log *slog.Logger, emit EmitFunc, enabled EnabledFunc, configChanged ConfigChangedFunc) {
	var ticker *time.Ticker
	defer func() {
		if ticker != nil {
			ticker.Stop()
		}
	}()

	var lastErrText string
	var lastErrAt time.Time

	var prevIdle, prevKernel, prevUser uint64
	havePrev := false

	initPrev := func() bool {
		lastErrText = ""
		lastErrAt = time.Time{}
		idle, kernel, user, err := readSystemTimes()
		if err != nil {
			havePrev = false
			if log != nil {
				log.Warn("read cpu failed", "err", err.Error())
			}
			emit(MetricCPUPercent, "-1")
			return true
		}
		prevIdle, prevKernel, prevUser = idle, kernel, user
		havePrev = true
		select {
		case <-ctx.Done():
			return false
		case <-time.After(200 * time.Millisecond):
			return true
		}
	}

	push := func() {
		idle, kernel, user, err := readSystemTimes()
		if err != nil {
			errText := err.Error()
			if errText != lastErrText || time.Since(lastErrAt) >= cpuErrLogMinInterval {
				lastErrText = errText
				lastErrAt = time.Now()
				if log != nil {
					log.Warn("read cpu failed", "err", errText)
				}
			}
			emit(MetricCPUPercent, "-1")
			return
		}
		lastErrText = ""
		lastErrAt = time.Time{}

		if !havePrev {
			prevIdle, prevKernel, prevUser = idle, kernel, user
			havePrev = true
			emit(MetricCPUPercent, "-1")
			return
		}
		if idle < prevIdle || kernel < prevKernel || user < prevUser {
			prevIdle, prevKernel, prevUser = idle, kernel, user
			emit(MetricCPUPercent, "-1")
			return
		}

		dIdle := idle - prevIdle
		dKernel := kernel - prevKernel
		dUser := user - prevUser
		prevIdle, prevKernel, prevUser = idle, kernel, user

		total := dKernel + dUser
		if total == 0 || dIdle > total {
			emit(MetricCPUPercent, "-1")
			return
		}

		// On Windows, kernel time includes idle time; busy = (kernel+user)-idle.
		busy := total - dIdle
		percent := int(math.Round(float64(busy) * 100 / float64(total)))
		if percent < 0 {
			percent = 0
		}
		if percent > 100 {
			percent = 100
		}
		emit(MetricCPUPercent, fmt.Sprintf("%d", percent))
	}

	for {
		if !waitUntilEnabled(ctx, enabled, configChanged, MetricCPUPercent) {
			return
		}
		if ticker == nil {
			if ok := initPrev(); !ok {
				return
			}
			ticker = time.NewTicker(defaultCPUPollInterval)
			push()
		}

		ch := configChangedChan(configChanged)
		select {
		case <-ctx.Done():
			return
		case <-ch:
			if !anyMetricEnabled(enabled, MetricCPUPercent) {
				ticker.Stop()
				ticker = nil
				havePrev = false
			}
		case <-ticker.C:
			if !anyMetricEnabled(enabled, MetricCPUPercent) {
				ticker.Stop()
				ticker = nil
				havePrev = false
				continue
			}
			push()
		}
	}
}

type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

func memLoop(ctx context.Context, log *slog.Logger, emit EmitFunc, enabled EnabledFunc, configChanged ConfigChangedFunc) {
	var ticker *time.Ticker
	defer func() {
		if ticker != nil {
			ticker.Stop()
		}
	}()

	var lastErrText string
	var lastErrAt time.Time

	push := func() {
		percent, ok, err := readMemoryLoadPercent()
		if err != nil {
			errText := err.Error()
			if errText != lastErrText || time.Since(lastErrAt) >= memErrLogMinInterval {
				lastErrText = errText
				lastErrAt = time.Now()
				if log != nil {
					log.Warn("read memory failed", "err", errText)
				}
			}
			emit(MetricMemPercent, "-1")
			return
		}
		lastErrText = ""
		lastErrAt = time.Time{}
		if !ok {
			emit(MetricMemPercent, "-1")
			return
		}
		if percent < 0 {
			percent = 0
		}
		if percent > 100 {
			percent = 100
		}
		emit(MetricMemPercent, fmt.Sprintf("%d", percent))
	}

	for {
		if !waitUntilEnabled(ctx, enabled, configChanged, MetricMemPercent) {
			return
		}
		if ticker == nil {
			ticker = time.NewTicker(defaultMemPollInterval)
			push()
		}

		ch := configChangedChan(configChanged)
		select {
		case <-ctx.Done():
			return
		case <-ch:
			if !anyMetricEnabled(enabled, MetricMemPercent) {
				ticker.Stop()
				ticker = nil
			}
		case <-ticker.C:
			if !anyMetricEnabled(enabled, MetricMemPercent) {
				ticker.Stop()
				ticker = nil
				continue
			}
			push()
		}
	}
}

func netLoop(ctx context.Context, log *slog.Logger, emit EmitFunc, enabled EnabledFunc, configChanged ConfigChangedFunc) {
	var ticker *time.Ticker
	defer func() {
		if ticker != nil {
			ticker.Stop()
		}
	}()

	var lastErrText string
	var lastErrAt time.Time

	push := func() {
		online, netType, err := readNetStatus()
		if err != nil {
			errText := err.Error()
			if errText != lastErrText || time.Since(lastErrAt) >= netErrLogMinInterval {
				lastErrText = errText
				lastErrAt = time.Now()
				if log != nil {
					log.Warn("read network failed", "err", errText)
				}
			}
			emit(MetricNetOnline, "-1")
			emit(MetricNetType, "-1")
			return
		}

		lastErrText = ""
		lastErrAt = time.Time{}
		if online {
			emit(MetricNetOnline, "1")
		} else {
			emit(MetricNetOnline, "0")
		}
		emit(MetricNetType, netType)
	}

	for {
		if !waitUntilEnabled(ctx, enabled, configChanged, MetricNetOnline, MetricNetType) {
			return
		}
		if ticker == nil {
			ticker = time.NewTicker(defaultNetPollInterval)
			push()
		}

		ch := configChangedChan(configChanged)
		select {
		case <-ctx.Done():
			return
		case <-ch:
			if !anyMetricEnabled(enabled, MetricNetOnline, MetricNetType) {
				ticker.Stop()
				ticker = nil
			}
		case <-ticker.C:
			if !anyMetricEnabled(enabled, MetricNetOnline, MetricNetType) {
				ticker.Stop()
				ticker = nil
				continue
			}
			push()
		}
	}
}

func volumeLoop(ctx context.Context, log *slog.Logger, emit EmitFunc, enabled EnabledFunc, configChanged ConfigChangedFunc) {
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

	var ticker *time.Ticker
	defer func() {
		if ticker != nil {
			ticker.Stop()
		}
	}()

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

	for {
		if !waitUntilEnabled(ctx, enabled, configChanged, MetricVolumePercent, MetricVolumeMuted) {
			return
		}
		if ticker == nil {
			ticker = time.NewTicker(defaultVolumePollInterval)
			push()
		}

		ch := configChangedChan(configChanged)
		select {
		case <-ctx.Done():
			return
		case <-ch:
			if !anyMetricEnabled(enabled, MetricVolumePercent, MetricVolumeMuted) {
				ticker.Stop()
				ticker = nil
			}
		case <-ticker.C:
			if !anyMetricEnabled(enabled, MetricVolumePercent, MetricVolumeMuted) {
				ticker.Stop()
				ticker = nil
				continue
			}
			push()
		}
	}
}

type physicalMonitor struct {
	Handle syscall.Handle
	Desc   [128]uint16
}

func brightnessLoop(ctx context.Context, log *slog.Logger, emit EmitFunc, enabled EnabledFunc, configChanged ConfigChangedFunc) {
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

	var ticker *time.Ticker
	defer func() {
		if ticker != nil {
			ticker.Stop()
		}
	}()

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

	for {
		if !waitUntilEnabled(ctx, enabled, configChanged, MetricBrightnessPercent) {
			return
		}
		if ticker == nil {
			ticker = time.NewTicker(defaultBrightnessPollInterval)
			push()
		}

		ch := configChangedChan(configChanged)
		select {
		case <-ctx.Done():
			return
		case <-ch:
			if !anyMetricEnabled(enabled, MetricBrightnessPercent) {
				ticker.Stop()
				ticker = nil
			}
		case <-ticker.C:
			if !anyMetricEnabled(enabled, MetricBrightnessPercent) {
				ticker.Stop()
				ticker = nil
				continue
			}
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

func readSystemPowerStatus() (systemPowerStatus, error) {
	var st systemPowerStatus
	r1, _, err := procGetSystemPowerStatus.Call(uintptr(unsafe.Pointer(&st)))
	if r1 == 0 {
		if err != nil && !errors.Is(err, syscall.Errno(0)) {
			return systemPowerStatus{}, err
		}
		return systemPowerStatus{}, errors.New("GetSystemPowerStatus failed")
	}
	return st, nil
}

func batteryPercentFromStatus(st systemPowerStatus) (percent int, hasBattery bool, _ error) {
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

func acLineStatusBoolish(v byte) string {
	switch v {
	case 0:
		return "0"
	case 1:
		return "1"
	default:
		return "-1"
	}
}

func readSystemTimes() (idle, kernel, user uint64, _ error) {
	var idleFT fileTime
	var kernelFT fileTime
	var userFT fileTime
	r1, _, err := procGetSystemTimes.Call(
		uintptr(unsafe.Pointer(&idleFT)),
		uintptr(unsafe.Pointer(&kernelFT)),
		uintptr(unsafe.Pointer(&userFT)),
	)
	if r1 == 0 {
		if err != nil && !errors.Is(err, syscall.Errno(0)) {
			return 0, 0, 0, err
		}
		return 0, 0, 0, errors.New("GetSystemTimes failed")
	}
	return idleFT.uint64(), kernelFT.uint64(), userFT.uint64(), nil
}

func readMemoryLoadPercent() (percent int, ok bool, _ error) {
	var st memoryStatusEx
	st.Length = uint32(unsafe.Sizeof(st))
	r1, _, err := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&st)))
	if r1 == 0 {
		if err != nil && !errors.Is(err, syscall.Errno(0)) {
			return 0, false, err
		}
		return 0, false, errors.New("GlobalMemoryStatusEx failed")
	}
	if st.MemoryLoad > 100 {
		return 0, false, fmt.Errorf("memory load out of range: %d", st.MemoryLoad)
	}
	return int(st.MemoryLoad), true, nil
}

const (
	ifTypeWWANPP  = 243
	ifTypeWWANPP2 = 244
)

func readNetStatus() (online bool, netType string, _ error) {
	flags := uint32(windows.GAA_FLAG_SKIP_ANYCAST | windows.GAA_FLAG_SKIP_MULTICAST | windows.GAA_FLAG_SKIP_DNS_SERVER | windows.GAA_FLAG_SKIP_FRIENDLY_NAME)
	size := uint32(15 * 1024)
	for i := 0; i < 3; i++ {
		buf := make([]byte, size)
		aa := (*windows.IpAdapterAddresses)(unsafe.Pointer(&buf[0]))
		err := windows.GetAdaptersAddresses(windows.AF_UNSPEC, flags, 0, aa, &size)
		if err == windows.ERROR_BUFFER_OVERFLOW {
			continue
		}
		if err != nil {
			return false, "", err
		}

		bestRank := -1
		bestType := "unknown"
		haveCandidate := false
		for p := aa; p != nil; p = p.Next {
			if p == nil {
				break
			}
			if p.OperStatus != windows.IfOperStatusUp {
				continue
			}
			if p.IfType == windows.IF_TYPE_SOFTWARE_LOOPBACK || p.IfType == windows.IF_TYPE_TUNNEL {
				continue
			}
			if p.FirstUnicastAddress == nil {
				continue
			}
			haveCandidate = true
			nt := netTypeFromIfType(p.IfType)
			rank := netTypeRank(nt)
			if rank > bestRank {
				bestRank = rank
				bestType = nt
			}
		}

		if !haveCandidate {
			return false, "none", nil
		}
		if strings.TrimSpace(bestType) == "" {
			bestType = "unknown"
		}
		return true, bestType, nil
	}
	return false, "", errors.New("GetAdaptersAddresses buffer overflow")
}

func netTypeFromIfType(ifType uint32) string {
	switch ifType {
	case windows.IF_TYPE_IEEE80211:
		return "wifi"
	case windows.IF_TYPE_ETHERNET_CSMACD:
		return "ethernet"
	case ifTypeWWANPP, ifTypeWWANPP2:
		return "cellular"
	default:
		return "unknown"
	}
}

func netTypeRank(netType string) int {
	switch netType {
	case "ethernet":
		return 3
	case "wifi":
		return 2
	case "cellular":
		return 1
	case "unknown":
		return 0
	default:
		return -1
	}
}

func readBatteryPercent() (percent int, hasBattery bool, _ error) {
	st, err := readSystemPowerStatus()
	if err != nil {
		return 0, false, err
	}
	return batteryPercentFromStatus(st)
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
