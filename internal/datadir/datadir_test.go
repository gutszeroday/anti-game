//go:build windows

package datadir

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func put(t *testing.T, dir, name string, n int) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), make([]byte, n), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestValidateAcceptsAnEmptyTarget(t *testing.T) {
	from, to := t.TempDir(), t.TempDir()
	if err := Validate(from, to); err != nil {
		t.Errorf("gecerli hedef reddedildi: %v", err)
	}
}

func TestValidateRejectsTheSourceItself(t *testing.T) {
	from := t.TempDir()
	if err := Validate(from, from); err == nil {
		t.Error("kaynagin kendisi hedef olarak kabul edildi")
	}
}

// Hedef kaynagin icindeyse kopyalama kendi ciktisini kopyalamaya
// calisir; sonu dolan disk.
func TestValidateRejectsATargetInsideTheSource(t *testing.T) {
	from := t.TempDir()
	to := filepath.Join(from, "alt")
	if err := Validate(from, to); err == nil {
		t.Error("kaynagin icindeki hedef kabul edildi")
	}
}

func TestValidateRejectsARelativePath(t *testing.T) {
	if err := Validate(t.TempDir(), "veriler"); err == nil {
		t.Error("goreli yol kabul edildi")
	}
}

// Iki veri kumesini birlestirmek belirsiz: hangi kisinin sayaci
// gecerli sorusunun dogru cevabi yok.
func TestValidateRejectsATargetThatAlreadyHasData(t *testing.T) {
	from, to := t.TempDir(), t.TempDir()
	put(t, to, "config.json", 10)
	err := Validate(from, to)
	if err == nil {
		t.Fatal("dolu hedef kabul edildi")
	}
	if !strings.Contains(err.Error(), "zaten") {
		t.Errorf("sebep anlatilmamis: %v", err)
	}
}

func TestValidateAcceptsATargetThatDoesNotExistYet(t *testing.T) {
	from := t.TempDir()
	to := filepath.Join(t.TempDir(), "yeni")
	if err := Validate(from, to); err != nil {
		t.Errorf("olmayan hedef reddedildi: %v", err)
	}
}

func TestCopyMovesEveryFile(t *testing.T) {
	from, to := t.TempDir(), t.TempDir()
	put(t, from, "config.json", 10)
	put(t, from, "secret-p1.bin", 246)
	put(t, from, "events-2026-08.jsonl", 1000)

	if err := Copy(from, to); err != nil {
		t.Fatal(err)
	}
	for name, size := range map[string]int64{
		"config.json": 10, "secret-p1.bin": 246, "events-2026-08.jsonl": 1000,
	} {
		fi, err := os.Stat(filepath.Join(to, name))
		if err != nil {
			t.Errorf("%s kopyalanmamis: %v", name, err)
			continue
		}
		if fi.Size() != size {
			t.Errorf("%s boyutu %d, istenen %d", name, fi.Size(), size)
		}
	}
}

func TestCopyCreatesTheTarget(t *testing.T) {
	from := t.TempDir()
	to := filepath.Join(t.TempDir(), "yeni", "derin")
	put(t, from, "config.json", 5)

	if err := Copy(from, to); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(to, "config.json")); err != nil {
		t.Errorf("hedef olusturulmamis: %v", err)
	}
}

// antigame duz bir dizin kullaniyor; alt dizinler baskasina ait.
func TestCopySkipsSubdirectories(t *testing.T) {
	from, to := t.TempDir(), t.TempDir()
	if err := os.Mkdir(filepath.Join(from, "alt"), 0o700); err != nil {
		t.Fatal(err)
	}
	put(t, from, "config.json", 5)

	if err := Copy(from, to); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(to, "alt")); !os.IsNotExist(err) {
		t.Error("alt dizin kopyalanmis")
	}
}

func TestVerifyAcceptsAFaithfulCopy(t *testing.T) {
	from, to := t.TempDir(), t.TempDir()
	put(t, from, "config.json", 10)
	put(t, from, "state.json", 20)
	if err := Copy(from, to); err != nil {
		t.Fatal(err)
	}
	if err := Verify(from, to); err != nil {
		t.Errorf("dogru kopya reddedildi: %v", err)
	}
}

func TestVerifyCatchesAMissingFile(t *testing.T) {
	from, to := t.TempDir(), t.TempDir()
	put(t, from, "config.json", 10)
	put(t, from, "state.json", 20)
	put(t, to, "config.json", 10)

	err := Verify(from, to)
	if err == nil {
		t.Fatal("eksik dosya fark edilmedi")
	}
	if !strings.Contains(err.Error(), "state.json") {
		t.Errorf("hangi dosyanin eksik oldugu yazmiyor: %v", err)
	}
}

func TestVerifyCatchesATruncatedFile(t *testing.T) {
	from, to := t.TempDir(), t.TempDir()
	put(t, from, "config.json", 100)
	put(t, to, "config.json", 40)

	if err := Verify(from, to); err == nil {
		t.Fatal("yarim kopyalanmis dosya fark edilmedi")
	}
}

func TestRemoveContentsLeavesTheDirectory(t *testing.T) {
	dir := t.TempDir()
	put(t, dir, "config.json", 5)
	put(t, dir, "state.json", 5)

	if err := RemoveContents(dir); err != nil {
		t.Fatal(err)
	}
	es, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("dizin de silinmis: %v", err)
	}
	if len(es) != 0 {
		t.Errorf("%d giris kalmis", len(es))
	}
}
