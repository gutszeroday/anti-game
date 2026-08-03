//go:build windows

package winproc

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/guts/antigame/internal/testutil"
)

func TestListIncludesCurrentProcess(t *testing.T) {
	procs, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	me := os.Getpid()
	for _, p := range procs {
		if p.PID == me {
			if p.Exe == "" {
				t.Fatal("mevcut process'in exe adi bos")
			}
			return
		}
	}
	t.Fatalf("mevcut process (%d) listede yok, %d process listelendi", me, len(procs))
}

func TestPathReturnsFullPathOfCurrentProcess(t *testing.T) {
	p, err := Path(os.Getpid())
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if !filepath.IsAbs(p) {
		t.Errorf("tam yol beklendi, %q geldi", p)
	}
}

func TestTerminateKillsGatedProcess(t *testing.T) {
	bin := testutil.BuildFakeGame(t, "fakegame.exe")
	cmd := exec.Command(bin)
	if err := cmd.Start(); err != nil {
		t.Fatalf("sahte oyun baslatilamadi: %v", err)
	}
	pid := cmd.Process.Pid
	t.Cleanup(func() { _ = cmd.Process.Kill() })

	// Process'in listede gorundugunu dogrula.
	found := false
	for i := 0; i < 40 && !found; i++ {
		procs, _ := List()
		for _, p := range procs {
			if p.PID == pid && strings.EqualFold(p.Exe, "fakegame.exe") {
				found = true
			}
		}
		if !found {
			time.Sleep(50 * time.Millisecond)
		}
	}
	if !found {
		t.Fatal("baslatilan sahte oyun listede gorunmedi")
	}

	if err := Terminate(pid); err != nil {
		t.Fatalf("Terminate: %v", err)
	}
	_ = cmd.Wait()

	procs, _ := List()
	for _, p := range procs {
		if p.PID == pid {
			t.Fatal("sonlandirilan process hala listede")
		}
	}
}

func TestTerminateUnknownPIDReturnsError(t *testing.T) {
	if err := Terminate(0x7fffffff); err == nil {
		t.Fatal("olmayan PID hata vermeden sonlandirildi")
	}
}

func TestTrimSucceeds(t *testing.T) {
	if err := Trim(); err != nil {
		t.Fatalf("Trim: %v", err)
	}
}
