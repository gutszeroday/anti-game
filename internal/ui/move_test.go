//go:build windows

package ui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/guts/antigame/internal/store"
)

func seed(t *testing.T, dir string) {
	t.Helper()
	for name, n := range map[string]int{
		"config.json": 120, "secret-p1.bin": 246, "state.json": 80,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), make([]byte, n), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func quiet(string) {}

func TestMoveDataCopiesThenClearsTheOldDirectory(t *testing.T) {
	from, to := t.TempDir(), t.TempDir()
	seed(t, from)

	var saved string
	res, err := moveData(from, to, false, func(p string) error { saved = p; return nil }, quiet)
	if err != nil {
		t.Fatal(err)
	}
	if res.OldKept {
		t.Error("eski klasör silinmeliydi")
	}
	if saved != to {
		t.Errorf("ayar %q, istenen %q", saved, to)
	}
	for _, name := range []string{"config.json", "secret-p1.bin", "state.json"} {
		if _, err := os.Stat(filepath.Join(to, name)); err != nil {
			t.Errorf("%s hedefe gitmemis", name)
		}
		if _, err := os.Stat(filepath.Join(from, name)); !os.IsNotExist(err) {
			t.Errorf("%s kaynakta kalmis", name)
		}
	}
}

// Tasima kod istemiyor; gunluge dusmesi onu gorunur kiliyor.
func TestMoveDataRecordsTheMoveInTheNewLog(t *testing.T) {
	from, to := t.TempDir(), t.TempDir()
	seed(t, from)

	if _, err := moveData(from, to, false, func(string) error { return nil }, quiet); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	ev, err := store.Read(to, now.AddDate(0, 0, -1), now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	var found *store.Event
	for i := range ev {
		if ev[i].Ev == "data_moved" {
			found = &ev[i]
		}
	}
	if found == nil {
		t.Fatal("data_moved olayi yazilmamis")
	}
	if found.From != from || found.To != to {
		t.Errorf("olay %q -> %q, istenen %q -> %q", found.From, found.To, from, to)
	}
}

// Dogrulama patlarsa hicbir sey degismemeli: ayar hala eski klasoru
// gostermeli ve veri yerinde durmali.
func TestMoveDataChangesNothingWhenTheTargetIsRejected(t *testing.T) {
	from, to := t.TempDir(), t.TempDir()
	seed(t, from)
	seed(t, to) // hedefte zaten veri var

	called := false
	_, err := moveData(from, to, false, func(string) error { called = true; return nil }, quiet)
	if err == nil {
		t.Fatal("dolu hedef kabul edildi")
	}
	if called {
		t.Error("dogrulama basarisizken ayar yazilmis")
	}
	if _, err := os.Stat(filepath.Join(from, "config.json")); err != nil {
		t.Error("kaynak bozulmus")
	}
}

// Ayar yazilamazsa eski klasor aktif kalir; onu silmek veriyi
// ulasilamaz yapardi.
func TestMoveDataKeepsTheOldDirectoryWhenTheSettingCannotBeSaved(t *testing.T) {
	from, to := t.TempDir(), t.TempDir()
	seed(t, from)

	boom := errors.New("kayit defteri yazilamadi")
	_, err := moveData(from, to, false, func(string) error { return boom }, quiet)
	if !errors.Is(err, boom) {
		t.Fatalf("hata yutuldu: %v", err)
	}
	if _, err := os.Stat(filepath.Join(from, "config.json")); err != nil {
		t.Error("ayar yazilamamisken kaynak silinmis")
	}
}

// Izleyici yeni klasore gecmediyse eski veriyi silmek, hala oraya yazan
// izleyicinin altindan zemini cekmek olurdu.
func TestMoveDataKeepsTheOldDirectoryWhenTheWatcherDoesNotFollow(t *testing.T) {
	from, to := t.TempDir(), t.TempDir()
	seed(t, from)

	// Bekleme suresini testin katlanabilecegi bir degere indiriyoruz.
	old := watcherSwitchTimeout
	setWatcherSwitchTimeout(300 * time.Millisecond)
	defer setWatcherSwitchTimeout(old)

	res, err := moveData(from, to, true, func(string) error { return nil }, quiet)
	if err != nil {
		t.Fatal(err)
	}
	if !res.OldKept {
		t.Error("izleyici gecmediyse eski klasör silinmemeliydi")
	}
	if !strings.Contains(res.Note, from) {
		t.Errorf("kullaniciya eski klasorun yeri soylenmemis: %q", res.Note)
	}
	if _, err := os.Stat(filepath.Join(from, "config.json")); err != nil {
		t.Error("eski veri silinmis")
	}
}

func TestMoveDataReportsEachStep(t *testing.T) {
	from, to := t.TempDir(), t.TempDir()
	seed(t, from)

	var steps []string
	if _, err := moveData(from, to, false, func(string) error { return nil },
		func(s string) { steps = append(steps, s) }); err != nil {
		t.Fatal(err)
	}
	if len(steps) < 4 {
		t.Errorf("yalnizca %d adim bildirilmis: %v", len(steps), steps)
	}
}

func TestWaitForWatcherSeesTheSwitch(t *testing.T) {
	dir := t.TempDir()
	if err := store.Append(dir, store.Event{
		TS: time.Now().UTC(), Ev: "data_dir_changed", From: "eski", To: dir,
	}); err != nil {
		t.Fatal(err)
	}
	if !waitForWatcher(dir, time.Second) {
		t.Error("yazilmis olay gorulmedi")
	}
}

// Baska bir klasore yapilmis gecis bu tasimanin kaniti degil.
func TestWaitForWatcherIgnoresASwitchToSomewhereElse(t *testing.T) {
	dir := t.TempDir()
	if err := store.Append(dir, store.Event{
		TS: time.Now().UTC(), Ev: "data_dir_changed", From: "eski", To: "baska",
	}); err != nil {
		t.Fatal(err)
	}
	if waitForWatcher(dir, 300*time.Millisecond) {
		t.Error("baska klasore gecis kanit sayilmis")
	}
}
