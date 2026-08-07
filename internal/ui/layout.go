//go:build windows

// Package ui, antigame'in ana penceresini ve diyaloglarini ham Win32 ile
// cizer. gate ve tray ile ayni gerekce: GUI kutuphanesi gomulmedi, cunku
// tek dosya tasinabilirligi ve dusuk bellek bu projenin sartlari.
//
// Bu paket karar vermez. Her secim cekirdek paketlere (config, people,
// gamelist, pairing, task, status, report, uninstall) aittir; buradaki kod
// yalnizca gorsel kabuktur.
package ui

// Rect, bir kontrolun istemci alanindaki yeridir. Win32'nin RECT'inden
// farkli olarak sag/alt yerine genislik/yukseklik tutuyor; MoveWindow ve
// CreateWindowEx ikisini de bu bicimde istiyor.
type Rect struct{ X, Y, W, H int32 }

// Ana pencerenin en kucuk boyutu (96 DPI'da). Bundan kucugunde alt
// dugmeler ust uste biner.
const (
	MinW int32 = 560
	MinH int32 = 440
)

// 96 DPI'daki temel olculer. Hepsi Scale'den gecirilerek kullanilir.
const (
	pad        int32 = 12
	gap        int32 = 8
	lineH      int32 = 18
	statusLine int32 = 4 // durum blogundaki satir sayisi
	btnW       int32 = 118
	btnH       int32 = 28
	smallBtnW  int32 = 84
	labelH     int32 = 18
	checkH     int32 = 22
	noteH      int32 = 34
)

// Scale, 96 DPI icin yazilmis bir olcuyu hedef DPI'ya cevirir.
// GetDpiForWindow basarisiz oldugunda 0 dondurdugu icin sifir 96 sayilir.
func Scale(dpi uint32, v int32) int32 {
	if dpi == 0 {
		dpi = 96
	}
	return v * int32(dpi) / 96
}

// MainLayout, ana penceredeki her kontrolun yeridir.
type MainLayout struct {
	Status       Rect
	GamesLabel   Rect
	Games        Rect
	AddBtn       Rect
	RemoveBtn    Rect
	AutoStart    Rect
	Note         Rect
	WatchBtn     Rect
	ReportBtn    Rect
	PeopleBtn    Rect
	RemoveAppBtn Rect
}

// Main, verilen istemci alani icin yerlesimi hesaplar.
//
// Alt dugmeler yukseklikten geriye dogru yerlestiriliyor: pencere
// buyudugunde bosluk listeye gitsin, dugmeler altta kalsin.
func Main(w, h int32, dpi uint32) MainLayout {
	s := func(v int32) int32 { return Scale(dpi, v) }
	p, g := s(pad), s(gap)
	left, right := p, w-p
	width := right - left

	var l MainLayout

	l.Status = Rect{left, p, width, s(lineH)*statusLine + s(4)}

	y := l.Status.Y + l.Status.H + g
	l.GamesLabel = Rect{left, y, width, s(labelH)}

	// Alt siradan geriye dogru: dugme sirasi, not, baslangic kutusu.
	bh, bw := s(btnH), s(btnW)
	btnY := h - p - bh
	for i, r := range []*Rect{&l.WatchBtn, &l.ReportBtn, &l.PeopleBtn, &l.RemoveAppBtn} {
		*r = Rect{left + int32(i)*(bw+g), btnY, bw, bh}
	}

	l.Note = Rect{left, btnY - g - s(noteH), width, s(noteH)}
	l.AutoStart = Rect{left, l.Note.Y - g - s(checkH), width, s(checkH)}

	// Ekle/Cikar listenin hemen altinda, saga yasli.
	sbw := s(smallBtnW)
	smallY := l.AutoStart.Y - g - bh
	l.RemoveBtn = Rect{right - sbw, smallY, sbw, bh}
	l.AddBtn = Rect{l.RemoveBtn.X - g - sbw, smallY, sbw, bh}

	// Liste kalan boslugu alir.
	listY := l.GamesLabel.Y + l.GamesLabel.H + s(4)
	listH := smallY - g - listY
	if listH < s(60) {
		listH = s(60)
	}
	l.Games = Rect{left, listY, width, listH}

	return l
}
