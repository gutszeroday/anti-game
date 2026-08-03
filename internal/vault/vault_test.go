//go:build windows

package vault

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadWithoutSetupReturnsErrNoSecret(t *testing.T) {
	if _, err := Load(t.TempDir()); !errors.Is(err, ErrNoSecret) {
		t.Fatalf("ErrNoSecret bekleniyordu, %v geldi", err)
	}
}

func TestSaveThenLoad(t *testing.T) {
	dir := t.TempDir()
	secret := bytes.Repeat([]byte{0xa7}, 20)
	if err := Save(dir, secret); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !bytes.Equal(got, secret) {
		t.Errorf("secret bozuldu: %x", got)
	}
}

func TestSavedFileDoesNotContainPlaintext(t *testing.T) {
	dir := t.TempDir()
	secret := []byte("AAAAAAAAAAAAAAAAAAAA")
	if err := Save(dir, secret); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "secret.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, secret) {
		t.Fatal("secret.bin duz metin iceriyor")
	}
}
