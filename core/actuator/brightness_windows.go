//go:build windows

package actuator

import (
	"errors"
	"fmt"
	goruntime "runtime"
	"syscall"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

type physicalMonitor struct {
	Handle syscall.Handle
	Desc   [128]uint16
}

const monitorDefaultToPrimary = 1

var (
	modUser32             = syscall.NewLazyDLL("user32.dll")
	procGetDesktopWindow  = modUser32.NewProc("GetDesktopWindow")
	procMonitorFromWindow = modUser32.NewProc("MonitorFromWindow")

	modDxva2                                    = syscall.NewLazyDLL("dxva2.dll")
	procGetNumberOfPhysicalMonitorsFromHMONITOR = modDxva2.NewProc("GetNumberOfPhysicalMonitorsFromHMONITOR")
	procGetPhysicalMonitorsFromHMONITOR         = modDxva2.NewProc("GetPhysicalMonitorsFromHMONITOR")
	procDestroyPhysicalMonitor                  = modDxva2.NewProc("DestroyPhysicalMonitor")
	procGetMonitorBrightness                    = modDxva2.NewProc("GetMonitorBrightness")
	procSetMonitorBrightness                    = modDxva2.NewProc("SetMonitorBrightness")
)

func SetPrimaryMonitorBrightnessPercent(percent int) error {
	percent = clampBrightnessPercent(percent)

	if err := setPrimaryMonitorBrightnessPercentDXVA2(percent); err == nil {
		return nil
	} else if wmiErr := setBrightnessPercentWMI(percent); wmiErr == nil {
		return nil
	} else {
		return fmt.Errorf("set brightness failed: dxva2: %w; wmi: %w", err, wmiErr)
	}
}

func setPrimaryMonitorBrightnessPercentDXVA2(percent int) error {
	hwnd, _, err := procGetDesktopWindow.Call()
	if hwnd == 0 {
		if err != nil && !errors.Is(err, syscall.Errno(0)) {
			return err
		}
		return errors.New("GetDesktopWindow failed")
	}

	hmon, _, err := procMonitorFromWindow.Call(hwnd, monitorDefaultToPrimary)
	if hmon == 0 {
		if err != nil && !errors.Is(err, syscall.Errno(0)) {
			return err
		}
		return errors.New("MonitorFromWindow failed")
	}

	var count uint32
	r1, _, err := procGetNumberOfPhysicalMonitorsFromHMONITOR.Call(hmon, uintptr(unsafe.Pointer(&count)))
	if r1 == 0 {
		if err != nil && !errors.Is(err, syscall.Errno(0)) {
			return err
		}
		return errors.New("GetNumberOfPhysicalMonitorsFromHMONITOR failed")
	}
	if count == 0 {
		return errors.New("no physical monitors")
	}

	monitors := make([]physicalMonitor, int(count))
	r1, _, err = procGetPhysicalMonitorsFromHMONITOR.Call(hmon, uintptr(count), uintptr(unsafe.Pointer(&monitors[0])))
	if r1 == 0 {
		if err != nil && !errors.Is(err, syscall.Errno(0)) {
			return err
		}
		return errors.New("GetPhysicalMonitorsFromHMONITOR failed")
	}

	defer func() {
		for i := range monitors {
			h := monitors[i].Handle
			if h == 0 {
				continue
			}
			_, _, _ = procDestroyPhysicalMonitor.Call(uintptr(h))
		}
	}()

	h := monitors[0].Handle
	if h == 0 {
		return errors.New("physical monitor handle is 0")
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
			return err
		}
		return errors.New("GetMonitorBrightness failed")
	}
	if max <= min {
		return fmt.Errorf("brightness range invalid: min=%d max=%d", min, max)
	}

	rangeVal := max - min
	newVal := min + uint32((uint64(rangeVal)*uint64(percent)+50)/100)
	if newVal < min {
		newVal = min
	}
	if newVal > max {
		newVal = max
	}

	r1, _, err = procSetMonitorBrightness.Call(uintptr(h), uintptr(newVal))
	if r1 == 0 {
		if err != nil && !errors.Is(err, syscall.Errno(0)) {
			return err
		}
		return errors.New("SetMonitorBrightness failed")
	}
	return nil
}

