package watch

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/guts/antigame/internal/config"
	"github.com/guts/antigame/internal/winproc"
)

// dirWatcher, veri klasoru degistirilebilen bir izleyici kurar.
// Donen islev, klasoru degistirmek icin cagrilir.
func dirWatcher(t *testing.T, start string) (*Watcher, func(string)) {
	t.Helper()
	cfg := config.Default()
	cfg.Gated = []config.Game{{Name: "Valorant", Exe: "VALORANT.exe"}}

	cur := start
	w, err := New(Options{
		Dir:           start,
		DirFunc:       func() string { return cur },
		Cfg:           cfg,
		List:          func() ([]winproc.Proc, error) { return nil, nil },
		Path:          func(int) (string, error) { return "", nil },
		Terminate:     func(int) error { return nil },
		Trim:          func() error { return nil },
		Idle:          func() (int, error) { return 0, nil },
		ForegroundPID: func() (int, error) { return 0, nil },
		SpawnGate:     func(string) error { return nil },
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return w, func(next string) { cur = next }
}

// Klasor arayuzden degistirilebiliyor ve arayuz ayri bir process.
// Izleyici fark etmezse kapi yeni klasore oturum acar, izleyici eskiye
// bakmaya devam eder ve oyunu oldurmeyi surdurur.
func TestWatcherFollowsTheDataDirectory(t *testing.T) {
	oldDir, newDir := t.TempDir(), t.TempDir()
	w, move := dirWatcher(t, oldDir)

	if err := w.Step(t0); err != nil {
		t.Fatal(err)
	}
	move(newDir)
	if err := w.Step(t0.Add(time.Second)); err != nil {
		t.Fatal(err)
	}

	if w.dir() != newDir {
		t.Errorf("izleyici %q kullaniyor, istenen %q", w.dir(), newDir)
	}
	if !hasEvent(events(t, newDir), "data_dir_changed", "") {
		t.Error("tasima yeni klasorun gunlugune yazilmamis")
	}
}

func TestWatcherWritesToTheNewDirectoryAfterAMove(t *testing.T) {
	oldDir, newDir := t.TempDir(), t.TempDir()
	w, move := dirWatcher(t, oldDir)

	if err := w.Step(t0); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadDir(oldDir)
	if err != nil {
		t.Fatal(err)
	}
	move(newDir)
	for i := range 3 {
		if err := w.Step(t0.Add(time.Duration(i+1) * time.Second)); err != nil {
			t.Fatal(err)
		}
	}

	after, err := os.ReadDir(oldDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Errorf("eski klasore yazilmaya devam edilmis: %d -> %d dosya", len(before), len(after))
	}
	if len(events(t, newDir)) == 0 {
		t.Error("yeni klasore hicbir sey yazilmamis")
	}
}

// Yarim kurulmus bir hedefe gecmek acik oturumu gorunmez kilardi.
func TestWatcherStaysPutWhenTheNewDirectoryCannotBeRead(t *testing.T) {
	oldDir := t.TempDir()
	w, move := dirWatcher(t, oldDir)

	if err := w.Step(t0); err != nil {
		t.Fatal(err)
	}
	// Dizin degil dosya: LoadState okuyamaz.
	bad := filepath.Join(t.TempDir(), "dosya")
	if err := os.WriteFile(bad, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	move(bad)
	if err := w.Step(t0.Add(time.Second)); err != nil {
		t.Fatalf("okunamayan hedef izleyiciyi oldurdu: %v", err)
	}
	if w.dir() != oldDir {
		t.Errorf("okunamayan hedefe gecilmis: %q", w.dir())
	}
}

// Klasoru silinen izleyici olmemeli: once store.Append hata verip Run'i
// donduruyordu, yani koruma tamamen duruyordu.
func TestWatcherSurvivesLosingItsDirectory(t *testing.T) {
	oldDir, newDir := t.TempDir(), t.TempDir()
	w, move := dirWatcher(t, oldDir)

	if err := w.Step(t0); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(oldDir); err != nil {
		t.Fatal(err)
	}
	move(newDir)

	if err := w.Step(t0.Add(time.Second)); err != nil {
		t.Fatalf("klasor silinince izleyici oldu: %v", err)
	}
	if w.dir() != newDir {
		t.Errorf("izleyici %q kullaniyor, istenen %q", w.dir(), newDir)
	}
}

func TestWatcherIgnoresAnEmptyDirectorySetting(t *testing.T) {
	dir := t.TempDir()
	w, move := dirWatcher(t, dir)
	if err := w.Step(t0); err != nil {
		t.Fatal(err)
	}
	move("")
	if err := w.Step(t0.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if w.dir() != dir {
		t.Errorf("bos ayar klasoru degistirmis: %q", w.dir())
	}
}
