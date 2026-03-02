//go:build windows

package actuator

import (
	"errors"
	"fmt"
	"syscall"
	"unsafe"
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
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}

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
