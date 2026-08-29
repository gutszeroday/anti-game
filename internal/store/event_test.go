package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAppendWritesMonthlyFile(t *testing.T) {
	dir := t.TempDir()
	ts := time.Date(2026, 8, 3, 18, 4, 11, 0, time.UTC)
	if err := Append(dir, Event{TS: ts, Ev: "game_start", Exe: "VALORANT.exe"}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "events-2026-08.jsonl")); err != nil {
		t.Fatalf("aylik dosya olusmadi: %v", err)
	}
}

func TestReadSkipsCorruptTrailingLine(t *testing.T) {
	dir := t.TempDir()
	ts := time.Date(2026, 8, 3, 18, 0, 0, 0, time.UTC)
	if err := Append(dir, Event{TS: ts, Ev: "game_start", Exe: "a.exe"}); err != nil {
		t.Fatal(err)
	}
	// Yarim kalmis son satiri elle ekle (izleyici yazarken kapanmis senaryosu).
	f, _ := os.OpenFile(filepath.Join(dir, "events-2026-08.jsonl"), os.O_APPEND|os.O_WRONLY, 0o600)
	f.WriteString(`{"ts":"2026-08-03T18:05:0`)
	f.Close()

	got, err := Read(dir, ts.Add(-time.Hour), ts.Add(time.Hour))
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(got) != 1 || got[0].Exe != "a.exe" {
		t.Fatalf("bozuk satir dogru islenmedi: %+v", got)
	}
}

func TestReadFiltersByRangeAcrossMonths(t *testing.T) {
	dir := t.TempDir()
	july := time.Date(2026, 7, 31, 23, 0, 0, 0, time.UTC)
	august := time.Date(2026, 8, 1, 1, 0, 0, 0, time.UTC)
	Append(dir, Event{TS: july, Ev: "usage", Exe: "temmuz.exe"})
	Append(dir, Event{TS: august, Ev: "usage", Exe: "agustos.exe"})

	got, err := Read(dir, july.Add(-time.Hour), august.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("ay siniri asilamadi, %d olay okundu", len(got))
	}

	got, err = Read(dir, august, august.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Exe != "agustos.exe" {
		t.Fatalf("aralik filtresi calismadi: %+v", got)
	}
}

func TestEarliestEventMonthPicksOldest(t *testing.T) {
	dir := t.TempDir()
	Append(dir, Event{TS: time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC), Ev: "usage"})
	Append(dir, Event{TS: time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC), Ev: "usage"})
	Append(dir, Event{TS: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), Ev: "usage"})

	got, ok, err := EarliestEventMonth(dir)
	if err != nil {
		t.Fatalf("EarliestEventMonth: %v", err)
	}
	if !ok {
		t.Fatalf("ok=false, en az bir ay var")
	}
	want := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("got %v want %v", got, want)
	}
}

func TestEarliestEventMonthEmptyDir(t *testing.T) {
	_, ok, err := EarliestEventMonth(t.TempDir())
	if err != nil {
		t.Fatalf("EarliestEventMonth: %v", err)
	}
	if ok {
		t.Fatalf("bos dizinde ok=true olmamali")
	}
}

func TestReadEmptyDirReturnsNoError(t *testing.T) {
	got, err := Read(t.TempDir(), time.Now().Add(-time.Hour), time.Now())
	if err != nil {
		t.Fatalf("bos dizin hata verdi: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("bos dizinden olay geldi: %+v", got)
	}
}
