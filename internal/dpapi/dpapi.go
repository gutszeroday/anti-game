//go:build windows

// Package dpapi, Windows Veri Koruma API'sini sarmalar. Sifre cozme yalnizca
// ayni kullanici hesabinda ve ayni makinede mumkundur; bu, secret.bin'in
// baska bir makineye kopyalanip okunmasini engeller.
package dpapi

import (
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	crypt32       = windows.NewLazySystemDLL("crypt32.dll")
	kernel32      = windows.NewLazySystemDLL("kernel32.dll")
	procProtect   = crypt32.NewProc("CryptProtectData")
	procUnprotect = crypt32.NewProc("CryptUnprotectData")
	procLocalFree = kernel32.NewProc("LocalFree")
)

type dataBlob struct {
	cbData uint32
	pbData *byte
}

func newBlob(b []byte) dataBlob {
	if len(b) == 0 {
		return dataBlob{}
	}
	return dataBlob{cbData: uint32(len(b)), pbData: &b[0]}
}

// bytes, blob icerigini Go yonetimindeki yeni bir dilime kopyalar.
func (b dataBlob) bytes() []byte {
	if b.cbData == 0 || b.pbData == nil {
		return nil
	}
	out := make([]byte, b.cbData)
	copy(out, unsafe.Slice(b.pbData, b.cbData))
	return out
}

func call(proc *windows.LazyProc, in []byte) ([]byte, error) {
	inBlob := newBlob(in)
	var outBlob dataBlob
	r, _, err := proc.Call(
		uintptr(unsafe.Pointer(&inBlob)),
		0, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(&outBlob)),
	)
	if r == 0 {
		if err == nil {
			return nil, errors.New("dpapi cagrisi basarisiz")
		}
		return nil, err
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(outBlob.pbData)))
	return outBlob.bytes(), nil
}

func Protect(plain []byte) ([]byte, error) { return call(procProtect, plain) }

func Unprotect(blob []byte) ([]byte, error) { return call(procUnprotect, blob) }
