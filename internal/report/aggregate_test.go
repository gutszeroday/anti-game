package report

import (
	"testing"
	"time"

	"github.com/guts/antigame/internal/config"
	"github.com/guts/antigame/internal/store"
)

var loc = time.FixedZone("TRT", 3*3600)

func at(day, hour int) time.Time {
	return time.Date(2026, 8, day, hour, 0, 0, 0, loc).UTC()
}

func cfg() *config.Config {
	return &config.Config{Gated: []config.Game{{Name: "Valorant", Exe: "VALORANT.exe"}}}
}

func TestTotalSumsGameDurations(t *testing.T) {
	ev := []store.Event{
		{TS: at(3, 20), Ev: "game_end", Exe: "VALORANT.exe", Name: "Valorant", DurS: 3600, ActiveS: 3000},
		{TS: at(4, 22), Ev: "game_end", Exe: "VALORANT.exe", Name: "Valorant", DurS: 1800, ActiveS: 1500},
	}
	s := Aggregate(ev, cfg(), at(5, 12), loc)
	if s.TotalS != 5400 {
		t.Errorf("toplam 5400 olmaliydi, %d geldi", s.TotalS)
	}
	if len(s.Games) != 1 || s.Games[0].ActiveS != 4500 {
		t.Errorf("oyun bazinda toplam yanlis: %+v", s.Games)
	}
}

func TestDayTotalsUseSessionStartDay(t *testing.T) {
	// Gece 00:30'da biten 2 saatlik oturum bir onceki gune yazilmali.
	ev := []store.Event{
		{TS: at(4, 0).Add(30 * time.Minute), Ev: "game_end", Exe: "VALORANT.exe", DurS: 7200},
	}
	s := Aggregate(ev, cfg(), at(5, 12), loc)
	var found bool
	for _, d := range s.Days {
		if d.Day.Day() == 3 && d.DurS == 7200 {
			found = true
		}
	}
	if !found {
		t.Errorf("oturum baslangic gunune yazilmadi: %+v", s.Days)
	}
}

func TestHoursSpreadAcrossSessionSpan(t *testing.T) {
	// 21:00'de baslayan 3 saatlik oturum 21, 22 ve 23'e dagilmali.
	ev := []store.Event{
		{TS: at(4, 0), Ev: "game_end", Exe: "VALORANT.exe", DurS: 3 * 3600},
	}
	s := Aggregate(ev, cfg(), at(5, 12), loc)
	for _, h := range []int{21, 22, 23} {
		if s.Hours[h] == 0 {
			t.Errorf("%d. saate sure dagitilmadi: %v", h, s.Hours)
		}
	}
	if s.Hours[12] != 0 {
		t.Errorf("oturum disindaki saate sure yazildi: %v", s.Hours)
	}
}

// Baslaticilar da game_start/game_end yaziyor. Sayilirlarsa bir mac hem
// oyun hem istemci hem arayuz olarak ust uste toplanir: olculen sisme
// 3.83 saatlik oyunu 10.65 saat gosteriyordu.
func TestLaunchersAreNotCountedAsPlaytime(t *testing.T) {
	c := &config.Config{Gated: []config.Game{
		{Name: "Riot Client", Exe: "RiotClientServices.exe", Launcher: true},
		{Name: "League of Legends (Oyun)", Exe: "League of Legends.exe"},
	}}
	ev := []store.Event{
		{TS: at(4, 21), Ev: "game_end", Exe: "League of Legends.exe", Name: "LoL", DurS: 1800},
		{TS: at(4, 21), Ev: "game_end", Exe: "RiotClientServices.exe", Name: "Riot Client", DurS: 7200},
	}

	s := Aggregate(ev, c, at(5, 12), loc)

	if s.TotalS != 1800 {
		t.Errorf("toplam %d, 1800 olmaliydi: baslatici suresi sayilmis", s.TotalS)
	}
	for _, g := range s.Games {
		if g.Exe == "RiotClientServices.exe" {
			t.Error("baslatici oyun tablosunda gorunuyor")
		}
	}
	var dayTotal, hourTotal int
	for _, d := range s.Days {
		dayTotal += d.DurS
	}
	for _, h := range s.Hours {
		hourTotal += h
	}
	if dayTotal != 1800 {
		t.Errorf("gunluk dagilim %d, 1800 olmaliydi", dayTotal)
	}
	if hourTotal != 1800 {
		t.Errorf("saat dagilimi %d, 1800 olmaliydi", hourTotal)
	}
}

