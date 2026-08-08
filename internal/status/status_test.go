package status

import (
	"strconv"
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
		session.Open(st, t0.Add(-4*time.Minute), "")
	})
	if !strings.Contains(s, "açık") {
		t.Errorf("oturum acik oldugu soylenmedi:\n%s", s)
	}
	// 10 dakikalik odemesiz surenin 4'u gecti; 6 dakika kalmali.
	if !strings.Contains(s, "6 dk") {
		t.Errorf("kalan odemesiz sure yanlis:\n%s", s)
	}
}

func TestExpiredSessionCountsAsClosed(t *testing.T) {
	s := withState(t, func(st *store.State) {
		session.Open(st, t0.Add(-30*time.Minute), "")
	})
	if !strings.Contains(s, "kapalı") {
		t.Errorf("suresi dolmus oturum acik sayildi:\n%s", s)
	}
}

func TestShowsGatedGameCount(t *testing.T) {
	// Sayi kaynagindan turetiliyor: listeye oyun eklemek bu testi
	// kirmamali, yalnizca sayinin hic yazilmamasi kirmali.
	s := withState(t, nil)
	want := strconv.Itoa(len(config.Default().Gated))
	if !strings.Contains(s, want) {
		t.Errorf("kapidaki oyun sayisi (%s) yazilmadi:\n%s", want, s)
	}
}

func TestOldStateWithoutLastSeenShowsSaneRemaining(t *testing.T) {
	// last_seen alani eklenmeden once yazilmis state.json'da kalan sure
	// ham alandan hesaplaninca -153722867 dakika gibi sayilar cikiyordu.
	s := withState(t, func(st *store.State) {
		st.Session = &store.Session{
			OpenedAt:     t0.Add(-4 * time.Minute),
			LastGameSeen: t0.Add(-4 * time.Minute),
		}
	})
	if strings.Contains(s, "-") {
		t.Errorf("kalan sure negatif cikti:\n%s", s)
	}
	if !strings.Contains(s, "6 dk") {
		t.Errorf("kalan sure eski bicimli oturumda yanlis:\n%s", s)
	}
}

func TestMissingDataDirIsNotAnError(t *testing.T) {
	// Kurulum yapilmamis makinede tepsi menusu yine acilabilmeli.
	if _, err := Text(t.TempDir(), t0); err != nil {
		t.Fatalf("kurulumsuz dizinde hata: %v", err)
	}
}

func TestStatusNamesWhoOpenedTheSession(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	cfg.People = []config.Person{{ID: "p1", Name: "Baran"}}
	if err := config.Save(dir, cfg); err != nil {
		t.Fatal(err)
	}
	st := &store.State{}
	session.Open(st, t0, "p1")
	if err := store.SaveState(dir, st); err != nil {
		t.Fatal(err)
	}

	got, err := Text(dir, t0)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "Baran açtı") {
		t.Errorf("oturumu acan kisi yazilmadi:\n%s", got)
	}
}

func briefFor(t *testing.T, mutate func(*store.State)) string {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Default()
	cfg.GraceMinutes = 10
	cfg.LauncherWindowMinutes = 10
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
	s, err := Brief(dir, t0)
	if err != nil {
		t.Fatalf("Brief: %v", err)
	}
	return s
}

func TestBriefClosedSession(t *testing.T) {
	if s := briefFor(t, nil); !strings.Contains(s, "kod gerekiyor") {
		t.Errorf("kapali oturum icin kod istenmeli: %q", s)
	}
}

func TestBriefWaitsForTheGameToStart(t *testing.T) {
	// Kod 2 dakika once girildi, oyun hic acilmadi: 8 dakika kaldi.
	s := briefFor(t, func(st *store.State) {
		session.Open(st, t0.Add(-2*time.Minute), "")
	})
	if !strings.Contains(s, "Oyunu açmak için") || !strings.Contains(s, "8 dk") {
		t.Errorf("oyun bekleniyor metni yanlis: %q", s)
	}
}

func TestBriefSaysGameIsRunning(t *testing.T) {
	// LastGameSeen taze: izleyici oyunu bu tur gordu.
	s := briefFor(t, func(st *store.State) {
		session.Open(st, t0.Add(-30*time.Minute), "")
		st.Session.LastSeen = t0
		st.Session.LastGameSeen = t0
	})
	if !strings.Contains(s, "Oyun açık") {
		t.Errorf("oyun calisiyor metni yanlis: %q", s)
	}
}

func TestBriefCountsDownAfterTheGameCloses(t *testing.T) {
	// Oyun 4 dakika once son gorulmus: 6 dakika kaldi.
	s := briefFor(t, func(st *store.State) {
		session.Open(st, t0.Add(-30*time.Minute), "")
		st.Session.LastSeen = t0.Add(-4 * time.Minute)
		st.Session.LastGameSeen = t0.Add(-4 * time.Minute)
	})
	if !strings.Contains(s, "Tekrar kod istenene kadar") || !strings.Contains(s, "6 dk") {
		t.Errorf("oyun kapandi metni yanlis: %q", s)
	}
}

func TestBriefTreatsAStaleGameStampAsClosed(t *testing.T) {
	// Tazelik penceresinin disinda kalan damga "oyun acik" sayilmamali.
	cfg := config.Default()
	st := &store.State{}
	session.Open(st, t0.Add(-30*time.Minute), "")
	st.Session.LastSeen = t0.Add(-time.Minute)
	st.Session.LastGameSeen = t0.Add(-time.Minute)
	if s := briefLine(cfg, st, t0); strings.Contains(s, "Oyun açık") {
		t.Errorf("bayat damga oyunu acik gosterdi: %q", s)
	}
}

func TestFmtDurShrinksTheUnitWithTheRemainder(t *testing.T) {
	for _, c := range []struct {
		d    time.Duration
		want string
	}{
		{40 * time.Second, "40 sn"},
		{2*time.Minute + 30*time.Second, "2 dk 30 sn"},
		{6 * time.Minute, "6 dk"},
		{11 * time.Minute, "11 dk"},
		{-time.Minute, "0 sn"},
	} {
		if got := fmtDur(c.d); got != c.want {
			t.Errorf("fmtDur(%v) = %q, istenen %q", c.d, got, c.want)
		}
	}
}
