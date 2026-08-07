//go:build windows

package datainfo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/guts/antigame/internal/config"
)

func write(t *testing.T, dir, name string, n int) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), make([]byte, n), 0o600); err != nil {
		t.Fatal(err)
	}
}

func find(es []Entry, name string) (Entry, bool) {
	for _, e := range es {
		if e.Name == name {
			return e, true
		}
	}
	return Entry{}, false
}

func TestRecognisesEachFileKind(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "config.json", 10)
	write(t, dir, "state.json", 20)
	write(t, dir, "events-2026-08.jsonl", 30)
	write(t, dir, "secret-p1.bin", 40)

	es, err := List(dir, []config.Person{{ID: "p1", Name: "Ali"}})
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]Kind{
		"config.json":          KindConfig,
		"state.json":           KindState,
		"events-2026-08.jsonl": KindEvents,
		"secret-p1.bin":        KindKey,
	}
	for name, kind := range want {
		e, ok := find(es, name)
		if !ok {
			t.Errorf("%s listelenmemis", name)
			continue
		}
		if e.Kind != kind {
			t.Errorf("%s turu %v, istenen %v", name, e.Kind, kind)
		}
	}
}

func TestKeyFileNamesItsOwner(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "secret-p1.bin", 40)

	es, err := List(dir, []config.Person{{ID: "p1", Name: "Ayşe"}})
	if err != nil {
		t.Fatal(err)
	}
	e, _ := find(es, "secret-p1.bin")
	if !strings.Contains(e.Desc, "Ayşe") {
		t.Errorf("anahtar sahibinin adi yazmiyor: %q", e.Desc)
	}
}

// Sahipsiz anahtar dosyasi silinmiyor, yalnizca bildiriliyor: kullanici
// elle sildiginde geri donusu yok, once gormeli.
func TestOrphanKeyIsMarkedNotHidden(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "secret-p9.bin", 40)

	es, err := List(dir, []config.Person{{ID: "p1", Name: "Ali"}})
	if err != nil {
		t.Fatal(err)
	}
	e, ok := find(es, "secret-p9.bin")
	if !ok {
		t.Fatal("sahipsiz anahtar listelenmemis")
	}
	if e.Kind != KindKey {
		t.Errorf("turu %v, istenen KindKey", e.Kind)
	}
	if !strings.Contains(e.Desc, "sahibi yok") {
		t.Errorf("sahipsiz oldugu yazmiyor: %q", e.Desc)
	}
}

// Dizine elle bir sey kopyalanmissa kullanici gorebilmeli.
func TestUnknownFileIsStillListed(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "rastgele.txt", 7)

	es, err := List(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	e, ok := find(es, "rastgele.txt")
	if !ok {
		t.Fatal("taninmayan dosya listelenmemis")
	}
	if e.Kind != KindUnknown {
		t.Errorf("turu %v, istenen KindUnknown", e.Kind)
	}
	if e.Desc == "" {
		t.Error("taninmayan dosyanin da bir aciklamasi olmali")
	}
}

func TestReadsFileSize(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "config.json", 1234)

	es, err := List(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	e, _ := find(es, "config.json")
	if e.Size != 1234 {
		t.Errorf("boyut %d, istenen 1234", e.Size)
	}
}

// Kurulum yapilmamis makinede dizin henuz yoktur; bu bir ariza degil.
func TestMissingDirectoryIsEmptyNotAnError(t *testing.T) {
	es, err := List(filepath.Join(t.TempDir(), "yok"), nil)
	if err != nil {
		t.Fatalf("olmayan dizin hata verdi: %v", err)
	}
	if len(es) != 0 {
		t.Errorf("%d giris, istenen 0", len(es))
	}
}

func TestSubdirectoriesAreSkipped(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "altdizin"), 0o700); err != nil {
		t.Fatal(err)
	}
	write(t, dir, "config.json", 5)

	es, err := List(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := find(es, "altdizin"); ok {
		t.Error("dizin dosya gibi listelenmis")
	}
	if len(es) != 1 {
		t.Errorf("%d giris, istenen 1", len(es))
	}
}

// Sira kararli olmali: her acilista farkli siralanan bir liste
// kullanicinin aradigini bulmasini zorlastirir.
func TestOrderIsStableByKindThenName(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "zzz.txt", 1)
	write(t, dir, "events-2026-08.jsonl", 1)
	write(t, dir, "config.json", 1)
	write(t, dir, "secret-p1.bin", 1)
	write(t, dir, "state.json", 1)

	es, err := List(dir, []config.Person{{ID: "p1", Name: "Ali"}})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range es {
		got = append(got, e.Name)
	}
	want := []string{"config.json", "secret-p1.bin", "state.json", "events-2026-08.jsonl", "zzz.txt"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("sira %v, istenen %v", got, want)
	}
}
