package telegramwatch

import (
	"testing"
	"time"

	"github.com/guts/antigame/internal/config"
	"github.com/guts/antigame/internal/store"
	"github.com/guts/antigame/internal/telegram"
)

func TestHandleUpdateApprovedChatDurumRepliesWithSummary(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{TelegramChats: []config.TelegramChat{{ID: 7}}}
	st := &store.State{}
	fs := &fakeSender{}
	now := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)

	if err := handleUpdate(dir, cfg, st, telegram.Update{Chat: 7, Text: "/durum"}, fs, now); err != nil {
		t.Fatalf("handleUpdate: %v", err)
	}
	if len(fs.sent) != 1 || fs.sent[0].chat != 7 {
		t.Fatalf("ozet yaniti beklenen sohbete gitmedi: %+v", fs.sent)
	}
}

func TestHandleUpdateUnapprovedChatCommandIsIgnored(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{}
	st := &store.State{}
	fs := &fakeSender{}

	if err := handleUpdate(dir, cfg, st, telegram.Update{Chat: 99, Text: "/durum"}, fs, time.Now()); err != nil {
		t.Fatalf("handleUpdate: %v", err)
	}
	if len(fs.sent) != 0 {
		t.Fatalf("onaysiz sohbete yanit gitmemeli: %+v", fs.sent)
	}
}

func TestHandleUpdateMatchingPairingCodeApprovesChat(t *testing.T) {
	dir := t.TempDir()
	if err := config.Save(dir, &config.Config{}); err != nil {
		t.Fatalf("config.Save: %v", err)
	}
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	expiry := time.Now().Add(10 * time.Minute)
	st := &store.State{TelegramPendingCode: "483920", TelegramPendingExpiry: &expiry}
	fs := &fakeSender{}

	if err := handleUpdate(dir, cfg, st, telegram.Update{Chat: 42, Text: "483920"}, fs, time.Now()); err != nil {
		t.Fatalf("handleUpdate: %v", err)
	}
	if !approvedChat(cfg, 42) {
		t.Fatalf("sohbet onaylanmadi: %+v", cfg.TelegramChats)
	}
	if st.TelegramPendingCode != "" || st.TelegramPendingExpiry != nil {
		t.Fatalf("bekleyen kod temizlenmedi: %+v", st)
	}
	onDisk, err := config.Load(dir)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if !approvedChat(onDisk, 42) {
		t.Fatalf("onay diske yazilmadi")
	}
	if len(fs.sent) != 1 || fs.sent[0].chat != 42 {
		t.Fatalf("onay mesaji gonderilmedi: %+v", fs.sent)
	}
}

func TestHandleUpdateExpiredPairingCodeIsIgnored(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{}
	expiry := time.Now().Add(-time.Minute)
	st := &store.State{TelegramPendingCode: "111111", TelegramPendingExpiry: &expiry}
	fs := &fakeSender{}

	if err := handleUpdate(dir, cfg, st, telegram.Update{Chat: 42, Text: "111111"}, fs, time.Now()); err != nil {
		t.Fatalf("handleUpdate: %v", err)
	}
	if approvedChat(cfg, 42) {
		t.Fatalf("suresi dolmus kod sohbeti onaylamamali")
	}
}

func TestHandleUpdateWrongTextDoesNotApprove(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{}
	expiry := time.Now().Add(10 * time.Minute)
	st := &store.State{TelegramPendingCode: "483920", TelegramPendingExpiry: &expiry}
	fs := &fakeSender{}

	if err := handleUpdate(dir, cfg, st, telegram.Update{Chat: 42, Text: "yanlış"}, fs, time.Now()); err != nil {
		t.Fatalf("handleUpdate: %v", err)
	}
	if approvedChat(cfg, 42) {
		t.Fatalf("yanlis metin sohbeti onaylamamali")
	}
}

func TestHandleUpdateNonMessageUpdateIsIgnored(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{}
	st := &store.State{}
	fs := &fakeSender{}

	if err := handleUpdate(dir, cfg, st, telegram.Update{UpdateID: 1, Chat: 0}, fs, time.Now()); err != nil {
		t.Fatalf("handleUpdate: %v", err)
	}
	if len(fs.sent) != 0 {
		t.Fatalf("mesajsiz guncelleme yanit uretmemeli: %+v", fs.sent)
	}
}
