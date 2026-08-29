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

// PersonTotal, bir kisinin bu haftaki payidir: oturumunu actigi
// oyunlarda gecen sure ve kac kez kapiyi actigi.
type PersonTotal struct {
	ID      string
	Name    string
	DurS    int
	Unlocks int
}

type Summary struct {
	From        time.Time
	To          time.Time
	TotalS      int
	PrevTotalS  int
	Games       []GameTotal
	People      []PersonTotal
	Days        []DayTotal
	Hours       [24]int
	Weeks       []WeekTotal
	Gaps        []Gap
	Suggestions []Suggestion
}

// personName, kisi ID'sini rapora yazilacak ada cevirir.
//
// Kayittan silinmis kisinin adi bilinmez ama suresi durur; ID'siyle ve
// "silinmiş" notuyla gosterilir. Bos ID, kapi kurulmadan ya da kurtarma
// koduyla acilmis oturumlarin suresidir.
func personName(cfg *config.Config, id string) string {
	if id == "" {
		return "Kapı yokken"
	}
	if p, ok := cfg.FindPerson(id); ok {
		return p.Name
	}
	return id + " (silinmiş)"
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
	persons := map[string]*PersonTotal{}
	var beats []time.Time

	// person, kisi satirini olusturur. Silinmis kisiler de sayilir:
	// gecmis sure, kisinin listeden cikmasiyla yok olmamali.
	person := func(id string) *PersonTotal {
		p, ok := persons[id]
		if !ok {
			p = &PersonTotal{ID: id, Name: personName(cfg, id)}
			persons[id] = p
		}
		return p
	}

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

			switch {
			case !w.Before(thisWeek):
				s.TotalS += e.DurS
				person(e.Who).DurS += e.DurS
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

		case "unlock":
			// Kurtarma koduyla acilan kapinin sahibi yok; "Kapı yokken"
			// satirina yazilmamasi icin ayri tutulur.
			if e.Method == "recovery" || weekStart(e.TS, loc).Before(thisWeek) {
				continue
			}
			person(e.Who).Unlocks++

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

	for _, p := range persons {
		s.People = append(s.People, *p)
	}
	sort.Slice(s.People, func(i, j int) bool {
		if s.People[i].DurS != s.People[j].DurS {
			return s.People[i].DurS > s.People[j].DurS
		}
		return s.People[i].ID < s.People[j].ID
	})

	for d, v := range days {
		s.Days = append(s.Days, DayTotal{Day: d, DurS: v})
	}
	sort.Slice(s.Days, func(i, j int) bool { return s.Days[i].Day.Before(s.Days[j].Day) })

	for exe, v := range usage {
		if v >= suggestThreshold {
			s.Suggestions = append(s.Suggestions, Suggestion{Exe: exe, DurS: v})
		}
	}
	sort.Slice(s.Suggestions, func(i, j int) bool { return s.Suggestions[i].DurS > s.Suggestions[j].DurS })

	s.Gaps = findGaps(beats)
	return s
}

// WeeklyTotals, kurulumdan (ilk kayitli olay) bu hafta DAHIL, her
// haftanin toplam oyun suresini kronolojik sirayla dondurur.
//
// Tek bir "%X azaldi" sayisina indirgemek yerine ham haftalik veriyi
// veriyoruz: kullanici trendi kendi gozuyle gorebilsin, tek bir
// ortalama-yuzde hesabi hangi yone gittigini gizler ya da yanlis
// yorumlanabilir. Bos haftalar da (DurS=0) listede yer alir, yoksa
// "hic oynanmadi" haftalari sessizce atlanip grafik/liste yanlis
// aralikli gorunurdu.
//
// Kurulum tarihi, en eski kayitli ayin ilk gunu DEGIL, o ay icindeki ilk
// olayin gercek zamanidir: ay basi kullanilsaydi (ornegin uygulama ayin
// 20'sinde kurulduysa) 1-19 arasi hicbir olayin olmadigi hayalet bir
// hafta listenin basina eklenirdi.
//
// Kurulum bu hafta olduysa (gosterilecek gecmis hafta yok) bos liste
// doner.
func WeeklyTotals(dir string, cfg *config.Config, thisWeekStart time.Time, loc *time.Location) ([]WeekTotal, error) {
	earliestMonth, found, err := store.EarliestEventMonth(dir)
	if err != nil || !found {
		return nil, err
	}
	if earliestMonth.After(thisWeekStart) {
		return nil, nil
	}
	// store.Read olaylari zaman sirasinda dondurur (bkz. Read belgesi):
	// ev[0] bu araliktaki en eski olaydir, kurulum zamani odur.
	ev, err := store.Read(dir, earliestMonth, thisWeekStart.AddDate(0, 0, 7).Add(-time.Second))
	if err != nil {
		return nil, err
	}
	if len(ev) == 0 {
		return nil, nil
	}
	firstWeek := weekStart(ev[0].TS, loc)
	if firstWeek.After(thisWeekStart) {
		return nil, nil
	}

	totals := map[time.Time]int{}
	for _, e := range ev {
		if e.Ev != "game_end" {
			continue
		}
		if g, gated := cfg.Match(e.Exe, ""); gated && g.Launcher {
			continue
		}
		// Oturum, bitis anina degil basladigi ana ait sayilir (bkz.
		// Aggregate'teki ayni kural): gece yarisini asan oturumlar
		// basladigi haftaya yazilmali.
		start := e.TS.Add(-time.Duration(e.DurS) * time.Second)
		totals[weekStart(start, loc)] += e.DurS
	}

	var out []WeekTotal
	for w := firstWeek; !w.After(thisWeekStart); w = w.AddDate(0, 0, 7) {
		out = append(out, WeekTotal{Start: w, DurS: totals[w]})
	}
	return out, nil
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
