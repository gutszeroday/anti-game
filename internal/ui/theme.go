//go:build windows

package ui

import (
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

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
	// clrBorderSubtle ve clrSelectedRow (asagida) kasitli olarak kullanilmiyor:
	// plan (Task 7, sadelestirme notu) secili satir zeminini ve satir
	// ayiricilarini NM_CUSTOMDRAW gerektirdigi icin bilincli olarak kapsam
	// disi birakti, liste OS'in varsayilan gorunumunu koruyor. Bu ikisi o
	// dusulen ozellik icin ayrilmis — olu kod degil.
	clrBorderSubtle   = rgb(0xE0, 0xE0, 0xE0)
	clrBorderStrong   = rgb(0x8D, 0x8D, 0x8D)
	clrInteractive    = rgb(0x0F, 0x62, 0xFE)
	clrHoverPrimary   = rgb(0x03, 0x53, 0xE9)
	clrActivePrimary  = rgb(0x00, 0x2D, 0x9C)
	clrSecondaryBg    = rgb(0x39, 0x39, 0x39)
	clrHoverSecondary = rgb(0x4C, 0x4C, 0x4C)
	clrDangerBg       = rgb(0xDA, 0x1E, 0x28)
	clrHoverDanger    = rgb(0xBA, 0x1B, 0x23)
	clrDisabledBg     = rgb(0xC6, 0xC6, 0xC6)
	clrDisabledText   = rgb(0x8D, 0x8D, 0x8D)
	clrSelectedRow    = rgb(0xE0, 0xE7, 0xFF)
	clrOnColor        = rgb(0xFF, 0xFF, 0xFF)
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

// fontCache (win.go) desenini aynen izleyen ayrı bir önbellek — büyük font
// paylaşımı DPI değişince bozulmasın diye aynı kilit kullanılmıyor, kendi
// kilidini taşıyor.
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
		Height:  -Scale(dpi, 14),
		Weight:  600, // FW_SEMIBOLD
		CharSet: 1,   // DEFAULT_CHARSET
	}
	copy(lf.FaceName[:], windows.StringToUTF16("Segoe UI"))
	f, _, _ := procCreateFontIndirect.Call(uintptr(unsafe.Pointer(&lf)))
	semiboldCache[dpi] = f
	return f
}

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
//
// registerClass'taki desenle ayni (win.go, sync.Once icinde
// windows.NewCallback) — dogrudan bir var initializer olarak yazilirsa
// (buttonSubclassCB = windows.NewCallback(buttonSubclassProc)) Go derleyicisi
// "initialization cycle" hatasi veriyor: buttonSubclassProc govdesi
// buttonSubclassCB'yi okuyor, bu da Go'nun paket-baslatma bagimlilik
// analizinde (govde de taranir) bir dongu sayiliyor — calisma zamaninda
// gercek bir dongu olmasa bile. sync.Once ile geciktirmek bu bagimlilik
// kenarini kirar.
var (
	buttonSubclassOnce sync.Once
	buttonSubclassCB   uintptr
)

func buttonSubclassCallback() uintptr {
	buttonSubclassOnce.Do(func() {
		buttonSubclassCB = windows.NewCallback(buttonSubclassProc)
	})
	return buttonSubclassCB
}

// createButton, Carbon uslubunda owner-draw bir buton olusturur. Cizim
// WM_DRAWITEM ile ust pencereye (parent) geliyor; hover/basili durumu
// butonun kendi WndProc'una subclass ile ekleniyor.
func createButton(parent uintptr, text string, r Rect, id int, variant buttonVariant) uintptr {
	h := create("BUTTON", text, bsOwnerDraw|wsTabStop, 0, r, parent, id, 0)
	buttonVariants[h] = variant
	buttonStates[h] = &buttonState{}
	procSetWindowSubclass.Call(h, buttonSubclassCallback(), buttonSubclassID, 0)
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
		procRemoveWindowSubclass.Call(hwnd, buttonSubclassCallback(), buttonSubclassID)
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

// editState, tek bir EDIT kontrolunun odak durumudur — kenar rengi buna
// gore degisiyor (odaksiz: borderStrong, odakli: interactive, 2px).
type editState struct{ focused bool }

var editStates = map[uintptr]*editState{}

const editSubclassID = 1

// editSubclassCB, buttonSubclassCB (yukarida) ile ayni nedenle sync.Once
// arkasinda gec baslatiliyor: editSubclassProc govdesi editSubclassCB'yi
// okuyor (RemoveWindowSubclass cagrisinda), bu da dogrudan bir var
// initializer olarak yazilirsa Go'da "initialization cycle" hatasi verir.
var (
	editSubclassOnce sync.Once
	editSubclassCB   uintptr
)

func editSubclassCallback() uintptr {
	editSubclassOnce.Do(func() {
		editSubclassCB = windows.NewCallback(editSubclassProc)
	})
	return editSubclassCB
}

// createEdit, gomuk 3D kenar yerine duz, Carbon renkli kenarli bir EDIT
// kontrolu olusturur. Kenar WM_NCPAINT'te elle ciziliyor (EDIT sinifinin
// kendi kenar cizimi WS_EX_CLIENTEDGE'e bagli; o kaldirildigi icin
// kenarsiz kalirdi).
func createEdit(parent uintptr, text string, r Rect, extra uint32, id int, font uintptr) uintptr {
	h := create("EDIT", text, esAutoHScroll|wsTabStop|wsBorder|extra, 0, r, parent, id, font)
	editStates[h] = &editState{}
	procSetWindowSubclass.Call(h, editSubclassCallback(), editSubclassID, 0)
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
		procRemoveWindowSubclass.Call(hwnd, editSubclassCallback(), editSubclassID)
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
