package telegramwatch

import (
	"errors"
	"testing"
	"time"

	"github.com/guts/antigame/internal/config"
	"github.com/guts/antigame/internal/store"
)

// fakeSender, gercek aga cikmadan gonderimleri kaydeder. Diger test
// dosyalarinda (command_test.go) da kullanilir.
type fakeSender struct {
	sent []sentMsg
	// failChat, gonderimi kasten basarisiz kilinacak sohbet ID'sidir.
	failChat int64
}

type sentMsg struct {
	chat int64
	text string
}

func (f *fakeSender) SendMessage(chatID int64, text string) error {
	if chatID == f.failChat {
		return errors.New("gonderim basarisiz")
	}
	f.sent = append(f.sent, sentMsg{chatID, text})
	return nil
}

func TestScanUnlocksFirstRunSetsBookmarkWithoutSending(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC)
	if err := store.Append(dir, store.Event{TS: now.Add(-time.Hour), Ev: "unlock", Who: "p1"}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	cfg := &config.Config{TelegramChats: []config.TelegramChat{{ID: 1}}}
	fs := &fakeSender{}

	if err := scanUnlocks(dir, cfg, fs, now); err != nil {
		t.Fatalf("scanUnlocks: %v", err)
	}
	if len(fs.sent) != 0 {
		t.Fatalf("ilk taramada gecmis bildirilmemeli, %d mesaj gonderildi", len(fs.sent))
	}
	st, err := store.LoadState(dir)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if st.TelegramLastUnlockTS == nil || !st.TelegramLastUnlockTS.Equal(now) {
		t.Fatalf("isaret simdiye kurulmadi: %+v", st.TelegramLastUnlockTS)
	}
}

func TestScanUnlocksSendsNewUnlocksToAllChats(t *testing.T) {
	dir := t.TempDir()
	base := time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC)
	if err := store.SaveState(dir, &store.State{TelegramLastUnlockTS: &base}); err != nil {
		t.Fatalf("SaveState: %v", err)
	}
	evTS := base.Add(5 * time.Minute)
	if err := store.Append(dir, store.Event{TS: evTS, Ev: "unlock", Who: "p1"}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	cfg := &config.Config{
		People:        []config.Person{{ID: "p1", Name: "Baran"}},
		TelegramChats: []config.TelegramChat{{ID: 1}, {ID: 2}},
	}
	fs := &fakeSender{}
	now := evTS.Add(time.Minute)

	if err := scanUnlocks(dir, cfg, fs, now); err != nil {
		t.Fatalf("scanUnlocks: %v", err)
	}
	if len(fs.sent) != 2 {
		t.Fatalf("2 sohbete gonderim bekleniyordu, %d geldi", len(fs.sent))
	}
	want := "Kapı açıldı: Baran, " + evTS.Local().Format("15:04")
	for _, m := range fs.sent {
		if m.text != want {
			t.Errorf("beklenmeyen mesaj: got %q want %q", m.text, want)
		}
	}
	st, err := store.LoadState(dir)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if !st.TelegramLastUnlockTS.Equal(evTS) {
		t.Fatalf("isaret ilerlemedi: %+v", st.TelegramLastUnlockTS)
	}
}

func TestScanUnlocksOneChatFailureDoesNotBlockOthersOrBookmark(t *testing.T) {
	dir := t.TempDir()
	base := time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC)
	if err := store.SaveState(dir, &store.State{TelegramLastUnlockTS: &base}); err != nil {
		t.Fatalf("SaveState: %v", err)
	}
	evTS := base.Add(time.Minute)
	if err := store.Append(dir, store.Event{TS: evTS, Ev: "unlock", Who: ""}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	cfg := &config.Config{TelegramChats: []config.TelegramChat{{ID: 1}, {ID: 2}}}
	fs := &fakeSender{failChat: 1}

	if err := scanUnlocks(dir, cfg, fs, evTS.Add(time.Second)); err != nil {
		t.Fatalf("scanUnlocks: %v", err)
	}
	if len(fs.sent) != 1 || fs.sent[0].chat != 2 {
		t.Fatalf("basarisiz sohbet digerini engellememeli: %+v", fs.sent)
	}
	st, err := store.LoadState(dir)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if !st.TelegramLastUnlockTS.Equal(evTS) {
		t.Fatalf("basarisiz gonderim isareti ilerletmeyi engellememeli: %+v", st.TelegramLastUnlockTS)
	}
}

func TestFormatUnlockUsesRecoveryLabel(t *testing.T) {
	e := store.Event{TS: time.Date(2026, 8, 23, 14, 32, 0, 0, time.UTC), Method: "recovery"}
	got := formatUnlock(e, &config.Config{})
	want := "Kapı açıldı: Kurtarma kodu, " + e.TS.Local().Format("15:04")
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestFormatUnlockFallsBackToIDWhenPersonUnknown(t *testing.T) {
	e := store.Event{TS: time.Date(2026, 8, 23, 14, 32, 0, 0, time.UTC), Who: "p9"}
	got := formatUnlock(e, &config.Config{})
	want := "Kapı açıldı: p9, " + e.TS.Local().Format("15:04")
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}
