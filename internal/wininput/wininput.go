//go:build windows

// Package wininput, AFK tespiti ve odak orneklemesi icin gereken iki Win32
// cagrisini sarmalar. Pencere basligi bilerek okunmaz: basliklar tarayici
// sekmesi, dosya adi, sohbet ismi sizdirir.
package wininput

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32                  = windows.NewLazySystemDLL("user32.dll")
	kernel32                = windows.NewLazySystemDLL("kernel32.dll")
	procGetLastInputInfo    = user32.NewProc("GetLastInputInfo")
	procGetForegroundWindow = user32.NewProc("GetForegroundWindow")
	procGetWindowThreadPID  = user32.NewProc("GetWindowThreadProcessId")
	procGetTickCount64      = kernel32.NewProc("GetTickCount64")
)

type lastInputInfo struct {
	cbSize uint32
	dwTime uint32
}

// IdleSeconds, son klavye veya fare girdisinden bu yana gecen saniyeyi dondurur.
//
// Bilinen sinir: oyun kumandasi girdisi bu API tarafindan gorulmez.
// Yalnizca kumandayla oynanan oyunlarda aktif sure oldugundan dusuk cikar.
func IdleSeconds() (int, error) {
	info := lastInputInfo{cbSize: uint32(unsafe.Sizeof(lastInputInfo{}))}
	r, _, err := procGetLastInputInfo.Call(uintptr(unsafe.Pointer(&info)))
	if r == 0 {
		return 0, err
	}
	ticks, _, _ := procGetTickCount64.Call()

	// dwTime 32 bit ve yaklasik 49 gunde bir basa sarar; GetTickCount64'un
	// alt 32 bitiyle karsilastirarak sarma sonrasi negatif deger uretmeyiz.
	elapsed := uint32(ticks) - info.dwTime
	return int(elapsed / 1000), nil
}

// ForegroundPID, odaktaki pencerenin process kimligini dondurur.
// Odakta pencere yoksa (kilit ekrani, masaustu gecisi) 0 doner.
func ForegroundPID() (int, error) {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return 0, nil
	}
	var pid uint32
	r, _, err := procGetWindowThreadPID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if r == 0 {
		return 0, err
	}
	return int(pid), nil
}
