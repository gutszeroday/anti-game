//go:build windows

package ui

// theme.go, Carbon Design System'in (Gray 10 tema) renk paletini ve buton
// cizim mantigini tasir. Butun ownder-draw/renk kararlari burada toplaniyor
// ki window.go ve modal.go yalnizca cagirsin, ayni renk mantigi iki yerde
// yasamasin.

// rgb, Win32'nin COLORREF'ini (0x00BBGGRR) r/g/b bayttan uretir. Elle hex
// yazmak yerine bu fonksiyon kullaniliyor: BGR/RGB karisikligi sessiz bir
// renk hatasina yol acardi.
func rgb(r, g, b byte) uint32 {
	return uint32(r) | uint32(g)<<8 | uint32(b)<<16
}

// Carbon Gray 10 tema tokenleri. Sabit tema — sistem acik/koyu ayarina gore
// degismiyor (spec: basitlik icin tek tema).
var (
	clrBackground    = rgb(0xF4, 0xF4, 0xF4)
	clrLayer01       = rgb(0xFF, 0xFF, 0xFF)
	clrTextPrimary   = rgb(0x16, 0x16, 0x16)
	clrTextSecondary = rgb(0x52, 0x52, 0x52)
	clrBorderSubtle  = rgb(0xE0, 0xE0, 0xE0)
	clrBorderStrong  = rgb(0x8D, 0x8D, 0x8D)
	clrInteractive   = rgb(0x0F, 0x62, 0xFE)
	clrHoverPrimary  = rgb(0x03, 0x53, 0xE9)
	clrActivePrimary = rgb(0x00, 0x2D, 0x9C)
	clrSecondaryBg   = rgb(0x39, 0x39, 0x39)
	clrHoverSecondary = rgb(0x4C, 0x4C, 0x4C)
	clrDangerBg      = rgb(0xDA, 0x1E, 0x28)
	clrHoverDanger   = rgb(0xBA, 0x1B, 0x23)
	clrDisabledBg    = rgb(0xC6, 0xC6, 0xC6)
	clrDisabledText  = rgb(0x8D, 0x8D, 0x8D)
	clrSelectedRow   = rgb(0xE0, 0xE7, 0xFF)
	clrOnColor       = rgb(0xFF, 0xFF, 0xFF)
)

// buttonVariant, butonun Carbon hiyerarsisindeki rolu. Her diyalogda en
// fazla bir primary olur (varsayilan/ana eylem); geri donusu olmayan
// eylemler (sil/kaldir) danger; gerisi secondary.
type buttonVariant int

const (
	variantSecondary buttonVariant = iota
	variantPrimary
	variantDanger
)

// buttonColorSet, bir butonun tek bir durumdaki (hover/basili/disabled)
// zemin, metin ve odak cercevesi rengidir.
type buttonColorSet struct {
	Bg, Text, Focus uint32
}

// buttonColors, varyant ve durumdan renk uretir. Saf fonksiyon — Win32
// cagrisi yok, dogrudan test edilebilir.
func buttonColors(v buttonVariant, hover, pressed, disabled bool) buttonColorSet {
	if disabled {
		return buttonColorSet{Bg: clrDisabledBg, Text: clrDisabledText, Focus: clrDisabledBg}
	}
	switch v {
	case variantPrimary:
		bg := clrInteractive
		switch {
		case pressed:
			bg = clrActivePrimary
		case hover:
			bg = clrHoverPrimary
		}
		return buttonColorSet{Bg: bg, Text: clrOnColor, Focus: clrInteractive}
	case variantDanger:
		bg := clrDangerBg
		if hover || pressed {
			bg = clrHoverDanger
		}
		return buttonColorSet{Bg: bg, Text: clrOnColor, Focus: clrInteractive}
	default:
		bg := clrSecondaryBg
		if hover || pressed {
			bg = clrHoverSecondary
		}
		return buttonColorSet{Bg: bg, Text: clrOnColor, Focus: clrInteractive}
	}
}
