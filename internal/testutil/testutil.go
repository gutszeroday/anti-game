// Package testutil, entegrasyon testleri icin ortak yardimcilar saglar.
package testutil

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// BuildFakeGame, sahte oyun binary'sini verilen adla derler ve yolunu dondurur.
// Ad onemlidir: kapida durdurma testleri exe adiyla eslesir.
func BuildFakeGame(t *testing.T, name string) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), name)
	cmd := exec.Command("go", "build", "-o", out, "github.com/guts/antigame/testdata/fakegame")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("sahte oyun derlenemedi: %v\n%s", err, b)
	}
	return out
}
