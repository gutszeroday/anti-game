//go:build windows

package main

import (
	"os"

	"golang.org/x/sys/windows"
)

// Konsol baglama. Exe -H=windowsgui ile derleniyor: cift tiklandiginda
// GUI acilirken arkada siyah bir konsol penceresi yanip sonmesin diye.
// Bedeli, komut satirindan calistirildiginda otomatik bir konsolun
// olmamasi; asagidaki kod cagiran terminale geri bagliyor.

var (
	kernel32          = windows.NewLazySystemDLL("kernel32.dll")
	procAttachConsole = kernel32.NewProc("AttachConsole")
)

// attachParentProcess, AttachConsole'a "beni cagiran process'in
// konsoluna bagla" demektir (DWORD olarak -1).
const attachParentProcess = ^uintptr(0)

// attachConsole, ciktinin cagiran terminalde gorunmesini saglar.
//
// Yonlendirme veya boru varsa (antigame report > dosya.txt) kabuk std
// tutamaclari zaten kurmustur; o durumda dokunulmuyor, aksi halde
// dosyaya gitmesi gereken cikti konsola kacardi.
func attachConsole() {
	if h, err := windows.GetStdHandle(windows.STD_OUTPUT_HANDLE); err == nil && h != 0 &&
		h != windows.InvalidHandle {
		return
	}
	if r, _, _ := procAttachConsole.Call(attachParentProcess); r == 0 {
		// Cagiranin konsolu yok (ornegin Gorev Zamanlayici baslatti).
		// Yazacak yer olmamasi calismayi engellemiyor.
		return
	}
	bind(&os.Stdout, "CONOUT$", os.O_WRONLY, windows.STD_OUTPUT_HANDLE)
	bind(&os.Stderr, "CONOUT$", os.O_WRONLY, windows.STD_ERROR_HANDLE)
	bind(&os.Stdin, "CONIN$", os.O_RDONLY, windows.STD_INPUT_HANDLE)
}

// bind, verilen konsol aygitini acar ve hem Go tarafindaki dosyaya hem
// de process'in std tutamacina baglar. Ikincisi, izleyici gibi alt
// process'lerin de ayni konsolu gormesi icin.
func bind(target **os.File, device string, flag int, std uint32) {
	f, err := os.OpenFile(device, flag, 0)
	if err != nil {
		return
	}
	*target = f
	windows.SetStdHandle(std, windows.Handle(f.Fd()))
}
