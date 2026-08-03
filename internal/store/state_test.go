package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadStateMissingReturnsEmpty(t *testing.T) {
	st, err := LoadState(t.TempDir())
	if err != nil {
		t.Fatalf("dosya yokken hata donmemeli: %v", err)
	}
	if st.Session != nil || st.LastTOTPCounter != 0 {
		t.Fatalf("bos durum beklendi: %+v", st)
	}
}

func TestSaveStateRoundTrip(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 8, 3, 18, 0, 0, 0, time.UTC)
	in := &State{
		LastTOTPCounter: 58291837,
		FailCount:       2,
		Session:         &Session{OpenedAt: now, LastGameSeen: now},
		Heartbeat:       now,
		RecoveryHash:    "abc",
		RecoverySalt:    "def",
	}
	if err := SaveState(dir, in); err != nil {
		t.Fatalf("SaveState: %v", err)
	}
	out, err := LoadState(dir)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if out.LastTOTPCounter != in.LastTOTPCounter || out.FailCount != 2 {
		t.Errorf("gidis-donus bozuk: %+v", out)
	}
	if out.Session == nil || !out.Session.OpenedAt.Equal(now) {
		t.Errorf("oturum korunmadi: %+v", out.Session)
	}
}

func TestSaveStateLeavesNoTempFile(t *testing.T) {
	dir := t.TempDir()
	if err := SaveState(dir, &State{}); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Fatalf("gecici dosya temizlenmedi: %s", e.Name())
		}
	}
}
