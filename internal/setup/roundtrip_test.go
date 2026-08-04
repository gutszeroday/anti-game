package setup

import (
	"encoding/base32"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/guts/antigame/internal/totp"
)

// TestPairingRoundTripAcceptsCodeFromURI, eslestirmenin kendi icinde tutarli
// oldugunu kanitlar: URI'ye yazilan anahtardan uretilen kod, setup'in
// dogrulayicisindan gecmeli. Gecmezse hata bizde; gecerse telefon veya saat
// tarafinda.
func TestPairingRoundTripAcceptsCodeFromURI(t *testing.T) {
	secret, err := NewSecret()
	if err != nil {
		t.Fatal(err)
	}
	uri := OTPAuthURI(secret, "guts")

	u, err := url.Parse(uri)
	if err != nil {
		t.Fatalf("URI cozulemedi: %v", err)
	}
	b32 := u.Query().Get("secret")
	if b32 == "" {
		t.Fatal("URI'de secret yok")
	}

	// Telefonun yaptigini yapiyoruz: base32'yi coz, koda cevir.
	raw, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(b32)
	if err != nil {
		t.Fatalf("URI'deki anahtar base32 degil: %v", err)
	}
	now := time.Now().UTC()
	code := totp.Code(raw, totp.Counter(now))

	if _, res := totp.Verify(secret, code, now, 0); res != totp.ResultOK {
		t.Fatalf("URI'den uretilen kod reddedildi: %s", res)
	}
}

// TestManualKeyRoundTrip, terminale basilan gruplanmis anahtarin da ayni
// secret'i verdigini dogrular: bosluklar yok sayilinca kod tutmali.
func TestManualKeyRoundTrip(t *testing.T) {
	secret, err := NewSecret()
	if err != nil {
		t.Fatal(err)
	}
	shown := GroupKey(encodeKey(secret))

	raw, err := base32.StdEncoding.WithPadding(base32.NoPadding).
		DecodeString(strings.ReplaceAll(shown, " ", ""))
	if err != nil {
		t.Fatalf("gosterilen anahtar cozulemedi: %v", err)
	}
	now := time.Now().UTC()
	if _, res := totp.Verify(secret, totp.Code(raw, totp.Counter(now)), now, 0); res != totp.ResultOK {
		t.Fatal("elle girilen anahtardan uretilen kod reddedildi")
	}
}

// TestClockSkewBeyondToleranceIsRejected, kabul penceresinin sinirini
// gosterir: +-1 adim (30 sn) disi kodlar bad_code olur. Kullanicinin
// gordugu hatanin en olasi sebebi budur.
func TestClockSkewBeyondToleranceIsRejected(t *testing.T) {
	secret, _ := NewSecret()
	now := time.Now().UTC()

	// Telefon 2 dakika ileride olsun.
	phoneCode := totp.Code(secret, totp.Counter(now.Add(2*time.Minute)))
	if _, res := totp.Verify(secret, phoneCode, now, 0); res == totp.ResultOK {
		t.Fatal("2 dakikalik saat farkinda kod kabul edildi; tolerans cok genis")
	}

	// 30 saniye ileri hala kabul edilmeli.
	nearCode := totp.Code(secret, totp.Counter(now.Add(30*time.Second)))
	if _, res := totp.Verify(secret, nearCode, now, 0); res != totp.ResultOK {
		t.Error("bir adimlik fark reddedildi; tolerans cok dar")
	}
}
