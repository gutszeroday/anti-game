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
		TOTPCounters: map[string]uint64{"p1": 58291837},
		FailCount:    2,
		Session:      &Session{OpenedAt: now, LastGameSeen: now, OpenedBy: "p1"},
		Heartbeat:    now,
		RecoveryHash: "abc",
		RecoverySalt: "def",
	}
	if err := SaveState(dir, in); err != nil {
		t.Fatalf("SaveState: %v", err)
	}
	out, err := LoadState(dir)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if out.Counter("p1") != in.Counter("p1") || out.FailCount != 2 {
		t.Errorf("gidis-donus bozuk: %+v", out)
	}
	if out.Session == nil || !out.Session.OpenedAt.Equal(now) || out.Session.OpenedBy != "p1" {
		t.Errorf("oturum korunmadi: %+v", out.Session)
	}
}

// Tek kisilik donemden kalan state.json'da sayac last_totp_counter
// alanindadir; ilk kisiye devredilmezse kullanilmis bir kod yeniden
// gecerli hale gelir.
func TestLegacyCounterMovesToFirstPerson(t *testing.T) {
	dir := t.TempDir()
	legacy := `{"last_totp_counter":58291837,"fail_count":1}`
	if err := os.WriteFile(filepath.Join(dir, stateFile), []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	st, err := LoadState(dir)
	if err != nil {
		t.Fatal(err)
	}
	if st.Counter("p1") != 58291837 {
		t.Errorf("eski sayac p1'e devredilmedi: %+v", st.TOTPCounters)
	}
	if st.LastTOTPCounter != 0 {
		t.Error("devirden sonra eski alan sifirlanmadi")
	}
}

func TestClearCounterForgetsPerson(t *testing.T) {
	st := &State{}
	st.SetCounter("p2", 42)
	st.ClearCounter("p2")
	if st.Counter("p2") != 0 {
		t.Error("anahtar yenilendiginde sayac silinmedi")
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
