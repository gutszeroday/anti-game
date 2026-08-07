//go:build windows

// Package pairing, bir kisiye TOTP anahtari baglama akisidir: anahtar
// uretir, QR sayfasini acar ve dogru kod girilene kadar sorar.
//
// Kurulum sihirbazi ve kisi ekranindan ayni sekilde cagrilir; ikisi de
// ayni akisi kendi icinde tekrarlamasin diye ayri paket oldu.
// go-qrcode yalnizca buradan cagrilir.
package pairing

import (
	"bufio"
	"crypto/rand"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	qrcode "github.com/skip2/go-qrcode"

	"github.com/guts/antigame/internal/totp"
)

func NewSecret() ([]byte, error) {
	s := make([]byte, 20)
	if _, err := rand.Read(s); err != nil {
		return nil, err
	}
	return s, nil
}

// EncodeKey, anahtari authenticator uygulamalarinin bekledigi base32
// gosterime cevirir. Disa acik: arayuz, anahtari elle girmek isteyen
// kullaniciya gostermek icin ham base32'ye ihtiyac duyuyor.
func EncodeKey(secret []byte) string {
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secret)
}

// GroupKey, anahtari 4'erli gruplara ayirir. Telefonla dikte etmeyi
// kolaylastiriyor ve kesintisiz base32 dizgesinin hicbir yerde olusmamasini
// sagliyor; authenticator uygulamalari bosluklari yok sayar.
func GroupKey(b32 string) string {
	var parts []string
	for i := 0; i < len(b32); i += 4 {
		parts = append(parts, b32[i:min(i+4, len(b32))])
	}
	return strings.Join(parts, " ")
}

func OTPAuthURI(secret []byte, account string) string {
	b32 := EncodeKey(secret)
	q := url.Values{}
	q.Set("secret", b32)
	q.Set("issuer", "anti-game")
	q.Set("algorithm", "SHA1")
	q.Set("digits", "6")
	q.Set("period", "30")
	return "otpauth://totp/" + url.PathEscape("anti-game:"+account) + "?" + q.Encode()
}

// QRPageHTML, eslestirme sayfasini uretir. Secret yalnizca QR gorselinin
// icindedir; duz metin olarak yazilmaz, boylece ekran goruntusu veya
// omuz ustu bakis tek basina yeterli olmaz.
func QRPageHTML(uri, pngBase64 string) string {
	return `<!doctype html><html lang="tr"><head><meta charset="utf-8">
<title>anti-game — MFA eşleştirme</title>
<style>
body{font-family:Segoe UI,system-ui,sans-serif;margin:0;min-height:100vh;
display:grid;place-items:center;background:#111;color:#eee}
.card{text-align:center;max-width:34rem;padding:2rem}
img{width:280px;height:280px;background:#fff;padding:12px;border-radius:12px}
p{line-height:1.6;color:#bbb}
strong{color:#fff}
</style></head><body><div class="card">
<h1>Arkadaşınız bu kodu okutsun</h1>
<img alt="QR kod" src="data:image/png;base64,` + pngBase64 + `">
<p>Google Authenticator veya benzeri bir uygulamada <strong>QR kodu tara</strong>
seçeneğiyle okutun. Okuttuktan sonra bu pencereyi kapatın ve
uygulamada görünen <strong>6 haneli kodu</strong> kuruluma girin.</p>
<p>Arkadaşınız uzaktaysa ve QR okutamıyorsa, kurulum penceresine
<strong>anahtar</strong> yazın; anahtar orada gösterilir.</p>
<p>Bu sayfa kurulum bitince silinir.</p>
</div></body></html>`
}

// readCode, dogrulama kodunu okur. Kullanici kod yerine "anahtar" yazarsa
// secret'i gruplanmis halde basar, onReveal ile eylemi kaydeder ve kodu
// istemeye devam eder.
//
// Aciga cikarma tarayicidaki sayfada degil burada yapiliyor: sayfadan
// sihirbaza geri kanal yok, dolayisiyla orada gosterilse kaydedilemezdi.
func readCode(r *bufio.Reader, out io.Writer, b32 string, onReveal func() error) (string, error) {
	for {
		fmt.Fprint(out, `Uygulamada görünen 6 haneli kodu girin (arkadaşınız uzaktaysa "anahtar" yazın): `)
		s, err := r.ReadString('\n')
		s = strings.TrimSpace(s)
		if err != nil && s == "" {
			return "", err
		}
		if !strings.EqualFold(s, "anahtar") {
			return s, nil
		}
		fmt.Fprintf(out, `
DİKKAT: Bu anahtarı gören herkes kapıyı açabilir. Arkadaşınıza iletin,
kendinizde saklamayın. Bu adım kayda geçiyor.

Anahtar: %s

`, GroupKey(b32))
		if onReveal != nil {
			if err := onReveal(); err != nil {
				return "", err
			}
		}
	}
}

