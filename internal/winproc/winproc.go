//go:build windows

// Package winproc, izleyicinin ihtiyac duydugu process islemlerini sarmalar.
package winproc

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	psapi       = windows.NewLazySystemDLL("psapi.dll")
	procEmptyWS = psapi.NewProc("EmptyWorkingSet")
)

// Proc, calisan bir process'in izleyici icin gereken minimum bilgisidir.
// Tam yol bilerek yok: her turda her process icin yol sorgulamak pahalidir.
type Proc struct {
	PID int
	Exe string
}

// List, calisan process'leri dondurur.
func List() ([]Proc, error) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, fmt.Errorf("process anlik goruntusu alinamadi: %w", err)
	}
	defer windows.CloseHandle(snap)

	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	if err := windows.Process32First(snap, &e); err != nil {
		return nil, err
	}

	// Tipik bir Windows oturumunda 200-400 process olur; bastan yer ayirmak
	// her turda yeniden buyume maliyetini onler.
	out := make([]Proc, 0, 400)
	for {
		out = append(out, Proc{
			PID: int(e.ProcessID),
			Exe: windows.UTF16ToString(e.ExeFile[:]),
		})
		if err := windows.Process32Next(snap, &e); err != nil {
			if err == syscall.ERROR_NO_MORE_FILES {
				break
			}
			return nil, err
		}
	}
	return out, nil
}

// Path, process'in tam yolunu dondurur. Korumali process'lerde erisim
// reddedilebilir; cagiran taraf hatayi yol bilinmiyor diye yorumlamalidir.
func Path(pid int) (string, error) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(h)

	buf := make([]uint16, windows.MAX_LONG_PATH)
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &size); err != nil {
		return "", err
	}
	return windows.UTF16ToString(buf[:size]), nil
}

func Terminate(pid int) error {
	h, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		return fmt.Errorf("process %d acilamadi: %w", pid, err)
	}
	defer windows.CloseHandle(h)
	if err := windows.TerminateProcess(h, 1); err != nil {
		return fmt.Errorf("process %d sonlandirilamadi: %w", pid, err)
	}
	return nil
}

// Trim, izleyicinin calisma kumesini bosaltir. Sayfalar standby listesine
// gider ve gerektiginde geri yuklenir; cogu zaman uyuyan bir dongu icin
// maliyeti ihmal edilebilir, kazanci kalici bellek tabaninda buyuktur.
func Trim() error {
	r, _, err := procEmptyWS.Call(uintptr(windows.CurrentProcess()))
	if r == 0 {
		return err
	}
	return nil
}
