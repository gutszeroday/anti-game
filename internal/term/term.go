// Package term, konsol ciktisini renklendirir.
//
// Renk karari tek yerden verilir: terminal desteklemiyorsa, cikti bir
// boruya gidiyorsa ya da NO_COLOR tanimliysa tum bicimlendirme sessizce
// devre disi kalir. Boylece testler bytes.Buffer'a yazip duz metin okur.
package term

import (
	"io"
	"os"
	"strings"
)

// Varsayilan genislik: kapi ve kisi ekranlarindaki cizgiler bu genisligi
// kullanir. 52, dar bir konsol penceresinde bile tasmaz.
const Width = 52

const (
	reset    = "\x1b[0m"
	codeBold = "\x1b[1;36m" // baslik
	codeKey  = "\x1b[1;33m" // menu tusu
	codeGood = "\x1b[32m"
	codeWarn = "\x1b[33m"
	codeBad  = "\x1b[31m"
	codeDim  = "\x1b[90m"
)

// Theme, bir cikti akisinin bicimlendirme yetenegini tutar.
type Theme struct {
	color   bool
	unicode bool
}

// New, akisi inceleyip uygun temayi dondurur. Renk kapaliysa tum
// bicimlendirme metni oldugu gibi birakir.
func New(w io.Writer) *Theme {
	if os.Getenv("NO_COLOR") != "" {
		return &Theme{}
	}
	f, ok := w.(*os.File)
	if !ok {
		return &Theme{}
	}
	// Cikti dosyaya ya da boruya yonlendirilmisse kacis dizileri metnin
	// icinde kalir; kimse okuyamaz.
	st, err := f.Stat()
	if err != nil || st.Mode()&os.ModeCharDevice == 0 {
		return &Theme{}
	}
	color, unicode := prepare(f)
	return &Theme{color: color, unicode: unicode}
}

// Plain, hicbir bicimlendirme yapmayan temadir. Testler icin.
func Plain() *Theme { return &Theme{} }

// Color, renk acik mi soyler.
func (t *Theme) Color() bool { return t.color }

func (t *Theme) wrap(code, s string) string {
	if !t.color || s == "" {
		return s
	}
	return code + s + reset
}

func (t *Theme) Title(s string) string { return t.wrap(codeBold, s) }
func (t *Theme) Key(s string) string   { return t.wrap(codeKey, s) }
func (t *Theme) Good(s string) string  { return t.wrap(codeGood, s) }
func (t *Theme) Warn(s string) string  { return t.wrap(codeWarn, s) }
func (t *Theme) Bad(s string) string   { return t.wrap(codeBad, s) }
func (t *Theme) Dim(s string) string   { return t.wrap(codeDim, s) }

// Rule, yatay cizgi dondurur. Konsol kod sayfasi UTF-8'e alinamadiysa
// cizgi karakteri yerine tire kullanilir; yoksa ekranda soru isareti
// yigini cikar.
func (t *Theme) Rule(width int) string {
	ch := "-"
	if t.unicode {
		ch = "─"
	}
	return t.Dim(strings.Repeat(ch, width))
}

// Banner, ust basligi cizgiler arasinda dondurur.
func (t *Theme) Banner(title string) string {
	rule := t.Rule(Width)
	return rule + "\n " + t.Title(title) + "\n" + rule + "\n"
}

// Clear, ekrani temizler. Renk kapaliyken kacis dizisi basmak yerine bos
// satir birakilir: menu her cizimde alt alta birikiyordu.
func (t *Theme) Clear() string {
	if !t.color {
		return "\n"
	}
	return "\x1b[2J\x1b[H"
}

// Mark, listelerde durum isaretidir: ok/uyari.
func (t *Theme) Mark(ok bool) string {
	if ok {
		if t.unicode {
			return t.Good("✓")
		}
		return t.Good("+")
	}
	if t.unicode {
		return t.Warn("!")
	}
	return t.Warn("!")
}
