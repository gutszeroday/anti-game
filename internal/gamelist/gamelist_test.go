package gamelist

import (
	"bytes"
	"strings"
	"testing"

	"github.com/guts/antigame/internal/config"
)

func TestFormatListsGamesWithIndex(t *testing.T) {
	c := &config.Config{Gated: []config.Game{
		{Name: "Valorant", Exe: "VALORANT.exe"},
		{Name: "Sabitli", Exe: "client.exe", Path: `C:\Games\client.exe`},
	}}
	s := Format(c)
	for _, want := range []string{"Valorant", "VALORANT.exe", "Sabitli", `C:\Games\client.exe`} {
		if !strings.Contains(s, want) {
			t.Errorf("cikti %q icermiyor:\n%s", want, s)
		}
	}
}

func TestFormatEmptyListExplainsConsequence(t *testing.T) {
	s := Format(&config.Config{})
	if !strings.Contains(strings.ToLower(s), "boş") {
		t.Errorf("bos liste uyarisi yok: %s", s)
	}
}

func TestAddAppendsGame(t *testing.T) {
	dir := t.TempDir()
	if err := Add(dir, "Test Oyun", "test.exe", ""); err != nil {
		t.Fatalf("Add: %v", err)
	}
	c, _ := config.Load(dir)
	if _, ok := c.Match("test.exe", ""); !ok {
		t.Fatal("eklenen oyun listede yok")
	}
}

func TestAddRejectsDuplicateExe(t *testing.T) {
	dir := t.TempDir()
	if err := Add(dir, "Test", "test.exe", ""); err != nil {
		t.Fatal(err)
	}
	if err := Add(dir, "Tekrar", "TEST.EXE", ""); err == nil {
		t.Fatal("ayni exe ikinci kez eklendi")
	}
}

func TestAddRejectsEmptyExe(t *testing.T) {
	if err := Add(t.TempDir(), "Ad", "  ", ""); err == nil {
		t.Fatal("bos exe kabul edildi")
	}
}

func TestRemoveDeletesGame(t *testing.T) {
	dir := t.TempDir()
	c := config.Default()
	if err := config.Save(dir, c); err != nil {
		t.Fatal(err)
	}
	if err := Remove(dir, "valorant.exe"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	got, _ := config.Load(dir)
	if _, ok := got.Match("VALORANT.exe", ""); ok {
		t.Fatal("silinen oyun hala listede")
	}
}

func addWith(t *testing.T, dir, input string, running []string) string {
	t.Helper()
	var out bytes.Buffer
	if err := AddInteractive(dir, strings.NewReader(input), &out, running); err != nil {
		t.Fatalf("AddInteractive: %v", err)
	}
	return out.String()
}

func TestAddInteractivePicksFromRunningPrograms(t *testing.T) {
	dir := t.TempDir()
	config.Save(dir, &config.Config{})
	// 2. sirada Palworld, ad varsayilan, baslatici degil.
	addWith(t, dir, "2\n\nh\n", []string{"chrome.exe", "Palworld.exe", "steam.exe"})

	c, _ := config.Load(dir)
	g, ok := c.Match("Palworld.exe", "")
	if !ok {
		t.Fatal("secilen program listeye eklenmedi")
	}
	if g.Name != "Palworld" {
		t.Errorf("varsayilan ad exe'den turetilmedi: %q", g.Name)
	}
	if g.Launcher {
		t.Error("oyun baslatici olarak isaretlendi")
	}
}

func TestAddInteractiveAcceptsTypedExeName(t *testing.T) {
	dir := t.TempDir()
	config.Save(dir, &config.Config{})
	addWith(t, dir, "Factorio.exe\nFactorio\nh\n", []string{"chrome.exe"})

	c, _ := config.Load(dir)
	if _, ok := c.Match("Factorio.exe", ""); !ok {
		t.Fatal("elle yazilan exe eklenmedi")
	}
}

func TestAddInteractiveMarksLauncher(t *testing.T) {
	dir := t.TempDir()
	config.Save(dir, &config.Config{})
	addWith(t, dir, "Steam.exe\nSteam\ne\n", nil)

	c, _ := config.Load(dir)
	g, _ := c.Match("Steam.exe", "")
	if !g.Launcher {
		t.Error("baslatici olarak isaretlenmedi")
	}
}

func TestAddInteractiveHidesAlreadyGatedPrograms(t *testing.T) {
	dir := t.TempDir()
	config.Save(dir, &config.Config{Gated: []config.Game{{Name: "Valorant", Exe: "VALORANT.exe"}}})
	out := addWith(t, dir, "chrome.exe\n\nh\n", []string{"VALORANT.exe", "chrome.exe"})
	if strings.Contains(out, "VALORANT.exe") {
		t.Errorf("zaten listede olan program secenek olarak sunuldu:\n%s", out)
	}
}

func TestRemoveInteractiveDeletesChosenIndex(t *testing.T) {
	dir := t.TempDir()
	config.Save(dir, &config.Config{Gated: []config.Game{
		{Name: "Bir", Exe: "bir.exe"},
		{Name: "Iki", Exe: "iki.exe"},
	}})
	var out bytes.Buffer
	if err := RemoveInteractive(dir, strings.NewReader("2\n"), &out); err != nil {
		t.Fatalf("RemoveInteractive: %v", err)
	}
	c, _ := config.Load(dir)
	if _, ok := c.Match("iki.exe", ""); ok {
		t.Error("secilen oyun silinmedi")
	}
	if _, ok := c.Match("bir.exe", ""); !ok {
		t.Error("yanlis oyun silindi")
	}
}

func TestRemoveInteractiveRejectsBadIndex(t *testing.T) {
	dir := t.TempDir()
	config.Save(dir, &config.Config{Gated: []config.Game{{Name: "Bir", Exe: "bir.exe"}}})
	var out bytes.Buffer
	if err := RemoveInteractive(dir, strings.NewReader("9\n"), &out); err == nil {
		t.Fatal("gecersiz sira numarasi kabul edildi")
	}
	c, _ := config.Load(dir)
	if _, ok := c.Match("bir.exe", ""); !ok {
		t.Error("gecersiz secimde liste degisti")
	}
}

func TestRemoveUnknownExeReturnsError(t *testing.T) {
	dir := t.TempDir()
	config.Save(dir, config.Default())
	if err := Remove(dir, "yok.exe"); err == nil {
		t.Fatal("olmayan oyun hatasiz silindi")
	}
}
