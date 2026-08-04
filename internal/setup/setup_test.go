package setup

import (
	"strings"
	"testing"
)

func TestNewSecretIs160Bit(t *testing.T) {
	s, err := NewSecret()
	if err != nil {
		t.Fatal(err)
	}
	if len(s) != 20 {
		t.Errorf("20 bayt bekleniyordu, %d geldi", len(s))
	}
}

func TestNewSecretIsRandom(t *testing.T) {
	a, _ := NewSecret()
	b, _ := NewSecret()
	if string(a) == string(b) {
		t.Fatal("ust uste iki secret ayni cikti")
	}
}

func TestOTPAuthURIFormat(t *testing.T) {
	uri := OTPAuthURI([]byte("12345678901234567890"), "guts")
	for _, want := range []string{
		"otpauth://totp/anti-game:guts?",
		"issuer=anti-game",
		"algorithm=SHA1",
		"digits=6",
		"period=30",
		"secret=GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ",
	} {
		if !strings.Contains(uri, want) {
			t.Errorf("URI %q icermiyor: %s", want, uri)
		}
	}
}

func TestOTPAuthURIEscapesAccount(t *testing.T) {
	uri := OTPAuthURI([]byte("12345678901234567890"), "ali veli")
	if strings.Contains(uri, "ali veli") {
		t.Errorf("hesap adi URL kacislanmamis: %s", uri)
	}
}

func TestQRPageEmbedsImageAndNoSecretText(t *testing.T) {
	uri := OTPAuthURI([]byte("12345678901234567890"), "guts")
	html := QRPageHTML(uri, "AAAA")
	if !strings.Contains(html, "data:image/png;base64,AAAA") {
		t.Error("QR gorseli gomulmemis")
	}
	// Secret duz metin olarak sayfada gorunmemeli; yalnizca QR icinde olmali.
	if strings.Contains(html, "GEZDGNBVGY3TQOJQ") {
		t.Error("secret sayfada duz metin olarak yaziyor")
	}
}
