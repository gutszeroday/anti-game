//go:build windows

package ui

import (
	"fmt"
	"unsafe"
)

// Modal diyaloglarin ortak iskeleti. Dort diyalog da ayni seyi yapiyor:
// sahibinin ustunde bir pencere ac, kontrolleri yerlestir, dugmeye
// basilinca bir islev cagir, kapanana kadar bekle.
//
// Kontroller WM_CREATE yerine pencere olusturulduktan hemen sonra
// ekleniyor. Ikisi de gecerli; boylesi kapali degiskenlerin (dir, secilen
// kisi, hata durumu) diyalogla birlikte yasamasina izin veriyor.

const modalClassName = "AntigameDialog"

type modal struct {
	hwnd uintptr
	dpi  uint32
	font uintptr

	// onCmd, bir dugmeye basildiginda veya Enter/Esc geldiginde cagrilir.
	onCmd func(id int)
	// onPaint, pencere yeniden cizildiginde cagrilir. QR diyalogu disinda bos.
	onPaint func(hdc uintptr)

	nextID int
}

// Etkin diyaloglar. WndProc bir C geri cagrimidir ve kapali degisken
// tasiyamaz; durum pencere tutamagiyla eslestiriliyor. Arayuz tek
// thread'de calistigi icin kilit gerekmiyor.
var modals = map[uintptr]*modal{}

var modalClass = &windowClass{name: modalClassName, proc: modalProc}

type paintStruct struct {
	Hdc         uintptr
	Erase       int32
	RcPaint     winRect
	Restore     int32
	IncUpdate   int32
	RgbReserved [32]byte
}

func modalProc(hwnd, msg, wparam, lparam uintptr) uintptr {
	m := modals[hwnd]
	switch msg {
	case wmCommand:
		if m != nil && m.onCmd != nil {
			m.onCmd(int(uint16(wparam)))
			return 0
		}
	case wmPaint:
		if m != nil && m.onPaint != nil {
			var ps paintStruct
			hdc, _, _ := procBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
			m.onPaint(hdc)
			procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
			return 0
		}
	case wmClose:
		destroy(hwnd)
		return 0
	case wmDestroy:
		delete(modals, hwnd)
		return 0
	}
	r, _, _ := procDefWindowProc.Call(hwnd, msg, wparam, lparam)
	return r
}

// newModal, verilen istemci alani olculerinde bir diyalog penceresi acar.
// Olculer 96 DPI icin verilir; sahibin DPI'sina gore olceklenir.
func newModal(parent uintptr, title string, w, h int32) (*modal, error) {
	if err := modalClass.register(); err != nil {
		return nil, err
	}
	dpi := dpiOf(parent)
	if parent == 0 {
		dpi = 96
	}

	const style = wsPopup | wsCaption | wsSysMenu | wsClipChildren

	// CreateWindowEx toplam pencere olcusu ister; istenen ic alani
	// korumak icin cerceve payi ekleniyor.
	r := winRect{0, 0, Scale(dpi, w), Scale(dpi, h)}
	procAdjustWindowRectEx.Call(uintptr(unsafe.Pointer(&r)), style, 0, 0)

	hwnd, _, err := procCreateWindowEx.Call(0,
		uintptr(unsafe.Pointer(utf16(modalClassName))),
		uintptr(unsafe.Pointer(utf16(title))),
		style,
		uintptr(0x80000000), uintptr(0x80000000), // CW_USEDEFAULT
		uintptr(r.Right-r.Left), uintptr(r.Bottom-r.Top),
		parent, 0, instance(), 0)
	if hwnd == 0 {
		return nil, fmt.Errorf("diyalog penceresi olusturulamadi: %w", err)
	}

	m := &modal{hwnd: hwnd, dpi: dpi, font: uiFont(dpi), nextID: 100}
	modals[hwnd] = m
	center(hwnd, parent)
	return m, nil
}

// s, 96 DPI icin yazilmis bir olcuyu diyalogun DPI'sina cevirir.
func (m *modal) s(v int32) int32 { return Scale(m.dpi, v) }

// add, diyaloga bir kontrol ekler ve ona bir kimlik atar.
func (m *modal) add(class, text string, style, exStyle uint32, r Rect) (uintptr, int) {
	id := m.nextID
	m.nextID++
	h := create(class, text, style, exStyle, Rect{
		X: m.s(r.X), Y: m.s(r.Y), W: m.s(r.W), H: m.s(r.H),
	}, m.hwnd, id, m.font)
	return h, id
}

func (m *modal) label(text string, r Rect) uintptr {
	h, _ := m.add("STATIC", text, ssLeft, 0, r)
	return h
}

func (m *modal) button(text string, r Rect, def bool) (uintptr, int) {
	style := uint32(bsPushButton | wsTabStop)
	if def {
		style = bsDefPushButton | wsTabStop
	}
	return m.add("BUTTON", text, style, 0, r)
}

func (m *modal) edit(text string, r Rect, extra uint32) uintptr {
	h, _ := m.add("EDIT", text, esAutoHScroll|wsTabStop|wsBorder|extra, wsExClientEdge, r)
	return h
}

func (m *modal) checkbox(text string, r Rect) uintptr {
	h, _ := m.add("BUTTON", text, bsAutoCheckBox|wsTabStop, 0, r)
	return h
}

func (m *modal) list(r Rect, titles []string, widths []int32) uintptr {
	h, _ := m.add("SysListView32", "",
		lvsReport|lvsSingleSel|lvsShowSelAlways|lvsNoSortHeader|wsTabStop|wsBorder,
		wsExClientEdge, r)
	lvSetColumns(h, titles, widths, m.dpi)
	return h
}

// run, diyalogu gosterir ve kapanana kadar bekler.
func (m *modal) run(parent uintptr) { runModal(m.hwnd, parent) }

// close, diyalogu kapatir.
func (m *modal) close() { destroy(m.hwnd) }
