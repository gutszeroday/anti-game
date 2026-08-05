//go:build windows

package vault

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/guts/antigame/internal/dpapi"
)

func TestLoadWithoutSetupReturnsErrNoSecret(t *testing.T) {
	if _, err := LoadPerson(t.TempDir(), "p1"); !errors.Is(err, ErrNoSecret) {
		t.Fatalf("ErrNoSecret bekleniyordu, %v geldi", err)
	}
}

func TestSaveThenLoad(t *testing.T) {
	dir := t.TempDir()
	secret := bytes.Repeat([]byte{0xa7}, 20)
	if err := SavePerson(dir, "p1", secret); err != nil {
		t.Fatalf("SavePerson: %v", err)
	}
	got, err := LoadPerson(dir, "p1")
	if err != nil {
		t.Fatalf("LoadPerson: %v", err)
	}
	if !bytes.Equal(got, secret) {
		t.Errorf("secret bozuldu: %x", got)
	}
}

func TestPeopleKeepSeparateSecrets(t *testing.T) {
	dir := t.TempDir()
	first := bytes.Repeat([]byte{0x01}, 20)
	second := bytes.Repeat([]byte{0x02}, 20)
	if err := SavePerson(dir, "p1", first); err != nil {
		t.Fatal(err)
	}
	if err := SavePerson(dir, "p2", second); err != nil {
		t.Fatal(err)
	}
	got, err := LoadPerson(dir, "p1")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, first) {
		t.Error("ikinci kisi birincinin anahtarini ezdi")
	}
}

func TestRemovePersonLeavesOthers(t *testing.T) {
	dir := t.TempDir()
	if err := SavePerson(dir, "p1", bytes.Repeat([]byte{1}, 20)); err != nil {
		t.Fatal(err)
	}
	if err := SavePerson(dir, "p2", bytes.Repeat([]byte{2}, 20)); err != nil {
		t.Fatal(err)
	}
	if err := RemovePerson(dir, "p1"); err != nil {
		t.Fatal(err)
	}
	if HasPerson(dir, "p1") {
		t.Error("p1 silinmedi")
	}
	if !HasPerson(dir, "p2") {
		t.Error("p2 de silindi")
	}
}

func TestRemoveMissingPersonIsNotAnError(t *testing.T) {
	if err := RemovePerson(t.TempDir(), "p9"); err != nil {
		t.Fatalf("olmayan dosya hata verdi: %v", err)
	}
}

func TestRejectsIDThatEscapesDirectory(t *testing.T) {
	dir := t.TempDir()
	for _, id := range []string{"../evil", "p1/../..", "P1", "", "p 1"} {
		if err := SavePerson(dir, id, []byte("x")); err == nil {
			t.Errorf("%q kabul edildi", id)
		}
	}
}

func TestSavedFileDoesNotContainPlaintext(t *testing.T) {
	dir := t.TempDir()
	secret := []byte("AAAAAAAAAAAAAAAAAAAA")
	if err := SavePerson(dir, "p1", secret); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "secret-p1.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, secret) {
		t.Fatal("anahtar dosyasi duz metin iceriyor")
	}
}

// writeLegacy, tek kisilik donemin secret.bin dosyasini olusturur.
func writeLegacy(t *testing.T, dir string, secret []byte) {
	t.Helper()
	blob, err := dpapi.Protect(secret)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, legacyFile), blob, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateLegacyMovesSecretToPerson(t *testing.T) {
	dir := t.TempDir()
	secret := bytes.Repeat([]byte{0x5c}, 20)
	writeLegacy(t, dir, secret)

	moved, err := MigrateLegacy(dir, "p1")
	if err != nil {
		t.Fatalf("MigrateLegacy: %v", err)
	}
	if !moved {
		t.Fatal("tasima olmadi")
	}
	got, err := LoadPerson(dir, "p1")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, secret) {
		t.Error("tasinan anahtar bozuldu")
	}
	if _, err := os.Stat(filepath.Join(dir, legacyFile)); !os.IsNotExist(err) {
		t.Error("eski secret.bin duruyor")
	}
}

func TestMigrateLegacyIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	writeLegacy(t, dir, bytes.Repeat([]byte{0x5c}, 20))
	if _, err := MigrateLegacy(dir, "p1"); err != nil {
		t.Fatal(err)
	}
	moved, err := MigrateLegacy(dir, "p1")
	if err != nil {
		t.Fatalf("ikinci tasima hata verdi: %v", err)
	}
	if moved {
		t.Error("ikinci kez tasidi")
	}
}

// Yarida kesilen bir tasimada iki dosya da diskte kalir; sonraki calisma
// eski dosyayi yeni anahtarla ayni gorup temizlemeli.
func TestMigrateLegacyCleansUpAfterInterruptedRun(t *testing.T) {
	dir := t.TempDir()
	secret := bytes.Repeat([]byte{0x5c}, 20)
	writeLegacy(t, dir, secret)
	if err := SavePerson(dir, "p1", secret); err != nil {
		t.Fatal(err)
	}

	moved, err := MigrateLegacy(dir, "p1")
	if err != nil {
		t.Fatal(err)
	}
	if moved {
		t.Error("zaten tasinmis anahtar yeniden tasindi")
	}
	if _, err := os.Stat(filepath.Join(dir, legacyFile)); !os.IsNotExist(err) {
		t.Error("artik gereksiz secret.bin silinmedi")
	}
}

func TestMigrateLegacyKeepsDifferentExistingKey(t *testing.T) {
	dir := t.TempDir()
	writeLegacy(t, dir, bytes.Repeat([]byte{0x01}, 20))
	current := bytes.Repeat([]byte{0x02}, 20)
	if err := SavePerson(dir, "p1", current); err != nil {
		t.Fatal(err)
	}

	if _, err := MigrateLegacy(dir, "p1"); err != nil {
		t.Fatal(err)
	}
	got, err := LoadPerson(dir, "p1")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, current) {
		t.Error("calisan anahtar eski dosyayla ezildi")
	}
}

func TestOrphansCountsUnknownKeyFiles(t *testing.T) {
	dir := t.TempDir()
	for _, id := range []string{"p1", "p2", "p7"} {
		if err := SavePerson(dir, id, []byte("secret")); err != nil {
			t.Fatal(err)
		}
	}
	n, err := Orphans(dir, []string{"p1", "p2"})
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("yetim sayisi 1 olmaliydi, %d geldi", n)
	}
}
