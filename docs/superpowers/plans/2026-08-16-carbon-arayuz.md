# Carbon Arayüz Yenileme Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `internal/ui` altındaki ham Win32 arayüzünü (ana pencere + tüm diyaloglar) IBM Carbon Design System'in (Gray 10 tema) renk/tipografi/bileşen diline göre yeniden çizmek — davranış değişmeden, yeni bağımlılık eklenmeden.

**Architecture:** Mevcut Win32 kontrolleri (BUTTON, EDIT, SysListView32) korunuyor; görsel katman owner-draw (`WM_DRAWITEM`), subclassing (`SetWindowSubclass`) ve `WM_CTLCOLOR*`/`WM_ERASEBKGND` mesajlarıyla elle çiziliyor. Tüm renk/font/çizim mantığı yeni `internal/ui/theme.go` dosyasında toplanıyor; diğer dosyalar (`window.go`, `modal.go`, dialoglar) yalnızca yeni yardımcı fonksiyonları çağıracak şekilde güncelleniyor.

**Tech Stack:** Go 1.26.4, `golang.org/x/sys/windows` (mevcut, yeni bağımlılık yok), ham Win32 API (user32/gdi32/comctl32).

**Spec:** `docs/superpowers/specs/2026-08-16-carbon-arayuz-design.md`

## Global Constraints

