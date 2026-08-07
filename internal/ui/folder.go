//go:build windows

package ui

import (
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Klasor secici. SHBrowseForFolder seciliyor: COM tabanli IFileOpenDialog
// daha modern gorunuyor ama bu pencerede kazanci yok ve arayuz kodunu
// birkac yuz satir buyuturdu.

var (
	shell32 = windows.NewLazySystemDLL("shell32.dll")
	ole32   = windows.NewLazySystemDLL("ole32.dll")

	procSHBrowseForFolder   = shell32.NewProc("SHBrowseForFolderW")
	procSHGetPathFromIDList = shell32.NewProc("SHGetPathFromIDListW")
	procCoTaskMemFree       = ole32.NewProc("CoTaskMemFree")
	procCoInitializeEx      = ole32.NewProc("CoInitializeEx")
)

// coinitOnce, COM'u process basina bir kez baslatir. BIF_NEWDIALOGSTYLE
// COM istiyor; baslatilmazsa Windows sessizce 1995 gorunumlu eski
// diyaloga duser.
var coinitOnce sync.Once

const coinitApartmentThreaded = 0x2

type browseInfo struct {
	Owner       uintptr
	Root        uintptr
	DisplayName *uint16
	Title       *uint16
	Flags       uint32
	Callback    uintptr
	LParam      uintptr
	Image       int32
}

const (
	bifReturnOnlyFSDirs = 0x00000001
	bifNewDialogStyle   = 0x00000040
	bifEditBox          = 0x00000010
	maxPath             = 260
)

// pickFolder, klasor secme diyalogunu acar. Kullanici vazgecerse bos
// dize doner.
func pickFolder(parent uintptr, title string) string {
	coinitOnce.Do(func() { procCoInitializeEx.Call(0, coinitApartmentThreaded) })

	display := make([]uint16, maxPath)
	bi := browseInfo{
		Owner:       parent,
		DisplayName: &display[0],
		Title:       utf16(title),
		Flags:       bifReturnOnlyFSDirs | bifNewDialogStyle | bifEditBox,
	}

	idl, _, _ := procSHBrowseForFolder.Call(uintptr(unsafe.Pointer(&bi)))
	if idl == 0 {
		return ""
	}
	// Donen liste COM tarafindan ayrildi; cagiran serbest birakmak
	// zorunda.
	defer procCoTaskMemFree.Call(idl)

	buf := make([]uint16, maxPath)
	if r, _, _ := procSHGetPathFromIDList.Call(idl, uintptr(unsafe.Pointer(&buf[0]))); r == 0 {
		return ""
	}
	return windows.UTF16ToString(buf)
}
