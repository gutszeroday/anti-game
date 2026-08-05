// Package auth, kapiya girilen kodun kabul edilip edilmeyecegine karar verir
// ve sonucu kalici duruma isler. DPAPI'ye bagimli degildir: secret disaridan
// verilir, boylece tum kararlar test edilebilir.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/guts/antigame/internal/session"
	"github.com/guts/antigame/internal/store"
	"github.com/guts/antigame/internal/totp"
)

// attemptsPerLevel, kilit kademesinin kac hatada bir yukseldigini belirler.
const attemptsPerLevel = 5

// Outcome, kapinin kullaniciya gosterecegi sonuctur.
type Outcome struct {
	OK          bool
	Message     string
	LockedUntil time.Time
	Remaining   int
	// Who, kodu kabul edilen kisinin ID'sidir. Kurtarma kodunda bostur.
	Who string
}

// Key, bir kisinin TOTP anahtaridir.
type Key struct {
	ID     string
	Secret []byte
}

type Verifier struct {
	Dir   string
	Keys  []Key
	Grace time.Duration
	Now   func() time.Time
}

func (v Verifier) now() time.Time {
	if v.Now != nil {
		return v.Now()
	}
	return time.Now().UTC()
}

// HashRecovery, kurtarma kodunun tuzlanmis ozetini dondurur.
func HashRecovery(salt, code string) string {
	sum := sha256.Sum256([]byte(salt + strings.TrimSpace(code)))
	return hex.EncodeToString(sum[:])
}

// NewRecoveryCode, tek kullanimlik kurtarma kodu ve saklanacak ozetini uretir.
// Kod base32'dir: telefonda okunup elle yazilmasi gerektigi icin karisik
// karakterler barindirmayan bir alfabe tercih edildi.
func NewRecoveryCode() (code, salt, hash string, err error) {
	raw := make([]byte, 10)
	if _, err = rand.Read(raw); err != nil {
		return "", "", "", err
	}
	saltRaw := make([]byte, 8)
	if _, err = rand.Read(saltRaw); err != nil {
		return "", "", "", err
	}
	code = base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)
	salt = hex.EncodeToString(saltRaw)
	return code, salt, HashRecovery(salt, code), nil
}

// Attempt, girilen kodu degerlendirir ve durumu diske isler.
func (v Verifier) Attempt(code string) (Outcome, error) {
	now := v.now()
	st, err := store.LoadState(v.Dir)
	if err != nil {
		return Outcome{}, err
	}

	if st.LockUntil != nil && now.Before(*st.LockUntil) {
		left := st.LockUntil.Sub(now).Round(time.Minute)
		return Outcome{
			Message:     fmt.Sprintf("Çok fazla hatalı deneme. %d dakika sonra tekrar deneyin.", int(left.Minutes())),
			LockedUntil: *st.LockUntil,
		}, nil
	}

	if ok := v.tryRecovery(st, code); ok {
		st.RecoveryUsed = true
		return v.succeed(st, now, "", "recovery", "Kurtarma kodu kabul edildi. Oyunu şimdi başlatabilirsiniz.")
	}

	// Anahtarlarin tamami denenir, ilk eslesmede cikilmaz: bir anahtar
	// "kullanilmis kod" derken bir baskasi ayni kodu kabul edebilir ve
	// erken cikis kullaniciya yanlis gerekce gosterirdi.
	replay := false
	for _, k := range v.Keys {
		counter, res := totp.Verify(k.Secret, code, now, st.Counter(k.ID))
		switch res {
		case totp.ResultOK:
			st.SetCounter(k.ID, counter)
			return v.succeed(st, now, k.ID, "totp", "Kod kabul edildi. Oyunu şimdi başlatabilirsiniz.")
		case totp.ResultReplay:
			replay = true
		}
	}
	if replay {
		return v.fail(st, now, "Bu kod daha önce kullanılmış. Arkadaşınızdan yeni kod isteyin.")
	}
	return v.fail(st, now, "Kod hatalı.")
}

func (v Verifier) tryRecovery(st *store.State, code string) bool {
	if st.RecoveryUsed || st.RecoveryHash == "" {
		return false
	}
	want := HashRecovery(st.RecoverySalt, code)
	return subtle.ConstantTimeCompare([]byte(want), []byte(st.RecoveryHash)) == 1
}

func (v Verifier) succeed(st *store.State, now time.Time, who, method, msg string) (Outcome, error) {
	st.FailCount = 0
	st.LockUntil = nil
	session.Open(st, now, who)
	if err := store.SaveState(v.Dir, st); err != nil {
		return Outcome{}, err
	}
	if err := store.Append(v.Dir, store.Event{TS: now, Ev: "unlock", Method: method, Who: who}); err != nil {
		return Outcome{}, err
	}
	return Outcome{OK: true, Message: msg, Remaining: attemptsPerLevel, Who: who}, nil
}

func (v Verifier) fail(st *store.State, now time.Time, msg string) (Outcome, error) {
	st.FailCount++
	out := Outcome{Message: msg}

	if d := totp.LockDuration(st.FailCount); d > 0 && st.FailCount%attemptsPerLevel == 0 {
		until := now.Add(d)
		st.LockUntil = &until
		out.LockedUntil = until
		out.Message = fmt.Sprintf("%s Çok fazla hatalı deneme, %d dakika kilitlendi.", msg, int(d.Minutes()))
	}
	out.Remaining = attemptsPerLevel - (st.FailCount % attemptsPerLevel)

	if err := store.SaveState(v.Dir, st); err != nil {
		return Outcome{}, err
	}
	if err := store.Append(v.Dir, store.Event{TS: now, Ev: "unlock_fail", Fails: st.FailCount}); err != nil {
		return Outcome{}, err
	}
	return out, nil
}
