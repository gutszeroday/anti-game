package task

import (
	"strings"
	"testing"
)

func TestXMLContainsRestartPolicy(t *testing.T) {
	x := XML(`C:\bin\antigame.exe`, `MAKINE\guts`)
	if !strings.Contains(x, "<RestartOnFailure>") {
		t.Error("yeniden baslatma politikasi yok")
	}
	if !strings.Contains(x, "<Interval>PT1M</Interval>") {
		t.Error("yeniden baslatma araligi 1 dakika olmali")
	}
	if !strings.Contains(x, "<Count>3</Count>") {
		t.Error("yeniden baslatma denemesi 3 olmali")
	}
}

func TestXMLRunsWatchSubcommandAtLogon(t *testing.T) {
	x := XML(`C:\bin\antigame.exe`, `MAKINE\guts`)
	if !strings.Contains(x, "<LogonTrigger>") {
		t.Error("oturum acilis tetikleyicisi yok")
	}
	if !strings.Contains(x, "<Arguments>watch</Arguments>") {
		t.Error("watch alt komutu calistirilmiyor")
	}
	if !strings.Contains(x, `<Command>C:\bin\antigame.exe</Command>`) {
		t.Error("exe yolu XML'e yazilmadi")
	}
}

func TestXMLHasNoExecutionTimeLimit(t *testing.T) {
	// Izleyici surekli calisir; varsayilan 72 saatlik sinir gorevi oldurur.
	x := XML(`C:\bin\antigame.exe`, `MAKINE\guts`)
	if !strings.Contains(x, "<ExecutionTimeLimit>PT0S</ExecutionTimeLimit>") {
		t.Error("calisma sure siniri kaldirilmamis")
	}
}

func TestXMLRunsWithLeastPrivilege(t *testing.T) {
	x := XML(`C:\bin\antigame.exe`, `MAKINE\guts`)
	if !strings.Contains(x, "<RunLevel>LeastPrivilege</RunLevel>") {
		t.Error("gorev yonetici yetkisiyle calisiyor; gerekli degil")
	}
}

func TestXMLEscapesUserID(t *testing.T) {
	x := XML(`C:\bin\a.exe`, `MAKINE\ali&veli`)
	if strings.Contains(x, "ali&veli") {
		t.Error("XML ozel karakterleri kacislanmamis")
	}
	if !strings.Contains(x, "ali&amp;veli") {
		t.Error("& karakteri &amp; olarak kacislanmalıydı")
	}
}

func TestUTF16WithBOM(t *testing.T) {
	b := utf16LE("ab")
	want := []byte{0xff, 0xfe, 'a', 0x00, 'b', 0x00}
	if len(b) != len(want) {
		t.Fatalf("uzunluk %d, %d bekleniyordu", len(b), len(want))
	}
	for i := range want {
		if b[i] != want[i] {
			t.Fatalf("bayt %d: %#x, %#x bekleniyordu", i, b[i], want[i])
		}
	}
}
