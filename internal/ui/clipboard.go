//go:build windows

package ui

import (
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Panoya gizli tutulacak metin yazmak.
//
// Duz bir SetClipboardData yetmiyor: Windows 10'dan beri panoya yazilan
// her sey Win+V gecmisine dusuyor ve hesap acikssa bulut uzerinden diger
// cihazlara senkronlanabiliyor. Kapiyi acan bir anahtar icin bu, dosyayi
// DPAPI ile sifrelemenin anlamini ortadan kaldirirdi. Asagidaki iki
// ozel bicim Windows'a "bunu kaydetme" diyor.

var (
	procOpenClipboard           = user32.NewProc("OpenClipboard")
	procCloseClipboard          = user32.NewProc("CloseClipboard")
	procEmptyClipboard          = user32.NewProc("EmptyClipboard")
	procSetClipboardData        = user32.NewProc("SetClipboardData")
	procRegisterClipboardFormat = user32.NewProc("RegisterClipboardFormatW")
	procGlobalAlloc             = kernel32.NewProc("GlobalAlloc")
	procGlobalFree              = kernel32.NewProc("GlobalFree")
	procGlobalLock              = kernel32.NewProc("GlobalLock")
	procGlobalUnlock            = kernel32.NewProc("GlobalUnlock")
)

const (
	cfUnicodeText = 13
	gmemMoveable  = 0x0002
)

// copySecret, metni panoya yazar ve Windows'un onu saklamasini engeller.
func copySecret(owner uintptr, s string) error {
	if r, _, err := procOpenClipboard.Call(owner); r == 0 {
		return errors.Join(errors.New("pano açılamadı, başka bir uygulama kullanıyor olabilir"), err)
	}
	defer procCloseClipboard.Call()

	if r, _, err := procEmptyClipboard.Call(); r == 0 {
		return errors.Join(errors.New("pano boşaltılamadı"), err)
	}

	h, err := globalText(s)
	if err != nil {
		return err
	}
	if r, _, err := procSetClipboardData.Call(cfUnicodeText, h); r == 0 {
		procGlobalFree.Call(h)
		return errors.Join(errors.New("panoya yazılamadı"), err)
	}
	// Basarili SetClipboardData bellegin sahipligini panoya devreder;
	// GlobalFree cagirmak kullanimdaki bellegi serbest birakirdi.

	excludeFromHistory()
	return nil
}

// excludeFromHistory, pano gecmisi ve bulut senkronu icin "bunu alma"
// isaretlerini koyar. Basarisiz olmasi kopyalamayi gecersiz kilmaz;
// cagiran zaten kullaniciya panoyu temizlemesini soyluyor.
func excludeFromHistory() {
	// Bicimin varligi yeterli, icerigi onemsiz.
	if f := clipboardFormat("ExcludeClipboardContentFromMonitorProcessing"); f != 0 {
		if h, err := globalBytes([]byte{0}); err == nil {
			if r, _, _ := procSetClipboardData.Call(f, h); r == 0 {
				procGlobalFree.Call(h)
			}
		}
	}
	// Bu bicim bir DWORD bekliyor: 0 = gecmise alma.
	if f := clipboardFormat("CanIncludeInClipboardHistory"); f != 0 {
		if h, err := globalBytes([]byte{0, 0, 0, 0}); err == nil {
			if r, _, _ := procSetClipboardData.Call(f, h); r == 0 {
				procGlobalFree.Call(h)
			}
		}
	}
}

func clipboardFormat(name string) uintptr {
	f, _, _ := procRegisterClipboardFormat.Call(uintptr(unsafe.Pointer(utf16(name))))
	return f
}

// globalText, metni panonun bekledigi bicimde (NUL ile biten UTF-16)
// tasinabilir bellege kopyalar.
func globalText(s string) (uintptr, error) {
	u, err := windows.UTF16FromString(s)
	if err != nil {
		return 0, err
	}
	buf := unsafe.Slice((*byte)(unsafe.Pointer(&u[0])), len(u)*2)
	return globalBytes(buf)
}

func globalBytes(b []byte) (uintptr, error) {
	h, _, err := procGlobalAlloc.Call(gmemMoveable, uintptr(len(b)))
	if h == 0 {
		return 0, errors.Join(errors.New("bellek ayrılamadı"), err)
	}
	p, _, _ := procGlobalLock.Call(h)
	if p == 0 {
		procGlobalFree.Call(h)
		return 0, errors.New("bellek kilitlenemedi")
	}
	copy(unsafe.Slice((*byte)(osPointer(p)), len(b)), b)
	procGlobalUnlock.Call(h)
	return h, nil
}
