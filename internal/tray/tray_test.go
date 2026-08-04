//go:build windows

package tray

import (
	"context"
	"testing"
	"time"
)

func TestRunAddsIconAndExitsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, "antigame — test", []Item{
			{Label: "Bir şey", Run: func() {}},
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
	go func() { done <- Run(ctx, "antigame", nil) }()

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
