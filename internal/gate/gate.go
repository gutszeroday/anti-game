//go:build windows

// Package gate, kapida durdurulan oyun icin kod giris penceresini gosterir.
// Pencere ham Win32'dir: GUI kutuphanesi gomulmedi cunku bu kod yolu
// izleyiciyle ayni binary'de bulunur ve bagimliliklari dusuk tutmak sarttir.
//
// Karar mantiginin tamami internal/auth icindedir ve orada test edilir;
// buradaki kod yalnizca gorsel kabuktur.
package gate

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/guts/antigame/internal/auth"
	"github.com/guts/antigame/internal/config"
	"github.com/guts/antigame/internal/vault"
)

type Params struct {
	AppName    string
	FriendName string
	FriendHint string
	Verify     func(code string) (auth.Outcome, error)
}

// check, girilen kodu kirpip dogrulayiciya iletir.
func (p Params) check(code string) (auth.Outcome, error) {
	if p.Verify == nil {
		return auth.Outcome{}, errors.New("doğrulayıcı tanımlı değil")
	}
	return p.Verify(strings.TrimSpace(code))
}

// AcquireSingleInstance, ayni anda ikinci bir kapi penceresi acilmasini
// engeller. Oyunu ust uste hizli baslatma denemeleri tek pencere uretmelidir.
//
// Local\ ad alani kullaniliyor: kilidin kapsami tek oturumdur ve Global\
// gibi ek yetki gerektirmez.
func AcquireSingleInstance() (func(), bool) {
	name, err := windows.UTF16PtrFromString(`Local\antigame-gate`)
	if err != nil {
		return func() {}, false
	}
	h, err := windows.CreateMutex(nil, false, name)
	if err != nil {
		if h != 0 {
			windows.CloseHandle(h)
		}
		return func() {}, false
	}
	return func() { windows.CloseHandle(h) }, true
}

// Run, cmd tarafindan cagrilan ust seviye giristir.
func Run(dir, appName string) error {
	release, ok := AcquireSingleInstance()
	if !ok {
		// Kapi zaten acik; sessizce cik.
		return nil
	}
	defer release()

	cfg, err := config.Load(dir)
	if err != nil {
		return err
	}
	secret, err := vault.Load(dir)
	if err != nil {
		return err
	}
	v := auth.Verifier{
		Dir:    dir,
		Secret: secret,
		Grace:  time.Duration(cfg.GraceMinutes) * time.Minute,
	}
	return Show(Params{
		AppName:    appName,
		FriendName: cfg.FriendName,
		FriendHint: cfg.FriendHint,
		Verify:     v.Attempt,
	})
}

// ---- Win32 kabugu ----

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")
	gdi32    = windows.NewLazySystemDLL("gdi32.dll")

	procRegisterClassEx = user32.NewProc("RegisterClassExW")
	procCreateWindowEx  = user32.NewProc("CreateWindowExW")
	procDefWindowProc   = user32.NewProc("DefWindowProcW")
	procGetMessage      = user32.NewProc("GetMessageW")
	procTranslateMsg    = user32.NewProc("TranslateMessage")
	procDispatchMessage = user32.NewProc("DispatchMessageW")
	procPostQuitMessage = user32.NewProc("PostQuitMessage")
	procSendMessage     = user32.NewProc("SendMessageW")
	procShowWindow      = user32.NewProc("ShowWindow")
	procSetForeground   = user32.NewProc("SetForegroundWindow")
	procSetFocus        = user32.NewProc("SetFocus")
	procLoadCursor      = user32.NewProc("LoadCursorW")
	procGetModuleHandle = kernel32.NewProc("GetModuleHandleW")
	procGetStockObject  = gdi32.NewProc("GetStockObject")
)

const (
	wsOverlappedWindow = 0x00CF0000
	wsChild            = 0x40000000
	wsVisible          = 0x10000000
	wsBorder           = 0x00800000
	wsTabStop          = 0x00010000
	esNumber           = 0x2000
	esCenter           = 0x0001
	bsDefPushButton    = 0x0001
	ssLeft             = 0x0000

	wmDestroy = 0x0002
	wmCommand = 0x0111
	wmSetText = 0x000C
	wmGetText = 0x000D
	wmSetFont = 0x0030
	wmKeyDown = 0x0100

	vkReturn = 0x0D

	idButton = 102

	swShow          = 5
	cwUseDefault    = 0x80000000
	idcArrow        = 32512
	defaultGUIFont  = 17
	colorWindowPlus = 6
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

// Pencere durumu paket duzeyinde tutuluyor cunku WndProc bir C geri
// cagrimidir ve kapali degisken tasiyamaz. Ayni anda tek kapi acildigi
// icin (AcquireSingleInstance) bu guvenlidir.
var (
	curParams Params
	hEdit     uintptr
	hStatus   uintptr
	unlocked  bool
)

func utf16(s string) *uint16 {
	p, _ := windows.UTF16PtrFromString(s)
	return p
}

func setText(h uintptr, s string) {
	procSendMessage.Call(h, wmSetText, 0, uintptr(unsafe.Pointer(utf16(s))))
}

func editText() string {
	buf := make([]uint16, 32)
	procSendMessage.Call(hEdit, wmGetText, uintptr(len(buf)), uintptr(unsafe.Pointer(&buf[0])))
	return windows.UTF16ToString(buf)
}

func onSubmit() {
	out, err := curParams.check(editText())
	if err != nil {
		setText(hStatus, "Hata: "+err.Error())
		return
	}
	if out.OK {
		unlocked = true
		setText(hStatus, out.Message)
		procPostQuitMessage.Call(0)
		return
	}
	msg := out.Message
	if out.LockedUntil.IsZero() && out.Remaining > 0 {
		msg = fmt.Sprintf("%s Kalan deneme: %d", msg, out.Remaining)
	}
	setText(hStatus, msg)
	setText(hEdit, "")
	procSetFocus.Call(hEdit)
}

const gateClass = "AntigameGate"

// Pencere sinifi process basina bir kez kaydedilir; ikinci kayit
// "sinif zaten var" hatasi verir ve pencere hic acilmaz.
var (
	classOnce sync.Once
	classErr  error
)

func registerClass() error {
	classOnce.Do(func() {
		hInst, _, _ := procGetModuleHandle.Call(0)
		cursor, _, _ := procLoadCursor.Call(0, idcArrow)
		wc := wndClassEx{
			Size:       uint32(unsafe.Sizeof(wndClassEx{})),
			WndProc:    windows.NewCallback(wndProc),
			Instance:   windows.Handle(hInst),
			Cursor:     windows.Handle(cursor),
			Background: windows.Handle(colorWindowPlus),
			ClassName:  utf16(gateClass),
		}
		if r, _, err := procRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
			classErr = fmt.Errorf("pencere sınıfı kaydedilemedi: %w", err)
		}
	})
	return classErr
}

