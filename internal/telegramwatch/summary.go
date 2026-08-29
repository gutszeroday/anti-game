package telegramwatch

import (
	"fmt"
	"strings"
	"time"

	"github.com/guts/antigame/internal/config"
	"github.com/guts/antigame/internal/report"
	"github.com/guts/antigame/internal/store"
)

// weekSummary, bu haftanin (yerel takvim gunuyle Pazartesi 00:00'dan
// bugune) YALNIZCA personID'ye ait oyun suresini ve kapi acma sayisini
// duz metin olarak uretir, ardindan hanenin kurulumdan bu yana HER
// haftaki toplam oyun suresini (kisiye ozel olmayan, guvenle
// gosterilebilecek bir liste) satir satir ekler.
//
// Tek bir "%X azaldi" sayisi yerine ham haftalik liste veriliyor:
// kullanici trendi kendi gozuyle gorup yorumlayabilsin.
//
// Baskasinin verisi hic islenmez: soran kisi botla /durum yazdiginda
// yalnizca kendi sohbetine bagli PersonID'nin olaylarini gorur (bkz.
// command.go, handleUpdate). Bos personID (eslesmemis eski bir sohbet)
// icin kimseye ait olmayan bir ozet uretmemek adina ayri bir mesaj
// donuyoruz.
//
// report.Aggregate kullanilmaz: o, GameTotal/Suggestions gibi kapi
// listesinde olmayan uygulamalari da goruyor (parent'in haftalik
// raporundaki "ekle" onerileri icin). Asagidaki dongu zaten yalnizca
// "game_end" olaylarini topluyor ve bunlar sadece cfg.Gated'deki
// oyunlar icin uretiliyor (bkz. internal/watch) — ek bir filtre
// gerekmiyor.
func weekSummary(dir string, cfg *config.Config, personID string, now time.Time) (string, error) {
	if personID == "" {
		return "Bu sohbet bir kişiyle eşleşmemiş. Kapı kodunuzu bota tekrar gönderin.", nil
	}

	loc := now.Location()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	// Pazartesi = 0: time.Weekday Pazar'i 0 sayar, +6 mod 7 ile kaydiriyoruz.
	offset := (int(today.Weekday()) + 6) % 7
	weekStart := today.AddDate(0, 0, -offset)
	events, err := store.Read(dir, weekStart, now)
	if err != nil {
		return "", err
	}

	durS, unlocks := 0, 0
	var closeTimes []time.Time
	for _, e := range events {
		// watch_stop'un sahibi yok: izleyici kapaninca kimin sordugunu
		// bilmiyoruz, bu yuzden Who filtresine tabi degil, hane geneli.
		if e.Ev == "watch_stop" {
			closeTimes = append(closeTimes, e.TS)
			continue
		}
		if e.Who != personID {
			continue
		}
		switch e.Ev {
		case "game_end":
			// Baslaticilar rapor/aggregate.go'daki ayni kuralla elenir:
			// istemci hem baslatici hem oyun olarak sayilirsa sure katlanir.
			if g, gated := cfg.Match(e.Exe, ""); gated && g.Launcher {
				continue
			}
			durS += e.DurS
		case "unlock":
			if e.Method == "recovery" {
				continue
			}
			unlocks++
		}
	}

	var b strings.Builder
	if cfg.CodeUnlockOff {
		b.WriteString("⚠ Kod ile açma şu an kapalı — oyunlar direkt açılıyor.\n")
	}
	if durS == 0 && unlocks == 0 {
		b.WriteString("Bu hafta henüz hareket yok.\n")
	} else {
		fmt.Fprintf(&b, "Bu hafta: %s, kapı %d kez açıldı\n", formatDur(durS), unlocks)
	}
	if len(closeTimes) > 0 {
		fmt.Fprintf(&b, "\nBu hafta izleyici %d kez kapandı:\n", len(closeTimes))
		for _, t := range closeTimes {
			fmt.Fprintf(&b, "  %s\n", t.Local().Format("02.01 15:04"))
		}
	}

	weeks, err := report.WeeklyTotals(dir, cfg, weekStart, loc)
	if err != nil {
		return "", err
	}
	if len(weeks) > 1 {
		b.WriteString("\nHanede haftalık toplam (kurulumdan bu yana):\n")
		for _, w := range weeks {
			label := w.Start.Format("02.01")
			if w.Start.Equal(weekStart) {
				label += " (bu hafta)"
			}
			fmt.Fprintf(&b, "  %s: %s\n", label, formatDur(w.DurS))
		}
	}
	return b.String(), nil
}

// formatDur, saniyeyi "2s 14dk" ya da saat yoksa "14dk" bicimine cevirir.
func formatDur(s int) string {
	h := s / 3600
	m := (s % 3600) / 60
	if h > 0 {
		return fmt.Sprintf("%ds %ddk", h, m)
	}
	return fmt.Sprintf("%ddk", m)
}
