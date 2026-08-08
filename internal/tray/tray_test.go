//go:build windows

package tray

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRunAddsIconAndExitsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, Options{
			Tip:   "antigame — test",
			Items: []Item{{Label: "Bir şey", Run: func() {}}},
		})
	}()

	// Simgenin eklenmesi ve mesaj dongusunun kurulmasi icin kisa sure taniyoruz.
	time.Sleep(300 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ctx iptal edildi ama mesaj dongusu cikmadi")
	}
}

func TestRunWithoutItemsStillExits(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, Options{Tip: "antigame"}) }()

	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("bos menuyle cikilamadi")
	}
}

func TestDefaultItemFindsTheMarkedOne(t *testing.T) {
	items := []Item{{Label: "a"}, {Label: "b", Default: true}, {Label: "c"}}
	if got := defaultItem(items); got != 1 {
		t.Errorf("defaultItem = %d, istenen 1", got)
	}
}

func TestDefaultItemReportsNoneWhenUnmarked(t *testing.T) {
	if got := defaultItem([]Item{{Label: "a"}, {Label: "b"}}); got != -1 {
		t.Errorf("defaultItem = %d, istenen -1", got)
	}
}

func TestDefaultItemTakesTheFirstMark(t *testing.T) {
	items := []Item{{Label: "a", Default: true}, {Label: "b", Default: true}}
	if got := defaultItem(items); got != 0 {
		t.Errorf("defaultItem = %d, istenen 0", got)
	}
}

func TestDefaultItemOnEmptyList(t *testing.T) {
	if got := defaultItem(nil); got != -1 {
		t.Errorf("defaultItem = %d, istenen -1", got)
	}
}

func TestTipTextPrefersTipFunc(t *testing.T) {
	o := Options{Tip: "sabit", TipFunc: func() string { return "canlı" }}
	if got := o.tipText(); got != "canlı" {
		t.Errorf("tipText = %q, istenen \"canlı\"", got)
	}
	o.TipFunc = nil
	if got := o.tipText(); got != "sabit" {
		t.Errorf("TipFunc yokken tipText = %q, istenen \"sabit\"", got)
	}
}

func TestTipTextTrimsToTheTooltipLimit(t *testing.T) {
	// Windows sinira takilan metni sessizce atiyor; kirpilmis metin
	// atilmis metinden iyi.
	long := strings.Repeat("a", 300)
	o := Options{TipFunc: func() string { return long }}
	if n := len([]rune(o.tipText())); n != tooltipMax {
		t.Errorf("tooltip %d karakter, istenen %d", n, tooltipMax)
	}
}
