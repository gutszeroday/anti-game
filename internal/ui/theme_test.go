//go:build windows

package ui

import "testing"

func TestButtonColorsPrimaryDefault(t *testing.T) {
	c := buttonColors(variantPrimary, false, false, false)
	if c.Bg != clrInteractive {
		t.Errorf("primary zemin = %#x, istenen %#x", c.Bg, clrInteractive)
	}
	if c.Text != clrOnColor {
		t.Errorf("primary metin = %#x, istenen %#x", c.Text, clrOnColor)
	}
}

func TestButtonColorsPrimaryHoverAndPressed(t *testing.T) {
	if c := buttonColors(variantPrimary, true, false, false); c.Bg != clrHoverPrimary {
		t.Errorf("primary hover zemin = %#x, istenen %#x", c.Bg, clrHoverPrimary)
	}
	if c := buttonColors(variantPrimary, false, true, false); c.Bg != clrActivePrimary {
		t.Errorf("primary basılı zemin = %#x, istenen %#x", c.Bg, clrActivePrimary)
	}
}

func TestButtonColorsSecondaryAndDanger(t *testing.T) {
	if c := buttonColors(variantSecondary, false, false, false); c.Bg != clrSecondaryBg {
		t.Errorf("secondary zemin = %#x, istenen %#x", c.Bg, clrSecondaryBg)
	}
	if c := buttonColors(variantDanger, false, false, false); c.Bg != clrDangerBg {
		t.Errorf("danger zemin = %#x, istenen %#x", c.Bg, clrDangerBg)
	}
}

func TestButtonColorsDisabledOverridesVariant(t *testing.T) {
	for _, v := range []buttonVariant{variantPrimary, variantSecondary, variantDanger} {
		c := buttonColors(v, false, false, true)
		if c.Bg != clrDisabledBg || c.Text != clrDisabledText {
			t.Errorf("variant=%d disabled renkleri yanlış: %+v", v, c)
		}
	}
}

func TestRGBPacksLittleEndian(t *testing.T) {
	// COLORREF = 0x00BBGGRR; RGB(0x0F,0x62,0xFE) Carbon interactive-mavisi.
	if got := rgb(0x0F, 0x62, 0xFE); got != 0x00FE620F {
		t.Errorf("rgb(0x0F,0x62,0xFE) = %#x, istenen %#x", got, 0x00FE620F)
	}
}
