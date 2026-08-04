// Package totp, RFC 6238 zaman tabanli tek kullanimlik sifre dogrulamasi
// yapar ve buna iki ek koruma getirir: kabul edilmis bir sayacin tekrar
// kullanilamamasi ve kademeli kilitlenme.
package totp

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/binary"
	"fmt"
	"strings"
	"time"
)

// StepSeconds, RFC 6238 varsayilan adim suresidir.
const StepSeconds = 30

// Digits, uretilen kodun hane sayisidir.
const Digits = 6

type Result int

const (
	ResultOK Result = iota
	ResultBadCode
	ResultReplay
)

func (r Result) String() string {
	switch r {
	case ResultOK:
		return "ok"
	case ResultReplay:
		return "replay"
	default:
		return "bad_code"
	}
}

// Counter, verilen zamana karsilik gelen TOTP adim sayacini dondurur.
func Counter(t time.Time) uint64 {
	return uint64(t.UTC().Unix()) / StepSeconds
}

// Code, verilen sayac icin 6 haneli kodu uretir.
func Code(secret []byte, counter uint64) string {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], counter)

	mac := hmac.New(sha1.New, secret)
	mac.Write(buf[:])
	sum := mac.Sum(nil)

	// RFC 4226 dinamik kirpma.
	offset := sum[len(sum)-1] & 0x0f
	value := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff

	mod := uint32(1)
	for range Digits {
		mod *= 10
	}
	return fmt.Sprintf("%0*d", Digits, value%mod)
}

// Verify, kodu +-1 adim toleransiyla dogrular.
//
// Kabul edilen kodun sayaci last'tan buyuk olmak zorundadir. Bu tek kural
// hem ayni kodun 30 saniye icinde ikinci kez kullanilmasini hem de sistem
// saati geri alinarak eski bir kodun tekrar girilmesini engeller.
func Verify(secret []byte, code string, now time.Time, last uint64) (uint64, Result) {
	code = strings.TrimSpace(code)
	if len(code) != Digits {
		return 0, ResultBadCode
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			return 0, ResultBadCode
		}
	}

	base := Counter(now)
	for delta := -1; delta <= 1; delta++ {
		if delta < 0 && base == 0 {
			continue
		}
		c := uint64(int64(base) + int64(delta))
		if subtle.ConstantTimeCompare([]byte(Code(secret, c)), []byte(code)) != 1 {
			continue
		}
		if c <= last {
			return c, ResultReplay
		}
		return c, ResultOK
	}
	return 0, ResultBadCode
}

// skewSearchSteps, teshis aramasinin kac adim geriye ve ileriye bakacagini
// belirler: 20 adim = +-10 dakika, tipik telefon saat kaymasini kapsar.
const skewSearchSteps = 20

// FindSkew, kodun hangi zaman kaymasiyla uretildigini bulur. Yalnizca
// teshis icindir: kabul karari Verify'de kalir ve orasi +-1 adimla sinirlidir.
// Kod dogru anahtardan uretilmisse buradan donen sure, dogrulayan makine ile
// kod ureten cihaz arasindaki saat farkidir.
func FindSkew(secret []byte, code string, now time.Time) (time.Duration, bool) {
	code = strings.TrimSpace(code)
	base := Counter(now)
	for delta := -skewSearchSteps; delta <= skewSearchSteps; delta++ {
		c := int64(base) + int64(delta)
		if c < 0 {
			continue
		}
		if Code(secret, uint64(c)) == code {
			return time.Duration(delta) * StepSeconds * time.Second, true
		}
	}
	return 0, false
}

// LockDuration, toplam hatali deneme sayisina gore kilit suresini dondurur.
// Her 5 hatada bir kademe yukselir: 15 dk, 30 dk, sonra 60 dk sabit.
func LockDuration(fails int) time.Duration {
	level := fails / 5
	switch {
	case level <= 0:
		return 0
	case level == 1:
		return 15 * time.Minute
	case level == 2:
		return 30 * time.Minute
	default:
		return 60 * time.Minute
	}
}
