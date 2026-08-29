package store

import (
	"testing"
	"time"
)

func TestAppendSentThenReadSentReturnsNewestFirst(t *testing.T) {
	dir := t.TempDir()
	t1 := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 8, 24, 11, 0, 0, 0, time.UTC)
	if err := AppendSent(dir, SentNotification{TS: t1, ChatID: 1, Label: "Ali", Text: "Kapı açıldı: Ali, 13:00"}); err != nil {
		t.Fatalf("AppendSent: %v", err)
	}
	if err := AppendSent(dir, SentNotification{TS: t2, ChatID: 1, Label: "Ali", Text: "İzleyici kapatıldı: 14:00"}); err != nil {
		t.Fatalf("AppendSent: %v", err)
	}

	got, err := ReadSent(dir, 10)
	if err != nil {
		t.Fatalf("ReadSent: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("2 kayıt bekleniyordu, %d geldi", len(got))
	}
	if !got[0].TS.Equal(t2) || got[0].Text != "İzleyici kapatıldı: 14:00" {
		t.Errorf("en yeni kayıt önde olmalı: %+v", got[0])
	}
	if !got[1].TS.Equal(t1) {
		t.Errorf("ikinci kayıt en eski olmalı: %+v", got[1])
	}
}

func TestReadSentLimitsToMostRecent(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 5; i++ {
		ts := time.Date(2026, 8, 24, 10, i, 0, 0, time.UTC)
		if err := AppendSent(dir, SentNotification{TS: ts, ChatID: 1, Text: "m"}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := ReadSent(dir, 2)
	if err != nil {
		t.Fatalf("ReadSent: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("limit=2 iken %d kayıt geldi", len(got))
	}
	if got[0].TS.Minute() != 4 || got[1].TS.Minute() != 3 {
		t.Errorf("en yeni 2 kayıt beklenirdi: %+v", got)
	}
}

func TestReadSentEmptyDirReturnsNoError(t *testing.T) {
	got, err := ReadSent(t.TempDir(), 10)
	if err != nil {
		t.Fatalf("bos dizin hata verdi: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("bos dizinden kayit geldi: %+v", got)
	}
}