// wndProc imzasindaki tum parametreler isaretci boyutunda olmak zorunda:
// windows.NewCallback daha dar tipleri kabul etmez ve calisma aninda panikler.
func wndProc(hwnd, message, wparam, lparam uintptr) uintptr {
	switch message {
	case wmCommand:
		if uint16(wparam) == idButton {
			onSubmit()
			return 0
		}
	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := procDefWindowProc.Call(hwnd, message, wparam, lparam)
	return r
}

// Show, kapi penceresini acar ve kod kabul edilene veya pencere
// kapatilana kadar bloklar.
func Show(p Params) error {
	// Windows'ta mesaj kuyrugu thread basinadir: dongu, pencereyi olusturan
	// thread'de donmek zorunda. Kilitlenmezse goroutine syscall'dan
	// dondukten sonra baska bir OS thread'inde uyanabiliyor ve o andan
	// itibaren pencerenin kuyrugu hic bosalmiyor — pencere "Yanit Vermiyor"
	// haline geliyor.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	curParams = p
	unlocked = false

	if err := registerClass(); err != nil {
		return err
	}
	hInst, _, _ := procGetModuleHandle.Call(0)
	className := utf16(gateClass)

	title := fmt.Sprintf("%s kapıda durduruldu", p.AppName)
	hwnd, _, err := procCreateWindowEx.Call(
		0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(utf16(title))),
		wsOverlappedWindow, cwUseDefault, cwUseDefault, 480, 270, 0, 0, hInst, 0,
	)
	if hwnd == 0 {
		return fmt.Errorf("pencere oluşturulamadı: %w", err)
	}

	prompt := fmt.Sprintf("%s açılmadan önce MFA kodu gerekiyor.", p.AppName)
	who := "Kodu arkadaşınızdan isteyin."
	if p.FriendName != "" {
		who = fmt.Sprintf("Kodu %s'ten isteyin.", p.FriendName)
		if p.FriendHint != "" {
			who += " (" + p.FriendHint + ")"
		}
	}

	font, _, _ := procGetStockObject.Call(defaultGUIFont)
	static := utf16("STATIC")
	newChild := func(class *uint16, text string, style uintptr, x, y, w, h int, id uintptr) uintptr {
		r, _, _ := procCreateWindowEx.Call(
			0, uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(utf16(text))),
			wsChild|wsVisible|style, uintptr(x), uintptr(y), uintptr(w), uintptr(h),
			hwnd, id, hInst, 0,
		)
		// Varsayilan font 1995 gorunumlu sistem fontudur; arayuz fontuna gec.
		procSendMessage.Call(r, wmSetFont, font, 1)
		return r
	}

	newChild(static, prompt, ssLeft, 20, 20, 430, 22, 0)
	newChild(static, who, ssLeft, 20, 46, 430, 22, 0)
	hEdit = newChild(utf16("EDIT"), "", wsBorder|wsTabStop|esNumber|esCenter, 20, 86, 200, 30, 0)
	newChild(utf16("BUTTON"), "Aç", wsTabStop|bsDefPushButton, 240, 86, 100, 30, idButton)
	hStatus = newChild(static, "", ssLeft, 20, 134, 430, 60, 0)

	procShowWindow.Call(hwnd, swShow)
	procSetForeground.Call(hwnd)
	procSetFocus.Call(hEdit)

	var m msgStruct
	for {
		r, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		// Enter tusu: diyalog yoneticisi olmadigi icin elle ele aliniyor.
		if m.Message == wmKeyDown && m.WParam == vkReturn {
			onSubmit()
			continue
		}
		procTranslateMsg.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessage.Call(uintptr(unsafe.Pointer(&m)))
	}

	if !unlocked {
		return errors.New("kod girilmeden kapatıldı")
	}
	return nil
}
