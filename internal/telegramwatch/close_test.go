package telegramwatch

import (
	"strings"
	"testing"
	"time"

	"github.com/guts/antigame/internal/config"
	"github.com/guts/antigame/internal/store"
)

func TestNotifyCloseSendsOnlyToOptedInChats(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		TelegramToken: "tok",
		TelegramChats: []config.TelegramChat{
			{ID: 1, Label: "Ebeveyn", NotifyOnClose: true},
			{ID: 2, Label: "Baran", NotifyOnClose: false},
		},
	}
	fs := &fakeSender{}
	now := time.Date(2026, 8, 24, 22, 15, 0, 0, time.UTC)

	if err := notifyClose(dir, cfg, fs, now); err != nil {
		t.Fatalf("notifyClose: %v", err)
	}
	if len(fs.sent) != 1 || fs.sent[0].chat != 1 {
		t.Fatalf("yalnizca opt-in sohbete gitmeli: %+v", fs.sent)
	}
	if !strings.Contains(fs.sent[0].text, now.Local().Format("15:04")) {
		t.Errorf("mesajda saat yok: %q", fs.sent[0].text)
	}
}

func TestNotifyCloseNoOptedInChatsSendsNothing(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{TelegramChats: []config.TelegramChat{{ID: 1}}}
	fs := &fakeSender{}

	if err := notifyClose(dir, cfg, fs, time.Now()); err != nil {
		t.Fatalf("notifyClose: %v", err)
	}
	if len(fs.sent) != 0 {
		t.Fatalf("opt-in yokken gonderim olmamali: %+v", fs.sent)
	}
}

func TestNotifyCloseLogsSuccessfulSend(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{TelegramChats: []config.TelegramChat{{ID: 1, Label: "Ebeveyn", NotifyOnClose: true}}}
	fs := &fakeSender{}
	now := time.Date(2026, 8, 24, 22, 15, 0, 0, time.UTC)

	if err := notifyClose(dir, cfg, fs, now); err != nil {
		t.Fatalf("notifyClose: %v", err)
	}
	sent, err := store.ReadSent(dir, 10)
	if err != nil {
		t.Fatalf("ReadSent: %v", err)
	}
	if len(sent) != 1 || sent[0].ChatID != 1 || sent[0].Label != "Ebeveyn" {
		t.Fatalf("gonderim loglanmadi: %+v", sent)
	}
}

func TestNotifyCloseFailedSendIsNotLogged(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{TelegramChats: []config.TelegramChat{{ID: 1, NotifyOnClose: true}}}
	fs := &fakeSender{failChat: 1}

	if err := notifyClose(dir, cfg, fs, time.Now()); err != nil {
		t.Fatalf("notifyClose: %v", err)
	}
	sent, err := store.ReadSent(dir, 10)
	if err != nil {
		t.Fatalf("ReadSent: %v", err)
	}
	if len(sent) != 0 {
		t.Fatalf("basarisiz gonderim loglanmamali: %+v", sent)
	}
}
