//go:build windows

// Package datadir, veri klasorunu bir yerden bir yere tasir.
//
// Kayit defterine dokunmuyor: hangi klasorun etkin oldugu config
// paketinin isi. Buradaki dort adim (dogrula, kopyala, dogrula, sil)
// ayri ayri cagriliyor cunku aralarina ayarin yazilmasi ve izleyicinin
// yeni yeri okumasi giriyor.
package datadir

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// markerFile, bir klasorde antigame verisi olup olmadigini soyleyen
// dosyadir.
const markerFile = "config.json"

// Validate, tasimanin guvenli olup olmadigini soyler.
func Validate(from, to string) error {
	if !filepath.IsAbs(to) {
		return fmt.Errorf("tam bir yol girin (ör. D:\\antigame)")
	}
	from = filepath.Clean(from)
	to = filepath.Clean(to)

	if strings.EqualFold(from, to) {
		return fmt.Errorf("veriler zaten burada")
	}
	// Hedef kaynagin icindeyse kopyalama kendi ciktisini kopyalamaya
	// calisir ve disk dolana kadar surer.
	if strings.HasPrefix(strings.ToLower(to), strings.ToLower(from)+string(filepath.Separator)) {
		return fmt.Errorf("hedef klasör kaynağın içinde olamaz")
	}

	fi, err := os.Stat(to)
	switch {
	case os.IsNotExist(err):
		return nil
	case err != nil:
		return err
	case !fi.IsDir():
		return fmt.Errorf("%s bir klasör değil", to)
	}

	// Iki veri kumesini birlestirmek belirsiz: hangi kisinin sayaci
	// gecerli sorusunun dogru cevabi yok.
	if _, err := os.Stat(filepath.Join(to, markerFile)); err == nil {
		return fmt.Errorf("bu klasörde zaten antigame verisi var; boş bir klasör seçin")
	}
	return nil
}

// files, dizindeki duz dosyalari verir. Alt dizinler atlaniyor:
// antigame duz bir dizin kullaniyor, alt dizin varsa baskasinindir.
func files(dir string) (map[string]int64, error) {
	des, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := make(map[string]int64, len(des))
	for _, de := range des {
		if de.IsDir() {
			continue
		}
		fi, err := de.Info()
		if err != nil {
			return nil, err
		}
		out[de.Name()] = fi.Size()
	}
	return out, nil
}

// Copy, kaynaktaki dosyalari hedefe kopyalar. Hedef yoksa olusturulur.
func Copy(from, to string) error {
	src, err := files(from)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(to, 0o700); err != nil {
		return err
	}
	for name := range src {
		if err := copyFile(filepath.Join(from, name), filepath.Join(to, name)); err != nil {
			return fmt.Errorf("%s kopyalanamadı: %w", name, err)
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		// Yarim dosya birakmiyoruz: dogrulama zaten yakalardi ama
		// hedefte gecerli gorunen bir cop kalmasi kotu.
		os.Remove(dst)
		return err
	}
	return out.Close()
}

// Verify, hedefin kaynaktaki her dosyayi ayni boyutta icerdigini
// dogrular. Icerik karsilastirilmiyor: kopyalama hatasi pratikte
// eksik ya da yarim dosya olarak gorunur, ve tam karsilastirma
// yuz megabaytlik gunlukleri iki kez okumak demek olurdu.
func Verify(from, to string) error {
	src, err := files(from)
	if err != nil {
		return err
	}
	dst, err := files(to)
	if err != nil {
		return err
	}
	for name, size := range src {
		got, ok := dst[name]
		if !ok {
			return fmt.Errorf("%s hedefe kopyalanmamış", name)
		}
		if got != size {
			return fmt.Errorf("%s eksik kopyalanmış (%d/%d bayt)", name, got, size)
		}
	}
	return nil
}

// RemoveContents, dizindeki dosyalari siler; dizinin kendisi kalir.
//
// Dizini silmemek bilincli: Dosya Gezgini'nde acik duran bir pencereyi
// bozardi ve bos bir dizin zarar vermiyor.
func RemoveContents(dir string) error {
	names, err := files(dir)
	if err != nil {
		return err
	}
	for name := range names {
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			return err
		}
	}
	return nil
}
