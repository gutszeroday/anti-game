package task

import (
	"strings"
	"testing"

	"golang.org/x/sys/windows"
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
	// Gorev konsol penceresi acmadan calismali; izleyici bunu --background
	// ile anliyor ve kendi konsolundan ayriliyor.
	if !strings.Contains(x, "<Arguments>watch --background</Arguments>") {
		t.Error("watch arka plan modunda calistirilmiyor")
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

// Arayuz -H=windowsgui ile derlendigi icin process'in konsolu yok ve
// schtasks bir konsol uygulamasi. Bayrak verilmezse her cagri ekranda
// siyah bir pencere acip kapatiyor ve aktivasyonu caliyor; iki saniyede
// bir tazelenen durum blogu bunu surekli hale getiriyordu.
func TestSchtasksNeverOpensAConsoleWindow(t *testing.T) {
	cases := map[string][]string{
		"query":  {"/Query", "/TN", Name},
		"create": {"/Create", "/TN", Name, "/XML", "x.xml", "/F"},
		"delete": {"/Delete", "/TN", Name, "/F"},
	}
	for name, args := range cases {
		c := command(args...)
		if c.SysProcAttr == nil {
			t.Errorf("%s: SysProcAttr kurulmamis", name)
			continue
		}
		if c.SysProcAttr.CreationFlags&windows.CREATE_NO_WINDOW == 0 {
			t.Errorf("%s: CREATE_NO_WINDOW yok, konsol penceresi acilir", name)
		}
		if !c.SysProcAttr.HideWindow {
			t.Errorf("%s: HideWindow kurulmamis", name)
		}
	}
}

func TestCommandKeepsItsArguments(t *testing.T) {
	c := command("/Query", "/TN", Name)
	got := strings.Join(c.Args[1:], " ")
	want := "/Query /TN " + Name
	if got != want {
		t.Errorf("argumanlar %q, istenen %q", got, want)
	}
}
