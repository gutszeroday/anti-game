// Package config, kapida durdurulacak oyun listesini ve izleyici ayarlarini yonetir.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Game, kapida durdurulacak tek bir oyunu tanimlar.
// Path bos degilse eslesme icin hem exe adi hem tam yol tutmak zorundadir.
type Game struct {
	Name string `json:"name"`
	Exe  string `json:"exe"`
	Path string `json:"path,omitempty"`
	// Launcher, bu girdinin oyunun kendisi degil baslatici oldugunu
	// soyler. Baslatici da kapida durdurulur, ama tek basina calisirken
	// oturumu sonsuza kadar tazeleyemez: tepside acik unutulan bir
	// istemci gun boyu kod sorulmamasina yol aciyordu.
	Launcher bool `json:"launcher,omitempty"`
}

// Person, kapiyi acabilen bir kisidir. Her kisinin kendi TOTP anahtari
// vardir; anahtar dosyasi vault tarafinda ID'ye gore adlandirilir.
type Person struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Hint string `json:"hint,omitempty"`
}

// TelegramChat, kapı bildirimlerini almaya onaylanmış bir Telegram
// sohbetidir. Onay, UI'da üretilen tek kullanımlık eşleştirme koduyla
// gerçekleşir (bkz. internal/telegramwatch).
type TelegramChat struct {
	ID    int64  `json:"id"`
	Label string `json:"label,omitempty"`
	// PersonID, sohbeti onaylayan kisinin ID'sidir (kapi kodu onunla
	// eslesti). /durum yanitini yalnizca bu kisiye ait veriyle
	// sinirlamak icin kullanilir; Label'dan farkli olarak kisi adi
	// degistiginde otomatik guncel kalir.
	PersonID string `json:"person_id,omitempty"`
	// AddedAt, sohbetin ne zaman onaylandığıdır; yalnızca bilgi
	// amaçlıdır, davranışı etkilemez.
	AddedAt time.Time `json:"added_at"`
	// NotifyOnClose, izleyici kapandığında bu sohbete bildirim
	// gönderilip gönderilmeyeceğidir. Yalnızca Telegram'daki
	// /kapanis_bildirimi komutuyla değişir (bkz. telegramwatch/command.go);
	// istemci arayüzünden kasıtlı olarak değiştirilemez.
	NotifyOnClose bool `json:"notify_on_close,omitempty"`
}

type Config struct {
	// FriendName ve FriendHint artik okunmuyor; People bos gelen eski
	// kurulumlari tasimak icin duruyor.
	FriendName string   `json:"friend_name,omitempty"`
	FriendHint string   `json:"friend_hint,omitempty"`
	People     []Person `json:"people"`
	Gated      []Game   `json:"gated"`
	// GraceMinutes, listedeki hicbir sey calismadiginda oturumun ne kadar
	// daha acik kalacagidir.
	GraceMinutes int `json:"grace_minutes"`
	// LauncherWindowMinutes, son gercek oyundan sonra yalnizca baslatici
	// calisirken oturumun en fazla ne kadar yasayacagidir. Mac arasini
	// kapsayacak kadar uzun, gece boyu acik unutulan istemciyi
	// kapsamayacak kadar kisa olmali.
	LauncherWindowMinutes int `json:"launcher_window_minutes"`
	PollMS                int `json:"poll_ms"`
	FocusSampleS          int `json:"focus_sample_s"`
	IdleThresholdS        int `json:"idle_threshold_s"`
	// NextPersonSeq, bir sonraki kisiye verilecek numaradir. Listeye
	// bakip en buyuk numarayi bulmak yetmez: son kisi silindiginde ayni
	// numara geri doner ve silinen kisinin gecmis suresi yeni kisiye
	// yazilirdi.
	NextPersonSeq int `json:"next_person_seq,omitempty"`
	// TelegramToken bosken bildirim ozelligi tamamen kapalidir: hicbir
	// goroutine aga cikmaz. Duz metin: kullanici bilerek secti (bkz.
	// spec).
	TelegramToken string         `json:"telegram_token,omitempty"`
	TelegramChats []TelegramChat `json:"telegram_chats,omitempty"`
	// CodeUnlockOff true ise kapida oyun durdurulmaz, kod hic sorulmaz;
	// oyunlar dogrudan acilir. Sure ve olay kaydi (game_start/game_end)
	// yine tutulur — yalnizca engelleme devre disi kalir.
	CodeUnlockOff bool `json:"code_unlock_off,omitempty"`
}

