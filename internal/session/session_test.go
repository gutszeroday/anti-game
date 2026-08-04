package session

import (
	"testing"
	"time"

	"github.com/guts/antigame/internal/store"
)

const (
	grace    = 10 * time.Minute
	launcher = 45 * time.Minute
)

func TestNoSessionIsInactive(t *testing.T) {
	if Active(&store.State{}, time.Now(), grace, launcher) {
		t.Fatal("oturum yokken aktif gorundu")
	}
}

func TestOpenMakesActiveImmediately(t *testing.T) {
	now := time.Date(2026, 8, 3, 18, 0, 0, 0, time.UTC)
	st := &store.State{}
	Open(st, now)
	if !Active(st, now, grace, launcher) {
		t.Fatal("acilan oturum aktif degil")
	}
}

func TestGraceLetsUserLaunchGameAfterUnlock(t *testing.T) {
	// Kod girildikten sonra oyunu baslatmak icin odemesiz sure taninir.
	now := time.Date(2026, 8, 3, 18, 0, 0, 0, time.UTC)
	st := &store.State{}
	Open(st, now)
	if !Active(st, now.Add(9*time.Minute), grace, launcher) {
		t.Fatal("odemesiz sure icinde oturum dustu")
	}
}

func TestSessionExpiresAfterGrace(t *testing.T) {
	now := time.Date(2026, 8, 3, 18, 0, 0, 0, time.UTC)
	st := &store.State{}
	Open(st, now)
	if Active(st, now.Add(11*time.Minute), grace, launcher) {
		t.Fatal("odemesiz sure dolmasina ragmen oturum aktif")
	}
}

func TestTouchKeepsSessionAliveWhileGameRuns(t *testing.T) {
	now := time.Date(2026, 8, 3, 18, 0, 0, 0, time.UTC)
	st := &store.State{}
	Open(st, now)
	// Uc saat boyunca izleyici oyunu gormeye devam ediyor.
	for i := 1; i <= 180; i++ {
		Touch(st, now.Add(time.Duration(i)*time.Minute), true)
	}
	if !Active(st, now.Add(180*time.Minute), grace, launcher) {
		t.Fatal("oyun calisirken oturum dustu")
	}
}

func TestSessionExpiresGraceAfterLastTouch(t *testing.T) {
	now := time.Date(2026, 8, 3, 18, 0, 0, 0, time.UTC)
	st := &store.State{}
	Open(st, now)
	Touch(st, now.Add(2*time.Hour), true) // oyun kapandi
	if !Active(st, now.Add(2*time.Hour+9*time.Minute), grace, launcher) {
		t.Fatal("oyun kapandiktan hemen sonra oturum dustu")
	}
	if Active(st, now.Add(2*time.Hour+11*time.Minute), grace, launcher) {
		t.Fatal("oyun kapandiktan sonra oturum sonsuza kadar acik kaldi")
	}
}

func TestTouchOnClosedSessionDoesNotReopenIt(t *testing.T) {
	st := &store.State{}
	Touch(st, time.Now(), true)
	if st.Session != nil {
		t.Fatal("Touch kapali oturumu acti")
	}
}

func TestCloseClearsSession(t *testing.T) {
	now := time.Now()
	st := &store.State{}
	Open(st, now)
	Close(st)
	if st.Session != nil || Active(st, now, grace, launcher) {
		t.Fatal("Close oturumu temizlemedi")
	}
}

func TestLauncherAloneDoesNotKeepSessionForever(t *testing.T) {
	// Riot Client tepside acik kaldigi surece oturum sonsuza kadar
	// tazeleniyordu; kod bir kez alininca gun boyu girip cikilabiliyordu.
	now := time.Date(2026, 8, 3, 18, 0, 0, 0, time.UTC)
	st := &store.State{}
	Open(st, now)
	Touch(st, now.Add(time.Minute), true) // oyun oynandi
	Touch(st, now.Add(5*time.Minute), true)

	// Oyun kapandi ama baslatici acik kalmaya devam ediyor.
	for i := 6; i <= 300; i++ {
		Touch(st, now.Add(time.Duration(i)*time.Minute), false)
	}

	if !Active(st, now.Add(40*time.Minute), grace, launcher) {
		t.Error("baslatici acikken oturum penceresi dolmadan dustu")
	}
	if Active(st, now.Add(50*time.Minute), grace, launcher) {
		t.Error("baslatici acik kaldigi icin oturum sonsuza kadar yasadi")
	}
}

func TestLauncherKeepsSessionAliveBetweenMatches(t *testing.T) {
	// Mac arasinda yalnizca istemci calisir; yeni maca girerken oturumun
	// dusmus olmasi oyunu yuklenirken oldururdu.
	now := time.Date(2026, 8, 3, 18, 0, 0, 0, time.UTC)
	st := &store.State{}
	Open(st, now)
	Touch(st, now.Add(30*time.Minute), true) // mac bitti

	for i := 31; i <= 55; i++ {
		Touch(st, now.Add(time.Duration(i)*time.Minute), false) // lobide
	}
	if !Active(st, now.Add(55*time.Minute), grace, launcher) {
		t.Fatal("mac arasi 25 dakikada oturum dustu")
	}
}

func TestOldStateWithoutLastSeenStillWorks(t *testing.T) {
	// Yeni alan eklenmeden once yazilmis state.json okunabilmeli.
	now := time.Date(2026, 8, 3, 18, 0, 0, 0, time.UTC)
	st := &store.State{Session: &store.Session{
		OpenedAt:     now,
		LastGameSeen: now,
	}}
	if !Active(st, now.Add(time.Minute), grace, launcher) {
		t.Fatal("eski bicimli oturum aktif sayilmadi")
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