- Go 1.26.4, modül `github.com/guts/antigame` — `go.mod` değişmiyor, yeni bağımlılık yok.
- Tüm `internal/ui` dosyaları `//go:build windows` taşıyor — bu paket yalnızca Windows'ta derlenir/test edilir.
- Mevcut desen korunuyor: Win32 proc'ları `windows.NewLazySystemDLL(...).NewProc(...)` ile paket düzeyinde tanımlanıyor; pencere/kontrol durumu `map[uintptr]*T` ile HWND'ye eşleniyor (tek thread, kilit yok — mevcut `modals` haritasıyla aynı desen).
- Kullanıcıya görünen Türkçe metinler değişmiyor — yalnızca görsel katman.
- Davranış regresyonu yok: Tab gezinme, Enter ile varsayılan buton, Esc ile iptal, DPI ölçekleme, tüm mevcut testler (`layout_test.go`, `rows_test.go`, `move_test.go`, `qr_test.go`) aynen geçmeli.
- Sadeleştirme kararları (spec'ten bilinçli sapmalar) her ilgili görevde açıkça not ediliyor, gizlenmiyor.

---

### Task 1: Carbon renk paleti ve buton renk mantığı

**Files:**
- Create: `internal/ui/theme.go`
- Test: `internal/ui/theme_test.go`

**Interfaces:**
- Produces: `rgb(r, g, b byte) uint32`, renk sabitleri (`clrBackground`, `clrLayer01`, `clrTextPrimary`, `clrTextSecondary`, `clrBorderSubtle`, `clrBorderStrong`, `clrInteractive`, `clrHoverPrimary`, `clrActivePrimary`, `clrSecondaryBg`, `clrHoverSecondary`, `clrDangerBg`, `clrHoverDanger`, `clrDisabledBg`, `clrDisabledText`, `clrSelectedRow`, `clrOnColor`), `type buttonVariant int` (`variantSecondary`, `variantPrimary`, `variantDanger`), `type buttonColorSet struct{ Bg, Text, Focus uint32 }`, `func buttonColors(v buttonVariant, hover, pressed, disabled bool) buttonColorSet`. Sonraki görevler bu isimleri aynen kullanıyor.

Bu görev saf Go — hiçbir Win32 çağrısı yok, tam test edilebilir.

- [ ] **Step 1: Başarısız testi yaz**

`internal/ui/theme_test.go`:
```go
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
```

- [ ] **Step 2: Test derlenmediğini doğrula**

Çalıştır: `go test ./internal/ui/... -run TestButtonColors -v`
Beklenen: derleme hatası — `buttonColors`, `rgb`, `clrInteractive` vb. tanımsız.

- [ ] **Step 3: `theme.go`'yu yaz**

```go
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
```

- [ ] **Step 4: Testleri çalıştır ve geçtiğini doğrula**

Çalıştır: `go test ./internal/ui/... -run TestButtonColors -v` ve `go test ./internal/ui/... -run TestRGB -v`
Beklenen: tümü PASS.

- [ ] **Step 5: Tüm paketin hâlâ derlendiğini doğrula**

Çalıştır: `go build ./...`
Beklenen: hatasız.

- [ ] **Step 6: Commit**

```bash
git add internal/ui/theme.go internal/ui/theme_test.go
git commit -m "feat(ui): Carbon Gray 10 renk paleti ve buton renk mantığı"
```

---

### Task 2: Owner-draw buton sistemi ve ana pencere entegrasyonu

Font notu (spec'ten bilinçli sapma): spec dört tipografi rolü (başlık/gövde/buton/not) tanımlıyordu, ama kod tabanında yalnızca STATIC etiketler (gövde, zaten mevcut `uiFont`) ve butonlar (yeni, semibold gerekiyor) var — diyalog başlıkları OS'in pencere başlığı çubuğunda, ayrıca çizilmiyor. Kullanılmayan bir "başlık fontu" eklemek YAGNI ihlali olurdu; yalnızca **semibold** (buton metni için) ekleniyor, gövde/not `uiFont`'u aynen kullanmaya devam ediyor.

**Files:**
- Modify: `internal/ui/theme.go`
- Modify: `internal/ui/win.go`
- Modify: `internal/ui/window.go`

**Interfaces:**
- Consumes: `buttonColors` (Task 1), `create(class, text string, style, exStyle uint32, r Rect, parent uintptr, id int, font uintptr) uintptr`, `setFont`, `textOf`, `dpiOf`, `osPointer`, `winRect`, `Rect`, `fontCache` deseni (win.go).
- Produces: `func semiboldFont(dpi uint32) uintptr`, `func createButton(parent uintptr, text string, r Rect, id int, variant buttonVariant) uintptr`, `func drawButton(dis *drawItemStruct)`. Task 3 bu üçünü aynen kullanıyor.

- [ ] **Step 1: `win.go`'ya yeni Win32 proc/sabit/struct'ları ekle**

`internal/ui/win.go` içindeki `var (...)` proc bloğuna ekle:
```go
	procCreateSolidBrush     = gdi32.NewProc("CreateSolidBrush")
	procDeleteObject         = gdi32.NewProc("DeleteObject")
	procCreatePen            = gdi32.NewProc("CreatePen")
	procSelectObject         = gdi32.NewProc("SelectObject")
	procRectangle            = gdi32.NewProc("Rectangle")
	procSetBkMode            = gdi32.NewProc("SetBkMode")
	procSetTextColor         = gdi32.NewProc("SetTextColor")
	procSetBkColor           = gdi32.NewProc("SetBkColor")
	procGetStockObject       = gdi32.NewProc("GetStockObject")

	procFillRect             = user32.NewProc("FillRect")
	procDrawText             = user32.NewProc("DrawTextW")
	procInvalidateRect       = user32.NewProc("InvalidateRect")
	procTrackMouseEvent      = user32.NewProc("TrackMouseEvent")
	procGetWindowDC          = user32.NewProc("GetWindowDC")
	procReleaseDC            = user32.NewProc("ReleaseDC")
	procRedrawWindow         = user32.NewProc("RedrawWindow")

	procSetWindowSubclass    = comctl32.NewProc("SetWindowSubclass")
	procDefSubclassProc      = comctl32.NewProc("DefSubclassProc")
	procRemoveWindowSubclass = comctl32.NewProc("RemoveWindowSubclass")
```

Stil/mesaj sabitlerinin bulunduğu bloklara ekle:
```go
	bsOwnerDraw = 0x0000000B
```
```go
	wmDrawItem      = 0x002B
	wmEraseBkgnd    = 0x0014
	wmCtlColorStatic = 0x0138
	wmCtlColorEdit  = 0x0133
	wmMouseMove     = 0x0200
	wmMouseLeave    = 0x02A3
	wmLButtonDown   = 0x0201
	wmLButtonUp     = 0x0202
	wmSetFocus      = 0x0007
	wmKillFocus     = 0x0008
	wmNcPaint       = 0x0085
	wmNcDestroy     = 0x0082
	wmKeyDown       = 0x0100

	vkReturn = 0x0D
```
Yeni bir sabit blok ekle (owner-draw ve GDI için):
```go
const (
	odtButton   = 4
	odsFocus    = 0x0010
	odsDisabled = 0x0004

	psSolid        = 0
	stockNullBrush = 5

	transparentBkMode = 1

	dtCenter     = 0x0001
	dtVcenter    = 0x0004
	dtSingleLine = 0x0020

	tmeLeave = 0x00000002

	rdwInvalidate = 0x0001
	rdwFrame      = 0x0400
	rdwUpdateNow  = 0x0100
)
```
Struct'ların bulunduğu yere ekle (DRAWITEMSTRUCT ve TRACKMOUSEEVENT):
```go
// drawItemStruct, Win32'nin DRAWITEMSTRUCT'idir. Alan sirasi ve tipleri
// birebir eslesiyor; Go derleyicisi C ile ayni hizalamayi uretiyor (bu
// dosyadaki digenr Win32 struct'larla ayni desen, ornek: paintStruct).
type drawItemStruct struct {
	CtlType    uint32
	CtlID      uint32
	ItemID     uint32
	ItemAction uint32
	ItemState  uint32
	HwndItem   uintptr
	Hdc        uintptr
	RcItem     winRect
	ItemData   uintptr
}

type trackMouseEvent struct {
	Size      uint32
	Flags     uint32
	HwndTrack uintptr
	HoverTime uint32
}
```

- [ ] **Step 2: `theme.go`'ya semibold font önbelleğini ekle**

`fontCache` (win.go) desenini aynen izleyen ayrı bir önbellek — büyük font paylaşımı DPI degisince bozulmasin diye ayni kilit kullanilmiyor, kendi kilidini tasiyor:
```go
var (
	semiboldMu    sync.Mutex
	semiboldCache = map[uint32]uintptr{}
)

// semiboldFont, buton metni icin Carbon'un "buton" tipografi rolune
// (14px semibold) yaklasan bir Segoe UI fontu doner. IBM Plex Sans
// gomulmuyor (spec: Windows'ta hazir yuklu degil, basitlik icin Segoe UI
// ile yaklasiliyor).
func semiboldFont(dpi uint32) uintptr {
	semiboldMu.Lock()
	defer semiboldMu.Unlock()
	if f, ok := semiboldCache[dpi]; ok {
		return f
	}
	lf := logFont{
		Height: -Scale(dpi, 14),
		Weight: 600, // FW_SEMIBOLD
		CharSet: 1,  // DEFAULT_CHARSET
	}
	copy(lf.FaceName[:], windows.StringToUTF16("Segoe UI"))
	f, _, _ := procCreateFontIndirect.Call(uintptr(unsafe.Pointer(&lf)))
	semiboldCache[dpi] = f
	return f
}
```
`theme.go`'nun importlarına `sync`, `unsafe` ve `golang.org/x/sys/windows` eklenmesi gerekiyor.

- [ ] **Step 3: Buton durum takibi ve owner-draw çizimi**

`theme.go`'ya ekle:
```go
// buttonState, tek bir owner-draw butonun fare durumudur (hover/basili).
// HWND'ye eslenir; WM_DRAWITEM ve subclass mesajlari ayni haritayi okur.
type buttonState struct{ hover, pressed bool }

var (
	buttonVariants = map[uintptr]buttonVariant{}
	buttonStates   = map[uintptr]*buttonState{}
)

// buttonSubclassID, butonlara takilan tek subclass'in kimligidir.
const buttonSubclassID = 1

// buttonSubclassCB, subclass geri cagrimi process basina bir kez
// olusturulur. windows.NewCallback her cagrildiginda yeni bir trampoline
// uretir; her butonda ayri cagirmak SetWindowSubclass/RemoveWindowSubclass
// eslesmesini bozardi.
var buttonSubclassCB = windows.NewCallback(buttonSubclassProc)

// createButton, Carbon uslubunda owner-draw bir buton olusturur. Cizim
// WM_DRAWITEM ile ust pencereye (parent) geliyor; hover/basili durumu
// butonun kendi WndProc'una subclass ile ekleniyor.
func createButton(parent uintptr, text string, r Rect, id int, variant buttonVariant) uintptr {
	h := create("BUTTON", text, bsOwnerDraw|wsTabStop, 0, r, parent, id, 0)
	buttonVariants[h] = variant
	buttonStates[h] = &buttonState{}
	procSetWindowSubclass.Call(h, buttonSubclassCB, buttonSubclassID, 0)
	return h
}

// buttonSubclassProc, fare durumunu izler ve yeniden cizimi tetikler.
// Tiklama/klavye davranisi DefSubclassProc'a birakiliyor — owner-draw
// yalnizca cizimi degistirir, BUTTON sinifinin girdi isleyisini degil.
func buttonSubclassProc(hwnd, msg, wparam, lparam, idSubclass, refData uintptr) uintptr {
	switch msg {
	case wmMouseMove:
		if st := buttonStates[hwnd]; st != nil && !st.hover {
			st.hover = true
			tme := trackMouseEvent{Flags: tmeLeave, HwndTrack: hwnd}
			tme.Size = uint32(unsafe.Sizeof(tme))
			procTrackMouseEvent.Call(uintptr(unsafe.Pointer(&tme)))
			procInvalidateRect.Call(hwnd, 0, 1)
		}
	case wmMouseLeave:
		if st := buttonStates[hwnd]; st != nil {
			st.hover = false
			procInvalidateRect.Call(hwnd, 0, 1)
		}
	case wmLButtonDown:
		if st := buttonStates[hwnd]; st != nil {
			st.pressed = true
			procInvalidateRect.Call(hwnd, 0, 1)
		}
	case wmLButtonUp:
		if st := buttonStates[hwnd]; st != nil {
			st.pressed = false
			procInvalidateRect.Call(hwnd, 0, 1)
		}
	case wmNcDestroy:
		delete(buttonStates, hwnd)
		delete(buttonVariants, hwnd)
		procRemoveWindowSubclass.Call(hwnd, buttonSubclassCB, buttonSubclassID)
	}
	r, _, _ := procDefSubclassProc.Call(hwnd, msg, wparam, lparam)
	return r
}

// drawButton, WM_DRAWITEM ile gelen bir butonu Carbon stilinde cizer:
// duz kenarli dikdortgen zemin, ortali semibold metin, focus'ta 2px
// interactive cerceve.
func drawButton(dis *drawItemStruct) {
	variant := buttonVariants[dis.HwndItem]
	hover, pressed := false, false
	if st := buttonStates[dis.HwndItem]; st != nil {
		hover, pressed = st.hover, st.pressed
	}
	disabled := dis.ItemState&odsDisabled != 0
	colors := buttonColors(variant, hover, pressed, disabled)

	hdc, r := dis.Hdc, dis.RcItem

	bg, _, _ := procCreateSolidBrush.Call(uintptr(colors.Bg))
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&r)), bg)
	procDeleteObject.Call(bg)

	if dis.ItemState&odsFocus != 0 {
		pen, _, _ := procCreatePen.Call(psSolid, 2, uintptr(colors.Focus))
		oldPen, _, _ := procSelectObject.Call(hdc, pen)
		nullBrush, _, _ := procGetStockObject.Call(stockNullBrush)
		oldBrush, _, _ := procSelectObject.Call(hdc, nullBrush)
		procRectangle.Call(hdc, uintptr(r.Left+1), uintptr(r.Top+1), uintptr(r.Right-1), uintptr(r.Bottom-1))
		procSelectObject.Call(hdc, oldPen)
		procSelectObject.Call(hdc, oldBrush)
		procDeleteObject.Call(pen)
	}

	font := semiboldFont(dpiOf(dis.HwndItem))
	oldFont, _, _ := procSelectObject.Call(hdc, font)
	procSetBkMode.Call(hdc, transparentBkMode)
	procSetTextColor.Call(hdc, uintptr(colors.Text))
	text := textOf(dis.HwndItem)
	procDrawText.Call(hdc, uintptr(unsafe.Pointer(utf16(text))), ^uintptr(0),
		uintptr(unsafe.Pointer(&r)), dtCenter|dtVcenter|dtSingleLine)
	procSelectObject.Call(hdc, oldFont)
}
```

- [ ] **Step 4: Ana pencerede buton oluşturmayı `createButton`'a geçir**

`internal/ui/window.go`'daki `build()` fonksiyonunu değiştir:
```go
func (w *mainWindow) build() {
	z := Rect{}
	w.status = create("STATIC", "", ssLeft, 0, z, w.hwnd, 0, w.font)
	w.gamesLabel = create("STATIC", "Korunan oyunlar", ssLeft, 0, z, w.hwnd, 0, w.font)
	w.games = create("SysListView32", "",
		lvsReport|lvsSingleSel|lvsShowSelAlways|lvsNoSortHeader|wsTabStop|wsBorder,
		wsExClientEdge, z, w.hwnd, 0, w.font)
	lvSetColumns(w.games, gameColumnTitles, gameColumnWidths, w.dpi)

	w.addBtn = createButton(w.hwnd, "Ekle…", z, idAdd, variantSecondary)
	w.removeBtn = createButton(w.hwnd, "Çıkar", z, idRemoveGame, variantSecondary)
	w.autoStart = create("BUTTON", "Windows açılışında başlat",
		bsAutoCheckBox|wsTabStop|wsGroup, 0, z, w.hwnd, idAutoStart, w.font)
	w.note = create("STATIC", "", ssLeft, 0, z, w.hwnd, 0, w.font)

	w.watchBtn = createButton(w.hwnd, "İzleyiciyi başlat", z, idWatch, variantPrimary)
	w.reportBtn = createButton(w.hwnd, "Haftalık rapor", z, idReport, variantSecondary)
	w.codeBtn = createButton(w.hwnd, "Kod gir…", z, idCode, variantSecondary)
}
```
(`watchBtn` = primary: pencerenin ana çağrısı-eylemi; diğerleri secondary.)

`mainProc`'a (aynı dosya) `WM_DRAWITEM` işleyicisini ekle — `wmGetMinMaxInfo` case'inden önce:
```go
	case wmDrawItem:
		di := (*drawItemStruct)(osPointer(lparam))
		if di.CtlType == odtButton {
			drawButton(di)
			return 1
		}
```

- [ ] **Step 5: Derle**

Çalıştır: `go build ./...`
Beklenen: hatasız.

- [ ] **Step 6: Görsel doğrulama**

`run` skill ile uygulamayı başlat, ana pencereyi ekranda gör: 5 butonun (Ekle…, Çıkar, İzleyiciyi başlat, Haftalık rapor, Kod gir…) düz köşeli, "İzleyiciyi başlat" mavi (primary) diğerleri koyu gri (secondary) olduğunu doğrula. Butonun üstüne gelince/tıklayınca rengin değiştiğini gör. Tab ile gezinip Enter/Space ile tıklanabildiğini doğrula.

- [ ] **Step 7: Commit**

```bash
git add internal/ui/theme.go internal/ui/win.go internal/ui/window.go
git commit -m "feat(ui): owner-draw Carbon butonları — ana pencere"
```

---

### Task 3: Diyalog butonları — modal.go entegrasyonu ve varsayılan buton (Enter)

Not: `BS_OWNERDRAW` ve `BS_DEFPUSHBUTTON` aynı Win32 stil alt-alanını paylaşıyor (ikisi aynı anda verilemez), bu yüzden Enter tuşuyla "varsayılan butonu tetikle" davranışı artık `IsDialogMessage`'ın yerleşik mekanizmasından gelemiyor. `internal/gate/gate.go` zaten aynı sorunu elle `WM_KEYDOWN`/`VK_RETURN` yakalayarak çözüyor (`gate.go:414-417`) — aynı teknik burada da kullanılıyor.

**Files:**
- Modify: `internal/ui/modal.go`
- Modify: `internal/ui/win.go`

**Interfaces:**
- Consumes: `createButton`, `drawButton` (Task 2), `drawItemStruct`, `wmDrawItem`, `odtButton`, `osPointer`, `wmKeyDown`, `vkReturn` (Task 2/win.go).
- Produces: `func (m *modal) button(text string, r Rect, variant buttonVariant, def bool) (uintptr, int)` (imza değişti — eski `def bool` tek parametreliydi, şimdi `variant` eklendi). Task 4'teki tüm çağrı yerleri bu yeni imzayı kullanıyor.

- [ ] **Step 1: `win.go`'daki `runModal`'a Enter geri çağrımı ekle**

```go
// runModal, diyalogun kendi mesaj dongusunu calistirir ve pencere
// kapanana kadar bloklar. Sahip pencere bu sirada devre disi: modal
// olmasinin tek anlami bu.
//
// onEnter, Enter tusuna basildiginda cagrilir (varsayilan buton owner-draw
// oldugu icin IsDialogMessage'in yerlesik "varsayilan butonu tetikle"
// mekanizmasi calismiyor — bkz. Task 3 notu).
func runModal(hwnd, parent uintptr, onEnter func()) {
	if parent != 0 {
		enable(parent, false)
	}
	show(hwnd)

	var msg msgStruct
	for isWindow(hwnd) {
		r, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		if msg.Message == wmKeyDown && msg.WParam == vkReturn && onEnter != nil {
			onEnter()
			continue
		}
		if d, _, _ := procIsDialogMessage.Call(hwnd, uintptr(unsafe.Pointer(&msg))); d != 0 {
			continue
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
	}

	if parent != 0 {
		enable(parent, true)
		procSetForegroundWindow.Call(parent)
	}
	trim()
}
```
(Yalnızca imza ve gövdenin başındaki `onEnter` kontrolü eklendi; kalan mantık aynı.)

- [ ] **Step 2: `modal.go`'da `defaultID` alanı ve yeni `button`/`run`**

`modal` struct'ına ekle:
```go
	// defaultID, Enter'da tetiklenecek butonun kimligidir (0 = yok).
	defaultID int
```

`button` fonksiyonunu değiştir:
```go
func (m *modal) button(text string, r Rect, variant buttonVariant, def bool) (uintptr, int) {
	id := m.nextID
	m.nextID++
	h := createButton(m.hwnd, text, Rect{X: m.s(r.X), Y: m.s(r.Y), W: m.s(r.W), H: m.s(r.H)}, id, variant)
	if def {
		m.defaultID = id
	}
	return h, id
}
```

`run` fonksiyonunu değiştir:
```go
func (m *modal) run(parent uintptr) {
	runModal(m.hwnd, parent, func() {
		if m.onCmd != nil && m.defaultID != 0 {
			m.onCmd(m.defaultID)
		}
	})
}
```

`modalProc`'a `WM_DRAWITEM` işleyicisi ekle (`wmCommand` case'inden hemen sonra):
```go
	case wmDrawItem:
		di := (*drawItemStruct)(osPointer(lparam))
		if di.CtlType == odtButton {
			drawButton(di)
			return 1
		}
```

- [ ] **Step 3: Derle (henüz eski çağrı yerleri kırık olacak — beklenen)**

Çalıştır: `go build ./...`
Beklenen: `internal/ui/games.go`, `people.go`, `remove.go`, `pair.go`, `data.go` içindeki `m.button(...)` çağrılarında "not enough arguments" hatası. Bu Task 4'te düzeltilecek — burada yalnızca `modal.go`/`win.go`'nun kendisinin mantık hatası taşımadığını gözle doğrula (hata mesajları yalnızca çağrı yerlerinde olmalı, `modal.go`/`win.go` içinde olmamalı).

- [ ] **Step 4: Commit**

```bash
git add internal/ui/modal.go internal/ui/win.go
git commit -m "feat(ui): diyalog butonlarını owner-draw'a taşı, Enter/varsayılan buton elle işlensin"
```

(Not: paket bu commit'te derlenmiyor — sonraki görev çağrı yerlerini düzeltiyor. Ara commit kabul edilebilir çünkü değişiklik tek mantıksal birim ve bir sonraki görev hemen ardından geliyor.)

---

### Task 4: Tüm diyalogların buton çağrılarını yeni imzaya taşı

**Files:**
- Modify: `internal/ui/games.go`
- Modify: `internal/ui/people.go`
- Modify: `internal/ui/remove.go`
- Modify: `internal/ui/pair.go`
- Modify: `internal/ui/data.go`

**Interfaces:**
- Consumes: `m.button(text string, r Rect, variant buttonVariant, def bool) (uintptr, int)` (Task 3).

Varyant ataması: her diyalogda Enter'ın tetiklediği tek yapıcı eylem **primary**, geri dönüşü olmayan silme/kaldırma **danger**, gerisi **secondary**.

- [ ] **Step 1: `games.go` — `showAddGame`**

```go
	addBtn, addID := m.button("Ekle", Rect{224, 334, 90, 28}, variantPrimary, true)
	_, cancelID := m.button("Vazgeç", Rect{326, 334, 90, 28}, variantSecondary, false)
```
(`addBtn` değişkeni zaten `_ = addBtn` ile kullanılmıyordu, aynen kalıyor.)

- [ ] **Step 2: `people.go` — `showPeople` ve `askPerson`**

`showPeople` içinde:
```go
	_, addID := m.button("Ekle", Rect{12, 330, 96, 28}, variantSecondary, false)
	_, editID := m.button("Düzenle", Rect{114, 330, 96, 28}, variantSecondary, false)
	_, rotateID := m.button("Anahtar yenile", Rect{216, 330, 118, 28}, variantSecondary, false)
	_, removeID := m.button("Sil", Rect{340, 330, 80, 28}, variantDanger, false)
	_, closeID := m.button("Kapat", Rect{432, 330, 96, 28}, variantSecondary, true)
```

`askPerson` içinde:
```go
	_, okID := m.button("Tamam", Rect{170, 148, 90, 28}, variantPrimary, true)
	_, cancelID := m.button("Vazgeç", Rect{272, 148, 90, 28}, variantSecondary, false)
```

- [ ] **Step 3: `remove.go` — `showRemove`**

```go
	_, okID := m.button("Kaldır", Rect{252, 254, 90, 28}, variantDanger, false)
	_, cancelID := m.button("Vazgeç", Rect{354, 254, 90, 28}, variantSecondary, true)
```
(Varsayılan buton hâlâ "Vazgeç" — yıkıcı eylem yanlışlıkla Enter'la tetiklenmesin diye bilinçli olarak default değil; bu zaten mevcut davranıştı, korunuyor.)

- [ ] **Step 4: `pair.go` — `showPair`**

```go
	_, revealID := m.button("Anahtarı göster", Rect{152, 302, 130, 26}, variantSecondary, false)
	copyBtn, copyID := m.button("Kopyala", Rect{292, 302, 90, 26}, variantSecondary, false)
	...
	_, okID := m.button("Onayla", Rect{292, 424, 90, 28}, variantPrimary, true)
	_, cancelID := m.button("Vazgeç", Rect{394, 424, 90, 28}, variantSecondary, false)
```

- [ ] **Step 5: `data.go` — `showData`**

```go
	_, moveID := m.button("Taşı…", Rect{262, 390, 90, 28}, variantSecondary, false)
	_, openID := m.button("Klasörü aç", Rect{366, 390, 100, 28}, variantSecondary, false)
	_, closeID := m.button("Kapat", Rect{478, 390, 90, 28}, variantSecondary, true)
```

- [ ] **Step 6: Derle**

Çalıştır: `go build ./...`
Beklenen: hatasız.

- [ ] **Step 7: Mevcut testleri çalıştır**

Çalıştır: `go test ./internal/ui/...`
Beklenen: tümü PASS (bu görev yalnızca çağrı yerlerini değiştiriyor, `layout_test.go`/`rows_test.go`/`move_test.go`/`qr_test.go` etkilenmiyor).

- [ ] **Step 8: Görsel doğrulama**

`run` skill ile her diyaloğu aç: Oyun ekle, Kişiler (+ Kişi ekle, Anahtar eşleştirme/QR), Kaldır, Veriler. Her birinde:
- Primary buton (Ekle/Onayla/Tamam) mavi, diğerleri koyu gri, Sil/Kaldır kırmızı mı?
- Enter tuşu doğru butonu tetikliyor mu (ör. Kişi ekle'de Ad+İpucu girip Enter → "Tamam" çalışmalı)?
- Esc iptal ediyor mu?

- [ ] **Step 9: Commit**

```bash
git add internal/ui/games.go internal/ui/people.go internal/ui/remove.go internal/ui/pair.go internal/ui/data.go
git commit -m "feat(ui): diyalog butonlarına Carbon varyantları ata"
```

---

### Task 5: Pencere/diyalog zemini ve etiket renkleri (Gray 10 arka plan)

**Files:**
- Modify: `internal/ui/theme.go`
- Modify: `internal/ui/window.go`
- Modify: `internal/ui/modal.go`

**Interfaces:**
- Consumes: `clrBackground`, `clrTextPrimary` (Task 1), `wmEraseBkgnd`, `wmCtlColorStatic` (Task 2/win.go).
- Produces: `func ensureBackgroundBrush() uintptr`, `func paintBackground(hdc, hwnd uintptr)`. Kullanılıyor: `window.go`, `modal.go`.

- [ ] **Step 1: `theme.go`'ya arka plan fırçasını ekle**

```go
// backgroundBrush, pencere/diyalog zeminidir (Gray 10). Process basina bir
// kez olusturuluyor; WM_ERASEBKGND her tikte cagrilabilir, her seferinde
// CreateSolidBrush kaynak sizdirirdi.
var backgroundBrush uintptr

func ensureBackgroundBrush() uintptr {
	if backgroundBrush == 0 {
		backgroundBrush, _, _ = procCreateSolidBrush.Call(uintptr(clrBackground))
	}
	return backgroundBrush
}

// paintBackground, WM_ERASEBKGND icin pencerenin istemci alanini Carbon
// Gray 10 rengiyle doldurur.
func paintBackground(hdc, hwnd uintptr) {
	var r winRect
	procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&r)), ensureBackgroundBrush())
}
```

- [ ] **Step 2: `window.go`'nun `mainProc`'una ekle**

```go
	case wmEraseBkgnd:
		paintBackground(wparam, hwnd)
		return 1

	case wmCtlColorStatic:
		procSetBkMode.Call(wparam, transparentBkMode)
		procSetTextColor.Call(wparam, uintptr(clrTextPrimary))
		return ensureBackgroundBrush()
```
(`wparam` `WM_ERASEBKGND`'de HDC, `WM_CTLCOLORSTATIC`'te de HDC — ikisi de doğrudan kullanılabilir.)

- [ ] **Step 3: `modal.go`'nun `modalProc`'una aynısını ekle**

```go
	case wmEraseBkgnd:
		paintBackground(wparam, hwnd)
		return 1

	case wmCtlColorStatic:
		procSetBkMode.Call(wparam, transparentBkMode)
		procSetTextColor.Call(wparam, uintptr(clrTextPrimary))
		return ensureBackgroundBrush()
```

- [ ] **Step 4: Derle**

Çalıştır: `go build ./...`
Beklenen: hatasız.

- [ ] **Step 5: Görsel doğrulama**

`run` skill ile ana pencereyi ve iki diyaloğu (ör. Oyun ekle, Kişiler) aç: zeminin açık gri (#F4F4F4) olduğunu, etiket metinlerinin arkasında görünür bir kutu/kenar olmadığını (zeminle kaynaştığını) doğrula.

- [ ] **Step 6: Commit**

```bash
git add internal/ui/theme.go internal/ui/window.go internal/ui/modal.go
git commit -m "feat(ui): Carbon Gray 10 pencere zemini ve etiket renkleri"
```

---

### Task 6: Girdi alanları — düz kenar ve odak çerçevesi

**Files:**
- Modify: `internal/ui/theme.go`
- Modify: `internal/ui/modal.go`

**Interfaces:**
- Consumes: `clrLayer01`, `clrTextPrimary`, `clrBorderStrong`, `clrInteractive` (Task 1), `wmSetFocus`, `wmKillFocus`, `wmNcPaint`, `wmNcDestroy`, `wmCtlColorEdit`, `psSolid`, `stockNullBrush`, `rdwInvalidate`, `rdwFrame`, `rdwUpdateNow` (Task 2/win.go).
- Produces: `func createEdit(parent uintptr, text string, r Rect, extra uint32, id int, font uintptr) uintptr`. `modal.edit` bunu kullanıyor.

- [ ] **Step 1: `theme.go`'ya edit alanı subclass'ını ekle**

```go
// editState, tek bir EDIT kontrolunun odak durumudur — kenar rengi buna
// gore degisiyor (odaksiz: borderStrong, odakli: interactive, 2px).
type editState struct{ focused bool }

var editStates = map[uintptr]*editState{}

const editSubclassID = 1

var editSubclassCB = windows.NewCallback(editSubclassProc)

// createEdit, gomuk 3D kenar yerine duz, Carbon renkli kenarli bir EDIT
// kontrolu olusturur. Kenar WM_NCPAINT'te elle ciziliyor (EDIT sinifinin
// kendi kenar cizimi WS_EX_CLIENTEDGE'e bagli; o kaldirildigi icin
// kenarsiz kalirdi).
func createEdit(parent uintptr, text string, r Rect, extra uint32, id int, font uintptr) uintptr {
	h := create("EDIT", text, esAutoHScroll|wsTabStop|wsBorder|extra, 0, r, parent, id, font)
	editStates[h] = &editState{}
	procSetWindowSubclass.Call(h, editSubclassCB, editSubclassID, 0)
	return h
}

func editSubclassProc(hwnd, msg, wparam, lparam, idSubclass, refData uintptr) uintptr {
	switch msg {
	case wmSetFocus:
		if st := editStates[hwnd]; st != nil {
			st.focused = true
		}
		r, _, _ := procDefSubclassProc.Call(hwnd, msg, wparam, lparam)
		procRedrawWindow.Call(hwnd, 0, 0, rdwInvalidate|rdwFrame|rdwUpdateNow)
		return r
	case wmKillFocus:
		if st := editStates[hwnd]; st != nil {
			st.focused = false
		}
		r, _, _ := procDefSubclassProc.Call(hwnd, msg, wparam, lparam)
		procRedrawWindow.Call(hwnd, 0, 0, rdwInvalidate|rdwFrame|rdwUpdateNow)
		return r
	case wmNcPaint:
		r, _, _ := procDefSubclassProc.Call(hwnd, msg, wparam, lparam)
		paintEditBorder(hwnd)
		return r
	case wmNcDestroy:
		delete(editStates, hwnd)
		procRemoveWindowSubclass.Call(hwnd, editSubclassCB, editSubclassID)
	}
	r, _, _ := procDefSubclassProc.Call(hwnd, msg, wparam, lparam)
	return r
}

// paintEditBorder, EDIT kontrolunun kenarligini WM_NCPAINT sirasinda elle
// cizer. GetWindowDC pencere disinin tamamini (istemci + kenarlik) kapsayan
// bir DC verir, koordinatlar pencerenin sol-ustune gore — WM_NCPAINT'in
// bekledigi koordinat sistemi budur.
func paintEditBorder(hwnd uintptr) {
	hdc, _, _ := procGetWindowDC.Call(hwnd)
	if hdc == 0 {
		return
	}
	defer procReleaseDC.Call(hwnd, hdc)

	var wr winRect
	procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&wr)))
	w, h := wr.Right-wr.Left, wr.Bottom-wr.Top

	color, width := clrBorderStrong, int32(1)
	if st := editStates[hwnd]; st != nil && st.focused {
		color, width = clrInteractive, 2
	}

	pen, _, _ := procCreatePen.Call(psSolid, uintptr(width), uintptr(color))
	oldPen, _, _ := procSelectObject.Call(hdc, pen)
	nullBrush, _, _ := procGetStockObject.Call(stockNullBrush)
	oldBrush, _, _ := procSelectObject.Call(hdc, nullBrush)
	procRectangle.Call(hdc, 0, 0, uintptr(w), uintptr(h))
	procSelectObject.Call(hdc, oldPen)
	procSelectObject.Call(hdc, oldBrush)
	procDeleteObject.Call(pen)
}
```

- [ ] **Step 2: `theme.go`'ya `WM_CTLCOLOREDIT` yardımcı fonksiyonunu ekle**

```go
var layerBrush uintptr

func ensureLayerBrush() uintptr {
	if layerBrush == 0 {
		layerBrush, _, _ = procCreateSolidBrush.Call(uintptr(clrLayer01))
	}
	return layerBrush
}

// colorEdit, WM_CTLCOLOREDIT icin ortak isleyicidir: beyaz zemin, koyu
// metin. modalProc bunu dogrudan cagiriyor.
func colorEdit(hdc uintptr) uintptr {
	procSetTextColor.Call(hdc, uintptr(clrTextPrimary))
	procSetBkColor.Call(hdc, uintptr(clrLayer01))
	return ensureLayerBrush()
}
```

- [ ] **Step 3: `modal.go`'nun `edit` fonksiyonunu ve `modalProc`'unu güncelle**

`edit` fonksiyonu:
```go
func (m *modal) edit(text string, r Rect, extra uint32) uintptr {
	id := m.nextID
	m.nextID++
	return createEdit(m.hwnd, text, Rect{X: m.s(r.X), Y: m.s(r.Y), W: m.s(r.W), H: m.s(r.H)}, extra, id, m.font)
}
```

`modalProc`'a ekle:
```go
	case wmCtlColorEdit:
		return colorEdit(wparam)
```

- [ ] **Step 4: Derle**

Çalıştır: `go build ./...`
Beklenen: hatasız.

- [ ] **Step 5: Görsel doğrulama**

`run` skill ile Kişiler → Kişi ekle diyaloğunu aç (Ad/İpucu alanları), Kaldır diyaloğundaki Kod alanını dene: kenarların düz (gömük 3D değil) olduğunu, bir alana tıklayınca kenarın kalınlaşıp maviye döndüğünü, Tab ile başka alana geçince eski griye döndüğünü doğrula. Veriler diyaloğundaki salt-okunur yol alanının da beyaz zeminle göründüğünü kontrol et.

- [ ] **Step 6: Commit**

```bash
git add internal/ui/theme.go internal/ui/modal.go
git commit -m "feat(ui): girdi alanlarına düz Carbon kenarı ve odak çerçevesi"
```

---

### Task 7: Liste görünümü renkleri ve son görsel doğrulama

Sadeleştirme notu (spec'ten bilinçli sapma): spec seçili satır zemini için ayrı bir `selectedRow` (#E0E7FF) rengi öngörüyordu. Bunu uygulamak `NM_CUSTOMDRAW`/`WM_NOTIFY` ile tam owner-draw gerektiriyor — listenin geri kalanı (zemin/metin/header) için gereken basit `LVM_SETBKCOLOR` ailesinin çok ötesinde bir karmaşıklık. Kullanıcının "karmaşık olmasın" isteği gereği bu adım atlanıyor; seçili satır sistemin varsayılan vurgu rengini (zaten mavi tonlarında, Carbon'un interactive rengine yakın) kullanmaya devam ediyor.

**Files:**
- Modify: `internal/ui/win.go`

**Interfaces:**
- Consumes: `clrLayer01`, `clrTextPrimary` (Task 1), `semiboldFont` (Task 2).

- [ ] **Step 1: `win.go`'ya LVM sabitlerini ekle**

Liste sabitlerinin bulunduğu bloğa ekle:
```go
	lvmSetBkColor     = lvmFirst + 1
	lvmSetTextColor   = lvmFirst + 36
	lvmSetTextBkColor = lvmFirst + 38
	lvmGetHeader      = lvmFirst + 31
```

- [ ] **Step 2: `lvSetColumns`'u güncelle**

```go
// lvSetColumns, liste kontrolunun sutunlarini kurar ve Carbon renklerini
// uygular: beyaz zemin, izgarasiz (yalnizca Carbon'un varsayilan satir
// ayiricisiyla), semibold header.
func lvSetColumns(hwnd uintptr, titles []string, widths []int32, dpi uint32) {
	procSendMessage.Call(hwnd, lvmSetExtendedStyle, 0,
		lvsExFullRowSelect|lvsExDoubleBuffer)
	procSendMessage.Call(hwnd, lvmSetBkColor, 0, uintptr(clrLayer01))
	procSendMessage.Call(hwnd, lvmSetTextColor, 0, uintptr(clrTextPrimary))
	procSendMessage.Call(hwnd, lvmSetTextBkColor, 0, uintptr(clrLayer01))

	if hdr, _, _ := procSendMessage.Call(hwnd, lvmGetHeader, 0, 0); hdr != 0 {
		setFont(hdr, semiboldFont(dpi))
	}

	for i, t := range titles {
		c := lvColumn{
			Mask:     lvcfText | lvcfWidth | lvcfSubItem,
			Cx:       Scale(dpi, widths[i]),
			PszText:  utf16(t),
			ISubItem: int32(i),
		}
		procSendMessage.Call(hwnd, lvmInsertColumn, uintptr(i), uintptr(unsafe.Pointer(&c)))
	}
}
```
(`lvsExGridLines` kaldırıldı — Carbon tablosunda tam ızgara yok.)

- [ ] **Step 3: Derle ve testleri çalıştır**

Çalıştır: `go build ./...` ve `go test ./internal/ui/...`
Beklenen: hatasız, tümü PASS.

- [ ] **Step 4: Uçtan uca görsel doğrulama**

`run` skill ile uygulamayı başlat ve her ekranı sırayla kontrol et:
1. Ana pencere — boş listeyle ve en az bir oyun eklenmiş haliyle (izgarasız, beyaz zeminli liste, semibold header).
2. Oyun ekle — çalışan programlar listesinden seçim + elle exe yazma, başlatıcı kutusu.
3. Kişiler — liste, Ekle/Düzenle/Anahtar yenile/Sil/Kapat butonları, Kişi ekle diyaloğu, Anahtar eşleştirme (QR) diyaloğu.
4. Kaldır — kod alanı, "veriler de silinsin" kutusu.
5. Veriler (menü → Veriler → Nerede saklanıyor) — salt okunur yol, dosya listesi, Taşı diyaloğu (klasör seçici).
6. Hakkında (menü → Yardım → Hakkında).
7. Pencereyi yeniden boyutlandır: düzenin bozulmadığını, minimum boyutun hâlâ uygulandığını doğrula.
8. Varsa ikinci bir monitöre taşı (farklı DPI): fontların/butonların ölçeklendiğini doğrula. (İkinci monitör yoksa bu adımı atla ve notta belirt.)

Her ekranda: zemin Gray 10, butonlar düz köşeli ve doğru renkte (primary mavi / secondary koyu gri / danger kırmızı), girdi alanları düz kenarlı ve odakta mavi çerçeveli olmalı.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/win.go
git commit -m "feat(ui): liste görünümlerine Carbon renkleri ve semibold header"
```

---

## Self-Review Sonucu

- **Spec kapsaması:** Renk paleti (Task 1), tipografi (Task 2, semibold — başlık fontu bilinçli olarak düşürüldü), buton owner-draw (Task 2-4), pencere/diyalog zemini (Task 5), girdi alanı (Task 6), liste (Task 7, seçili satır rengi bilinçli olarak düşürüldü), menü çubuğu (spec: kapsam dışı, dokunulmadı). Tüm spec maddeleri bir görevle eşleşiyor veya dokümante edilmiş bir sadeleştirmeyle açıklanıyor.
- **Yer tutucu taraması:** Yok — her adımda gerçek kod var.
- **Tip tutarlılığı:** `buttonVariant`/`buttonColorSet`/`buttonColors` (Task 1) → `createButton`/`drawButton` (Task 2) → `m.button` (Task 3) → çağrı yerleri (Task 4) aynı imzaları kullanıyor; `createEdit` (Task 6) `modal.edit`'in tek çağrı yeriyle eşleşiyor.
