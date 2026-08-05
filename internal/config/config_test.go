package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultContainsRiotGames(t *testing.T) {
	c := Default()
	want := map[string]bool{
		"RiotClientServices.exe":      false,
		"LeagueClient.exe":            false,
		"League of Legends.exe":       false,
		"VALORANT.exe":                false,
		"VALORANT-Win64-Shipping.exe": false,
		"LoR.exe":                     false,
	}
	for _, g := range c.Gated {
		want[g.Exe] = true
	}
	for exe, found := range want {
		if !found {
			t.Errorf("varsayilan listede eksik: %s", exe)
		}
	}
}

// Riot ailesi eksik kalirsa sure dolunca servis oldurulur ama ayakta kalan
// Electron arayuz onu yeniden dogurur ve kapi delinir.
func TestDefaultCoversWholeRiotProcessFamily(t *testing.T) {
	want := map[string]bool{
		"Riot Client.exe":            true,
		"RiotClientCrashHandler.exe": true,
		"LeagueClientUx.exe":         true,
	}
	got := map[string]bool{}
	for _, g := range Default().Gated {
		got[g.Exe] = g.Launcher
	}
	for exe, launcher := range want {
		l, ok := got[exe]
		if !ok {
			t.Errorf("varsayilan listede eksik: %s", exe)
			continue
		}
		if l != launcher {
			t.Errorf("%s icin launcher=%v, %v bekleniyordu", exe, l, launcher)
		}
	}
}

func TestDefaultMarksLaunchersButNotRealGames(t *testing.T) {
	want := map[string]bool{
		"RiotClientServices.exe":      true,
		"LeagueClient.exe":            true,
		"VALORANT.exe":                true,
		"League of Legends.exe":       false,
		"VALORANT-Win64-Shipping.exe": false,
		"LoR.exe":                     false,
	}
	for _, g := range Default().Gated {
		if l, ok := want[g.Exe]; ok && g.Launcher != l {
			t.Errorf("%s icin launcher=%v, %v bekleniyordu", g.Exe, g.Launcher, l)
		}
	}
}

// Bozuk config.json'lar launcher alanini hic tasimiyor: omitempty yuzunden
// "yok" ile "false" ayirt edilemiyor, dosya kendi kendini duzeltemiyor.
// Bilinen baslaticilarda dosyadaki deger yok sayilmali.
func TestLoadForcesLauncherFlagOnKnownLaunchers(t *testing.T) {
	dir := t.TempDir()
	raw := `{"gated":[
		{"name":"Riot Client","exe":"RiotClientServices.exe"},
		{"name":"League of Legends","exe":"LeagueClient.exe"},
		{"name":"League of Legends (Oyun)","exe":"League of Legends.exe"}
	]}`
	if err := os.WriteFile(filepath.Join(dir, fileName), []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}

	c, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		exe  string
		want bool
	}{
		{"RiotClientServices.exe", true},
		{"LeagueClient.exe", true},
		{"League of Legends.exe", false},
	} {
		g, ok := c.Match(tc.exe, "")
		if !ok {
			t.Fatalf("%s listede bulunamadi", tc.exe)
		}
		if g.Launcher != tc.want {
			t.Errorf("%s icin launcher=%v, %v bekleniyordu", tc.exe, g.Launcher, tc.want)
		}
	}
}

// Dayatma yalnizca varsayilan listede adi gecen exe'ler icin gecerli;
// kullanicinin kendi ekledigi oyuna karisilmaz.
func TestLoadLeavesUserAddedGameFlagAlone(t *testing.T) {
	dir := t.TempDir()
	raw := `{"gated":[{"name":"Steam","exe":"steam.exe","launcher":true}]}`
	if err := os.WriteFile(filepath.Join(dir, fileName), []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}

	c, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	g, ok := c.Match("steam.exe", "")
	if !ok {
		t.Fatal("kullanicinin ekledigi oyun listede yok")
	}
	if !g.Launcher {
		t.Error("kullanicinin ekledigi baslatici bayragi silinmis")
	}
}

func TestDefaultTuning(t *testing.T) {
	c := Default()
	if c.GraceMinutes != 10 || c.PollMS != 250 || c.FocusSampleS != 5 || c.IdleThresholdS != 300 {
		t.Errorf("varsayilan ayarlar spec ile uyusmuyor: %+v", c)
	}
}

func TestMatchIsCaseInsensitive(t *testing.T) {
	c := &Config{Gated: []Game{{Name: "Valorant", Exe: "VALORANT.exe"}}}
	g, ok := c.Match("valorant.exe", `C:\Riot\VALORANT.exe`)
	if !ok || g.Name != "Valorant" {
		t.Fatalf("buyuk/kucuk harf duyarsiz eslesme basarisiz: %v %v", g, ok)
	}
}

func TestMatchRejectsUnlistedExe(t *testing.T) {
	c := &Config{Gated: []Game{{Name: "Valorant", Exe: "VALORANT.exe"}}}
	if _, ok := c.Match("chrome.exe", `C:\chrome.exe`); ok {
		t.Fatal("listede olmayan exe eslesti")
	}
}

func TestMatchWithPathPinRequiresBothToMatch(t *testing.T) {
	c := &Config{Gated: []Game{{Name: "Client", Exe: "client.exe", Path: `C:\Games\Real\client.exe`}}}
	if _, ok := c.Match("client.exe", `C:\Other\client.exe`); ok {
		t.Fatal("yol sabitlemesi yanlis yolu kabul etti")
	}
	if _, ok := c.Match("client.exe", `c:\games\real\CLIENT.EXE`); !ok {
		t.Fatal("yol sabitlemesi dogru yolu reddetti")
	}
}

