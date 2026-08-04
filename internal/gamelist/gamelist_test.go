package gamelist

import (
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

func TestRemoveUnknownExeReturnsError(t *testing.T) {
	dir := t.TempDir()
	config.Save(dir, config.Default())
	if err := Remove(dir, "yok.exe"); err == nil {
		t.Fatal("olmayan oyun hatasiz silindi")
	}
}
