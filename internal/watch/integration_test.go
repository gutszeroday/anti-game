//go:build windows

package watch

import (
	"os/exec"
	"testing"
	"time"

	"github.com/guts/antigame/internal/config"
	"github.com/guts/antigame/internal/store"
	"github.com/guts/antigame/internal/testutil"
	"github.com/guts/antigame/internal/winproc"
)

func TestRealProcessIsTerminatedAtGate(t *testing.T) {
	bin := testutil.BuildFakeGame(t, "fakegame.exe")
	cmd := exec.Command(bin)
	if err := cmd.Start(); err != nil {
		t.Fatalf("sahte oyun baslatilamadi: %v", err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })

	dir := t.TempDir()
	cfg := config.Default()
	cfg.Gated = []config.Game{{Name: "Sahte Oyun", Exe: "fakegame.exe"}}

	var gateCalls []string
	w, err := New(Options{
		Dir:           dir,
		Cfg:           cfg,
		List:          winproc.List,
		Path:          winproc.Path,
		Terminate:     winproc.Terminate,
		Trim:          winproc.Trim,
		Idle:          func() (int, error) { return 0, nil },
		ForegroundPID: func() (int, error) { return 0, nil },
		SpawnGate: func(app string) error {
			gateCalls = append(gateCalls, app)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if err := w.Step(time.Now().UTC()); err != nil {
			t.Fatalf("Step: %v", err)
		}
		if cmd.ProcessState != nil {
			break
		}
		if err := cmd.Process.Signal(nil); err != nil {
			break // process olmus
		}
		time.Sleep(100 * time.Millisecond)
	}
	_ = cmd.Wait()

	ev, _ := store.Read(dir, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	found := false
	for _, e := range ev {
		if e.Ev == "blocked" && e.Exe == "fakegame.exe" {
			found = true
		}
	}
	if !found {
		t.Fatalf("blocked olayi yazilmadi: %+v", ev)
	}
	if len(gateCalls) == 0 {
		t.Error("kapi penceresi acilmadi")
	}
}
