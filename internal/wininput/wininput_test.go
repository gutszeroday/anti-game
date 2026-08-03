//go:build windows

package wininput

import "testing"

func TestIdleSecondsIsNonNegativeAndPlausible(t *testing.T) {
	s, err := IdleSeconds()
	if err != nil {
		t.Fatalf("IdleSeconds: %v", err)
	}
	if s < 0 {
		t.Errorf("negatif bosta sure: %d", s)
	}
	// Sistem acilisindan bu yana gecen sureden buyuk olamaz; 10 yil ustu
	// bir deger tasma hatasina isaret eder.
	if s > 10*365*24*3600 {
		t.Errorf("makul olmayan bosta sure: %d", s)
	}
}

func TestForegroundPIDDoesNotError(t *testing.T) {
	// CI veya kilitli oturumda odakta pencere olmayabilir; bu hata degildir.
	pid, err := ForegroundPID()
	if err != nil {
		t.Fatalf("ForegroundPID: %v", err)
	}
	if pid < 0 {
		t.Errorf("negatif PID: %d", pid)
	}
}
