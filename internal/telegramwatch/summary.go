package telegramwatch

import (
	"fmt"
	"strings"
	"time"

	"github.com/guts/antigame/internal/config"
	"github.com/guts/antigame/internal/store"
)

// dailySummary, bugunun (yerel takvim gunu) kisi basina oyun suresini
// ve kapi acma sayisini duz metin olarak uretir.
//
// report.Aggregate kullanilmaz: o haftalik pencereye (weekStart) sabit,
// gunluk bir aralik kabul etmiyor.
func dailySummary(dir string, cfg *config.Config, now time.Time) (string, error) {
	loc := now.Location()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	events, err := store.Read(dir, dayStart, now)
	if err != nil {
		return "", err
	}

	type totals struct {
		durS    int
		unlocks int
	}
	per := map[string]*totals{}
	var order []string
	get := func(who string) *totals {
		t, ok := per[who]
		if !ok {
			t = &totals{}
			per[who] = t
			order = append(order, who)
		}
		return t
	}

	for _, e := range events {
		switch e.Ev {
		case "game_end":
			// Baslaticilar rapor/aggregate.go'daki ayni kuralla elenir:
			// istemci hem baslatici hem oyun olarak sayilirsa sure katlanir.
			if g, gated := cfg.Match(e.Exe, ""); gated && g.Launcher {
				continue
			}
			get(e.Who).durS += e.DurS
		case "unlock":
			if e.Method == "recovery" {
				continue
			}
			get(e.Who).unlocks++
		}
	}

	if len(order) == 0 {
		return "Bugün henüz hareket yok.", nil
	}
	var b strings.Builder
	b.WriteString("Bugün:\n")
	for _, who := range order {
		name := who
		if who == "" {
			name = "Kapı yokken"
		} else if p, ok := cfg.FindPerson(who); ok {
			name = p.Name
		}
		t := per[who]
		fmt.Fprintf(&b, "  %s — %s, kapı %d kez açıldı\n", name, formatDur(t.durS), t.unlocks)
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
