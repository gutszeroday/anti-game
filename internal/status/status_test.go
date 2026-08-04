package status

import (
	"strings"
	"testing"
	"time"

	"github.com/guts/antigame/internal/config"
	"github.com/guts/antigame/internal/session"
	"github.com/guts/antigame/internal/store"
)

var t0 = time.Date(2026, 8, 4, 20, 0, 0, 0, time.UTC)

func withState(t *testing.T, mutate func(*store.State)) string {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Default()
	cfg.GraceMinutes = 10
	if err := config.Save(dir, cfg); err != nil {
		t.Fatal(err)
	}
	st, _ := store.LoadState(dir)
	if mutate != nil {
		mutate(st)
	}
	if err := store.SaveState(dir, st); err != nil {
		t.Fatal(err)
	}
	s, err := Text(dir, t0)
	if err != nil {
		t.Fatalf("Text: %v", err)
	}
	return s
}

func TestClosedSessionSaysCodeNeeded(t *testing.T) {
	s := withState(t, nil)
	if !strings.Contains(s, "kapalı") {
		t.Errorf("oturum kapali oldugu soylenmedi:\n%s", s)
	}
}

func TestOpenSessionShowsRemainingGrace(t *testing.T) {
	s := withState(t, func(st *store.State) {
		session.Open(st, t0.Add(-4*time.Minute))
	})
	if !strings.Contains(s, "açık") {
		t.Errorf("oturum acik oldugu soylenmedi:\n%s", s)
	}
	// 10 dakikalik odemesiz surenin 4'u gecti; 6 dakika kalmali.
	if !strings.Contains(s, "6 dakika") {
		t.Errorf("kalan odemesiz sure yanlis:\n%s", s)
	}
}

func TestExpiredSessionCountsAsClosed(t *testing.T) {
	s := withState(t, func(st *store.State) {
		session.Open(st, t0.Add(-30*time.Minute))
	})
	if !strings.Contains(s, "kapalı") {
		t.Errorf("suresi dolmus oturum acik sayildi:\n%s", s)
	}
}

func TestShowsGatedGameCount(t *testing.T) {
	s := withState(t, nil)
	if !strings.Contains(s, "6") {
		t.Errorf("kapidaki oyun sayisi yazilmadi:\n%s", s)
	}
}

func TestMissingDataDirIsNotAnError(t *testing.T) {
	// Kurulum yapilmamis makinede tepsi menusu yine acilabilmeli.
	if _, err := Text(t.TempDir(), t0); err != nil {
		t.Fatalf("kurulumsuz dizinde hata: %v", err)
	}
}
