package telegramwatch

import (
	"context"
	"testing"
	"time"
)

func TestRunReturnsPromptlyWhenContextCancelled(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	doneCh := make(chan error, 1)
	go func() { doneCh <- Run(ctx, func() string { return dir }) }()

	select {
	case err := <-doneCh:
		if err != nil {
			t.Fatalf("Run hata dondu: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run, context iptalinden sonra donmedi")
	}
}
