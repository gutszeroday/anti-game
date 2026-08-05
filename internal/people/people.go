//go:build windows

// Package people, kapiyi acabilen kisileri yonetir: kayitlari config'de,
// anahtarlari vault'ta tutar ve ikisini tutarli birakir.
//
// Yazma sirasi her islemde aynidir: once anahtar dosyasi, sonra config.
// Ters sirada yarida kesilen bir islem "ismi var, anahtari yok" birakir.
package people

import (
	"errors"
	"fmt"
	"time"

	"github.com/guts/antigame/internal/auth"
	"github.com/guts/antigame/internal/config"
	"github.com/guts/antigame/internal/store"
	"github.com/guts/antigame/internal/vault"
)

// ErrLastKey, son calisan anahtari silmeye calisan islemden doner.
var ErrLastKey = errors.New("kapıyı açabilecek kimse kalmıyor: önce yeni bir kişi ekleyin")

// ErrNotFound, verilen kimlikte kayit olmadiginda doner.
var ErrNotFound = errors.New("kişi bulunamadı")

// Entry, ekranda gosterilecek kisi kaydidir. Anahtari olmayan kayit
// listeden dusurulmez: elle silinmis bir dosya, kisiyi kullaniciya
// gorunmeden yok etmemeli.
type Entry struct {
	config.Person
	HasKey bool
	// KeyErr, anahtar dosyasi var ama cozulemiyorsa doludur (Windows
	// profili degismis olabilir).
	KeyErr error
}

// Usable, kisinin kapiyi acabilecek durumda olup olmadigini soyler.
func (e Entry) Usable() bool { return e.HasKey && e.KeyErr == nil }

// Ensure, tek kisilik kurulumdan cok kisiliye gecisi tamamlar ve sonucu
// diske yazar. Kisi listesi bos degilse hicbir sey yapmaz.
func Ensure(dir string) (*config.Config, error) {
	needSave, err := config.NeedsPeopleMigration(dir)
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(dir)
	if err != nil {
		return nil, err
	}

	// Eski secret.bin yalnizca ilk kisiye ve o kisinin anahtari yokken
	// tasinir; baska bir durumda calisan bir anahtarin uzerine yazma
	// riski var.
	if len(cfg.People) > 0 {
		first := cfg.People[0].ID
		if !vault.HasPerson(dir, first) {
			moved, err := vault.MigrateLegacy(dir, first)
			if err != nil {
				return nil, err
			}
			needSave = needSave || moved
		}
	}
	if needSave {
		if err := config.Save(dir, cfg); err != nil {
			return nil, err
		}
	}
	return cfg, nil
}

// List, kisileri anahtar durumlariyla birlikte dondurur.
func List(dir string) ([]Entry, error) {
	cfg, err := Ensure(dir)
	if err != nil {
		return nil, err
	}
	out := make([]Entry, 0, len(cfg.People))
	for _, p := range cfg.People {
		e := Entry{Person: p, HasKey: vault.HasPerson(dir, p.ID)}
		if e.HasKey {
			if _, err := vault.LoadPerson(dir, p.ID); err != nil {
				e.KeyErr = err
			}
		}
		out = append(out, e)
	}
	return out, nil
}

// Keys, kapinin deneyecegi anahtarlari dondurur. Cozulemeyen anahtarlar
// atlanir: bir kisinin bozuk dosyasi digerlerini kapinin disinda
// birakmamali.
func Keys(dir string) ([]auth.Key, error) {
	cfg, err := Ensure(dir)
	if err != nil {
		return nil, err
	}
	var keys []auth.Key
	for _, p := range cfg.People {
		secret, err := vault.LoadPerson(dir, p.ID)
		if err != nil {
			continue
		}
		keys = append(keys, auth.Key{ID: p.ID, Secret: secret})
	}
	return keys, nil
}

// Add, yeni kisiyi ve dogrulanmis anahtarini kaydeder. counter,
// eslestirmede kullanilan koddur; kapiyi acmak icin tekrar
// kullanilamamasi icin yakilir.
func Add(dir, name, hint string, secret []byte, counter uint64) (config.Person, error) {
	cfg, err := config.Load(dir)
	if err != nil {
		return config.Person{}, err
	}
	p := config.Person{ID: cfg.TakePersonID(), Name: name, Hint: hint}
	if !config.ValidPersonID(p.ID) {
		return config.Person{}, fmt.Errorf("geçersiz kişi kimliği üretildi: %q", p.ID)
	}
	if err := vault.SavePerson(dir, p.ID, secret); err != nil {
		return config.Person{}, err
	}

	cfg.People = append(cfg.People, p)
	if err := config.Save(dir, cfg); err != nil {
		// Kayit yazilamadiysa anahtar dosyasi yetim kalmasin.
		vault.RemovePerson(dir, p.ID)
		return config.Person{}, err
	}
	if err := burnCounter(dir, p.ID, counter); err != nil {
		return config.Person{}, err
	}
	return p, log(dir, "person_add", p.ID, name)
}

