package uninstall

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPromptRejectsEmptyCode(t *testing.T) {
	var out bytes.Buffer
	err := run(t.TempDir(), strings.NewReader("\n"), &out,
		func(string) (bool, string, error) { return false, "Kod hatalı.", nil },
		func() error { t.Fatal("kod dogrulanmadan gorev kaldirildi"); return nil },
	)
	if err == nil {
		t.Fatal("bos kodla kaldirma basarili sayildi")
	}
}

func TestSuccessfulCodeRemovesTask(t *testing.T) {
	var out bytes.Buffer
	removed := false
	err := run(t.TempDir(), strings.NewReader("123456\nh\n"), &out,
		func(string) (bool, string, error) { return true, "ok", nil },
		func() error { removed = true; return nil },
	)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !removed {
		t.Fatal("zamanlanmis gorev kaldirilmadi")
	}
	if !strings.Contains(out.String(), "kaldırıldı") {
		t.Errorf("kullaniciya sonuc bildirilmedi: %s", out.String())
	}
}

func TestRejectedCodeLeavesTaskInstalled(t *testing.T) {
	var out bytes.Buffer
	err := run(t.TempDir(), strings.NewReader("000000\n"), &out,
		func(string) (bool, string, error) { return false, "Kod hatalı.", nil },
		func() error { t.Fatal("gecersiz kodla gorev kaldirildi"); return nil },
	)
	if err == nil {
		t.Fatal("gecersiz kod kabul edildi")
	}
}

func TestPurgeKeepsDataWhenNotAsked(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "iz.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := purge(dir, false, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "iz.txt")); err != nil {
		t.Error("veri silinmemeliydi")
	}
}

func TestPurgeDeletesDataWhenAsked(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "iz.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := purge(dir, true, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("veri dizini silinmeliydi")
	}
}

func TestPurgeKeepsDataWhenTaskRemovalFails(t *testing.T) {
	dir := t.TempDir()
	boom := errors.New("gorev kaldirilamadi")
	if err := purge(dir, true, func() error { return boom }); !errors.Is(err, boom) {
		t.Fatalf("hata yutuldu: %v", err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Error("gorev kaldirilamadiysa veri silinmemeliydi")
	}
}
