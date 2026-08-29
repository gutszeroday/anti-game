package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTelegramStateRoundTrip(t *testing.T) {
	dir := t.TempDir()
	ts := time.Date(2026, 8, 23, 9, 30, 0, 0, time.UTC)
	in := &TelegramState{
		Offset:       7,
		LastUnlockTS: &ts,
	}
	if err := SaveTelegramState(dir, in); err != nil {
		t.Fatalf("SaveTelegramState: %v", err)
	}
	out, err := LoadTelegramState(dir)
	if err != nil {
		t.Fatalf("LoadTelegramState: %v", err)
	}
	if out.Offset != 7 {
		t.Errorf("offset korunmadi: %d", out.Offset)
	}
	if out.LastUnlockTS == nil || !out.LastUnlockTS.Equal(ts) {
		t.Errorf("tarama isareti korunmadi: %+v", out.LastUnlockTS)
	}
}

func TestLoadTelegramStateDefaultEmpty(t *testing.T) {
	ts, err := LoadTelegramState(t.TempDir())
	if err != nil {
		t.Fatalf("LoadTelegramState: %v", err)
	}
	if ts.Offset != 0 || ts.LastUnlockTS != nil {
		t.Errorf("bos durumda telegram alanlari sifir olmali: %+v", ts)
	}
}

// state.json'dan ayri dosyada tutulmasi bu ozelligin tum amaci: watch
// paketi state.json'u eski bir bellek ici kopyadan geri yazdiginda
// telegram durumu etkilenmemeli.
func TestTelegramStateDoesNotTouchStateJSON(t *testing.T) {
	dir := t.TempDir()
	ts := time.Date(2026, 8, 23, 9, 30, 0, 0, time.UTC)
	if err := SaveTelegramState(dir, &TelegramState{Offset: 3, LastUnlockTS: &ts}); err != nil {
		t.Fatalf("SaveTelegramState: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "state.json")); !os.IsNotExist(err) {
		t.Fatalf("telegram durumu state.json'a yazilmis olmamali: %v", err)
	}

	// watch'in yaptigi gibi: eski bir State'i kosulsuz geri yaz.
	if err := SaveState(dir, &State{}); err != nil {
		t.Fatalf("SaveState: %v", err)
	}
	out, err := LoadTelegramState(dir)
	if err != nil {
		t.Fatalf("LoadTelegramState: %v", err)
	}
	if out.Offset != 3 || out.LastUnlockTS == nil {
		t.Fatalf("state.json yazimi telegram durumunu geri almis: %+v", out)
	}
}

func TestSaveTelegramStateLeavesNoTempFile(t *testing.T) {
	dir := t.TempDir()
	if err := SaveTelegramState(dir, &TelegramState{}); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Fatalf("gecici dosya temizlenmedi: %s", e.Name())
		}
	}
}
