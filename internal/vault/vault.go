//go:build windows

// Package vault, TOTP secret'lerini diskte sifreli tutar.
//
// Her kisinin kendi dosyasi vardir: secret-<id>.bin. Tek dosyada
// toplanmadilar, cunku bir kisiyi silmek digerlerinin anahtarini yeniden
// yazmayi gerektirmemeli.
package vault

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/guts/antigame/internal/dpapi"
)

// ErrNoSecret, kurulum henuz yapilmadiginda doner.
var ErrNoSecret = errors.New("MFA kurulumu yapılmamış")

// legacyFile, tek kisilik donemden kalan anahtar dosyasidir.
const legacyFile = "secret.bin"

const (
	filePrefix = "secret-"
	fileSuffix = ".bin"
)

// personFile, ID'yi dosya adina cevirir. ID burada bir kez daha
// dogrulanir: yazma katmani, cagiran kodun dogrulamayi atladigi durumda
// da dizin disina cikilmasina izin vermemeli.
func personFile(dir, id string) (string, error) {
	if id == "" || len(id) > 16 {
		return "", fmt.Errorf("geçersiz kişi kimliği: %q", id)
	}
	for _, r := range id {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return "", fmt.Errorf("geçersiz kişi kimliği: %q", id)
		}
	}
	return filepath.Join(dir, filePrefix+id+fileSuffix), nil
}

// write, blob'u once gecici dosyaya yazip yer degistirerek atomik yazar.
// Yarim yazilmis bir anahtar dosyasi, kisiyi kapidan tumden dusururdu.
func write(path string, blob []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, blob, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// SavePerson, kisinin anahtarini sifreleyip kaydeder.
func SavePerson(dir, id string, secret []byte) error {
	path, err := personFile(dir, id)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	blob, err := dpapi.Protect(secret)
	if err != nil {
		return fmt.Errorf("secret şifrelenemedi: %w", err)
	}
	return write(path, blob)
}

// LoadPerson, kisinin anahtarini cozer. Dosya yoksa ErrNoSecret doner.
func LoadPerson(dir, id string) ([]byte, error) {
	path, err := personFile(dir, id)
	if err != nil {
		return nil, err
	}
	blob, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, ErrNoSecret
	}
	if err != nil {
		return nil, err
	}
	secret, err := dpapi.Unprotect(blob)
	if err != nil {
		return nil, fmt.Errorf("anahtar çözülemedi (Windows profili değişmiş olabilir): %w", err)
	}
	return secret, nil
}

// RemovePerson, kisinin anahtar dosyasini siler. Dosya zaten yoksa hata
// vermez: kayit ile dosya arasindaki tutarsizligi silme islemi duzeltmeli.
func RemovePerson(dir, id string) error {
	path, err := personFile(dir, id)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// HasPerson, kisinin anahtar dosyasinin var olup olmadigini soyler.
// Dosyanin cozulup cozulemedigine bakmaz.
func HasPerson(dir, id string) bool {
	path, err := personFile(dir, id)
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

// MigrateLegacy, tek kisilik donemden kalan secret.bin dosyasini verilen
// ID'ye tasir. Tasindiysa true doner.
//
// Sira onemli: once kopya yazilir ve okunabildigi dogrulanir, sonra eski
// dosya silinir. Ters sirada, yarida kesilen bir tasima tek anahtari yok
// eder ve kullanici kapinin disinda kalir.
func MigrateLegacy(dir, id string) (bool, error) {
	legacy := filepath.Join(dir, legacyFile)
	blob, err := os.ReadFile(legacy)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	secret, err := dpapi.Unprotect(blob)
	if err != nil {
		return false, fmt.Errorf("secret.bin çözülemedi (Windows profili değişmiş olabilir); `antigame setup` ile yeniden kurun: %w", err)
	}

	// Hedef doluysa uzerine yazmiyoruz: ayni anahtarsa eski dosya
	// gereksiz, farkliysa silmek calisan bir anahtari yok etmek olurdu.
	if cur, err := LoadPerson(dir, id); err == nil {
		if bytes.Equal(cur, secret) {
			return false, os.Remove(legacy)
		}
		return false, nil
	}

	if err := SavePerson(dir, id, secret); err != nil {
		return false, err
	}
	if back, err := LoadPerson(dir, id); err != nil || !bytes.Equal(back, secret) {
		return false, fmt.Errorf("anahtar taşındı ama geri okunamadı: %w", err)
	}
	return true, os.Remove(legacy)
}

// Orphans, hicbir kisiye ait olmayan anahtar dosyalarinin sayisini
// dondurur. Bu dosyalar silinmez: elle bozulmus bir config.json'i sessizce
// temizlemek veri kaybidir.
func Orphans(dir string, known []string) (int, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	set := make(map[string]bool, len(known))
	for _, id := range known {
		set[id] = true
	}
	n := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasPrefix(name, filePrefix) || !strings.HasSuffix(name, fileSuffix) {
			continue
		}
		id := strings.TrimSuffix(strings.TrimPrefix(name, filePrefix), fileSuffix)
		if !set[id] {
			n++
		}
	}
	return n, nil
}
