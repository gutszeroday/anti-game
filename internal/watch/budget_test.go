//go:build windows

package watch

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/guts/antigame/internal/winproc"
)

// budgetBytes, spec'in izleyici icin koydugu ust sinirdir.
const budgetBytes = 5 << 20

func TestWatcherStaysWithinMemoryBudget(t *testing.T) {
	// Spec 1 saatlik olcum istiyor; normal test dongusunde kullanilamaz,
	// bu yuzden varsayilan kisa tutuldu ve elle uzatilabiliyor.
	seconds := 90
	if v := os.Getenv("ANTIGAME_BUDGET_SECONDS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			t.Fatalf("ANTIGAME_BUDGET_SECONDS sayi olmali: %v", err)
		}
		seconds = n
	}

	bin := filepath.Join(t.TempDir(), "antigame.exe")
	build := exec.Command("go", "build", "-ldflags=-s -w", "-o", bin, "github.com/guts/antigame/cmd/antigame")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("binary derlenemedi: %v\n%s", err, out)
	}

	// Varsayilan liste gercek Riot oyunlarini kapsiyor; test calisirken
	// gelistiricinin acik oyununu oldurmemeli. Eslesmeyen tek bir kayit
	// birakiyoruz: Match dongusu yine her turda calisiyor, olculen kod
	// yolu degismiyor.
	dataDir := t.TempDir()
	appDir := filepath.Join(dataDir, "antigame")
	if err := os.MkdirAll(appDir, 0o700); err != nil {
		t.Fatal(err)
	}
	const isolated = `{"gated":[{"name":"Butce Sondasi","exe":"antigame-budget-probe.exe"}]}`
	if err := os.WriteFile(filepath.Join(appDir, "config.json"), []byte(isolated), 0o600); err != nil {
		t.Fatal(err)
	}

	// --background gercekten calisan moddur; argumansiz "watch" artik
	// yalnizca ayri bir process baslatip cikar, olculecek bir sey birakmaz.
	cmd := exec.Command(bin, "watch", "--background")
	cmd.Env = append(os.Environ(), "LOCALAPPDATA="+dataDir)
	if err := cmd.Start(); err != nil {
		t.Fatalf("izleyici baslatilamadi: %v", err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })

	// Cocuk erken cikarsa Windows PID'i baska bir process'e verebilir ve
	// olcum alakasiz bir process'i gosterir (bir kez 34 MB olcup testi
	// yaniltmisti). En olasi sebep tek ornek kilidi: makinede zaten bir
	// izleyici varsa cocuk hemen cikar.
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()

	deadline := time.Now().Add(time.Duration(seconds) * time.Second)
	var peak uint64
	for time.Now().Before(deadline) {
		select {
		case err := <-exited:
			t.Fatalf("izleyici olcum bitmeden cikti (%v); "+
				"makinede baska bir izleyici calisiyor olabilir", err)
		default:
		}
		ws, err := winproc.WorkingSet(cmd.Process.Pid)
		if err != nil {
			t.Fatalf("olcum basarisiz: %v", err)
		}
		peak = max(peak, ws)
		time.Sleep(2 * time.Second)
	}

	t.Logf("izleyici tepe calisma kumesi: %.2f MB (%d sn boyunca)",
		float64(peak)/(1<<20), seconds)
	if peak > budgetBytes {
		t.Errorf("bellek butcesi asildi: %.2f MB, sinir %.2f MB",
			float64(peak)/(1<<20), float64(budgetBytes)/(1<<20))
	}
}
