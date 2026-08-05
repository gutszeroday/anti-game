// Package report, olay gunlugunden haftalik ozet cikarir ve HTML rapor uretir.
// Bu paket yalnizca `antigame report` yolunda calisir; izleyici import etmez.
package report

import (
	"sort"
	"time"

	"github.com/guts/antigame/internal/config"
	"github.com/guts/antigame/internal/store"
)

const (
	// gapThreshold, nabiz araligi 10 dakika oldugu icin bir tur kaciran
	// izleyiciyi hatali sekilde "kapali" saymamak adina genis tutuldu.
	gapThreshold = 25 * time.Minute
	// suggestThreshold, bir uygulamanin listeye eklenmesini onermek icin
	// gereken haftalik asgari suredir.
	suggestThreshold = 2 * 3600
)

type GameTotal struct {
	Name    string
	Exe     string
	DurS    int
	ActiveS int
}

type DayTotal struct {
	Day  time.Time
	DurS int
}

type WeekTotal struct {
	Start time.Time
	DurS  int
}

type Gap struct {
	From time.Time
	To   time.Time
}

type Suggestion struct {
	Exe  string
	DurS int
}

type Summary struct {
	From        time.Time
	To          time.Time
	TotalS      int
	PrevTotalS  int
	Games       []GameTotal
	Days        []DayTotal
	Hours       [24]int
	Weeks       []WeekTotal
	Gaps        []Gap
	Suggestions []Suggestion
}

// weekStart, verilen ani iceren haftanin pazartesi 00:00'ini dondurur.
func weekStart(t time.Time, loc *time.Location) time.Time {
	l := t.In(loc)
	offset := (int(l.Weekday()) + 6) % 7 // pazartesi = 0
	return time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, -offset)
}

func Aggregate(ev []store.Event, cfg *config.Config, now time.Time, loc *time.Location) Summary {
	thisWeek := weekStart(now, loc)
	prevWeek := thisWeek.AddDate(0, 0, -7)

	s := Summary{From: thisWeek, To: thisWeek.AddDate(0, 0, 7)}

	games := map[string]*GameTotal{}
	days := map[time.Time]int{}
	usage := map[string]int{}
	weeks := map[time.Time]int{}
	var beats []time.Time

	for _, e := range ev {
		switch e.Ev {
		case "hb", "watch_start", "watch_stop":
			beats = append(beats, e.TS)

		case "game_end":
			// Baslaticilar da game_end yazar ama oyun suresi degildir: ayni
			// mac hem oyun hem istemci hem arayuz olarak toplanirsa rapor
			// katlanarak siser. Listede olmayan exe (sonradan cikarilmis bir
			// oyun) oyun sayilmaya devam eder, yoksa gecmis silinirdi.
			if g, gated := cfg.Match(e.Exe, ""); gated && g.Launcher {
				continue
			}
			// Oturum bittigi ana degil basladigi ana yazilir: gece yarisini
			// asan oturumlar ertesi gune degil basladigi gune ait olmali.
			start := e.TS.Add(-time.Duration(e.DurS) * time.Second)
			w := weekStart(start, loc)
			weeks[w] += e.DurS

			switch {
			case !w.Before(thisWeek):
				s.TotalS += e.DurS
				name := e.Name
				if name == "" {
					name = e.Exe
				}
				g, ok := games[e.Exe]
				if !ok {
					g = &GameTotal{Name: name, Exe: e.Exe}
					games[e.Exe] = g
				}
				g.DurS += e.DurS
				g.ActiveS += e.ActiveS

				d := start.In(loc)
				days[time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, loc)] += e.DurS
				spreadHours(&s.Hours, start, e.DurS, loc)

			case w.Equal(prevWeek):
				s.PrevTotalS += e.DurS
			}

		case "usage":
			if _, gated := cfg.Match(e.Exe, ""); gated {
				continue
			}
			if weekStart(e.TS, loc).Before(thisWeek) {
				continue
			}
			usage[e.Exe] += e.DurS
		}
	}

	for _, g := range games {
		s.Games = append(s.Games, *g)
	}
	sort.Slice(s.Games, func(i, j int) bool { return s.Games[i].DurS > s.Games[j].DurS })

	for d, v := range days {
		s.Days = append(s.Days, DayTotal{Day: d, DurS: v})
	}
	sort.Slice(s.Days, func(i, j int) bool { return s.Days[i].Day.Before(s.Days[j].Day) })

	for i := 3; i >= 0; i-- {
		w := thisWeek.AddDate(0, 0, -7*i)
		s.Weeks = append(s.Weeks, WeekTotal{Start: w, DurS: weeks[w]})
	}

	for exe, v := range usage {
		if v >= suggestThreshold {
			s.Suggestions = append(s.Suggestions, Suggestion{Exe: exe, DurS: v})
		}
	}
	sort.Slice(s.Suggestions, func(i, j int) bool { return s.Suggestions[i].DurS > s.Suggestions[j].DurS })

	s.Gaps = findGaps(beats)
	return s
}

// spreadHours, oturumu basladigi andan itibaren saat dilimlerine dagitir.
// Gun ici dagilim grafigi bu yuzden "ne zaman oynuyorum" sorusuna dogru
// cevap verir; oturumu tek bir saate yazmak yaniltici olurdu.
func spreadHours(hours *[24]int, start time.Time, durS int, loc *time.Location) {
	// Saniye altini atiyoruz: kalan sure saniyeye cevrilirken asagi
	// yuvarlaniyor ve saat basina 1 sn'den yakin bir noktada adim sifira
	// dusup donguyu yerinde saydiriyordu.
	cur := start.In(loc).Truncate(time.Second)
	left := durS
	for left > 0 {
		// Saat sinirini yerel takvimden hesapliyoruz; Truncate mutlak
		// zamana gore keser ve tam saat olmayan dilimlerde kayar.
		next := time.Date(cur.Year(), cur.Month(), cur.Day(), cur.Hour(), 0, 0, 0, loc).Add(time.Hour)
		chunk := min(int(next.Sub(cur).Seconds()), left)
		if chunk <= 0 {
			// Beklenmedik bir takvim durumunda rapor kilitlenmektense
			// kalani icinde bulundugumuz saate yazip cikalim.
			hours[cur.Hour()] += left
			return
		}
		hours[cur.Hour()] += chunk
		left -= chunk
		cur = cur.Add(time.Duration(chunk) * time.Second)
	}
}

func findGaps(beats []time.Time) []Gap {
	if len(beats) < 2 {
		return nil
	}
	sort.Slice(beats, func(i, j int) bool { return beats[i].Before(beats[j]) })
	var out []Gap
	for i := 1; i < len(beats); i++ {
		if beats[i].Sub(beats[i-1]) > gapThreshold {
			out = append(out, Gap{From: beats[i-1], To: beats[i]})
		}
	}
	return out
}
