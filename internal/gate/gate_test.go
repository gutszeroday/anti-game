//go:build windows

package gate

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/guts/antigame/internal/auth"
	"github.com/guts/antigame/internal/config"
	"github.com/guts/antigame/internal/session"
	"github.com/guts/antigame/internal/store"
)

func TestSingleInstanceBlocksSecondGate(t *testing.T) {
	release, ok := AcquireSingleInstance()
	if !ok {
		t.Fatal("ilk kapi kilidi alinamadi")
	}
	defer release()

	if _, ok := AcquireSingleInstance(); ok {
		t.Fatal("ikinci kapi penceresi acilabildi")
	}
}

func TestSingleInstanceReleasableAndReacquirable(t *testing.T) {
	release, ok := AcquireSingleInstance()
	if !ok {
		t.Fatal("kilit alinamadi")
	}
	release()

	release2, ok := AcquireSingleInstance()
	if !ok {
		t.Fatal("birakildiktan sonra kilit tekrar alinamadi")
	}
	release2()
}

func TestParamsVerifyIsInvokedWithTrimmedCode(t *testing.T) {
	var got string
	p := Params{
		AppName: "Valorant",
		Verify: func(code string) (auth.Outcome, error) {
			got = code
			return auth.Outcome{OK: true}, nil
		},
	}
	out, err := p.check("  123456 \r\n")
	if err != nil {
		t.Fatal(err)
	}
	if !out.OK {
		t.Fatal("dogrulama sonucu tasinmadi")
	}
	if got != "123456" {
		t.Errorf("kod kirpilmadan gecti: %q", got)
	}
}

func TestCheckWithoutVerifierReturnsError(t *testing.T) {
	p := Params{AppName: "Valorant"}
	if _, err := p.check("123456"); err == nil {
		t.Fatal("Verify tanimsizken hata beklendi")
	}
}

func TestAskLineListsPeopleWithHints(t *testing.T) {
	got := AskLine([]config.Person{
		{ID: "p1", Name: "Baran", Hint: "WhatsApp"},
		{ID: "p2", Name: "Ali"},
	})
	want := "Kod kimde: Baran (WhatsApp), Ali"
	if got != want {
		t.Errorf("beklenen %q, gelen %q", want, got)
	}
}

func TestAskLineTruncatesLongList(t *testing.T) {
	var ps []config.Person
	for _, n := range []string{"Baran", "Ali", "Ayşe", "Can", "Deniz"} {
		ps = append(ps, config.Person{Name: n})
	}
	got := AskLine(ps)
	if !strings.Contains(got, "ve 2 kişi daha") {
		t.Errorf("uzun liste kirpilmadi: %q", got)
	}
	if strings.Contains(got, "Deniz") {
		t.Errorf("kirpilmis isim hala yaziliyor: %q", got)
	}
}

func TestAskLineWithoutPeopleFallsBack(t *testing.T) {
	if got := AskLine(nil); got != "Kodu arkadaşınızdan isteyin." {
		t.Errorf("bos listede varsayilan metin yok: %q", got)
	}
}

func TestAskLineDropsHintsWhenLineIsTooLong(t *testing.T) {
	ps := []config.Person{
		{Name: "Baran Yılmaz", Hint: "WhatsApp 0555 111 22 33"},
		{Name: "Ali Kaya", Hint: "Telegram @alikaya"},
		{Name: "Ayşe Demir", Hint: "Instagram @aysedemir"},
	}
	got := AskLine(ps)
	if strings.Contains(got, "WhatsApp") {
		t.Errorf("tasan satirda notlar dusurulmedi: %q", got)
	}
	for _, name := range []string{"Baran Yılmaz", "Ali Kaya", "Ayşe Demir"} {
		if !strings.Contains(got, name) {
			t.Errorf("%s satirdan dustu: %q", name, got)
		}
	}
}

func TestRunManualRefusesWhenTheSessionIsOpen(t *testing.T) {
	dir := t.TempDir()
	if err := config.Save(dir, config.Default()); err != nil {
		t.Fatal(err)
	}
	st := &store.State{}
	session.Open(st, time.Now().UTC(), "")
	if err := store.SaveState(dir, st); err != nil {
		t.Fatal(err)
	}
	if err := RunManual(dir); !errors.Is(err, ErrSessionOpen) {
		t.Errorf("acik oturumda RunManual = %v, istenen ErrSessionOpen", err)
	}
}

func TestManualTextsDoNotNameAGame(t *testing.T) {
	if got := title(""); !strings.Contains(got, "Kod") {
		t.Errorf("manuel baslik yanlis: %q", got)
	}
	if got := prompt(""); strings.HasPrefix(got, " ") {
		t.Errorf("bos oyun adi metinde bosluk birakiyor: %q", got)
	}
	if got := title("Valorant"); !strings.Contains(got, "Valorant") {
		t.Errorf("oyun adli baslik yanlis: %q", got)
	}
	if got := prompt("Valorant"); !strings.Contains(got, "Valorant") {
		t.Errorf("oyun adli ilk satir yanlis: %q", got)
	}
}
