//go:build windows

// Package datainfo, antigame'in veri dizinini insanin okuyabilecegi bir
// listeye cevirir: hangi dosya ne ise yariyor, ne kadar yer tutuyor.
//
// Karar vermez ve hicbir dosyayi degistirmez; yalnizca anlatir. Kisi
// listesi disaridan aliniyor cunku anahtar dosyasinin kime ait oldugunu
// soylemek icin gerekli, ama bu paketin yapilandirmayi okumasina gerek
// yok.
package datainfo

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/guts/antigame/internal/config"
)

type Kind int

const (
	KindConfig Kind = iota
	KindKey
	KindState
	KindEvents
	KindUnknown
)

type Entry struct {
	Name string
	Size int64
	Kind Kind
	Desc string
}

const (
	configFile = "config.json"
	stateFile  = "state.json"
	keyPrefix  = "secret-p"
	keySuffix  = ".bin"
	eventsPre  = "events-"
	eventsSuf  = ".jsonl"
)

// List, dizindeki dosyalari aciklamalariyla dondurur.
//
// Dizin yoksa bos liste doner, hata degil: kurulum yapilmamis bir
// makinede dizin henuz olusmamistir ve bu bir ariza degildir.
func List(dir string, people []config.Person) ([]Entry, error) {
	des, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	names := make(map[string]string, len(people))
	for _, p := range people {
		names[p.ID] = p.Name
	}

	out := make([]Entry, 0, len(des))
	for _, de := range des {
		if de.IsDir() {
			continue
		}
		e := Entry{Name: de.Name()}
		if fi, err := de.Info(); err == nil {
			e.Size = fi.Size()
		}
		e.Kind, e.Desc = describe(de.Name(), names)
		out = append(out, e)
	}

	// Tur sonra ad: her acilista ayni sirayi vermek, kullanicinin
	// aradigini ayni yerde bulmasini sagliyor.
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func describe(name string, names map[string]string) (Kind, string) {
	switch {
	case name == configFile:
		return KindConfig, "Oyun listesi ve kişiler"

	case name == stateFile:
		return KindState, "Açık oturum, kod sayaçları, son görülme"

	case strings.HasPrefix(name, keyPrefix) && strings.HasSuffix(name, keySuffix):
		id := "p" + strings.TrimSuffix(strings.TrimPrefix(name, keyPrefix), keySuffix)
		if who, ok := names[id]; ok {
			return KindKey, who + " kişisinin anahtarı (DPAPI ile şifreli)"
		}
		// Kisi silinmis ama dosyasi kalmis olabilir. Silmiyoruz,
		// yalnizca soyluyoruz: geri donusu olmayan bir islemi
		// kullanici gormeden yapmamali.
		return KindKey, "Anahtar dosyası — sahibi yok (kişi listesinde karşılığı bulunamadı)"

	case strings.HasPrefix(name, eventsPre) && strings.HasSuffix(name, eventsSuf):
		return KindEvents, "Süre kayıtları — " + month(name)

	default:
		// "antigame'e ait degil" demek yanlis olurdu: config.json.bak
		// gibi dosyalar antigame'in birakip artik okumadigi seyler
		// olabilir. Soylenebilecek dogru sey yalnizca su.
		return KindUnknown, "antigame bu dosyayı okumuyor"
	}
}

// month, events-2026-08.jsonl adindan "2026-08" cikarir. Bicim
// beklenenden farkliysa adin kendisi dondurulur; burada tahmin etmeye
// calismak yaniltici olurdu.
func month(name string) string {
	s := strings.TrimSuffix(strings.TrimPrefix(name, eventsPre), eventsSuf)
	var y, m int
	if _, err := fmt.Sscanf(s, "%d-%d", &y, &m); err != nil || m < 1 || m > 12 {
		return s
	}
	return fmt.Sprintf("%s %d", turkishMonths[m-1], y)
}

var turkishMonths = [12]string{
	"Ocak", "Şubat", "Mart", "Nisan", "Mayıs", "Haziran",
	"Temmuz", "Ağustos", "Eylül", "Ekim", "Kasım", "Aralık",
}
