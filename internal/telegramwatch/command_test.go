package telegramwatch

import (
	"strings"
	"testing"
	"time"

	"github.com/guts/antigame/internal/config"
	"github.com/guts/antigame/internal/store"
	"github.com/guts/antigame/internal/telegram"
	"github.com/guts/antigame/internal/totp"
	"github.com/guts/antigame/internal/vault"
)

func TestHandleUpdateApprovedChatDurumRepliesWithOwnSummaryOnly(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC) // pazartesi
	if err := store.Append(dir, store.Event{TS: now.Add(-time.Hour), Ev: "unlock", Who: "p1"}); err != nil {
		t.Fatalf("Append (p1): %v", err)
	}
	if err := store.Append(dir, store.Event{TS: now.Add(-time.Hour), Ev: "unlock", Who: "p2"}); err != nil {
		t.Fatalf("Append (p2): %v", err)
	}
	cfg := &config.Config{
		People:        []config.Person{{ID: "p1", Name: "Baran"}, {ID: "p2", Name: "Ece"}},
		TelegramChats: []config.TelegramChat{{ID: 7, PersonID: "p1"}},
	}
	fs := &fakeSender{}

	if err := handleUpdate(dir, cfg, telegram.Update{Chat: 7, Text: "/durum"}, fs, now); err != nil {
		t.Fatalf("handleUpdate: %v", err)
	}
	if len(fs.sent) != 1 || fs.sent[0].chat != 7 {
		t.Fatalf("ozet yaniti beklenen sohbete gitmedi: %+v", fs.sent)
	}
	if strings.Contains(fs.sent[0].text, "Ece") {
		t.Fatalf("baskasinin adi/verisi sizmis: %q", fs.sent[0].text)
	}
	if !strings.Contains(fs.sent[0].text, "kapı 1 kez açıldı") {
		t.Errorf("kendi verisi ozete girmemis: %q", fs.sent[0].text)
	}
}

func TestHandleUpdateDurumUnlinkedChatGetsNoticeInsteadOfData(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{TelegramChats: []config.TelegramChat{{ID: 7}}}
	fs := &fakeSender{}

	if err := handleUpdate(dir, cfg, telegram.Update{Chat: 7, Text: "/durum"}, fs, time.Now()); err != nil {
		t.Fatalf("handleUpdate: %v", err)
	}
	if len(fs.sent) != 1 || fs.sent[0].chat != 7 {
		t.Fatalf("yanit beklenen sohbete gitmedi: %+v", fs.sent)
	}
	if strings.Contains(fs.sent[0].text, "Bu hafta") {
		t.Errorf("kisisiz sohbete ozet verisi gitmemeli: %q", fs.sent[0].text)
	}
}

func TestHandleUpdateApprovedChatHelpListsCommands(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{TelegramChats: []config.TelegramChat{{ID: 7}}}
	fs := &fakeSender{}

	if err := handleUpdate(dir, cfg, telegram.Update{Chat: 7, Text: "/help"}, fs, time.Now()); err != nil {
		t.Fatalf("handleUpdate: %v", err)
	}
	if len(fs.sent) != 1 || fs.sent[0].chat != 7 || fs.sent[0].text != helpText {
		t.Fatalf("komut listesi beklenen sohbete gitmedi: %+v", fs.sent)
	}
}

func TestHandleUpdateKapanisBildirimiTogglesOnAndPersists(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{TelegramChats: []config.TelegramChat{{ID: 7, PersonID: "p1"}}}
	if err := config.Save(dir, cfg); err != nil {
		t.Fatalf("config.Save: %v", err)
	}
	fs := &fakeSender{}

	if err := handleUpdate(dir, cfg, telegram.Update{Chat: 7, Text: "/kapanis_bildirimi"}, fs, time.Now()); err != nil {
		t.Fatalf("handleUpdate: %v", err)
	}
	if len(fs.sent) != 1 || fs.sent[0].chat != 7 || !strings.Contains(fs.sent[0].text, "açıldı") {
		t.Fatalf("acilis onayi beklenen sohbete gitmedi: %+v", fs.sent)
	}
	if !cfg.TelegramChats[0].NotifyOnClose {
		t.Fatalf("bellekteki ayar acilmadi: %+v", cfg.TelegramChats)
	}
	onDisk, err := config.Load(dir)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if !onDisk.TelegramChats[0].NotifyOnClose {
		t.Fatalf("ayar diske yazilmadi: %+v", onDisk.TelegramChats)
	}
}

func TestHandleUpdateKapanisBildirimiTogglesOff(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{TelegramChats: []config.TelegramChat{{ID: 7, PersonID: "p1", NotifyOnClose: true}}}
	if err := config.Save(dir, cfg); err != nil {
		t.Fatalf("config.Save: %v", err)
	}
	fs := &fakeSender{}

	if err := handleUpdate(dir, cfg, telegram.Update{Chat: 7, Text: "/kapanis_bildirimi"}, fs, time.Now()); err != nil {
		t.Fatalf("handleUpdate: %v", err)
	}
	if len(fs.sent) != 1 || !strings.Contains(fs.sent[0].text, "kapandı") {
		t.Fatalf("kapanis onayi beklenmeyen: %+v", fs.sent)
	}
	if cfg.TelegramChats[0].NotifyOnClose {
		t.Fatalf("ayar kapanmadi: %+v", cfg.TelegramChats)
	}
}

