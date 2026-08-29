//go:build windows

package ui

import (
	"errors"
	"testing"
	"time"

	"github.com/guts/antigame/internal/config"
	"github.com/guts/antigame/internal/people"
	"github.com/guts/antigame/internal/store"
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

func TestChatRowsUsesLabelWhenSet(t *testing.T) {
	added := time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC)
	rows := ChatRows([]config.TelegramChat{{ID: 1, Label: "Ebeveyn", AddedAt: added}})
	if rows[0].Cells[0] != "Ebeveyn" {
		t.Errorf("etiket kullanilmadi: %q", rows[0].Cells[0])
	}
}

func TestChatRowsFallsBackToChatID(t *testing.T) {
	rows := ChatRows([]config.TelegramChat{{ID: 42}})
	if rows[0].Cells[0] != "Sohbet 42" {
		t.Errorf("beklenen varsayilan etiket degil: %q", rows[0].Cells[0])
	}
}

func TestChatRowsEmptyListIsEmptyNotNil(t *testing.T) {
	if rows := ChatRows(nil); rows == nil {
		t.Error("bos liste nil degil bos dilim olmali")
	}
}

func TestSentRowsFormatsTimeLabelAndText(t *testing.T) {
	ts := time.Date(2026, 8, 24, 22, 15, 0, 0, time.UTC)
	rows := SentRows([]store.SentNotification{{TS: ts, ChatID: 1, Label: "Ebeveyn", Text: "İzleyici kapatıldı: 22:15"}})
	if rows[0].Cells[0] != ts.Local().Format("2006-01-02 15:04") {
		t.Errorf("zaman bicimi beklenmedik: %q", rows[0].Cells[0])
	}
	if rows[0].Cells[1] != "Ebeveyn" {
		t.Errorf("etiket kullanilmadi: %q", rows[0].Cells[1])
	}
	if rows[0].Cells[2] != "İzleyici kapatıldı: 22:15" {
		t.Errorf("mesaj korunmadi: %q", rows[0].Cells[2])
	}
}

func TestSentRowsFallsBackToChatID(t *testing.T) {
	rows := SentRows([]store.SentNotification{{ChatID: 7, Text: "m"}})
	if rows[0].Cells[1] != "Sohbet 7" {
		t.Errorf("beklenen varsayilan etiket degil: %q", rows[0].Cells[1])
	}
}

func TestSentRowsEmptyListIsEmptyNotNil(t *testing.T) {
	if rows := SentRows(nil); rows == nil {
		t.Error("bos liste nil degil bos dilim olmali")
	}
}
