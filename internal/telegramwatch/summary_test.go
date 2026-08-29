package telegramwatch

import (
	"strings"
	"testing"
	"time"

	"github.com/guts/antigame/internal/config"
	"github.com/guts/antigame/internal/store"
)

func TestWeekSummaryUnlinkedPersonReturnsNotice(t *testing.T) {
	dir := t.TempDir()
	got, err := weekSummary(dir, &config.Config{}, "", time.Now())
	if err != nil {
		t.Fatalf("weekSummary: %v", err)
	}
	if strings.Contains(got, "Bu hafta") {
		t.Errorf("bos personID icin ozet uretilmemeli: %q", got)
	}
}

func TestWeekSummaryNoEventsThisWeek(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC) // pazar
	got, err := weekSummary(dir, &config.Config{}, "p1", now)
	if err != nil {
		t.Fatalf("weekSummary: %v", err)
	}
	if !strings.HasPrefix(got, "Bu hafta henüz hareket yok.") {
		t.Errorf("beklenmeyen ozet: %q", got)
	}
}

func TestWeekSummarySumsOnlyRequestedPerson(t *testing.T) {
	dir := t.TempDir()
	day := time.Date(2026, 8, 23, 9, 0, 0, 0, time.UTC) // pazar
	events := []store.Event{
		{TS: day.Add(time.Hour), Ev: "unlock", Who: "p1"},
		{TS: day.Add(2 * time.Hour), Ev: "game_end", Who: "p1", Exe: "VALORANT.exe", DurS: 5400},
		{TS: day.Add(3 * time.Hour), Ev: "game_end", Who: "p1", Exe: "VALORANT.exe", DurS: 3000},
		// Baska bir kisinin verisi ozete hic girmemeli.
		{TS: day.Add(time.Hour), Ev: "unlock", Who: "p2"},
		{TS: day.Add(2 * time.Hour), Ev: "game_end", Who: "p2", Exe: "VALORANT.exe", DurS: 999999},
	}
	for _, e := range events {
		if err := store.Append(dir, e); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}
	cfg := &config.Config{People: []config.Person{{ID: "p1", Name: "Baran"}, {ID: "p2", Name: "Ece"}}}
	now := day.Add(4 * time.Hour)

	got, err := weekSummary(dir, cfg, "p1", now)
	if err != nil {
		t.Fatalf("weekSummary: %v", err)
	}
	if !strings.Contains(got, "Bu hafta: 2s 20dk, kapı 1 kez açıldı") {
		t.Errorf("beklenen satiri icermiyor: %q", got)
	}
	if strings.Contains(got, "Ece") {
		t.Errorf("baskasinin adi sizmis: %q", got)
	}
}

