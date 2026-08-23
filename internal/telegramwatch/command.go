package telegramwatch

import (
	"strings"
	"time"

	"github.com/guts/antigame/internal/config"
	"github.com/guts/antigame/internal/store"
	"github.com/guts/antigame/internal/telegram"
)

// handleUpdate, tek bir Telegram guncellemesini isler:
//   - onayli sohbetten "/durum" -> gunun ozeti yollanir
//   - onaysiz sohbetten, bekleyen eslestirme koduyla birebir eslesen
//     metin -> sohbet onaylanir ve config'e yazilir
//   - diger her sey yok sayilir (botun varligini yabancilara sizdirmamak icin)
func handleUpdate(dir string, cfg *config.Config, ts *store.TelegramState, u telegram.Update, client sender, now time.Time) error {
	if u.Chat == 0 {
		return nil
	}
	text := strings.TrimSpace(u.Text)

	if approvedChat(cfg, u.Chat) {
		if text == "/durum" {
			summary, err := dailySummary(dir, cfg, now)
			if err != nil {
				return err
			}
			return client.SendMessage(u.Chat, summary)
		}
		return nil
	}

	if ts.PendingCode == "" || ts.PendingExpiry == nil || now.After(*ts.PendingExpiry) {
		return nil
	}
	if text != ts.PendingCode {
		return nil
	}

	cfg.TelegramChats = append(cfg.TelegramChats, config.TelegramChat{ID: u.Chat, AddedAt: now})
	if err := config.Save(dir, cfg); err != nil {
		return err
	}
	ts.PendingCode = ""
	ts.PendingExpiry = nil
	if err := store.SaveTelegramState(dir, ts); err != nil {
		return err
	}
	return client.SendMessage(u.Chat, "Kaydınız tamamlandı.")
}

func approvedChat(cfg *config.Config, chatID int64) bool {
	for _, c := range cfg.TelegramChats {
		if c.ID == chatID {
			return true
		}
	}
	return false
}
