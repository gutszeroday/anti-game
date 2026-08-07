//go:build windows

package ui

import (
	"image"
	"image/color"
	"testing"
)

func TestDIBHasFourBytesPerPixel(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 3, 2))
	buf, w, h := dibFromImage(img)
	if w != 3 || h != 2 {
		t.Fatalf("boyut %dx%d, istenen 3x2", w, h)
	}
	if len(buf) != 3*2*4 {
		t.Errorf("%d bayt, istenen 24", len(buf))
	}
}

func TestDIBUsesBGRAOrder(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 0x10, G: 0x20, B: 0x30, A: 0xff})
	buf, _, _ := dibFromImage(img)
	if buf[0] != 0x30 || buf[1] != 0x20 || buf[2] != 0x10 {
		t.Errorf("BGRA sirasi yanlis: %v", buf[:3])
	}
}

func TestDIBIsBottomUp(t *testing.T) {
	// Windows DIB'i alttan yukari bekler: gorselin son satiri once gelir.
	img := image.NewRGBA(image.Rect(0, 0, 1, 2))
	img.Set(0, 0, color.RGBA{R: 0xff, A: 0xff}) // ust satir kirmizi
	img.Set(0, 1, color.RGBA{B: 0xff, A: 0xff}) // alt satir mavi
	buf, _, _ := dibFromImage(img)
	if buf[0] != 0xff || buf[2] != 0x00 {
		t.Errorf("ilk satir alt satir olmali (mavi bekleniyordu): %v", buf[:4])
	}
	if buf[6] != 0xff {
		t.Errorf("ikinci satir ust satir olmali (kirmizi bekleniyordu): %v", buf[4:8])
	}
}

func TestDIBHandlesNonZeroOrigin(t *testing.T) {
	// Alt gorseller sifirdan baslamayan sinirlarla gelir; ofset
	// gozardi edilirse cizim kayar.
	img := image.NewRGBA(image.Rect(5, 7, 8, 9))
	img.Set(5, 7, color.RGBA{G: 0xff, A: 0xff})
	buf, w, h := dibFromImage(img)
	if w != 3 || h != 2 {
		t.Fatalf("boyut %dx%d, istenen 3x2", w, h)
	}
	if len(buf) != 24 {
		t.Errorf("%d bayt, istenen 24", len(buf))
	}
	// (5,7) ust sol; alttan yukari dizilimde ikinci satirin ilk pikseli.
	if buf[13] != 0xff {
		t.Errorf("ofsetli gorsel yanlis okundu: %v", buf[12:16])
	}
}
