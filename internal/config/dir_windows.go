//go:build windows

package config

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

// Veri klasorunun yeri kayit defterinde tutuluyor.
//
// Alternatif olarak varsayilan klasore bir isaretci dosya konabilirdi;
// ama o dosya tam da tasimanin sildigi yerde yasardi. Kayit defteri
// klasor silinse de kaliyor ve HKCU yonetici hakki gerektirmiyor.
const (
	regPath  = `Software\antigame`
	regValue = "DataDir"
)

// DefaultDir, ayar yapilmadiginda kullanilan yerdir.
func DefaultDir() string {
	return filepath.Join(os.Getenv("LOCALAPPDATA"), "antigame")
}

// Dir, kalici veri dizinini dondurur.
//
// Onbellege alinmiyor: izleyicinin klasor degisikligini fark etmesi
// buna bagli. Kayit defteri okumasi mikrosaniyeler suruyor, dongude
// sorun degil.
//
// Kayitli deger bozuksa (bos ya da goreli yol) varsayilana dusuluyor:
// okunamayan bir ayar yuzunden programin calismamasi, yanlis yere
// yazmasindan da kotu olurdu.
func Dir() string {
	k, err := registry.OpenKey(registry.CURRENT_USER, regPath, registry.QUERY_VALUE)
	if err != nil {
		return DefaultDir()
	}
	defer k.Close()

	s, _, err := k.GetStringValue(regValue)
	if err != nil || !filepath.IsAbs(s) {
		return DefaultDir()
	}
	return filepath.Clean(s)
}

// SetDir, veri klasorunu kalici olarak degistirir. Dosyalari tasimak
// cagirana ait (bkz. internal/datadir).
func SetDir(path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("veri klasörü için tam bir yol gerekiyor: %q", path)
	}
	k, _, err := registry.CreateKey(registry.CURRENT_USER, regPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetStringValue(regValue, filepath.Clean(path))
}

// ClearDir, ayari kaldirir ve varsayilan klasore doner.
func ClearDir() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, regPath, registry.SET_VALUE)
	if err != nil {
		// Anahtar hic yoksa yapacak bir sey yok.
		return nil
	}
	defer k.Close()
	if err := k.DeleteValue(regValue); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
