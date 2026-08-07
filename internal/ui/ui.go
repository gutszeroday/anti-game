//go:build windows

package ui

import (
	"errors"
	"runtime"
	"runtime/debug"
	"unsafe"

	"github.com/guts/antigame/internal/single"
)

// ErrNoGUI, arayuzun acilamadigini soyler. Cagiran bunu gorunce metin
// menusune dusmeli: pencere acilmadi diye program kullanilamaz hale
// gelmemeli.
var ErrNoGUI = errors.New("arayüz açılamadı")

// Deps, arayuzun kendi basina yapamayacagi isleri disaridan alir.
//
// Izleyiciyi baslatmak DETACHED_PROCESS bayragi ve tek-ornek kilidinin
// adini bilmeyi gerektiriyor; ikisi de cmd katmaninin bilgisi. Arayuz
// bunlari ogrenirse ayni mantik iki yerde yasamaya baslar.
type Deps struct {
	// WatcherRunning, izleyici ayakta mi soyler.
	WatcherRunning func() bool
	// StartWatcher, izleyiciyi arka planda baslatir.
	StartWatcher func() error
	// ExePath, zamanlanmis goreve yazilacak calistirilabilir yolu verir.
	ExePath func() (string, error)
}

// uiLock, ayni anda iki pencere acilmasini engeller.
const uiLock = "antigame-ui"

// Run, ana pencereyi acar ve kapanana kadar bloklar.
//
// Pencere zaten aciksa yenisi acilmiyor: kullanici simgeye ikinci kez
// cift tikladiginda ayni pencereyi one getirmek, iki kopya arasinda
// hangisinin dogru oldugunu sormaktan iyi.
func Run(dir string, d Deps) error {
	release, ok := single.Acquire(uiLock)
	if !ok {
		if h, _, _ := procFindWindow.Call(
			uintptr(unsafe.Pointer(utf16(mainClassName))), 0); h != 0 {
			procSetForegroundWindow.Call(h)
		}
		return nil
	}
	defer release()

	if err := initCommonControls(); err != nil {
		return errors.Join(ErrNoGUI, err)
	}

	// Pencere mesajlari yalnizca pencereyi olusturan thread'e teslim
	// edilir; goroutine baska bir OS thread'ine tasinirsa dongu sagir
	// kalir.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	w, err := newMainWindow(dir, d)
	if err != nil {
		return errors.Join(ErrNoGUI, err)
	}
	show(w.hwnd)

	// Arayuz cok az ayirma yapiyor ama diyaloglar tepe yapiyor (QR
	// gorseli, calisan process listesi). Varsayilan ayarlarla o tepe
	// kalici bir tabana donusuyor; izleyicideki ayni care burada da
	// gecerli.
	debug.SetGCPercent(20)
	trim()

	var msg msgStruct
	for {
		r, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) <= 0 {
			return nil
		}
		if d, _, _ := procIsDialogMessage.Call(w.hwnd, uintptr(unsafe.Pointer(&msg))); d != 0 {
			continue
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
	}
}
