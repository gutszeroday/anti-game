//go:build windows

package ui

import (
	"image"
	"unsafe"
)

// QR kodunu pencerede cizmek. Metin sihirbazi QR'i tarayicida aciyordu;
// pencerede buna gerek yok ve gerekmemesi iyi: eslestirme sirasinda
// diske gecici bir HTML dosyasi yazilmiyor.

type bitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

const (
	biRGB        = 0
	dibRGBColors = 0
	srcCopy      = 0x00CC0020
)

// dibFromImage, gorseli Windows'un bekledigi ham piksel dizisine cevirir:
// piksel basina 4 bayt, BGRA sirasiyla, satirlar alttan yukari.
//
// Alttan yukari olmasi DIB'in varsayilani; ustten asagi icin yukseklik
// negatif verilir ama bazi surucular bunu StretchDIBits'te farkli ele
// aliyor, o yuzden standart yol seciliyor.
func dibFromImage(img image.Image) ([]byte, int32, int32) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	buf := make([]byte, w*h*4)

	for y := range h {
		// Kaynagin son satiri hedefin ilk satiri olur.
		src := b.Min.Y + h - 1 - y
		row := buf[y*w*4:]
		for x := range w {
			r, g, bl, a := img.At(b.Min.X+x, src).RGBA()
			p := row[x*4:]
			p[0] = byte(bl >> 8)
			p[1] = byte(g >> 8)
			p[2] = byte(r >> 8)
			p[3] = byte(a >> 8)
		}
	}
	return buf, int32(w), int32(h)
}

// drawImage, gorseli verilen dikdortgene cizer.
func drawImage(hdc uintptr, img image.Image, r Rect) {
	buf, w, h := dibFromImage(img)
	if len(buf) == 0 {
		return
	}
	bi := bitmapInfoHeader{
		Width:       w,
		Height:      h,
		Planes:      1,
		BitCount:    32,
		Compression: biRGB,
	}
	bi.Size = uint32(unsafe.Sizeof(bi))

	procStretchDIBits.Call(hdc,
		uintptr(r.X), uintptr(r.Y), uintptr(r.W), uintptr(r.H),
		0, 0, uintptr(w), uintptr(h),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&bi)),
		dibRGBColors, srcCopy)
}
