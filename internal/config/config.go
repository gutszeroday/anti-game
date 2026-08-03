// Package config, kapida durdurulacak oyun listesini ve izleyici ayarlarini yonetir.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Game, kapida durdurulacak tek bir oyunu tanimlar.
// Path bos degilse eslesme icin hem exe adi hem tam yol tutmak zorundadir.
type Game struct {
	Name string `json:"name"`
	Exe  string `json:"exe"`
	Path string `json:"path,omitempty"`
}

type Config struct {
	FriendName     string `json:"friend_name"`
	FriendHint     string `json:"friend_hint"`
	Gated          []Game `json:"gated"`
	GraceMinutes   int    `json:"grace_minutes"`
	PollMS         int    `json:"poll_ms"`
	FocusSampleS   int    `json:"focus_sample_s"`
	IdleThresholdS int    `json:"idle_threshold_s"`
}

const fileName = "config.json"

// Dir, kalici veri dizinini dondurur: %LOCALAPPDATA%\antigame
func Dir() string {
	return filepath.Join(os.Getenv("LOCALAPPDATA"), "antigame")
}

func Default() *Config {
	return &Config{
		Gated: []Game{
			{Name: "Riot Client", Exe: "RiotClientServices.exe"},
			{Name: "League of Legends", Exe: "LeagueClient.exe"},
			{Name: "League of Legends (Oyun)", Exe: "League of Legends.exe"},
			{Name: "Valorant Başlatıcı", Exe: "VALORANT.exe"},
			{Name: "Valorant", Exe: "VALORANT-Win64-Shipping.exe"},
			{Name: "Legends of Runeterra", Exe: "LoR.exe"},
		},
		GraceMinutes:   10,
		PollMS:         250,
		FocusSampleS:   5,
		IdleThresholdS: 300,
	}
}

// Load, config.json'u okur. Dosya yoksa varsayilan yapilandirmayi dondurur
// ve hata uretmez; ilk calistirmada kurulum gerekmemesi icin boyle.
func Load(dir string) (*Config, error) {
	b, err := os.ReadFile(filepath.Join(dir, fileName))
	if os.IsNotExist(err) {
		return Default(), nil
	}
	if err != nil {
		return nil, err
	}
	c := Default()
	if err := json.Unmarshal(b, c); err != nil {
		return nil, err
	}
	return c, nil
}

func Save(dir string, c *Config) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, fileName), b, 0o600)
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
