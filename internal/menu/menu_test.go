package menu

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func drive(t *testing.T, input string, items []Item) string {
	t.Helper()
	return driveHeader(t, input, nil, items)
}

func driveHeader(t *testing.T, input string, header func() string, items []Item) string {
	t.Helper()
	var out bytes.Buffer
	if err := Run(strings.NewReader(input), &out, header, items); err != nil {
		t.Fatalf("Run: %v", err)
	}
	return out.String()
}

func TestHeaderShownAboveMenu(t *testing.T) {
	out := driveHeader(t, "0\n", func() string { return "Oturum: kapalı" }, nil)
	if !strings.Contains(out, "Oturum: kapalı") {
		t.Errorf("durum basligi gosterilmedi:\n%s", out)
	}
}

func TestHeaderRefreshedOnEveryRedraw(t *testing.T) {
	// Kullanici menuyu acik birakip durumun degistigini gormek istiyor.
	n := 0
	out := driveHeader(t, "1\n0\n", func() string {
		n++
		return fmt.Sprintf("okuma %d", n)
	}, []Item{{Key: "1", Label: "Bir", Run: func() error { return nil }}})

	if !strings.Contains(out, "okuma 1") || !strings.Contains(out, "okuma 2") {
		t.Errorf("baslik her cizimde yeniden okunmadi:\n%s", out)
	}
}

func TestSelectionRunsMatchingItem(t *testing.T) {
	ran := 0
	out := drive(t, "1\n0\n", []Item{
		{Key: "1", Label: "Kurulum", Run: func() error { ran++; return nil }},
	})
	if ran != 1 {
		t.Errorf("secilen eylem %d kez calisti, 1 bekleniyordu", ran)
	}
	if !strings.Contains(out, "Kurulum") {
		t.Error("menu ogesi ekrana yazilmadi")
	}
}

func TestMenuRedrawsAfterAction(t *testing.T) {
	out := drive(t, "1\n0\n", []Item{
		{Key: "1", Label: "Kurulum", Run: func() error { return nil }},
	})
	// Eylem bitince menu yeniden cizilmeli; aksi halde cift tiklayan
	// kullanicinin penceresi bos kalir.
	if strings.Count(out, "Kurulum") < 2 {
		t.Errorf("menu eylemden sonra yeniden cizilmedi:\n%s", out)
	}
}

func TestInvalidChoiceKeepsMenuOpen(t *testing.T) {
	ran := 0
	out := drive(t, "9\n1\n0\n", []Item{
		{Key: "1", Label: "Kurulum", Run: func() error { ran++; return nil }},
	})
	if !strings.Contains(strings.ToLower(out), "geçersiz") {
		t.Errorf("gecersiz secim bildirilmedi:\n%s", out)
	}
	if ran != 1 {
		t.Errorf("gecersiz secimden sonra menu calismaya devam etmedi: %d", ran)
	}
}

func TestActionErrorIsShownButDoesNotExit(t *testing.T) {
	// Cift tiklayan kullanici hatayi okuyamadan pencere kapanmamali.
	second := 0
	out := drive(t, "1\n2\n0\n", []Item{
		{Key: "1", Label: "Patlayan", Run: func() error { return errors.New("disk dolu") }},
		{Key: "2", Label: "Sonraki", Run: func() error { second++; return nil }},
	})
	if !strings.Contains(out, "disk dolu") {
		t.Errorf("hata kullaniciya gosterilmedi:\n%s", out)
	}
	if second != 1 {
		t.Error("hatadan sonra menu kapandi")
	}
}

func TestEmptyInputExitsCleanly(t *testing.T) {
	drive(t, "", []Item{
		{Key: "1", Label: "Kurulum", Run: func() error { t.Fatal("girdi yokken eylem calisti"); return nil }},
	})
}
