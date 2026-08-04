//go:build windows

// Package setup, MFA eslestirmesini ve zamanlanmis gorev kurulumunu yapar.
//
// go-qrcode yalnizca bu paketten cagrilir; izleyici kod yolu bu paketi
// import etmez.
package setup

import (
	"bufio"
	"crypto/rand"
	"encoding/base32"
	"encoding/base64"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	qrcode "github.com/skip2/go-qrcode"

	"github.com/guts/antigame/internal/auth"
	"github.com/guts/antigame/internal/config"
	"github.com/guts/antigame/internal/store"
	"github.com/guts/antigame/internal/task"
	"github.com/guts/antigame/internal/totp"
	"github.com/guts/antigame/internal/vault"
)

func NewSecret() ([]byte, error) {
	s := make([]byte, 20)
	if _, err := rand.Read(s); err != nil {
		return nil, err
	}
	return s, nil
}

func encodeKey(secret []byte) string {
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
	b32 := encodeKey(secret)
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

// Run, etkilesimli kurulum sihirbazidir.
func Run(dir string, in io.Reader, out io.Writer) error {
	r := bufio.NewReader(in)
	ask := func(q string) (string, error) {
		fmt.Fprint(out, q)
		s, err := r.ReadString('\n')
		return strings.TrimSpace(s), err
	}

	if _, err := vault.Load(dir); err == nil {
		a, _ := ask("Zaten bir MFA kurulumu var. Üzerine yazılsın mı? (e/h): ")
		if !strings.EqualFold(a, "e") {
			fmt.Fprintln(out, "Kurulum iptal edildi.")
			return nil
		}
	}

	cfg, err := config.Load(dir)
	if err != nil {
		return err
	}
	if cfg.FriendName, err = ask("MFA'yı tutacak arkadaşınızın adı: "); err != nil {
		return err
	}
	if cfg.FriendHint, err = ask("Ona nasıl ulaşacaksınız (kapıda gösterilir, ör. WhatsApp): "); err != nil {
		return err
	}

	secret, err := NewSecret()
	if err != nil {
		return err
	}
	uri := OTPAuthURI(secret, os.Getenv("USERNAME"))
	png, err := qrcode.Encode(uri, qrcode.Medium, 512)
	if err != nil {
		return err
	}

	page := filepath.Join(os.TempDir(), "antigame-pairing.html")
	if err := os.WriteFile(page, []byte(QRPageHTML(uri, base64.StdEncoding.EncodeToString(png))), 0o600); err != nil {
		return err
	}
	defer os.Remove(page)

	if err := exec.Command("cmd", "/c", "start", "", page).Start(); err != nil {
		fmt.Fprintf(out, "Tarayıcı açılamadı, sayfayı elle açın: %s\n", page)
	}

	fmt.Fprintln(out, "\nQR kod tarayıcıda açıldı. Arkadaşınız okutsun.")
	code, err := readCode(r, out, encodeKey(secret), func() error {
		return store.Append(dir, store.Event{TS: time.Now().UTC(), Ev: "pairing_manual"})
	})
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	counter, res := totp.Verify(secret, code, now, 0)
	if res != totp.ResultOK {
		return fmt.Errorf("kod doğrulanamadı (%s); eşleştirme yapılmadı, kurulumu baştan çalıştırın", res)
	}

	if err := vault.Save(dir, secret); err != nil {
		return err
	}

	st, err := store.LoadState(dir)
	if err != nil {
		return err
	}
	// Eslestirmede kullanilan kodu yakiyoruz: kapiyi acmak icin
	// tekrar kullanilamamali.
	st.LastTOTPCounter = counter
	st.FailCount = 0
	st.LockUntil = nil

	recovery, salt, hash, err := auth.NewRecoveryCode()
	if err != nil {
		return err
	}
	st.RecoverySalt, st.RecoveryHash, st.RecoveryUsed = salt, hash, false
	if err := store.SaveState(dir, st); err != nil {
		return err
	}
	if err := config.Save(dir, cfg); err != nil {
		return err
	}

	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if err := task.Install(exe); err != nil {
		return err
	}

	fmt.Fprintf(out, `
Kurulum tamamlandı.

KURTARMA KODU: %s

Bu kodu SİZ saklamayın — arkadaşınız telefonunu kaybederse diye
ikinci bir kişiye verin. Tek kullanımlıktır.

İzleyici oturum açılışında otomatik başlayacak. Şimdi başlatmak için:
  antigame watch
`, recovery)
	return nil
}
