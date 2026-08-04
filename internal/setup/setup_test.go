package setup

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
)

const sampleKey = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"

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

func TestQRPageExplainsManualKeyOption(t *testing.T) {
	html := QRPageHTML(OTPAuthURI([]byte("12345678901234567890"), "guts"), "AAAA")
	if !strings.Contains(html, "anahtar") {
		t.Error("uzaktaki arkadas icin manuel anahtar yolu sayfada anlatilmamis")
	}
}

func TestGroupKeySplitsIntoFours(t *testing.T) {
	got := GroupKey(sampleKey)
	if got != "GEZD GNBV GY3T QOJQ GEZD GNBV GY3T QOJQ" {
		t.Errorf("beklenmeyen gruplama: %q", got)
	}
	if strings.ReplaceAll(got, " ", "") != sampleKey {
		t.Error("gruplama anahtari bozdu")
	}
}

func TestGroupKeyHandlesRemainder(t *testing.T) {
	if got := GroupKey("ABCDEF"); got != "ABCD EF" {
		t.Errorf("artan haneler dusuruldu: %q", got)
	}
}

// readCode yardimcilari: gercek bir kasa veya tarayici olmadan istem
// dongusunu surebilmek icin.
func readWith(t *testing.T, input string) (code string, out string, reveals int) {
	t.Helper()
	var buf bytes.Buffer
	r := bufio.NewReader(strings.NewReader(input))
	code, err := readCode(r, &buf, sampleKey, func() error { reveals++; return nil })
	if err != nil {
		t.Fatalf("readCode: %v", err)
	}
	return code, buf.String(), reveals
}

func TestReadCodeRevealsKeyOnKeyword(t *testing.T) {
	code, out, reveals := readWith(t, "anahtar\n123456\n")
	if code != "123456" {
		t.Errorf("anahtar gosterildikten sonra kod istenmedi: %q", code)
	}
	if !strings.Contains(out, GroupKey(sampleKey)) {
		t.Errorf("anahtar gruplanmis halde basilmadi:\n%s", out)
	}
	if strings.Contains(out, sampleKey) {
		t.Error("anahtar kesintisiz dizge olarak basildi")
	}
	if reveals != 1 {
		t.Errorf("aciga cikarma %d kez kaydedildi, 1 bekleniyordu", reveals)
	}
}

func TestReadCodeDoesNotRevealForNormalCode(t *testing.T) {
	code, out, reveals := readWith(t, "123456\n")
	if code != "123456" {
		t.Errorf("kod okunamadi: %q", code)
	}
	if strings.Contains(out, "GEZD") {
		t.Error("istenmeden anahtar basildi")
	}
	if reveals != 0 {
		t.Errorf("aciga cikarma bosuna kaydedildi: %d", reveals)
	}
}

func TestReadCodeWarnsBeforeShowingKey(t *testing.T) {
	_, out, _ := readWith(t, "anahtar\n123456\n")
	if !strings.Contains(out, "DİKKAT") {
		t.Errorf("anahtar uyarisiz gosterildi:\n%s", out)
	}
}
