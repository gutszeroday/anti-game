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
func handleUpdate(dir string, cfg *config.Config, st *store.State, u telegram.Update, client sender, now time.Time) error {
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

	if st.TelegramPendingCode == "" || st.TelegramPendingExpiry == nil || now.After(*st.TelegramPendingExpiry) {
		return nil
	}
	if text != st.TelegramPendingCode {
		return nil
	}

	cfg.TelegramChats = append(cfg.TelegramChats, config.TelegramChat{ID: u.Chat, AddedAt: now})
	if err := config.Save(dir, cfg); err != nil {
		return err
	}
	st.TelegramPendingCode = ""
	st.TelegramPendingExpiry = nil
	if err := store.SaveState(dir, st); err != nil {
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
