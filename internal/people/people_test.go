//go:build windows

package people

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/guts/antigame/internal/config"
	"github.com/guts/antigame/internal/dpapi"
	"github.com/guts/antigame/internal/store"
	"github.com/guts/antigame/internal/vault"
)

func secretOf(b byte) []byte { return bytes.Repeat([]byte{b}, 20) }

// seed, verilen adlarla kisi ekler ve dizini dondurur.
func seed(t *testing.T, names ...string) string {
	t.Helper()
	dir := t.TempDir()
	for i, n := range names {
		if _, err := Add(dir, n, "", secretOf(byte(i+1)), 0); err != nil {
			t.Fatalf("Add(%s): %v", n, err)
		}
	}
	return dir
}

func TestAddStoresPersonAndKey(t *testing.T) {
	dir := seed(t, "Baran")
	entries, err := List(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name != "Baran" {
		t.Fatalf("kisi kaydedilmedi: %+v", entries)
	}
	if !entries[0].Usable() {
		t.Error("anahtar kullanilabilir degil")
	}
	got, err := vault.LoadPerson(dir, entries[0].ID)
	if err != nil || !bytes.Equal(got, secretOf(1)) {
		t.Errorf("anahtar bozuk: %v", err)
	}
}

func TestAddGivesEachPersonSeparateID(t *testing.T) {
	dir := seed(t, "Baran", "Ali")
	entries, _ := List(dir)
	if len(entries) != 2 || entries[0].ID == entries[1].ID {
		t.Fatalf("kimlikler ayrismadi: %+v", entries)
	}
}

func TestAddBurnsPairingCounter(t *testing.T) {
	dir := t.TempDir()
	p, err := Add(dir, "Baran", "", secretOf(1), 4242)
	if err != nil {
		t.Fatal(err)
	}
	st, _ := store.LoadState(dir)
	if st.Counter(p.ID) != 4242 {
		t.Errorf("eslestirme kodu yakilmadi: %d", st.Counter(p.ID))
	}
}

func TestKeysReturnsEveryReadableKey(t *testing.T) {
	dir := seed(t, "Baran", "Ali")
	keys, err := Keys(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 {
		t.Fatalf("iki anahtar bekleniyordu: %+v", keys)
	}
}

func TestEditKeepsKeyIntact(t *testing.T) {
	dir := seed(t, "Baran")
	entries, _ := List(dir)
	id := entries[0].ID

	if err := Edit(dir, id, "Baran Y.", "Telegram"); err != nil {
		t.Fatal(err)
	}
	entries, _ = List(dir)
	if entries[0].Name != "Baran Y." || entries[0].Hint != "Telegram" {
		t.Errorf("duzenleme uygulanmadi: %+v", entries[0])
	}
	got, err := vault.LoadPerson(dir, id)
	if err != nil || !bytes.Equal(got, secretOf(1)) {
		t.Error("duzenleme anahtari degistirdi")
	}
}

func TestRotateReplacesKeyAndBurnsNewCounter(t *testing.T) {
	dir := seed(t, "Baran")
	entries, _ := List(dir)
	id := entries[0].ID

	if err := Rotate(dir, id, secretOf(9), 77); err != nil {
		t.Fatal(err)
	}
	got, err := vault.LoadPerson(dir, id)
	if err != nil || !bytes.Equal(got, secretOf(9)) {
		t.Errorf("anahtar yenilenmedi: %v", err)
	}
	st, _ := store.LoadState(dir)
	if st.Counter(id) != 77 {
		t.Errorf("yeni sayac yazilmadi: %d", st.Counter(id))
	}
}

func TestRemoveDropsRecordAndKey(t *testing.T) {
	dir := seed(t, "Baran", "Ali")
	entries, _ := List(dir)
	id := entries[1].ID

	if err := Remove(dir, id); err != nil {
		t.Fatal(err)
	}
	entries, _ = List(dir)
	if len(entries) != 1 || entries[0].Name != "Baran" {
		t.Fatalf("kayit silinmedi: %+v", entries)
	}
	if vault.HasPerson(dir, id) {
		t.Error("anahtar dosyasi duruyor")
	}
}

// Silme sonrasi kapiyi acabilecek kimse kalmiyorsa kullanici disarida
// kalir; islem reddedilmeli.
func TestRemoveRefusesToLeaveNobodyWithAKey(t *testing.T) {
	dir := seed(t, "Baran")
	entries, _ := List(dir)

	err := Remove(dir, entries[0].ID)
	if !errors.Is(err, ErrLastKey) {
		t.Fatalf("son anahtar silindi: %v", err)
	}
	if entries, _ = List(dir); len(entries) != 1 {
		t.Error("reddedilen silme kaydi bozdu")
	}
}

// "Son kisi" kontrolu yetmez: anahtari olmayan bir kisi kalmasi da
// kapiyi acilmaz yapar.
func TestRemoveCountsOnlyUsableKeys(t *testing.T) {
	dir := seed(t, "Baran", "Ali")
	entries, _ := List(dir)
	// Ali'nin anahtar dosyasi elle silinmis olsun.
	if err := vault.RemovePerson(dir, entries[1].ID); err != nil {
		t.Fatal(err)
	}

	if err := Remove(dir, entries[0].ID); !errors.Is(err, ErrLastKey) {
		t.Fatalf("anahtarsiz kisi calisir sayildi: %v", err)
	}
}

func TestRemoveUnknownPersonReportsNotFound(t *testing.T) {
	dir := seed(t, "Baran", "Ali")
	if err := Remove(dir, "p99"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("bilinmeyen kisi icin ErrNotFound bekleniyordu: %v", err)
	}
}

func TestRemoveDetachesOpenSessionFromDeletedPerson(t *testing.T) {
	dir := seed(t, "Baran", "Ali")
	entries, _ := List(dir)
	id := entries[1].ID

	st, _ := store.LoadState(dir)
	st.Session = &store.Session{OpenedBy: id}
	if err := store.SaveState(dir, st); err != nil {
		t.Fatal(err)
	}

	if err := Remove(dir, id); err != nil {
		t.Fatal(err)
	}
	st, _ = store.LoadState(dir)
	if st.Session == nil || st.Session.OpenedBy != "" {
		t.Errorf("silinen kisi oturumda kaldi: %+v", st.Session)
	}
}

func TestListMarksPersonWithoutKey(t *testing.T) {
	dir := seed(t, "Baran", "Ali")
	entries, _ := List(dir)
	if err := vault.RemovePerson(dir, entries[1].ID); err != nil {
		t.Fatal(err)
	}

	entries, _ = List(dir)
	if len(entries) != 2 {
		t.Fatalf("anahtarsiz kisi listeden dusuruldu: %+v", entries)
	}
	if entries[1].HasKey || entries[1].Usable() {
		t.Error("anahtarsiz kisi calisir gorunuyor")
	}
}

// Tek kisilik kurulumdan gelen dizinde kisi listesi ve anahtar dosyasi
// otomatik olusmali; kullanici yeniden eslestirmeye zorlanmamali.
func TestEnsureMigratesLegacySetup(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	cfg.FriendName, cfg.FriendHint = "Baran", "WhatsApp"
	if err := config.Save(dir, cfg); err != nil {
		t.Fatal(err)
	}
	blob, err := dpapi.Protect(secretOf(3))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "secret.bin"), blob, 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := Ensure(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.People) != 1 || got.People[0].Name != "Baran" {
		t.Fatalf("kisi olusmadi: %+v", got.People)
	}
	secret, err := vault.LoadPerson(dir, "p1")
	if err != nil || !bytes.Equal(secret, secretOf(3)) {
		t.Errorf("anahtar tasinmadi: %v", err)
	}
	// Kalici olmali: ikinci okumada da ayni sonuc gelmeli.
	again, err := config.Load(dir)
	if err != nil || len(again.People) != 1 {
		t.Errorf("goc diske yazilmadi: %v %+v", err, again)
	}
	if _, err := os.Stat(filepath.Join(dir, "secret.bin")); !os.IsNotExist(err) {
		t.Error("eski anahtar dosyasi duruyor")
	}
}

func TestOrphanKeyFilesAreReportedNotDeleted(t *testing.T) {
	dir := seed(t, "Baran")
	if err := vault.SavePerson(dir, "p9", secretOf(9)); err != nil {
		t.Fatal(err)
	}
	n, err := Orphans(dir)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("yetim dosya sayisi 1 olmaliydi: %d", n)
	}
	if !vault.HasPerson(dir, "p9") {
		t.Error("yetim dosya silindi")
	}
}