// ValidPersonID, kisi ID'sinin dosya adinda kullanilmaya uygun olup
// olmadigini soyler. ID anahtar dosyasinin adina giriyor: elle duzenlenmis
// bir config.json'daki ".." ya da "/" iceren bir deger baska bir dosyayi
// ezebilirdi.
func ValidPersonID(id string) bool {
	if id == "" || len(id) > 16 {
		return false
	}
	for _, r := range id {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}

// FindPerson, ID'ye gore kisiyi dondurur.
func (c *Config) FindPerson(id string) (*Person, bool) {
	for i := range c.People {
		if c.People[i].ID == id {
			return &c.People[i], true
		}
	}
	return nil, false
}

// TakePersonID, kullanilmamis bir ID uretir ve sayaci ilerletir.
// Cagiran, ayni islemde config'i kaydetmek zorundadir.
func (c *Config) TakePersonID() string {
	if c.NextPersonSeq < 1 {
		c.NextPersonSeq = 1
		for _, p := range c.People {
			if n, err := strconv.Atoi(strings.TrimPrefix(p.ID, "p")); err == nil && n >= c.NextPersonSeq {
				c.NextPersonSeq = n + 1
			}
		}
	}
	id := "p" + strconv.Itoa(c.NextPersonSeq)
	c.NextPersonSeq++
	return id
}

// migratePeople, tek arkadas alanlarini kisi listesine tasir ve gecersiz
// kayitlari ayiklar. Degisiklik olduysa true doner; kaydetmek cagirana ait.
func migratePeople(c *Config) bool {
	changed := false
	kept := c.People[:0]
	seen := map[string]bool{}
	for _, p := range c.People {
		if !ValidPersonID(p.ID) || seen[p.ID] {
			changed = true
			continue
		}
		seen[p.ID] = true
		kept = append(kept, p)
	}
	c.People = kept

	if len(c.People) == 0 && c.FriendName != "" {
		c.People = []Person{{ID: "p1", Name: c.FriendName, Hint: c.FriendHint}}
		changed = true
	}
	return changed
}

const fileName = "config.json"

// FilePath, ayar dosyasinin tam yolunu dondurur. Izleyici dosyanin degisip
// degismedigini kendi basina yoklayabilsin diye disari aciliyor.
func FilePath(dir string) string {
	return filepath.Join(dir, fileName)
}

func Default() *Config {
	return &Config{
		// Riot tarafinda istemci tek bir process degil: servis, Electron
		// arayuz ve LoL arayuzu ayri ayri calisir. Yalnizca servis
		// oldurulurse ayakta kalan arayuz onu yeniden dogurur, bu yuzden
		// ailenin tamami listede olmak zorunda.
		Gated: []Game{
			{Name: "Riot Client", Exe: "RiotClientServices.exe", Launcher: true},
			{Name: "Riot Client (Arayüz)", Exe: "Riot Client.exe", Launcher: true},
			{Name: "Riot Client (Çökme)", Exe: "RiotClientCrashHandler.exe", Launcher: true},
			{Name: "League of Legends", Exe: "LeagueClient.exe", Launcher: true},
			{Name: "League of Legends (Arayüz)", Exe: "LeagueClientUx.exe", Launcher: true},
			{Name: "League of Legends (Oyun)", Exe: "League of Legends.exe"},
			{Name: "Valorant Başlatıcı", Exe: "VALORANT.exe", Launcher: true},
			{Name: "Valorant", Exe: "VALORANT-Win64-Shipping.exe"},
			{Name: "Legends of Runeterra", Exe: "LoR.exe"},
		},
		GraceMinutes:          10,
		LauncherWindowMinutes: 10,
		PollMS:                250,
		FocusSampleS:          5,
		IdleThresholdS:        300,
	}
}

// Load, config.json'u okur. Dosya yoksa varsayilan yapilandirmayi dondurur
// ve hata uretmez; ilk calistirmada kurulum gerekmemesi icin boyle.
//
// Cozme bos bir yapiya yapiliyor, varsayilanin uzerine degil:
// encoding/json dizileri cozerken mevcut slice elemanlarini yeniden
// kullanir ve JSON'da bulunmayan alanlari oldugu gibi birakir. Varsayilan
// listenin uzerine cozulurse kullanicinin oyunu, ayni indeksteki
// varsayilan girdinin launcher bayragini miras alir; baslatici sayilan bir
// oyun oturumu tazelemedigi icin oynarken oturum duser ve oyun kapatilir.
func Load(dir string) (*Config, error) {
	d := Default()
	b, err := os.ReadFile(FilePath(dir))
	if os.IsNotExist(err) {
		return d, nil
	}
	if err != nil {
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}

	// Dosyada bulunmayan ayarlar varsayilandan tamamlanir. Bilerek
	// bosaltilmis liste ("gated": []) korunur; yalnizca hic yazilmamis
	// olan (null) varsayilana doner.
	if c.Gated == nil {
		c.Gated = d.Gated
	}
	if c.GraceMinutes <= 0 {
		c.GraceMinutes = d.GraceMinutes
	}
	if c.LauncherWindowMinutes <= 0 {
		c.LauncherWindowMinutes = d.LauncherWindowMinutes
	}
	if c.PollMS <= 0 {
		c.PollMS = d.PollMS
	}
	if c.FocusSampleS <= 0 {
		c.FocusSampleS = d.FocusSampleS
	}
	if c.IdleThresholdS <= 0 {
		c.IdleThresholdS = d.IdleThresholdS
	}
	forceKnownLauncherFlags(c.Gated, d.Gated)
	migratePeople(&c)
	return &c, nil
}

// NeedsPeopleMigration, dosyadaki kayitlarin diske yazilmasi gereken bir
// donusumden gectigini soyler. Load donusumu bellekte her zaman yapar;
// kaydetmeyi yalnizca kisileri yoneten kod yapar, boylece izleyici gibi
// salt okuyan yollar dosyaya dokunmaz.
func NeedsPeopleMigration(dir string) (bool, error) {
	b, err := os.ReadFile(FilePath(dir))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return false, err
	}
	return migratePeople(&c), nil
}

