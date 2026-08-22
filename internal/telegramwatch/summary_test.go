package telegramwatch

import (
	"strings"
	"testing"
	"time"

	"github.com/guts/antigame/internal/config"
	"github.com/guts/antigame/internal/store"
)

func TestDailySummaryNoEventsToday(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	got, err := dailySummary(dir, &config.Config{}, now)
	if err != nil {
		t.Fatalf("dailySummary: %v", err)
	}
	if got != "Bugün henüz hareket yok." {
		t.Errorf("beklenmeyen ozet: %q", got)
	}
}

func TestDailySummarySumsDurationAndUnlocksPerPerson(t *testing.T) {
	dir := t.TempDir()
	day := time.Date(2026, 8, 23, 9, 0, 0, 0, time.UTC)
	events := []store.Event{
		{TS: day.Add(time.Hour), Ev: "unlock", Who: "p1"},
		{TS: day.Add(2 * time.Hour), Ev: "game_end", Who: "p1", Exe: "VALORANT.exe", DurS: 5400},
		{TS: day.Add(3 * time.Hour), Ev: "game_end", Who: "p1", Exe: "VALORANT.exe", DurS: 3000},
	}
	for _, e := range events {
		if err := store.Append(dir, e); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}
	cfg := &config.Config{People: []config.Person{{ID: "p1", Name: "Baran"}}}
	now := day.Add(4 * time.Hour)

	got, err := dailySummary(dir, cfg, now)
	if err != nil {
		t.Fatalf("dailySummary: %v", err)
	}
	if !strings.Contains(got, "Baran — 2s 20dk, kapı 1 kez açıldı") {
		t.Errorf("beklenen satiri icermiyor: %q", got)
	}
}

func TestDailySummaryExcludesLauncherDuration(t *testing.T) {
	dir := t.TempDir()
	day := time.Date(2026, 8, 23, 9, 0, 0, 0, time.UTC)
	if err := store.Append(dir, store.Event{
		TS: day.Add(time.Hour), Ev: "game_end", Who: "p1", Exe: "RiotClientServices.exe", DurS: 999,
	}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	cfg := &config.Config{Gated: []config.Game{{Exe: "RiotClientServices.exe", Launcher: true}}}
	got, err := dailySummary(dir, cfg, day.Add(2*time.Hour))
	if err != nil {
		t.Fatalf("dailySummary: %v", err)
	}
	if got != "Bugün henüz hareket yok." {
		t.Errorf("baslatici suresi ozete sizmis: %q", got)
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