func TestHandleUpdateUnapprovedChatCommandIsIgnored(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{}
	fs := &fakeSender{}

	if err := handleUpdate(dir, cfg, telegram.Update{Chat: 99, Text: "/durum"}, fs, time.Now()); err != nil {
		t.Fatalf("handleUpdate: %v", err)
	}
	if len(fs.sent) != 0 {
		t.Fatalf("onaysiz sohbete yanit gitmemeli: %+v", fs.sent)
	}
}

func TestHandleUpdateMatchingPersonCodeApprovesChatWithName(t *testing.T) {
	dir := t.TempDir()
	if err := config.Save(dir, &config.Config{}); err != nil {
		t.Fatalf("config.Save: %v", err)
	}
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	cfg.People = []config.Person{{ID: "p1", Name: "Baran"}}
	if err := config.Save(dir, cfg); err != nil {
		t.Fatalf("config.Save: %v", err)
	}
	secret := []byte("test-secret-baran")
	if err := vault.SavePerson(dir, "p1", secret); err != nil {
		t.Fatalf("vault.SavePerson: %v", err)
	}
	now := time.Now()
	code := totp.Code(secret, totp.Counter(now))
	fs := &fakeSender{}

	if err := handleUpdate(dir, cfg, telegram.Update{Chat: 42, Text: code}, fs, now); err != nil {
		t.Fatalf("handleUpdate: %v", err)
	}
	if !approvedChat(cfg, 42) {
		t.Fatalf("sohbet onaylanmadi: %+v", cfg.TelegramChats)
	}
	if cfg.TelegramChats[0].Label != "Baran" {
		t.Fatalf("sohbet kisinin adiyla etiketlenmedi: %+v", cfg.TelegramChats)
	}
	if cfg.TelegramChats[0].PersonID != "p1" {
		t.Fatalf("sohbet kisi ID'siyle iliskilendirilmedi: %+v", cfg.TelegramChats)
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

func TestHandleUpdateSamePersonCodeCanBeReusedForPairing(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{People: []config.Person{{ID: "p1", Name: "Baran"}}}
	if err := config.Save(dir, cfg); err != nil {
		t.Fatalf("config.Save: %v", err)
	}
	secret := []byte("test-secret-baran")
	if err := vault.SavePerson(dir, "p1", secret); err != nil {
		t.Fatalf("vault.SavePerson: %v", err)
	}
	now := time.Now()
	code := totp.Code(secret, totp.Counter(now))
	fs := &fakeSender{}

	// Kapinin kendi tekrar-kullanim korumasi Telegram eslestirmesini
	// etkilememeli: ayni kod ikinci bir sohbeti de onaylayabilmeli.
	if err := handleUpdate(dir, cfg, telegram.Update{Chat: 42, Text: code}, fs, now); err != nil {
		t.Fatalf("handleUpdate (ilk): %v", err)
	}
	if err := handleUpdate(dir, cfg, telegram.Update{Chat: 43, Text: code}, fs, now); err != nil {
		t.Fatalf("handleUpdate (ikinci): %v", err)
	}
	if !approvedChat(cfg, 42) || !approvedChat(cfg, 43) {
		t.Fatalf("her iki sohbet de onaylanmali: %+v", cfg.TelegramChats)
	}
}

func TestHandleUpdateWrongCodeDoesNotApprove(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{People: []config.Person{{ID: "p1", Name: "Baran"}}}
	if err := config.Save(dir, cfg); err != nil {
		t.Fatalf("config.Save: %v", err)
	}
	if err := vault.SavePerson(dir, "p1", []byte("test-secret-baran")); err != nil {
		t.Fatalf("vault.SavePerson: %v", err)
	}
	fs := &fakeSender{}

	if err := handleUpdate(dir, cfg, telegram.Update{Chat: 42, Text: "000000"}, fs, time.Now()); err != nil {
		t.Fatalf("handleUpdate: %v", err)
	}
	if approvedChat(cfg, 42) {
		t.Fatalf("yanlis kod sohbeti onaylamamali")
	}
	if len(fs.sent) != 0 {
		t.Fatalf("yanlis koda yanit gitmemeli: %+v", fs.sent)
	}
}

func TestHandleUpdateNonMessageUpdateIsIgnored(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{}
	fs := &fakeSender{}

	if err := handleUpdate(dir, cfg, telegram.Update{UpdateID: 1, Chat: 0}, fs, time.Now()); err != nil {
		t.Fatalf("handleUpdate: %v", err)
	}
	if len(fs.sent) != 0 {
		t.Fatalf("mesajsiz guncelleme yanit uretmemeli: %+v", fs.sent)
	}
}
