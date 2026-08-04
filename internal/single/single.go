//go:build windows

// Package single, ayni isten ikinci bir kopyanin calismasini engeller.
//
// Izleyici artik terminalden de ayri bir process olarak baslatildigi icin
// gerekli: menuden iki kez baslatmak veya zamanlanmis gorev zaten
// calisirken elle baslatmak iki izleyici uretirdi. Iki izleyici gunluge
// cift kayit yazar ve kapida iki pencere acar.
package single

import "golang.org/x/sys/windows"

// Acquire, verilen adi kilitler. Kilit baskasindaysa ok false doner.
//
// Local\ ad alani kullaniliyor: kapsam tek oturum, ek yetki gerekmiyor.
func Acquire(name string) (release func(), ok bool) {
	p, err := windows.UTF16PtrFromString(`Local\` + name)
	if err != nil {
		return func() {}, false
	}
	h, err := windows.CreateMutex(nil, false, p)
	// CreateMutex zaten var olan bir mutex icin gecerli bir tanitici ile
	// birlikte ERROR_ALREADY_EXISTS doner; tanitici yine kapatilmali.
	if err != nil {
		if h != 0 {
			windows.CloseHandle(h)
		}
		return func() {}, false
	}
	return func() { windows.CloseHandle(h) }, true
}