func TestWeekSummaryExcludesLauncherDuration(t *testing.T) {
	dir := t.TempDir()
	day := time.Date(2026, 8, 23, 9, 0, 0, 0, time.UTC)
	if err := store.Append(dir, store.Event{
		TS: day.Add(time.Hour), Ev: "game_end", Who: "p1", Exe: "RiotClientServices.exe", DurS: 999,
	}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	cfg := &config.Config{Gated: []config.Game{{Exe: "RiotClientServices.exe", Launcher: true}}}
	got, err := weekSummary(dir, cfg, "p1", day.Add(2*time.Hour))
	if err != nil {
		t.Fatalf("weekSummary: %v", err)
	}
	if !strings.HasPrefix(got, "Bu hafta henüz hareket yok.") {
		t.Errorf("baslatici suresi ozete sizmis: %q", got)
	}
}

// Haftanin baslangici Pazartesi 00:00 olmali: Pazartesi sabahi bir
// onceki Pazar'daki olay ozete girmemeli.
func TestWeekSummaryStartsOnMonday(t *testing.T) {
	dir := t.TempDir()
	sunday := time.Date(2026, 8, 23, 20, 0, 0, 0, time.UTC)
	if err := store.Append(dir, store.Event{TS: sunday, Ev: "unlock", Who: "p1"}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	monday := time.Date(2026, 8, 24, 9, 0, 0, 0, time.UTC)

	got, err := weekSummary(dir, &config.Config{}, "p1", monday)
	if err != nil {
		t.Fatalf("weekSummary: %v", err)
	}
	if !strings.HasPrefix(got, "Bu hafta henüz hareket yok.") {
		t.Errorf("gecen haftanin olayi bu haftaya sizmis: %q", got)
	}
}

// /durum'un hafta penceresi kullanicinin takvim gunu olmali, UTC'nin
// degil.
func TestWeekSummaryUsesLocalCalendarDayNotUTC(t *testing.T) {
	dir := t.TempDir()
	zone := time.FixedZone("TestZone", 3*3600)
	ev := time.Date(2026, 8, 24, 0, 30, 0, 0, zone)
	if err := store.Append(dir, store.Event{TS: ev, Ev: "unlock", Who: "p1"}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	now := time.Date(2026, 8, 24, 5, 0, 0, 0, zone)

	got, err := weekSummary(dir, &config.Config{}, "p1", now)
	if err != nil {
		t.Fatalf("weekSummary: %v", err)
	}
	if !strings.Contains(got, "kapı 1 kez açıldı") {
		t.Errorf("yerel hafta basindaki olay ozete girmedi: %q", got)
	}

	if got, err := weekSummary(dir, &config.Config{}, "p1", now.UTC()); err != nil {
		t.Fatalf("weekSummary: %v", err)
	} else if !strings.HasPrefix(got, "Bu hafta henüz hareket yok.") {
		t.Errorf("UTC gunu bu olayi icermemeliydi: %q", got)
	}
}

func TestWeekSummaryIncludesWeeklyHistoryWhenAvailable(t *testing.T) {
	dir := t.TempDir()
	firstWeekEvent := time.Date(2026, 8, 3, 9, 0, 0, 0, time.UTC) // pazartesi
	if err := store.Append(dir, store.Event{
		TS: firstWeekEvent, Ev: "game_end", Who: "p1", Exe: "VALORANT.exe", DurS: 3600,
	}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	now := time.Date(2026, 8, 24, 9, 0, 0, 0, time.UTC) // uc hafta sonra, pazartesi

	got, err := weekSummary(dir, &config.Config{}, "p1", now)
	if err != nil {
		t.Fatalf("weekSummary: %v", err)
	}
	if !strings.Contains(got, "Hanede haftalık toplam") {
		t.Errorf("hafta gecmisi satiri eksik: %q", got)
	}
	if !strings.Contains(got, "03.08: 1s 0dk") {
		t.Errorf("ilk hafta rakami eksik/yanlis: %q", got)
	}
	if !strings.Contains(got, "(bu hafta)") {
		t.Errorf("bu haftanin isaretlenmesi eksik: %q", got)
	}
}

func TestWeekSummaryOmitsWeeklyHistoryWhenInstalledThisWeek(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 8, 24, 9, 0, 0, 0, time.UTC)
	if err := store.Append(dir, store.Event{
		TS: now, Ev: "game_end", Who: "p1", Exe: "VALORANT.exe", DurS: 600,
	}); err != nil {
		t.Fatalf("Append: %v", err)
	}

	got, err := weekSummary(dir, &config.Config{}, "p1", now)
	if err != nil {
		t.Fatalf("weekSummary: %v", err)
	}
	if strings.Contains(got, "Hanede haftalık toplam") {
		t.Errorf("kiyaslanacak gecmis hafta yokken liste gosterilmemeli: %q", got)
	}
}

// Kapanis olaylarinin sahibi (Who) yok: izleyici kapaninca ne kadar
// sordugunu bilmiyoruz, bu yuzden hane geneli sayilir ve herkese
// gosterilir.
func TestWeekSummaryIncludesWatchStopCounts(t *testing.T) {
	dir := t.TempDir()
	day := time.Date(2026, 8, 24, 9, 0, 0, 0, time.UTC) // pazartesi
	events := []store.Event{
		{TS: day.Add(time.Hour), Ev: "watch_stop"},
		{TS: day.Add(3 * time.Hour), Ev: "watch_stop"},
	}
	for _, e := range events {
		if err := store.Append(dir, e); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}
	cfg := &config.Config{People: []config.Person{{ID: "p1", Name: "Baran"}}}
	now := day.Add(4 * time.Hour)

	got, err := weekSummary(dir, cfg, "p1", now)
	if err != nil {
		t.Fatalf("weekSummary: %v", err)
	}
	if !strings.Contains(got, "Bu hafta izleyici 2 kez kapandı:") {
		t.Errorf("kapanis sayisi eksik: %q", got)
	}
	if !strings.Contains(got, events[0].TS.Local().Format("02.01 15:04")) ||
		!strings.Contains(got, events[1].TS.Local().Format("02.01 15:04")) {
		t.Errorf("kapanis zamanlari eksik: %q", got)
	}
}

func TestWeekSummaryOmitsWatchStopLineWhenNone(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	got, err := weekSummary(dir, &config.Config{}, "p1", now)
	if err != nil {
		t.Fatalf("weekSummary: %v", err)
	}
	if strings.Contains(got, "kez kapandı") {
		t.Errorf("kapanis yokken satir gorunmemeli: %q", got)
	}
}

func TestWeekSummaryWarnsWhenCodeUnlockOff(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	cfg := &config.Config{People: []config.Person{{ID: "p1", Name: "Baran"}}, CodeUnlockOff: true}

	got, err := weekSummary(dir, cfg, "p1", now)
	if err != nil {
		t.Fatalf("weekSummary: %v", err)
	}
	if !strings.Contains(got, "Kod ile açma şu an kapalı") {
		t.Errorf("kod-ile-acma uyarisi eksik: %q", got)
	}
}

func TestWeekSummaryOmitsCodeUnlockWarningWhenOn(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	cfg := &config.Config{People: []config.Person{{ID: "p1", Name: "Baran"}}}

	got, err := weekSummary(dir, cfg, "p1", now)
	if err != nil {
		t.Fatalf("weekSummary: %v", err)
	}
	if strings.Contains(got, "Kod ile açma") {
		t.Errorf("kod ile acma acikken uyari gorunmemeli: %q", got)
	}
}

func TestFormatDurUnderHour(t *testing.T) {
	if got := formatDur(600); got != "10dk" {
		t.Errorf("got %q want 10dk", got)
	}
}

func TestFormatDurOverHour(t *testing.T) {
	if got := formatDur(8400); got != "2s 20dk" {
		t.Errorf("got %q want 2s 20dk", got)
	}
}