// Listeden sonradan cikarilan bir oyunun gecmis suresi rapordan silinmemeli.
func TestUnlistedExeStillCountsAsPlaytime(t *testing.T) {
	ev := []store.Event{
		{TS: at(4, 21), Ev: "game_end", Exe: "EskiOyun.exe", Name: "Eski Oyun", DurS: 900},
	}
	s := Aggregate(ev, cfg(), at(5, 12), loc)
	if s.TotalS != 900 {
		t.Errorf("toplam %d, 900 olmaliydi: listede olmayan oyun dusurulmus", s.TotalS)
	}
}

// Gunlukteki zaman damgalari saniye altini da tasir. Bir oturum saat
// basini kesirli bir kalanla asarsa dagitim adimi sifira yuvarlanip
// donguyu ilerletmez hale gelebilir; rapor o noktada kilitlenir.
func TestHoursSpreadTerminatesOnSubSecondTimestamps(t *testing.T) {
	ev := []store.Event{
		{TS: at(4, 8).Add(9900898 * time.Nanosecond), Ev: "game_end",
			Exe: "VALORANT.exe", DurS: 3 * 3600},
	}

	done := make(chan Summary, 1)
	go func() { done <- Aggregate(ev, cfg(), at(5, 12), loc) }()

	select {
	case s := <-done:
		var total int
		for _, h := range s.Hours {
			total += h
		}
		if total != 3*3600 {
			t.Errorf("saatlere dagitilan sure %d, 10800 olmaliydi: %v", total, s.Hours)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Aggregate donmedi: saat dagitimi sonsuz donguye girdi")
	}
}

func TestGapsDetectedFromMissingHeartbeats(t *testing.T) {
	ev := []store.Event{
		{TS: at(3, 10), Ev: "hb"},
		{TS: at(3, 10).Add(10 * time.Minute), Ev: "hb"},
		// 3 saatlik bosluk: izleyici kapaliydi.
		{TS: at(3, 13).Add(10 * time.Minute), Ev: "hb"},
		{TS: at(3, 13).Add(20 * time.Minute), Ev: "hb"},
	}
	s := Aggregate(ev, cfg(), at(5, 12), loc)
	if len(s.Gaps) != 1 {
		t.Fatalf("bir bosluk beklendi, %d bulundu: %+v", len(s.Gaps), s.Gaps)
	}
	if got := s.Gaps[0].To.Sub(s.Gaps[0].From); got != 3*time.Hour {
		t.Errorf("bosluk suresi 3 saat olmaliydi, %v geldi", got)
	}
}

func TestNoGapWhenHeartbeatsAreRegular(t *testing.T) {
	var ev []store.Event
	for i := range 12 {
		ev = append(ev, store.Event{TS: at(3, 10).Add(time.Duration(i) * 10 * time.Minute), Ev: "hb"})
	}
	s := Aggregate(ev, cfg(), at(5, 12), loc)
	if len(s.Gaps) != 0 {
		t.Errorf("duzenli nabizda bosluk bulundu: %+v", s.Gaps)
	}
}

func TestSuggestionsExcludeGatedGames(t *testing.T) {
	ev := []store.Event{
		{TS: at(3, 14), Ev: "usage", Exe: "steamgame.exe", DurS: 4 * 3600},
		{TS: at(3, 15), Ev: "usage", Exe: "VALORANT.exe", DurS: 5 * 3600},
		{TS: at(3, 16), Ev: "usage", Exe: "notepad.exe", DurS: 60},
	}
	s := Aggregate(ev, cfg(), at(5, 12), loc)
	for _, sg := range s.Suggestions {
		if sg.Exe == "VALORANT.exe" {
			t.Error("kapidaki oyun oneri olarak sunuldu")
		}
		if sg.Exe == "notepad.exe" {
			t.Error("esik altindaki uygulama oneri olarak sunuldu")
		}
	}
	if len(s.Suggestions) != 1 || s.Suggestions[0].Exe != "steamgame.exe" {
		t.Errorf("beklenen oneri gelmedi: %+v", s.Suggestions)
	}
}

func TestPreviousWeekComparison(t *testing.T) {
	// 3 Agustos 2026 pazartesidir; 12 Agustos carsamba, yani hafta ortasi.
	now := at(12, 12)
	ev := []store.Event{
		{TS: now.Add(-2 * 24 * time.Hour), Ev: "game_end", Exe: "VALORANT.exe", DurS: 3600},
		{TS: now.Add(-9 * 24 * time.Hour), Ev: "game_end", Exe: "VALORANT.exe", DurS: 7200},
	}
	s := Aggregate(ev, cfg(), now, loc)
	if s.TotalS != 3600 {
		t.Errorf("bu hafta 3600 olmaliydi, %d geldi", s.TotalS)
	}
	if s.PrevTotalS != 7200 {
		t.Errorf("gecen hafta 7200 olmaliydi, %d geldi", s.PrevTotalS)
	}
}

func TestAggregateWithNoEventsIsSafe(t *testing.T) {
	s := Aggregate(nil, cfg(), at(5, 12), loc)
	if s.TotalS != 0 || len(s.Games) != 0 || len(s.Gaps) != 0 {
		t.Errorf("bos girdi bos ozet uretmeliydi: %+v", s)
	}
}

func TestPersonTotalsSplitTimeByWhoOpenedTheDoor(t *testing.T) {
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	cfg := &config.Config{People: []config.Person{
		{ID: "p1", Name: "Baran"},
		{ID: "p2", Name: "Ali"},
	}}
	ev := []store.Event{
		{TS: now, Ev: "unlock", Method: "totp", Who: "p1"},
		{TS: now, Ev: "game_end", Exe: "VALORANT-Win64-Shipping.exe", DurS: 3600, Who: "p1"},
		{TS: now, Ev: "unlock", Method: "totp", Who: "p2"},
		{TS: now, Ev: "game_end", Exe: "VALORANT-Win64-Shipping.exe", DurS: 1800, Who: "p2"},
		{TS: now, Ev: "unlock", Method: "totp", Who: "p2"},
	}
	s := Aggregate(ev, cfg, now, time.UTC)

	if len(s.People) != 2 {
		t.Fatalf("iki kisi bekleniyordu: %+v", s.People)
	}
	if s.People[0].Name != "Baran" || s.People[0].DurS != 3600 || s.People[0].Unlocks != 1 {
		t.Errorf("ilk satir yanlis: %+v", s.People[0])
	}
	if s.People[1].Name != "Ali" || s.People[1].DurS != 1800 || s.People[1].Unlocks != 2 {
		t.Errorf("ikinci satir yanlis: %+v", s.People[1])
	}
}

func TestRemovedPersonKeepsPastTime(t *testing.T) {
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	cfg := &config.Config{}
	ev := []store.Event{
		{TS: now, Ev: "game_end", Exe: "LoR.exe", DurS: 600, Who: "p7"},
	}
	s := Aggregate(ev, cfg, now, time.UTC)
	if len(s.People) != 1 || s.People[0].Name != "p7 (silinmiş)" || s.People[0].DurS != 600 {
		t.Errorf("silinen kisinin suresi kayboldu: %+v", s.People)
	}
}

func TestTimeWithoutGateIsGroupedSeparately(t *testing.T) {
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	ev := []store.Event{
		{TS: now, Ev: "game_end", Exe: "LoR.exe", DurS: 900},
	}
	s := Aggregate(ev, &config.Config{}, now, time.UTC)
	if len(s.People) != 1 || s.People[0].Name != "Kapı yokken" {
		t.Errorf("sahipsiz sure ayri satirda toplanmadi: %+v", s.People)
	}
}

// Kurtarma kodu bir kisiye ait degil; acma sayisini kimseye yazmamali.
func TestRecoveryUnlockIsNotCountedForAnyPerson(t *testing.T) {
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	ev := []store.Event{
		{TS: now, Ev: "unlock", Method: "recovery"},
	}
	s := Aggregate(ev, &config.Config{}, now, time.UTC)
	if len(s.People) != 0 {
		t.Errorf("kurtarma kodu kisi satiri urretti: %+v", s.People)
	}
}

func TestLauncherTimeIsNotChargedToPerson(t *testing.T) {
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	cfg := config.Default()
	cfg.People = []config.Person{{ID: "p1", Name: "Baran"}}
	ev := []store.Event{
		{TS: now, Ev: "game_end", Exe: "RiotClientServices.exe", DurS: 7200, Who: "p1"},
		{TS: now, Ev: "game_end", Exe: "VALORANT-Win64-Shipping.exe", DurS: 600, Who: "p1"},
	}
	s := Aggregate(ev, cfg, now, time.UTC)
	if len(s.People) != 1 || s.People[0].DurS != 600 {
		t.Errorf("baslatici suresi kisiye yazildi: %+v", s.People)
	}
}
