//go:build windows

// Package tray, izleyici calisirken bildirim alaninda bir simge gosterir.
// Kapi penceresi gibi ham Win32: bu kod izleyiciyle ayni process'te durur,
// GUI kutuphanesi gomulmek bellek butcesini bozardi.
//
// Menude "izleyiciyi durdur" bilerek yok. Tek tikla kapatilabilen bir
// izleyici, atlatmayi fark edilir bir eylem olmaktan cikarirdi (spec §3).
package tray

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

type Item struct {
	Label string
	Run   func()
	// Default, simgeye cift tiklandiginda calisacak ogeyi isaretler.
	// Alternatif olarak "listenin ilki varsayilandir" denebilirdi; o
	// zaman menu sirasi degistiginde sessizce baska bir eylem
	// calisirdi.
	Default bool
}

// defaultItem, cift tiklamada calisacak ogenin sirasini verir.
// Isaretli oge yoksa -1.
func defaultItem(items []Item) int {
	for i, it := range items {
		if it.Default {
			return i
		}
	}
	return -1
}

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	shell32  = windows.NewLazySystemDLL("shell32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procRegisterClassEx = user32.NewProc("RegisterClassExW")
	procCreateWindowEx  = user32.NewProc("CreateWindowExW")
	procDestroyWindow   = user32.NewProc("DestroyWindow")
	procDefWindowProc   = user32.NewProc("DefWindowProcW")
	procGetMessage      = user32.NewProc("GetMessageW")
	procTranslateMsg    = user32.NewProc("TranslateMessage")
	procDispatchMessage = user32.NewProc("DispatchMessageW")
	procPostQuitMessage = user32.NewProc("PostQuitMessage")
	procPostMessage     = user32.NewProc("PostMessageW")
	procLoadIcon        = user32.NewProc("LoadIconW")
	procCreatePopupMenu = user32.NewProc("CreatePopupMenu")
	procAppendMenu      = user32.NewProc("AppendMenuW")
	procDestroyMenu     = user32.NewProc("DestroyMenu")
	procTrackPopupMenu  = user32.NewProc("TrackPopupMenu")
	procSetForeground   = user32.NewProc("SetForegroundWindow")
	procGetCursorPos    = user32.NewProc("GetCursorPos")
	procMessageBox      = user32.NewProc("MessageBoxW")
	procShellNotifyIcon = shell32.NewProc("Shell_NotifyIconW")
	procGetModuleHandle = kernel32.NewProc("GetModuleHandleW")
)

const (
	wmDestroy     = 0x0002
	wmRButtonUp   = 0x0205
	wmLButtonUp   = 0x0202
	wmLButtonDbl  = 0x0203
	wmApp         = 0x8000
	wmTrayMessage = wmApp + 1
	wmTrayQuit    = wmApp + 2

	nimAdd    = 0x0000
	nimDelete = 0x0002

	nifMessage = 0x0001
	nifIcon    = 0x0002
	nifTip     = 0x0004

	tpmRightButton = 0x0002
	tpmReturnCmd   = 0x0100
	mfString       = 0x0000

	idiApplication = 32512
	mbIconInfo     = 0x00000040

	className = "AntigameTray"

	// idFirst, menu ogelerinin komut kimliklerinin basladigi yerdir.
	// TrackPopupMenu secim yapilmadiginda 0 dondurur, bu yuzden 0'dan
	// uzak durmak gerekiyor.
	idFirst = 1000
)

type wndClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   windows.Handle
	Icon       windows.Handle
	Cursor     windows.Handle
	Background windows.Handle
	MenuName   *uint16
	ClassName  *uint16
	IconSm     windows.Handle
}

type msgStruct struct {
	HWnd    windows.Handle
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
}

type notifyIconData struct {
	CbSize           uint32
	HWnd             windows.Handle
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            windows.Handle
	SzTip            [128]uint16
	DwState          uint32
	DwStateMask      uint32
	SzInfo           [256]uint16
	UVersion         uint32
	SzInfoTitle      [64]uint16
	DwInfoFlags      uint32
	GuidItem         windows.GUID
	HBalloonIcon     windows.Handle
}

// Pencere durumu paket duzeyinde: WndProc bir C geri cagrimidir ve kapali
// degisken tasiyamaz. Process basina tek tepsi simgesi acildigi icin guvenli.
var curItems []Item

// Pencere sinifi process basina bir kez kaydedilir; ikinci kayit
// "sinif zaten var" hatasi verir.
var (
	classOnce sync.Once
	classErr  error
)