// forceKnownLauncherFlags, varsayilan listede adi gecen exe'ler icin
// dosyadaki launcher degerini yok sayar ve varsayilani dayatir.
//
// Alan omitempty tasidigi icin "yazilmamis" ile "false" ayirt edilemiyor:
// bayragi kaybetmis bir config.json kendi kendini onaramaz. Bu, izleyicinin
// Riot Client'i gercek oyun sanmasina ve tepside acik duran istemcinin
// oturumu sonsuza kadar tazelemesine yol aciyordu.
//
// Ayni zamanda bir guvenlik ozelligi: bilinen bir baslaticinin baslatici
// oldugu config.json elle duzenlenerek kaldirilamaz. Kullanicinin kendi
// ekledigi oyunlara dokunulmaz.
func forceKnownLauncherFlags(gated, defaults []Game) {
	known := make(map[string]bool, len(defaults))
	for _, d := range defaults {
		known[strings.ToLower(filepath.Base(d.Exe))] = d.Launcher
	}
	for i := range gated {
		if l, ok := known[strings.ToLower(filepath.Base(gated[i].Exe))]; ok {
			gated[i].Launcher = l
		}
	}
}

func Save(dir string, c *Config) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(FilePath(dir), b, 0o600)
}

// Match, verilen process'in kapida durdurulmasi gerekip gerekmedigini soyler.
// exe yalnizca dosya adi (ornek: "VALORANT.exe"), path tam yoldur.
func (c *Config) Match(exe, path string) (*Game, bool) {
	for i := range c.Gated {
		g := &c.Gated[i]
		if !strings.EqualFold(filepath.Base(g.Exe), filepath.Base(exe)) {
			continue
		}
		if g.Path != "" && !strings.EqualFold(g.Path, path) {
			continue
		}
		return g, true
	}
	return nil, false
}
