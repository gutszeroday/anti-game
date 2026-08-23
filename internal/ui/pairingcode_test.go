//go:build windows

package ui

import (
	"strconv"
	"testing"
)

func TestNewPairingCodeIsSixNumericDigits(t *testing.T) {
	for i := 0; i < 20; i++ {
		code, err := newPairingCode()
		if err != nil {
			t.Fatalf("newPairingCode: %v", err)
		}
		if len(code) != 6 {
			t.Fatalf("kod 6 haneli degil: %q", code)
		}
		if _, err := strconv.Atoi(code); err != nil {
			t.Fatalf("kod sayisal degil: %q (%v)", code, err)
		}
	}
}
