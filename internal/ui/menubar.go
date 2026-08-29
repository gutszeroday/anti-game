//go:build windows

package ui

import (
	"fmt"
	"unsafe"
)

// Menu cubugu. Alt dugme sirasi dolmustu: besinci dugme en kucuk
// pencere genisliginde tasiyordu. Menu, sirayi buyutmeden yer aciyor ve
// klavye gezinmesini bedavaya veriyor.
//
// Menu ogeleri dugmelerle ayni komut kimliklerini tasiyor: WM_COMMAND
// her ikisinde de ayni sayiyi getirdigi icin isleyici tek.

var (
	procCreateMenu      = user32.NewProc("CreateMenu")
	procCreatePopupMenu = user32.NewProc("CreatePopupMenu")
	procAppendMenu      = user32.NewProc("AppendMenuW")
	procSetMenu         = user32.NewProc("SetMenu")
	procDrawMenuBar     = user32.NewProc("DrawMenuBar")
)

const (
	mfString    = 0x00000000
	mfPopup     = 0x00000010
	mfSeparator = 0x00000800
)

// menuEntry, tek bir menu ogesidir. Label bossa ayirici cizgidir.
type menuEntry struct {
	Label string
	ID    int
}

var separator = menuEntry{}

// menuBar, cubugun tamami: basliklar ve altlarindaki ogeler.
var menuBar = []struct {
	Title   string
	Entries []menuEntry
}{
	{"&Yönet", []menuEntry{
		{"Oyun &ekle…", idAdd},
		{"Oyun &çıkar", idRemoveGame},
		separator,
		{"&Kişiler…", idPeople},
		{"&Bildirimler…", idNotifications},
		{"&Ayarlar…", idSettings},
		separator,
		{"Kal&dır…", idUninstall},
	}},
	{"&Veriler", []menuEntry{
		{"&Nerede saklanıyor…", idDataInfo},
		{"&Klasörü aç", idOpenFolder},
	}},
	{"&Yardım", []menuEntry{
		{"&Hakkında", idAbout},
	}},
}

// buildMenu, menu cubugunu olusturur ve pencereye takar.
func buildMenu(hwnd uintptr) error {
	bar, _, err := procCreateMenu.Call()
	if bar == 0 {
		return fmt.Errorf("menü çubuğu oluşturulamadı: %w", err)
	}

	for _, m := range menuBar {
		pop, _, err := procCreatePopupMenu.Call()
		if pop == 0 {
			return fmt.Errorf("menü oluşturulamadı (%s): %w", m.Title, err)
		}
		for _, e := range m.Entries {
			if e.Label == "" {
				procAppendMenu.Call(pop, mfSeparator, 0, 0)
				continue
			}
			procAppendMenu.Call(pop, mfString, uintptr(e.ID),
				uintptr(unsafe.Pointer(utf16(e.Label))))
		}
		procAppendMenu.Call(bar, mfPopup, pop,
			uintptr(unsafe.Pointer(utf16(m.Title))))
	}

	if r, _, err := procSetMenu.Call(hwnd, bar); r == 0 {
		return fmt.Errorf("menü çubuğu takılamadı: %w", err)
	}
	procDrawMenuBar.Call(hwnd)
	return nil
}