// Edit, kisinin adini ve iletisim notunu degistirir; anahtar aynidir.
func Edit(dir, id, name, hint string) error {
	cfg, err := config.Load(dir)
	if err != nil {
		return err
	}
	p, ok := cfg.FindPerson(id)
	if !ok {
		return ErrNotFound
	}
	p.Name, p.Hint = name, hint
	if err := config.Save(dir, cfg); err != nil {
		return err
	}
	return log(dir, "person_edit", id, name)
}

// Rotate, kisinin anahtarini degistirir. Sayac eski degerinde birakilmaz:
// eski yuksek sayac, yeni anahtarin uretecegi kodlari reddederdi. Yerine
// eslestirmede kullanilan kod yazilir, boylece o kod da yanar.
func Rotate(dir, id string, secret []byte, counter uint64) error {
	cfg, err := config.Load(dir)
	if err != nil {
		return err
	}
	p, ok := cfg.FindPerson(id)
	if !ok {
		return ErrNotFound
	}
	if err := vault.SavePerson(dir, id, secret); err != nil {
		return err
	}
	st, err := store.LoadState(dir)
	if err != nil {
		return err
	}
	st.ClearCounter(id)
	st.SetCounter(id, counter)
	if err := store.SaveState(dir, st); err != nil {
		return err
	}
	return log(dir, "person_rotate", id, p.Name)
}

// Remove, kisiyi ve anahtarini siler.
//
// Silme sonrasi kapiyi acabilecek kimse kalmiyorsa reddedilir. "Son kisi"
// kontrolu yetmez: anahtari olmayan iki kisi kalmasi da kapiyi acilmaz
// yapardi.
func Remove(dir, id string) error {
	entries, err := List(dir)
	if err != nil {
		return err
	}
	found := false
	usableLeft := 0
	for _, e := range entries {
		if e.ID == id {
			found = true
			continue
		}
		if e.Usable() {
			usableLeft++
		}
	}
	if !found {
		return ErrNotFound
	}
	if usableLeft == 0 {
		return ErrLastKey
	}

	if err := vault.RemovePerson(dir, id); err != nil {
		return err
	}
	cfg, err := config.Load(dir)
	if err != nil {
		return err
	}
	var name string
	kept := cfg.People[:0]
	for _, p := range cfg.People {
		if p.ID == id {
			name = p.Name
			continue
		}
		kept = append(kept, p)
	}
	cfg.People = kept
	if err := config.Save(dir, cfg); err != nil {
		return err
	}

	st, err := store.LoadState(dir)
	if err != nil {
		return err
	}
	st.ClearCounter(id)
	// Silinen kisinin acik oturumu varsa sahipsiz kalmasin; oturum
	// kapatilmaz, yalnizca sahibi dusurulur.
	if st.Session != nil && st.Session.OpenedBy == id {
		st.Session.OpenedBy = ""
	}
	if err := store.SaveState(dir, st); err != nil {
		return err
	}
	return log(dir, "person_remove", id, name)
}

// Orphans, hicbir kisiye ait olmayan anahtar dosyasi sayisini dondurur.
func Orphans(dir string) (int, error) {
	cfg, err := config.Load(dir)
	if err != nil {
		return 0, err
	}
	ids := make([]string, 0, len(cfg.People))
	for _, p := range cfg.People {
		ids = append(ids, p.ID)
	}
	return vault.Orphans(dir, ids)
}

// burnCounter, eslestirmede kullanilan kodu kullanilmis isaretler.
func burnCounter(dir, id string, counter uint64) error {
	if counter == 0 {
		return nil
	}
	st, err := store.LoadState(dir)
	if err != nil {
		return err
	}
	st.SetCounter(id, counter)
	return store.SaveState(dir, st)
}

func log(dir, ev, id, name string) error {
	return store.Append(dir, store.Event{
		TS: time.Now().UTC(), Ev: ev, Who: id, Name: name,
	})
}
