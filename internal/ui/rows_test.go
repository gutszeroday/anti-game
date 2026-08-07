//go:build windows

package ui

import (
	"errors"
	"testing"

	"github.com/guts/antigame/internal/config"
	"github.com/guts/antigame/internal/people"
)

func TestGameRowsSeparatesLaunchersFromGames(t *testing.T) {
	c := &config.Config{Gated: []config.Game{
		{Name: "Riot Client", Exe: "RiotClientServices.exe", Launcher: true},
		{Name: "Valorant", Exe: "VALORANT.exe"},
	}}
	rows := GameRows(c)
	if len(rows) != 2 {
		t.Fatalf("%d satir, istenen 2", len(rows))
	}
	if rows[0].Cells[2] != "Başlatıcı" {
		t.Errorf("baslatici isaretlenmemis: %q", rows[0].Cells[2])
	}
	if rows[1].Cells[2] != "Oyun" {
		t.Errorf("oyun yanlis etiketlenmis: %q", rows[1].Cells[2])
	}
}

func TestGameRowsKeepsNameAndExe(t *testing.T) {
	c := &config.Config{Gated: []config.Game{{Name: "Valorant", Exe: "VALORANT.exe"}}}
	rows := GameRows(c)
	if rows[0].Cells[0] != "Valorant" || rows[0].Cells[1] != "VALORANT.exe" {
		t.Errorf("beklenmeyen satir: %v", rows[0].Cells)
	}
}

func TestGameRowsEmptyListIsEmptyNotNil(t *testing.T) {
	if rows := GameRows(&config.Config{}); rows == nil {
		t.Error("bos liste nil degil bos dilim olmali")
	}
}

func TestPeopleRowsMarksMissingKey(t *testing.T) {
	rows := PeopleRows([]people.Entry{
		{Person: config.Person{Name: "Ali", Hint: "telefon"}, HasKey: true},
		{Person: config.Person{Name: "Ayşe"}},
	})
	if rows[0].Cells[2] != "var" {
		t.Errorf("anahtar var isaretlenmemis: %q", rows[0].Cells[2])
	}
	if rows[1].Cells[2] != "yok" {
		t.Errorf("eksik anahtar isaretlenmemis: %q", rows[1].Cells[2])
	}
}

func TestPeopleRowsSeparatesUnreadableKeyFromMissingKey(t *testing.T) {
	rows := PeopleRows([]people.Entry{
		{Person: config.Person{Name: "Ali"}, HasKey: true, KeyErr: errors.New("cozulemedi")},
		{Person: config.Person{Name: "Ayşe"}},
	})
	if rows[0].Cells[2] == rows[1].Cells[2] {
		t.Errorf("bozuk anahtar eksik anahtardan ayirt edilmiyor: %q", rows[0].Cells[2])
	}
	if rows[0].Cells[2] != "okunamıyor" {
		t.Errorf("bozuk anahtar yanlis etiketlenmis: %q", rows[0].Cells[2])
	}
}

func TestPeopleRowsShowsDashForEmptyHint(t *testing.T) {
	rows := PeopleRows([]people.Entry{{Person: config.Person{Name: "Ali"}, HasKey: true}})
	if rows[0].Cells[1] != "—" {
		t.Errorf("bos ipucu icin tire bekleniyordu: %q", rows[0].Cells[1])
	}
}

func TestPeopleRowsEmptyListIsEmptyNotNil(t *testing.T) {
	if rows := PeopleRows(nil); rows == nil {
		t.Error("bos liste nil degil bos dilim olmali")
	}
}