func registerClass() error {
	classOnce.Do(func() {
		hInst, _, _ := procGetModuleHandle.Call(0)
		wc := wndClassEx{
			Size:      uint32(unsafe.Sizeof(wndClassEx{})),
			WndProc:   windows.NewCallback(wndProc),
			Instance:  windows.Handle(hInst),
			ClassName: utf16(className),
		}
		if r, _, err := procRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
			classErr = fmt.Errorf("tepsi pencere sinifi kaydedilemedi: %w", err)
		}
	})
	return classErr
}

func utf16(s string) *uint16 {
	p, _ := windows.UTF16PtrFromString(s)
	return p
}

// Info, bilgilendirme kutusu gosterir. Arka planda calisan izleyicinin
// konsolu olmadigi icin metni baska turlu gosterecek yer yok.
func Info(title, body string) {
	procMessageBox.Call(0,
		uintptr(unsafe.Pointer(utf16(body))),
		uintptr(unsafe.Pointer(utf16(title))),
		mbIconInfo)
}

func showMenu(hwnd uintptr) {
	m, _, _ := procCreatePopupMenu.Call()
	if m == 0 {
		return
	}
	defer procDestroyMenu.Call(m)

	for i, it := range curItems {
		procAppendMenu.Call(m, mfString, uintptr(idFirst+i),
			uintptr(unsafe.Pointer(utf16(it.Label))))
	}

	var pt struct{ X, Y int32 }
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	// Menunun disina tiklayinca kapanmasi icin pencere on plana alinmali;
	// aksi halde Windows menuyu acik birakir.
	procSetForeground.Call(hwnd)

	cmd, _, _ := procTrackPopupMenu.Call(m, tpmRightButton|tpmReturnCmd,
		uintptr(pt.X), uintptr(pt.Y), 0, hwnd, 0)
	i := int(cmd) - idFirst
	if i < 0 || i >= len(curItems) || curItems[i].Run == nil {
		return
	}
	// Eylem mesaj dongusunu bloklamamali: rapor tarayici aciyor, bilgi
	// kutusu kendi modal dongusunu kuruyor.
	go curItems[i].Run()
}

func wndProc(hwnd, message, wparam, lparam uintptr) uintptr {
	switch message {
	case wmTrayMessage:
		switch uint32(lparam) {
		case wmRButtonUp:
			showMenu(hwnd)
			return 0
		case wmLButtonDbl:
			// Cift tiklama tek tiklamayi da uretir; sol tik menuyu
			// acsaydi menu acilip hemen kapanirdi. Windows'ta tepsi
			// menusu zaten sag tikla acilir.
			if i := defaultItem(curItems); i >= 0 && curItems[i].Run != nil {
				go curItems[i].Run()
			}
			return 0
		}
	case wmTrayQuit, wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := procDefWindowProc.Call(hwnd, message, wparam, lparam)
	return r
}

// Run, simgeyi ekler ve mesaj dongusunu calistirir. ctx iptal edilene kadar
// bloklar, cikarken simgeyi kaldirir.
//
// Cagiran goroutine bir OS thread'ine kilitleniyor: pencere mesajlari
// yalnizca pencereyi olusturan thread'e teslim edilir.
func Run(ctx context.Context, tip string, items []Item) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	curItems = items

	if err := registerClass(); err != nil {
		return err
	}
	hInst, _, _ := procGetModuleHandle.Call(0)

	// Pencere hicbir zaman gosterilmiyor; yalnizca simgenin mesajlarini
	// alacak bir alici olarak var.
	hwnd, _, err := procCreateWindowEx.Call(0,
		uintptr(unsafe.Pointer(utf16(className))),
		uintptr(unsafe.Pointer(utf16("antigame"))),
		0, 0, 0, 0, 0, 0, 0, hInst, 0)
	if hwnd == 0 {
		return fmt.Errorf("tepsi penceresi olusturulamadi: %w", err)
	}
	defer procDestroyWindow.Call(hwnd)

	icon, _, _ := procLoadIcon.Call(0, idiApplication)
	nid := notifyIconData{
		HWnd:             windows.Handle(hwnd),
		UID:              1,
		UFlags:           nifMessage | nifIcon | nifTip,
		UCallbackMessage: wmTrayMessage,
		HIcon:            windows.Handle(icon),
	}
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	copy(nid.SzTip[:len(nid.SzTip)-1], windows.StringToUTF16(tip))

	if r, _, err := procShellNotifyIcon.Call(nimAdd, uintptr(unsafe.Pointer(&nid))); r == 0 {
		return fmt.Errorf("tepsi simgesi eklenemedi: %w", err)
	}
	defer procShellNotifyIcon.Call(nimDelete, uintptr(unsafe.Pointer(&nid)))

	if ctx != nil {
		go func() {
			<-ctx.Done()
			procPostMessage.Call(hwnd, wmTrayQuit, 0, 0)
		}()
	}

	var msg msgStruct
	for {
		r, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) <= 0 {
			return nil
		}
		procTranslateMsg.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
	}
}
