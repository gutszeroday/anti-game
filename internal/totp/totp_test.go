package totp

import (
	"testing"
	"time"
)

// RFC 6238 Appendix B, SHA-1 test vektorleri. Belgedeki 8 haneli
// degerlerin son 6 hanesi 6 haneli kodun beklenen degeridir.
var rfcSecret = []byte("12345678901234567890")

func TestRFC6238Vectors(t *testing.T) {
	cases := []struct {
		unix int64
		want string
	}{
		{59, "287082"},
		{1111111109, "081804"},
		{1111111111, "050471"},
		{1234567890, "005924"},
		{2000000000, "279037"},
		{20000000000, "353130"},
	}
	for _, c := range cases {
		got := Code(rfcSecret, Counter(time.Unix(c.unix, 0).UTC()))
		if got != c.want {
			t.Errorf("t=%d: %s bekleniyordu, %s geldi", c.unix, c.want, got)
		}
	}
}

func TestVerifyAcceptsCurrentCode(t *testing.T) {
	now := time.Unix(1111111109, 0).UTC()
	code := Code(rfcSecret, Counter(now))
	c, res := Verify(rfcSecret, code, now, 0)
	if res != ResultOK {
		t.Fatalf("gecerli kod reddedildi: %v", res)
	}
	if c != Counter(now) {
		t.Errorf("yanlis sayac dondu: %d", c)
	}
}

func TestVerifyRejectsReusedCode(t *testing.T) {
	now := time.Unix(1111111109, 0).UTC()
	code := Code(rfcSecret, Counter(now))
	if _, res := Verify(rfcSecret, code, now, Counter(now)); res != ResultReplay {
		t.Fatalf("ikinci kez kullanilan kod kabul edildi: %v", res)
	}
}

func TestVerifyRejectsClockRollback(t *testing.T) {
	// Kullanici sistem saatini geri aliyor ve eski bir kodu tekrar giriyor.
	past := time.Unix(1111111109, 0).UTC()
	lastUsed := Counter(past) + 100 // ileride kabul edilmis bir kod var
	code := Code(rfcSecret, Counter(past))
	if _, res := Verify(rfcSecret, code, past, lastUsed); res != ResultReplay {
		t.Fatalf("saat geri alinarak eski kod kabul edildi: %v", res)
	}
}

func TestVerifyToleratesOneStepSkew(t *testing.T) {
	base := time.Unix(1111111109, 0).UTC()
	code := Code(rfcSecret, Counter(base))
	for _, delta := range []time.Duration{-30 * time.Second, 30 * time.Second} {
		if _, res := Verify(rfcSecret, code, base.Add(delta), 0); res != ResultOK {
			t.Errorf("%v kayma reddedildi", delta)
		}
	}
}

func TestVerifyRejectsTwoStepSkew(t *testing.T) {
	base := time.Unix(1111111109, 0).UTC()
	code := Code(rfcSecret, Counter(base))
	for _, delta := range []time.Duration{-90 * time.Second, 90 * time.Second} {
		if _, res := Verify(rfcSecret, code, base.Add(delta), 0); res != ResultBadCode {
			t.Errorf("%v kayma kabul edildi", delta)
		}
	}
}

func TestVerifyRejectsMalformedInput(t *testing.T) {
	now := time.Unix(1111111109, 0).UTC()
	for _, bad := range []string{"", "12345", "1234567", "abcdef", "12 34 56"} {
		if _, res := Verify(rfcSecret, bad, now, 0); res != ResultBadCode {
			t.Errorf("bozuk girdi kabul edildi: %q", bad)
		}
	}
}

func TestVerifyIgnoresSurroundingWhitespace(t *testing.T) {
	now := time.Unix(1111111109, 0).UTC()
	code := "  " + Code(rfcSecret, Counter(now)) + "\r\n"
	if _, res := Verify(rfcSecret, code, now, 0); res != ResultOK {
		t.Fatal("bosluklu kod reddedildi")
	}
}

func TestFindSkewReportsPhoneClockOffset(t *testing.T) {
	secret := []byte("12345678901234567890")
	now := time.Date(2026, 8, 4, 6, 0, 0, 0, time.UTC)

	// Telefon 3 dakika ileride: kod dogru anahtardan uretilmis ama
	// dogrulama penceresine girmiyor.
	code := Code(secret, Counter(now.Add(3*time.Minute)))
	skew, ok := FindSkew(secret, code, now)
	if !ok {
		t.Fatal("dogru anahtardan uretilen kod bulunamadi")
	}
	if skew.Round(time.Minute) != 3*time.Minute {
		t.Errorf("saat farki 3 dk olmaliydi, %v geldi", skew)
	}
}

func TestFindSkewHandlesBackwardOffset(t *testing.T) {
	secret := []byte("12345678901234567890")
	now := time.Date(2026, 8, 4, 6, 0, 0, 0, time.UTC)

	code := Code(secret, Counter(now.Add(-5*time.Minute)))
	skew, ok := FindSkew(secret, code, now)
	if !ok {
		t.Fatal("geri kalmis saatteki kod bulunamadi")
	}
	if skew.Round(time.Minute) != -5*time.Minute {
		t.Errorf("saat farki -5 dk olmaliydi, %v geldi", skew)
	}
}

func TestFindSkewFailsForDifferentSecret(t *testing.T) {
	now := time.Date(2026, 8, 4, 6, 0, 0, 0, time.UTC)
	code := Code([]byte("12345678901234567890"), Counter(now))
	if _, ok := FindSkew([]byte("09876543210987654321"), code, now); ok {
		t.Fatal("baska anahtardan uretilen kod eslesti")
	}
}

func TestFindSkewDoesNotAcceptCodes(t *testing.T) {
	// FindSkew yalnizca teshis icindir; kabul karari Verify'de kalmali.
	secret := []byte("12345678901234567890")
	now := time.Date(2026, 8, 4, 6, 0, 0, 0, time.UTC)
	code := Code(secret, Counter(now.Add(10*time.Minute)))

	if _, res := Verify(secret, code, now, 0); res != ResultBadCode {
		t.Fatal("teshis penceresi dogrulamayi gevsetmis")
	}
}

func TestLockDurationEscalates(t *testing.T) {
	cases := []struct {
		fails int
		want  time.Duration
	}{
		{0, 0}, {4, 0},
		{5, 15 * time.Minute}, {9, 15 * time.Minute},
		{10, 30 * time.Minute}, {14, 30 * time.Minute},
		{15, 60 * time.Minute}, {50, 60 * time.Minute},
	}
	for _, c := range cases {
		if got := LockDuration(c.fails); got != c.want {
			t.Errorf("%d hata: %v bekleniyordu, %v geldi", c.fails, c.want, got)
		}
	}
}
