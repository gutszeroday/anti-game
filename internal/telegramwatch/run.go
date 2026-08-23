package telegramwatch

import (
	"context"
	"time"

	"github.com/guts/antigame/internal/config"
	"github.com/guts/antigame/internal/store"
	"github.com/guts/antigame/internal/telegram"
)

const (
	unlockScanEvery  = 10 * time.Second
	updatesLongPollS = 25
	updatesRetryWait = 5 * time.Second
	idleRecheckEvery = 10 * time.Second
)

// Run, Telegram entegrasyonunu baslatir ve ctx iptal edilene kadar
// calisir. Iki bagimsiz dongu yurutur: biri kapi acma olaylarini
// tarayip bildirir, digeri komut/eslestirme mesajlarini dinler. Ikisi
// de yalnizca cfg.TelegramToken doluyken gercek ag cagrisi yapar; bos
// tokenla sifir trafik uretirler.
//
// watch paketiyle bellek ici hicbir sey paylasilmaz: ikisi de kendi
// config.Load dongusunu yurutur, tipki gate ve watch sureclerinin bugun
// de state.json/config.json uzerinden kilitsiz haberlestigi gibi. Bu
// paketin calisma zamani durumu state.json'da degil ayri bir dosyada
// tutulur (bkz. store.TelegramState): watch, state.json'u eski bir
// bellek ici kopyadan geri yazip buradaki yazimlari yok edebilirdi.
func Run(ctx context.Context, dirFunc func() string) error {
	done := make(chan struct{}, 2)
	go func() { runUnlockScanner(ctx, dirFunc); done <- struct{}{} }()
	go func() { runCommandListener(ctx, dirFunc); done <- struct{}{} }()
	<-done
	<-done
	return nil
}

func runUnlockScanner(ctx context.Context, dirFunc func() string) {
	t := time.NewTicker(unlockScanEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			dir := dirFunc()
			cfg, err := config.Load(dir)
			if err != nil || cfg.TelegramToken == "" || len(cfg.TelegramChats) == 0 {
				continue
			}
			client := telegram.Client{Token: cfg.TelegramToken}
			_ = scanUnlocks(dir, cfg, client, time.Now().UTC())
		}
	}
}

func runCommandListener(ctx context.Context, dirFunc func() string) {
	for {
		if ctx.Err() != nil {
			return
		}
		dir := dirFunc()
		cfg, err := config.Load(dir)
		if err != nil || cfg.TelegramToken == "" {
			if !sleepCtx(ctx, idleRecheckEvery) {
				return
			}
			continue
		}
		client := telegram.Client{Token: cfg.TelegramToken}
		ts, err := store.LoadTelegramState(dir)
		if err != nil {
			if !sleepCtx(ctx, updatesRetryWait) {
				return
			}
			continue
		}
		updates, err := client.GetUpdates(ts.Offset, updatesLongPollS)
		if err != nil {
			if !sleepCtx(ctx, updatesRetryWait) {
				return
			}
			continue
		}
		if len(updates) == 0 {
			continue
		}
		// Config anketten SONRA yeniden okunur: yukaridaki kopya 25-30
		// saniye once yuklendi ve eslestirme basarili olursa
		// handleUpdate onu oldugu gibi diske yazar — arada UI'dan
		// eklenen kisi/oyun o eski kopyayla geri alinirdi.
		cfg, err = config.Load(dir)
		if err != nil {
			if !sleepCtx(ctx, updatesRetryWait) {
				return
			}
			continue
		}
		for _, u := range updates {
			// Yerel saat: /durum ozetinin "bugun"u kullanicinin takvim
			// gunu olmali (bkz. dailySummary, now.Location()).
			_ = handleUpdate(dir, cfg, ts, u, client, time.Now())
			ts.Offset = u.UpdateID + 1
		}
		_ = store.SaveTelegramState(dir, ts)
	}
}

// sleepCtx, ctx iptal olana ya da sure dolana kadar bekler. ctx iptal
// olursa false doner; cagiran bunu "artik calismaya devam etme" olarak
// okur.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}
