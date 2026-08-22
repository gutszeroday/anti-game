// Package telegramwatch, kapı olaylarını Telegram'a bildirir ve
// /durum komutuna yanıt verir. watch ve gate paketlerinden tamamen
// bağımsız çalışır: ağ çağrıları hiçbir zaman anti-cheat'in kritik
// döngüsünü bloklamaz (bkz. spec, "Watcher entegrasyonu").
package telegramwatch

import (
	"time"

	"github.com/guts/antigame/internal/config"
	"github.com/guts/antigame/internal/store"
)

// sender, telegram.Client'in bu paketin kullandigi tek metodudur.
// Arayuz olmasi testte gercek aga cikmadan sahte bir gonderici
// takilabilmesini saglar.
type sender interface {
	SendMessage(chatID int64, text string) error
}

// scanUnlocks, son taramadan bu yana yazilan "unlock" olaylarini
// onayli sohbetlere bildirir ve tarama isaretini ilerletir.
//
// Ilk cagrida (state.json'da isaret yoksa) gecmis taranmaz, isaret
// yalnizca "simdi"ye kurulur: kurulumdan once acilmis kapilar icin
// geriye donuk bildirim atilmaz.
func scanUnlocks(dir string, cfg *config.Config, client sender, now time.Time) error {
	st, err := store.LoadState(dir)
	if err != nil {
		return err
	}
	if st.TelegramLastUnlockTS == nil {
		st.TelegramLastUnlockTS = &now
		return store.SaveState(dir, st)
	}

	from := st.TelegramLastUnlockTS.Add(time.Nanosecond)
	if from.After(now) {
		return nil
	}
	events, err := store.Read(dir, from, now)
	if err != nil {
		return err
	}

	last := *st.TelegramLastUnlockTS
	for _, e := range events {
		if e.Ev != "unlock" {
			continue
		}
		msg := formatUnlock(e, cfg)
		for _, chat := range cfg.TelegramChats {
			// Gonderim hatasi yutulur: bir sohbetin engellenmesi
			// digerlerini veya bir sonraki taramayi kilitlememeli.
			_ = client.SendMessage(chat.ID, msg)
		}
		if e.TS.After(last) {
			last = e.TS
		}
	}
	if !last.After(*st.TelegramLastUnlockTS) {
		return nil
	}
	st.TelegramLastUnlockTS = &last
	return store.SaveState(dir, st)
}

// formatUnlock, kapi acma bildirim metnini uretir.
func formatUnlock(e store.Event, cfg *config.Config) string {
	who := e.Who
	switch {
	case e.Method == "recovery":
		who = "Kurtarma kodu"
	case who == "":
		who = "Bilinmeyen"
	default:
		if p, ok := cfg.FindPerson(who); ok {
			who = p.Name
		}
	}
	return "Kapı açıldı: " + who + ", " + e.TS.Local().Format("15:04")
}
