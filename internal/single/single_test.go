//go:build windows

package single

import "testing"

func TestSecondAcquireIsRefused(t *testing.T) {
	release, ok := Acquire("antigame-test-lock")
	if !ok {
		t.Fatal("ilk kilit alinamadi")
	}
	defer release()

	if _, ok := Acquire("antigame-test-lock"); ok {
		t.Fatal("kilit ikinci kez verildi; iki izleyici calisabilirdi")
	}
}

func TestReleaseAllowsReacquire(t *testing.T) {
	release, ok := Acquire("antigame-test-lock-2")
	if !ok {
		t.Fatal("ilk kilit alinamadi")
	}
	release()

	release2, ok := Acquire("antigame-test-lock-2")
	if !ok {
		t.Fatal("birakilan kilit tekrar alinamadi")
	}
	release2()
}

func TestDifferentNamesDoNotCollide(t *testing.T) {
	a, ok := Acquire("antigame-test-a")
	if !ok {
		t.Fatal("a kilidi alinamadi")
	}
	defer a()

	b, ok := Acquire("antigame-test-b")
	if !ok {
		t.Fatal("farkli isimli kilit engellendi")
	}
	b()
}
