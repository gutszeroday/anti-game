package session

import (
	"testing"
	"time"

	"github.com/guts/antigame/internal/store"
)

const grace = 10 * time.Minute

func TestNoSessionIsInactive(t *testing.T) {
	if Active(&store.State{}, time.Now(), grace) {
		t.Fatal("oturum yokken aktif gorundu")
	}
}

func TestOpenMakesActiveImmediately(t *testing.T) {
	now := time.Date(2026, 8, 3, 18, 0, 0, 0, time.UTC)
	st := &store.State{}
	Open(st, now)
	if !Active(st, now, grace) {
		t.Fatal("acilan oturum aktif degil")
	}
}

func TestGraceLetsUserLaunchGameAfterUnlock(t *testing.T) {
	// Kod girildikten sonra oyunu baslatmak icin odemesiz sure taninir.
	now := time.Date(2026, 8, 3, 18, 0, 0, 0, time.UTC)
	st := &store.State{}
	Open(st, now)
	if !Active(st, now.Add(9*time.Minute), grace) {
		t.Fatal("odemesiz sure icinde oturum dustu")
	}
}

func TestSessionExpiresAfterGrace(t *testing.T) {
	now := time.Date(2026, 8, 3, 18, 0, 0, 0, time.UTC)
	st := &store.State{}
	Open(st, now)
	if Active(st, now.Add(11*time.Minute), grace) {
		t.Fatal("odemesiz sure dolmasina ragmen oturum aktif")
	}
}

func TestTouchKeepsSessionAliveWhileGameRuns(t *testing.T) {
	now := time.Date(2026, 8, 3, 18, 0, 0, 0, time.UTC)
	st := &store.State{}
	Open(st, now)
	// Uc saat boyunca izleyici oyunu gormeye devam ediyor.
	for i := 1; i <= 180; i++ {
		Touch(st, now.Add(time.Duration(i)*time.Minute))
	}
	if !Active(st, now.Add(180*time.Minute), grace) {
		t.Fatal("oyun calisirken oturum dustu")
	}
}

func TestSessionExpiresGraceAfterLastTouch(t *testing.T) {
	now := time.Date(2026, 8, 3, 18, 0, 0, 0, time.UTC)
	st := &store.State{}
	Open(st, now)
	Touch(st, now.Add(2*time.Hour)) // oyun kapandi
	if !Active(st, now.Add(2*time.Hour+9*time.Minute), grace) {
		t.Fatal("oyun kapandiktan hemen sonra oturum dustu")
	}
	if Active(st, now.Add(2*time.Hour+11*time.Minute), grace) {
		t.Fatal("oyun kapandiktan sonra oturum sonsuza kadar acik kaldi")
	}
}

func TestTouchOnClosedSessionDoesNotReopenIt(t *testing.T) {
	st := &store.State{}
	Touch(st, time.Now())
	if st.Session != nil {
		t.Fatal("Touch kapali oturumu acti")
	}
}

func TestCloseClearsSession(t *testing.T) {
	now := time.Now()
	st := &store.State{}
	Open(st, now)
	Close(st)
	if st.Session != nil || Active(st, now, grace) {
		t.Fatal("Close oturumu temizlemedi")
	}
}

func TestOpenPreservesOpenedAtOnReopen(t *testing.T) {
	first := time.Date(2026, 8, 3, 18, 0, 0, 0, time.UTC)
	st := &store.State{}
	Open(st, first)
	second := first.Add(3 * time.Hour)
	Open(st, second)
	if !st.Session.OpenedAt.Equal(second) {
		t.Errorf("yeni oturum acilisinda OpenedAt guncellenmedi: %v", st.Session.OpenedAt)
	}
}