// Check, tek bir eslestirme kodunu dogrular. Kabul edilirse kodun sayacini
// dondurur; reddedilirse kullaniciya gosterilecek aciklamayi.
//
// Karar burada veriliyor cunku iki kabuk da ayni cevabi vermek zorunda:
// metin sihirbazi bunu donguye sarar, pencere tek atista cagirir.
// "Kod hatali" demek yetmiyordu; saat kaymasiyla yanlis kaydi ayirmak
// kullanicinin ne yapacagini degistiriyor.
func Check(secret []byte, code string, now time.Time) (uint64, bool, string) {
	counter, res := totp.Verify(secret, code, now, 0)
	if res == totp.ResultOK {
		return counter, true, ""
	}

	// Kod dogru anahtardan uretilmis ama pencereye girmiyorsa sorun
	// saatlerin uyusmamasidir; bunu miktariyla soylemek gerekiyor.
	if skew, ok := totp.FindSkew(secret, code, now); ok {
		yon := "ileri"
		if skew < 0 {
			yon, skew = "geri", -skew
		}
		return 0, false, fmt.Sprintf(
			"Anahtar doğru, ama kodu üreten cihazın saati %d dakika %s. "+
				"Telefonda saati otomatik ayara alın (Google Authenticator: "+
				"Ayarlar > Zaman düzeltmesi > Kodlar için saati eşitle), sonra yeni kodu girin.",
			int(skew.Round(time.Minute).Minutes()), yon)
	}
	return 0, false, `Bu kod anahtarla eşleşmiyor. Uygulamada "anti-game" kaydını ` +
		"seçtiğinizden emin olun; başka bir hesabın kodu girilmiş olabilir."
}

// QRImage, eslestirme URI'sini kare bir gorsele cevirir. Pencere QR'i
// kendi icinde cizdigi icin PNG'ye gerek yok. go-qrcode yalnizca bu
// paketten cagrilir (bkz. paket yorumu).
func QRImage(uri string, size int) (image.Image, error) {
	q, err := qrcode.New(uri, qrcode.Medium)
	if err != nil {
		return nil, err
	}
	return q.Image(size), nil
}

// confirmPairing, dogru kod girilene kadar sorar ve kabul edilen kodun
// sayacini dondurur. Bos satir kurulumu iptal eder.
//
// Yanlis kodda pes edilmiyor: eskiden tek hata secret'i cope atiyor ve
// QR'in bastan okutulmasini gerektiriyordu.
func confirmPairing(r *bufio.Reader, out io.Writer, secret []byte, now func() time.Time, onReveal func() error) (uint64, error) {
	for {
		code, err := readCode(r, out, EncodeKey(secret), onReveal)
		if err != nil && code == "" {
			return 0, err
		}
		if code == "" {
			return 0, errors.New("kod girilmedi, eşleştirme iptal edildi")
		}
		counter, ok, msg := Check(secret, code, now())
		if ok {
			return counter, nil
		}
		fmt.Fprintf(out, "\n%s Çıkmak için boş bırakıp Enter.\n\n", msg)
	}
}

// Pair, yeni bir anahtar uretir, QR sayfasini tarayicida acar ve dogru
// kod girilene kadar sorar. Kabul edilen kodun sayacini da dondurur:
// cagiran onu yakarak ayni kodun kapiyi acmasini engeller.
//
// Anahtar yalnizca burada uretilir ve dogrulanmadan diske yazilmaz;
// yarida birakilan bir ekleme geride kayit birakmaz.
// Okuyucu bufio.Reader olarak isteniyor: cagiran zaten satir satir
// okuyorsa, burada ikinci bir tampon acmak onun onunden veri kapardi.
func Pair(r *bufio.Reader, out io.Writer, account string, onReveal func() error) ([]byte, uint64, error) {
	secret, err := NewSecret()
	if err != nil {
		return nil, 0, err
	}
	uri := OTPAuthURI(secret, account)
	png, err := qrcode.Encode(uri, qrcode.Medium, 512)
	if err != nil {
		return nil, 0, err
	}

	page := filepath.Join(os.TempDir(), "antigame-pairing.html")
	if err := os.WriteFile(page, []byte(QRPageHTML(uri, base64.StdEncoding.EncodeToString(png))), 0o600); err != nil {
		return nil, 0, err
	}
	defer os.Remove(page)

	if err := exec.Command("cmd", "/c", "start", "", page).Start(); err != nil {
		fmt.Fprintf(out, "Tarayıcı açılamadı, sayfayı elle açın: %s\n", page)
	}
	fmt.Fprintln(out, "\nQR kod tarayıcıda açıldı. Arkadaşınız okutsun.")

	counter, err := confirmPairing(r, out, secret,
		func() time.Time { return time.Now().UTC() }, onReveal)
	if err != nil {
		return nil, 0, err
	}
	return secret, counter, nil
}
