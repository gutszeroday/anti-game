package uninstall

import (
	"bytes"
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
