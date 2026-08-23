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
// config.Load/store.LoadState dongusunu yurutur, tipki gate ve watch
// sureclerinin bugun de state.json/config.json uzerinden kilitsiz
// haberlestigi gibi.
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
		st, err := store.LoadState(dir)
		if err != nil {
			if !sleepCtx(ctx, updatesRetryWait) {
				return
			}
			continue
		}
		updates, err := client.GetUpdates(st.TelegramOffset, updatesLongPollS)
		if err != nil {
			if !sleepCtx(ctx, updatesRetryWait) {
				return
			}
			continue
		}
		if len(updates) == 0 {
			continue
		}
		for _, u := range updates {
			_ = handleUpdate(dir, cfg, st, u, client, time.Now().UTC())
			st.TelegramOffset = u.UpdateID + 1
		}
		_ = store.SaveState(dir, st)
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