func TestLoadMissingFileReturnsDefault(t *testing.T) {
	c, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("dosya yokken hata donmemeli: %v", err)
	}
	if len(c.Gated) == 0 {
		t.Fatal("varsayilan liste bos dondu")
	}
}

func TestSaveThenLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	in := Default()
	in.FriendName = "Ahmet"
	in.Gated = append(in.Gated, Game{Name: "Test", Exe: "test.exe"})
	if err := Save(dir, in); err != nil {
		t.Fatalf("Save: %v", err)
	}
	out, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if out.FriendName != "Ahmet" || len(out.Gated) != len(in.Gated) {
		t.Errorf("gidis-donus bozuk: %+v", out)
	}
}

func TestLoadDoesNotInheritDefaultsIntoUserGames(t *testing.T) {
	// encoding/json dizileri cozerken mevcut slice elemanlarini yeniden
	// kullanir, sifirlamaz. Varsayilan listenin uzerine cozulurse
	// kullanicinin oyunu, ayni indeksteki varsayilan girdinin launcher
	// bayragini miras alir; oyun baslatici sayilir ve oynarken oturum duser.
	dir := t.TempDir()
	raw := `{"gated":[{"name":"Palworld","exe":"Palworld.exe"}]}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Gated) != 1 {
		t.Fatalf("liste kullanicinin yazdigi gibi okunmadi: %+v", c.Gated)
	}
	if c.Gated[0].Launcher {
		t.Error("kullanicinin oyunu varsayilandan launcher bayragi miras aldi")
	}
	if c.Gated[0].Path != "" {
		t.Error("kullanicinin oyunu varsayilandan yol miras aldi")
	}
}

func TestLoadStillFillsMissingScalarDefaults(t *testing.T) {
	dir := t.TempDir()
	raw := `{"gated":[{"name":"X","exe":"x.exe"}]}`
	os.WriteFile(filepath.Join(dir, "config.json"), []byte(raw), 0o600)
	c, _ := Load(dir)
	if c.GraceMinutes == 0 || c.PollMS == 0 || c.FocusSampleS == 0 ||
		c.IdleThresholdS == 0 || c.LauncherWindowMinutes == 0 {
		t.Errorf("eksik sayisal ayarlar varsayilanla doldurulmadi: %+v", c)
	}
}

func TestLoadKeepsIntentionallyEmptyList(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"gated":[]}`), 0o600)
	c, _ := Load(dir)
	if len(c.Gated) != 0 {
		t.Errorf("bilerek bosaltilan liste varsayilanla dolduruldu: %+v", c.Gated)
	}
}

func TestFriendFieldsMigrateToPerson(t *testing.T) {
	dir := t.TempDir()
	raw := `{"friend_name":"Baran","friend_hint":"WhatsApp","gated":[]}`
	if err := os.WriteFile(FilePath(dir), []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.People) != 1 {
		t.Fatalf("kisi listesi olusmadi: %+v", cfg.People)
	}
	p := cfg.People[0]
	if p.ID != "p1" || p.Name != "Baran" || p.Hint != "WhatsApp" {
		t.Errorf("tasima yanlis: %+v", p)
	}
	need, err := NeedsPeopleMigration(dir)
	if err != nil || !need {
		t.Errorf("goc diske yazilmasi gerektigi bildirilmedi: %v %v", need, err)
	}
}

func TestExistingPeopleAreNotOverwrittenByFriendFields(t *testing.T) {
	dir := t.TempDir()
	raw := `{"friend_name":"Baran","people":[{"id":"p3","name":"Ali"}],"gated":[]}`
	if err := os.WriteFile(FilePath(dir), []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.People) != 1 || cfg.People[0].Name != "Ali" {
		t.Errorf("mevcut liste ezildi: %+v", cfg.People)
	}
}

// Kimlik dosya adina giriyor; elle yazilmis bir deger dizin disina
// cikamamali.
func TestInvalidPersonIDsAreDropped(t *testing.T) {
	dir := t.TempDir()
	raw := `{"people":[{"id":"../evil","name":"Kotu"},{"id":"p2","name":"Ali"}],"gated":[]}`
	if err := os.WriteFile(FilePath(dir), []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.People) != 1 || cfg.People[0].ID != "p2" {
		t.Errorf("gecersiz kimlik ayiklanmadi: %+v", cfg.People)
	}
}

func TestTakePersonIDDoesNotReuseRemovedID(t *testing.T) {
	c := &Config{People: []Person{{ID: "p1"}, {ID: "p2"}}}
	if got := c.TakePersonID(); got != "p3" {
		t.Fatalf("beklenen p3, gelen %s", got)
	}
	// p3 silinmis gibi davran: sayac geri sarmamali.
	if got := c.TakePersonID(); got != "p4" {
		t.Errorf("kimlik yeniden kullanildi: %s", got)
	}
}

func TestValidPersonIDRejectsPathCharacters(t *testing.T) {
	for _, id := range []string{"", "../x", "p/1", "P1", "p 1", strings.Repeat("p", 17)} {
		if ValidPersonID(id) {
			t.Errorf("%q kabul edildi", id)
		}
	}
	if !ValidPersonID("p12") {
		t.Error("gecerli kimlik reddedildi")
	}
}
