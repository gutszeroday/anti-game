package telegramwatch

import (
	"time"

	"github.com/guts/antigame/internal/config"
	"github.com/guts/antigame/internal/store"
	"github.com/guts/antigame/internal/telegram"
)

// NotifyClose, izleyici duzgun kapandiginda NotifyOnClose=true olan
// sohbetlere bildirim yollar. cfg.TelegramToken bossa hicbir ag cagrisi
// yapmaz (bkz. run.go'daki ayni desen).
//
// Cagiran (main.go'daki runWatch) bunu izleyicinin ctx'i iptal
// edildikten SONRA cagirir: telegramwatch'in kendi arka plan
// goroutine'leri ayni ctx'e bagli oldugundan o an zaten donmus olabilir.
func NotifyClose(dir string, cfg *config.Config, now time.Time) error {
	if cfg.TelegramToken == "" {
		return nil
	}
	client := telegram.Client{Token: cfg.TelegramToken}
	return notifyClose(dir, cfg, client, now)
}

func notifyClose(dir string, cfg *config.Config, client sender, now time.Time) error {
	msg := "İzleyici kapatıldı: " + now.Local().Format("15:04")
	for _, chat := range cfg.TelegramChats {
		if !chat.NotifyOnClose {
			continue
		}
		// Gonderim hatasi yutulur: bir sohbetin engellenmesi digerlerini
		// kilitlememeli (bkz. scanUnlocks'taki ayni karar).
		if err := client.SendMessage(chat.ID, msg); err != nil {
			continue
		}
		_ = store.AppendSent(dir, store.SentNotification{TS: now, ChatID: chat.ID, Label: chat.Label, Text: msg})
	}
	return nil
}
