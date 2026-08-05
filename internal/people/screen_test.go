//go:build windows

package people

import (
	"bytes"
	"strings"
	"testing"

	"github.com/guts/antigame/internal/config"
	"github.com/guts/antigame/internal/term"
)

// run, ekrani verilen tuslarla surer ve ciktiyi dondurur.
func run(t *testing.T, dir, keys string) string {
	t.Helper()
	var out bytes.Buffer
	if err := Screen(dir, strings.NewReader(keys), &out); err != nil {
		t.Fatalf("Screen: %v", err)
	}
	return out.String()
}

func TestScreenListsPeopleWithKeyStatus(t *testing.T) {
	dir := seed(t, "Baran", "Ali")
	got := run(t, dir, "0\n")
	for _, want := range []string{"Baran", "Ali", "anahtar var"} {
		if !strings.Contains(got, want) {
			t.Errorf("ekranda %q yok:\n%s", want, got)
		}
	}
}

func TestScreenRefusesToRemoveLastKey(t *testing.T) {
	dir := seed(t, "Baran")
	got := run(t, dir, "s\n1\ne\n\n0\n")

	if !strings.Contains(got, "kapıyı açabilecek kimse kalmıyor") {
		t.Errorf("kilitlenme uyarisi yok:\n%s", got)
	}
	if entries, _ := List(dir); len(entries) != 1 {
		t.Error("reddedilen silme kaydi bozdu")
	}
}

func TestScreenRemoveCanBeCancelled(t *testing.T) {
	dir := seed(t, "Baran", "Ali")
	run(t, dir, "s\n2\nh\n\n0\n")

	if entries, _ := List(dir); len(entries) != 2 {
		t.Error("vazgecilen silme uygulandi")
	}
}

func TestScreenEditKeepsEmptyFieldsUnchanged(t *testing.T) {
	dir := seed(t, "Baran")
	entries, _ := List(dir)
	if err := Edit(dir, entries[0].ID, "Baran", "WhatsApp"); err != nil {
		t.Fatal(err)
	}

	// Ad degistirilir, iletisim bos birakilir.
	run(t, dir, "d\n1\nBaran Y.\n\n\n0\n")

	entries, _ = List(dir)
	if entries[0].Name != "Baran Y." || entries[0].Hint != "WhatsApp" {
		t.Errorf("bos birakilan alan silindi: %+v", entries[0])
	}
}

func TestScreenRejectsBadIndex(t *testing.T) {
	dir := seed(t, "Baran")
	got := run(t, dir, "d\n7\n\n0\n")
	if !strings.Contains(got, "geçersiz sıra numarası") {
		t.Errorf("gecersiz secim bildirilmedi:\n%s", got)
	}
}

func TestScreenOutputHasNoEscapesWhenNotATerminal(t *testing.T) {
	dir := seed(t, "Baran")
	if got := run(t, dir, "0\n"); strings.Contains(got, "\x1b") {
		t.Errorf("boruya kacis dizisi yazildi: %q", got)
	}
}

func TestRenderListMarksMissingKey(t *testing.T) {
	entries := []Entry{
		{Person: config.Person{ID: "p1", Name: "Baran", Hint: "WhatsApp"}, HasKey: true},
		{Person: config.Person{ID: "p2", Name: "Ali"}},
	}
	got := renderList(term.Plain(), entries)
	if !strings.Contains(got, "anahtar var") || !strings.Contains(got, "anahtarı yok") {
		t.Errorf("durumlar yazilmadi:\n%s", got)
	}
	if !strings.Contains(got, "—") {
		t.Errorf("bos iletisim notu isaretlenmedi:\n%s", got)
	}
}

func TestPadTruncatesByRuneNotByte(t *testing.T) {
	// Turkce harfler iki bayt; bayt sayilirsa sutunlar kayar.
	if got := pad("Ayşegül", 4); len([]rune(got)) != 4 {
		t.Errorf("kirpma rune sayisina gore degil: %q", got)
	}
	if got := pad("Ali", 6); len([]rune(got)) != 6 {
		t.Errorf("dolgu eksik: %q", got)
	}
}

func TestSummaryCountsUsableKeys(t *testing.T) {
	dir := seed(t, "Baran", "Ali")
	if got := Summary(dir); !strings.Contains(got, "2 kişi kapıyı açabiliyor") {
		t.Errorf("ozet yanlis: %q", got)
	}
}
