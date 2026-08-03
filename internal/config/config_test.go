package config

import "testing"

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