func setBrightnessPercentWMI(percent int) error {
	goruntime.LockOSThread()
	defer goruntime.UnlockOSThread()

	comOK := false
	if err := ole.CoInitialize(0); err != nil {
		if oe, ok := err.(*ole.OleError); !ok || oe.Code() != 1 {
			return err
		}
		comOK = true
	} else {
		comOK = true
	}
	if comOK {
		defer ole.CoUninitialize()
	}

	unknown, err := oleutil.CreateObject("WbemScripting.SWbemLocator")
	if err != nil {
		return err
	}
	defer unknown.Release()

	locator, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return err
	}
	defer locator.Release()

	svcVar, err := oleutil.CallMethod(locator, "ConnectServer", nil, "root\\wmi")
	if err != nil {
		svcVar, err = oleutil.CallMethod(locator, "ConnectServer", ".", "root\\wmi")
		if err != nil {
			return err
		}
	}
	if svcVar == nil {
		return errors.New("wmi service is nil")
	}
	defer func() { _ = svcVar.Clear() }()
	svc := svcVar.ToIDispatch()
	if svc == nil {
		return errors.New("wmi service is nil")
	}

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

	setVar, err := oleutil.CallMethod(svc, "ExecQuery", "SELECT * FROM WmiMonitorBrightnessMethods")
	if err != nil {
		return err
	}
	if setVar == nil {
		return errors.New("wmi query result is nil")
	}
	defer func() { _ = setVar.Clear() }()
	set := setVar.ToIDispatch()
	if set == nil {
		return errors.New("wmi query result is nil")
	}

	called := false
	var lastSetErr error
	err = oleutil.ForEach(set, func(v *ole.VARIANT) error {
		defer func() { _ = v.Clear() }()
		item := v.ToIDispatch()
		if item == nil {
			return nil
		}

		// WmiSetBrightness(Timeout, BrightnessPercent)
		if err := callWmiSetBrightness(item, percent); err != nil {
			lastSetErr = err
			return nil // try next instance
		}
		called = true
		return errWMIFound // stop iteration
	})
	if err != nil && !errors.Is(err, errWMIFound) {
		return err
	}
	if !called {
		if lastSetErr != nil {
			return lastSetErr
		}
		return errors.New("wmi brightness methods not found")
	}
	return nil
}

var errWMIFound = errors.New("wmi: found")

func callWmiSetBrightness(item *ole.IDispatch, percent int) error {
	if item == nil {
		return errors.New("wmi item is nil")
	}
	percent = clampBrightnessPercent(percent)
	attempts := [][2]interface{}{
		{uint32(0), uint8(percent)}, // per WMI docs: (uint32 Timeout, uint8 Brightness)
		{int32(0), int32(percent)},  // common fallback: VT_I4 args
		{int32(0), uint8(percent)},  // mixed variants
		{uint32(0), int32(percent)}, // mixed variants
		{int(0), int(percent)},      // generic
	}

	var errs []error
	for _, a := range attempts {
		res, err := oleutil.CallMethod(item, "WmiSetBrightness", a[0], a[1])
		if res != nil {
			_ = res.Clear()
		}
		if err == nil {
			return nil
		}
		errs = append(errs, err)
	}
	if len(errs) == 0 {
		return errors.New("wmi set brightness failed")
	}
	return fmt.Errorf("wmi set brightness failed: %w", errors.Join(errs...))
}

func clampBrightnessPercent(percent int) int {
	if percent < 0 {
		return 0
	}
	if percent > 100 {
		return 100
	}
	return percent
}
