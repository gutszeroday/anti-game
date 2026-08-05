//go:build windows

package term

import (
	"os"

	"golang.org/x/sys/windows"
)

// cpUTF8, konsol ciktisinin UTF-8 kod sayfasidir.
const cpUTF8 = 65001

var procSetConsoleOutputCP = windows.NewLazySystemDLL("kernel32.dll").NewProc("SetConsoleOutputCP")

// prepare, konsolu ANSI kacis dizilerine ve UTF-8'e hazirlar.
//
// Ikisi ayri yeteneklerdir: VT acilmadan renk basmak ekrana ham kacis
// dizisi dokerdi, kod sayfasi cevrilmeden de kutu cizgileri ve Turkce
// harfler bozuk cikardi. Hangisi basarisiz olursa yalnizca o ozellik
// kapanir.
func prepare(f *os.File) (color, unicode bool) {
	h := windows.Handle(f.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(h, &mode); err == nil {
		if mode&windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING != 0 {
			color = true
		} else if err := windows.SetConsoleMode(h, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING); err == nil {
			color = true
		}
	}
	if r, _, _ := procSetConsoleOutputCP.Call(uintptr(cpUTF8)); r != 0 {
		unicode = true
	}
	return color, unicode
}
