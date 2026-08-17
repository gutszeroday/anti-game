//go:build windows

package ui

import (
	"fmt"
	"runtime/debug"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/guts/antigame/internal/winproc"
)

// Win32 sarmalayicilari. Pencereler arasinda paylasilan sikici is burada
// toplaniyor: sinif kaydi, kontrol olusturma, font, DPI, liste kontrolu.
//
// tray paketindeki desenle ayni (LazyDLL + LazyProc). Iki paket arasinda
// ortak bir Win32 tip paketi kurulmadi; iki kullanim icin erken soyutlama
// olurdu ve her ikisinin de ihtiyaci farkli.

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	gdi32    = windows.NewLazySystemDLL("gdi32.dll")
	comctl32 = windows.NewLazySystemDLL("comctl32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procRegisterClassEx     = user32.NewProc("RegisterClassExW")
	procCreateWindowEx      = user32.NewProc("CreateWindowExW")
	procDestroyWindow       = user32.NewProc("DestroyWindow")
	procDefWindowProc       = user32.NewProc("DefWindowProcW")
	procGetMessage          = user32.NewProc("GetMessageW")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procDispatchMessage     = user32.NewProc("DispatchMessageW")
	procIsDialogMessage     = user32.NewProc("IsDialogMessageW")
	procPostQuitMessage     = user32.NewProc("PostQuitMessage")
	procSendMessage         = user32.NewProc("SendMessageW")
	procIsWindow            = user32.NewProc("IsWindow")
	procShowWindow          = user32.NewProc("ShowWindow")
	procUpdateWindow        = user32.NewProc("UpdateWindow")
	procEnableWindow        = user32.NewProc("EnableWindow")
	procIsWindowEnabled     = user32.NewProc("IsWindowEnabled")
	procMoveWindow          = user32.NewProc("MoveWindow")
	procGetClientRect       = user32.NewProc("GetClientRect")
	procGetWindowRect       = user32.NewProc("GetWindowRect")
	procSetWindowPos        = user32.NewProc("SetWindowPos")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procSetFocus            = user32.NewProc("SetFocus")
	procFindWindow          = user32.NewProc("FindWindowW")
	procLoadIcon            = user32.NewProc("LoadIconW")
	procLoadCursor          = user32.NewProc("LoadCursorW")
	procMessageBox          = user32.NewProc("MessageBoxW")
	procSetTimer            = user32.NewProc("SetTimer")
	procKillTimer           = user32.NewProc("KillTimer")
	procGetDpiForWindow     = user32.NewProc("GetDpiForWindow")
	procSPIForDpi           = user32.NewProc("SystemParametersInfoForDpi")
	procSPI                 = user32.NewProc("SystemParametersInfoW")
	procBeginPaint          = user32.NewProc("BeginPaint")
	procEndPaint            = user32.NewProc("EndPaint")
	procGetSysColorBrush    = user32.NewProc("GetSysColorBrush")
	procAdjustWindowRectEx  = user32.NewProc("AdjustWindowRectEx")

	procCreateFontIndirect = gdi32.NewProc("CreateFontIndirectW")
	procStretchDIBits      = gdi32.NewProc("StretchDIBits")

	procInitCommonControlsEx = comctl32.NewProc("InitCommonControlsEx")

	procGetModuleHandle = kernel32.NewProc("GetModuleHandleW")

	procCreateSolidBrush = gdi32.NewProc("CreateSolidBrush")
	procDeleteObject     = gdi32.NewProc("DeleteObject")
	procCreatePen        = gdi32.NewProc("CreatePen")
	procSelectObject     = gdi32.NewProc("SelectObject")
	procRectangle        = gdi32.NewProc("Rectangle")
	procSetBkMode        = gdi32.NewProc("SetBkMode")
	procSetTextColor     = gdi32.NewProc("SetTextColor")
	procSetBkColor       = gdi32.NewProc("SetBkColor")
	procGetStockObject   = gdi32.NewProc("GetStockObject")

	procFillRect        = user32.NewProc("FillRect")
	procDrawText        = user32.NewProc("DrawTextW")
	procInvalidateRect  = user32.NewProc("InvalidateRect")
	procTrackMouseEvent = user32.NewProc("TrackMouseEvent")
	procGetWindowDC     = user32.NewProc("GetWindowDC")
	procReleaseDC       = user32.NewProc("ReleaseDC")
	procRedrawWindow    = user32.NewProc("RedrawWindow")

	procSetWindowSubclass    = comctl32.NewProc("SetWindowSubclass")
	procDefSubclassProc      = comctl32.NewProc("DefSubclassProc")
	procRemoveWindowSubclass = comctl32.NewProc("RemoveWindowSubclass")
)

// Pencere ve kontrol stilleri.
const (
	wsOverlappedWindow = 0x00CF0000
	wsChild            = 0x40000000
	wsVisible          = 0x10000000
	wsTabStop          = 0x00010000
	wsGroup            = 0x00020000
	wsBorder           = 0x00800000
	wsCaption          = 0x00C00000
	wsSysMenu          = 0x00080000
	wsPopup            = 0x80000000
	wsClipChildren     = 0x02000000

	wsExClientEdge = 0x00000200

	bsPushButton    = 0x00000000
	bsDefPushButton = 0x00000001
	bsAutoCheckBox  = 0x00000003
	bsOwnerDraw     = 0x0000000B

	esAutoHScroll = 0x00000080
	esNumber      = 0x00002000
	esCenter      = 0x00000001
	esReadOnly    = 0x00000800

	ssLeft = 0x00000000

	lvsReport        = 0x0001
	lvsSingleSel     = 0x0004
	lvsShowSelAlways = 0x0008
	lvsNoSortHeader  = 0x00010000
)

// Pencere mesajlari.
const (
	wmDestroy       = 0x0002
	wmSize          = 0x0005
	wmSetFont       = 0x0030
	wmPaint         = 0x000F
	wmClose         = 0x0010
	wmGetMinMaxInfo = 0x0024
	wmSetText       = 0x000C
	wmGetText       = 0x000D
	wmGetTextLen    = 0x000E
	wmCommand       = 0x0111
	wmTimer         = 0x0113
	wmDpiChanged    = 0x02E0

	wmDrawItem       = 0x002B
	wmEraseBkgnd     = 0x0014
	wmCtlColorStatic = 0x0138
	wmCtlColorEdit   = 0x0133
	wmMouseMove      = 0x0200
	wmMouseLeave     = 0x02A3
	wmLButtonDown    = 0x0201
	wmLButtonUp      = 0x0202
	wmSetFocus       = 0x0007
	wmKillFocus      = 0x0008
	wmNcPaint        = 0x0085
	wmNcDestroy      = 0x0082
	wmKeyDown        = 0x0100

	vkReturn = 0x0D

	bmGetCheck = 0x00F0
	bmSetCheck = 0x00F1

	emSetSel = 0x00B1

	idOK     = 1
	idCancel = 2
)

// Owner-draw ve GDI sabitleri.
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

// Liste kontrolu mesajlari ve bayraklari.
const (
	lvmFirst            = 0x1000
	lvmDeleteAllItems   = lvmFirst + 9
	lvmGetNextItem      = lvmFirst + 12
	lvmInsertItem       = lvmFirst + 77
	lvmSetItem          = lvmFirst + 76
	lvmInsertColumn     = lvmFirst + 97
	lvmSetExtendedStyle = lvmFirst + 54
	lvmSetColumnWidth   = lvmFirst + 30
	lvmSetBkColor       = lvmFirst + 1
	lvmSetTextColor     = lvmFirst + 36
	lvmSetTextBkColor   = lvmFirst + 38
	lvmGetHeader        = lvmFirst + 31

	lvcfWidth   = 0x0002
	lvcfText    = 0x0004
	lvcfSubItem = 0x0008

	lvifText = 0x0001

	lvniSelected = 0x0002

	lvsExFullRowSelect = 0x00000020
	lvsExGridLines     = 0x00000001
	lvsExDoubleBuffer  = 0x00010000

	iccListViewClasses = 0x00000001
)

const (
	swShow = 5

	swpNoSize     = 0x0001
	swpNoZOrder   = 0x0004
	swpNoActivate = 0x0010

	mbIconInfo = 0x00000040
	mbIconWarn = 0x00000030
	mbYesNo    = 0x00000004
	idYes      = 6

	idiApplication = 32512
	idcArrow       = 32512

	colorBtnFace = 15

	spiGetNonClientMetrics = 0x0029
)

type wndClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   windows.Handle
	Icon       windows.Handle
	Cursor     windows.Handle
	Background windows.Handle
	MenuName   *uint16
	ClassName  *uint16
	IconSm     windows.Handle
}

type msgStruct struct {
	HWnd    windows.Handle
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
}

type winRect struct{ Left, Top, Right, Bottom int32 }

type minMaxInfo struct {
	Reserved     struct{ X, Y int32 }
	MaxSize      struct{ X, Y int32 }
	MaxPosition  struct{ X, Y int32 }
	MinTrackSize struct{ X, Y int32 }
	MaxTrackSize struct{ X, Y int32 }
}

type initCommonControlsEx struct {
	Size uint32
	ICC  uint32
}

type logFont struct {
	Height         int32
	Width          int32
	Escapement     int32
	Orientation    int32
	Weight         int32
	Italic         byte
	Underline      byte
	StrikeOut      byte
	CharSet        byte
	OutPrecision   byte
	ClipPrecision  byte
	Quality        byte
	PitchAndFamily byte
	FaceName       [32]uint16
}

type nonClientMetrics struct {
	Size              uint32
	BorderWidth       int32
	ScrollWidth       int32
	ScrollHeight      int32
	CaptionWidth      int32
	CaptionHeight     int32
	CaptionFont       logFont
	SmCaptionWidth    int32
	SmCaptionHeight   int32
	SmCaptionFont     logFont
	MenuWidth         int32
	MenuHeight        int32
	MenuFont          logFont
	StatusFont        logFont
	MessageFont       logFont
	PaddedBorderWidth int32
}

type lvColumn struct {
	Mask       uint32
	Fmt        int32
	Cx         int32
	PszText    *uint16
	CchTextMax int32
	ISubItem   int32
	IImage     int32
	IOrder     int32
	CxMin      int32
	CxDefault  int32
	CxIdeal    int32
}

type lvItem struct {
	Mask       uint32
	IItem      int32
	ISubItem   int32
	State      uint32
	StateMask  uint32
	PszText    *uint16
	CchTextMax int32
	IImage     int32
	LParam     uintptr
	IIndent    int32
	IGroupID   int32
	CColumns   uint32
	PuColumns  uintptr
	PiColFmt   uintptr
	IGroup     int32
}

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

func utf16(s string) *uint16 {
	p, err := windows.UTF16PtrFromString(s)
	if err != nil {
		// Metinde NUL varsa donusum basarisiz olur. Kullaniciya
		// gosterilen metinler sabit ya da dosyadan geliyor; bos
		// gostermek cokmekten iyidir.
		p, _ = windows.UTF16PtrFromString("")
	}
	return p
}

func instance() uintptr {
	h, _, _ := procGetModuleHandle.Call(0)
	return h
}

// Ortak kontroller process basina bir kez yuklenir. Liste kontrolu bu
// cagri yapilmadan olusturulamaz.
var (
	commonOnce sync.Once
	commonErr  error
)

func initCommonControls() error {
	commonOnce.Do(func() {
		icc := initCommonControlsEx{ICC: iccListViewClasses}
		icc.Size = uint32(unsafe.Sizeof(icc))
		if r, _, err := procInitCommonControlsEx.Call(uintptr(unsafe.Pointer(&icc))); r == 0 {
			commonErr = fmt.Errorf("ortak kontroller yuklenemedi: %w", err)
		}
	})
	return commonErr
}

// registerClass, pencere sinifini bir kez kaydeder. Ikinci kayit "sinif
// zaten var" hatasi verir, bu yuzden her sinif kendi Once'ini tasiyor.
type windowClass struct {
	name string
	proc func(hwnd, msg, wparam, lparam uintptr) uintptr
	once sync.Once
	err  error
}

func (c *windowClass) register() error {
	c.once.Do(func() {
		cursor, _, _ := procLoadCursor.Call(0, idcArrow)
		icon, _, _ := procLoadIcon.Call(0, idiApplication)
		brush, _, _ := procGetSysColorBrush.Call(colorBtnFace)
		wc := wndClassEx{
			Style:      0,
			WndProc:    windows.NewCallback(c.proc),
			Instance:   windows.Handle(instance()),
			Icon:       windows.Handle(icon),
			Cursor:     windows.Handle(cursor),
			Background: windows.Handle(brush),
			ClassName:  utf16(c.name),
			IconSm:     windows.Handle(icon),
		}
		wc.Size = uint32(unsafe.Sizeof(wc))
		if r, _, err := procRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
			c.err = fmt.Errorf("pencere sinifi kaydedilemedi (%s): %w", c.name, err)
		}
	})
	return c.err
}

// dpiOf, pencerenin bulundugu ekranin DPI'sini verir. GetDpiForWindow
// basarisiz olursa 96 varsayilir; Scale sifiri zaten 96 sayiyor ama
// cagiranin da makul bir deger gormesi gerekiyor.
func dpiOf(hwnd uintptr) uint32 {
	r, _, _ := procGetDpiForWindow.Call(hwnd)
	if r == 0 {
		return 96
	}
	return uint32(r)
}

// Font, DPI basina onbelleklenir: PerMonitorV2 ile pencere ekran
// degistirdiginde yeni bir font gerekiyor, ama her cizimde font
// uretmek de kaynak sizintisi olurdu.
var (
	fontMu    sync.Mutex
	fontCache = map[uint32]uintptr{}
)

// uiFont, kullanicinin sistem arayuz fontunu verilen DPI icin dondurur.
// Bu yapilmazsa Windows kontrollere 1995'ten kalma bitmap fontunu verir.
func uiFont(dpi uint32) uintptr {
	fontMu.Lock()
	defer fontMu.Unlock()
	if f, ok := fontCache[dpi]; ok {
		return f
	}

	var m nonClientMetrics
	m.Size = uint32(unsafe.Sizeof(m))
	// SystemParametersInfoForDpi Windows 10 1607+ ile geldi ve olculeri
	// hedef DPI'ya gore verir; yoksa birincil ekranin olculerine duseriz.
	r, _, _ := procSPIForDpi.Call(spiGetNonClientMetrics,
		uintptr(m.Size), uintptr(unsafe.Pointer(&m)), 0, uintptr(dpi))
	if r == 0 {
		m = nonClientMetrics{}
		m.Size = uint32(unsafe.Sizeof(m))
		if r, _, _ = procSPI.Call(spiGetNonClientMetrics,
			uintptr(m.Size), uintptr(unsafe.Pointer(&m)), 0); r == 0 {
			fontCache[dpi] = 0
			return 0
		}
		m.MessageFont.Height = Scale(dpi, m.MessageFont.Height)
	}

	f, _, _ := procCreateFontIndirect.Call(uintptr(unsafe.Pointer(&m.MessageFont)))
	fontCache[dpi] = f
	return f
}

func setFont(hwnd uintptr, font uintptr) {
	if font != 0 {
		procSendMessage.Call(hwnd, wmSetFont, font, 1)
	}
}

// create, bir alt kontrol olusturur ve arayuz fontunu uygular.
func create(class, text string, style uint32, exStyle uint32, r Rect, parent uintptr, id int, font uintptr) uintptr {
	h, _, _ := procCreateWindowEx.Call(
		uintptr(exStyle),
		uintptr(unsafe.Pointer(utf16(class))),
		uintptr(unsafe.Pointer(utf16(text))),
		uintptr(style|wsChild|wsVisible),
		uintptr(r.X), uintptr(r.Y), uintptr(r.W), uintptr(r.H),
		parent, uintptr(id), instance(), 0)
	setFont(h, font)
	return h
}

func move(hwnd uintptr, r Rect) {
	procMoveWindow.Call(hwnd, uintptr(r.X), uintptr(r.Y), uintptr(r.W), uintptr(r.H), 1)
}

func clientSize(hwnd uintptr) (int32, int32) {
	var r winRect
	procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
	return r.Right - r.Left, r.Bottom - r.Top
}

// setText, metni yalnizca degistiyse yazar. Ana pencerenin durum blogu
// iki saniyede bir tazeleniyor; her seferinde yazmak gorunur bir titreme
// yaratiyordu.
func setText(hwnd uintptr, s string) {
	if textOf(hwnd) == s {
		return
	}
	procSendMessage.Call(hwnd, wmSetText, 0, uintptr(unsafe.Pointer(utf16(s))))
}

func textOf(hwnd uintptr) string {
	n, _, _ := procSendMessage.Call(hwnd, wmGetTextLen, 0, 0)
	if n == 0 {
		return ""
	}
	buf := make([]uint16, n+1)
	procSendMessage.Call(hwnd, wmGetText, uintptr(len(buf)), uintptr(unsafe.Pointer(&buf[0])))
	return windows.UTF16ToString(buf)
}

// enable, kontrolu etkinlestirir veya devre disi birakir. Durum zaten
// istenen haldeyse dokunmuyor: odaklanmis bir kontrolu devre disi
// birakmak odagi kaydiriyor ve zamanlayicidan her cagrildiginda
// kullanicinin klavye gezinmesini bozardi.
func enable(hwnd uintptr, on bool) {
	if isEnabled(hwnd) == on {
		return
	}
	var v uintptr
	if on {
		v = 1
	}
	procEnableWindow.Call(hwnd, v)
}

func isEnabled(hwnd uintptr) bool {
	r, _, _ := procIsWindowEnabled.Call(hwnd)
	return r != 0
}

func setChecked(hwnd uintptr, on bool) {
	var v uintptr
	if on {
		v = 1
	}
	procSendMessage.Call(hwnd, bmSetCheck, v, 0)
}

func isChecked(hwnd uintptr) bool {
	r, _, _ := procSendMessage.Call(hwnd, bmGetCheck, 0, 0)
	return r == 1
}

func focus(hwnd uintptr) { procSetFocus.Call(hwnd) }

// selectAll, metin alanindaki yaziyi secer. Yanlis kod girildiginde
// kullanicinin silmeden ustune yazabilmesi icin.
func selectAll(hwnd uintptr) {
	procSendMessage.Call(hwnd, emSetSel, 0, ^uintptr(0))
}

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

// lvSetRows, listeyi bosaltip yeniden doldurur.
func lvSetRows(hwnd uintptr, rows []Row) {
	procSendMessage.Call(hwnd, lvmDeleteAllItems, 0, 0)
	for i, row := range rows {
		for j, cell := range row.Cells {
			it := lvItem{
				Mask:     lvifText,
				IItem:    int32(i),
				ISubItem: int32(j),
				PszText:  utf16(cell),
			}
			msg := uintptr(lvmSetItem)
			if j == 0 {
				msg = lvmInsertItem
			}
			procSendMessage.Call(hwnd, msg, 0, uintptr(unsafe.Pointer(&it)))
		}
	}
}

// lvSetColumnWidths, sutun genisliklerini yeniden olcekler. Pencere
// baska bir ekrana tasindiginda cagriliyor.
func lvSetColumnWidths(hwnd uintptr, widths []int32, dpi uint32) {
	for i, w := range widths {
		procSendMessage.Call(hwnd, lvmSetColumnWidth, uintptr(i), uintptr(Scale(dpi, w)))
	}
}

// osPointer, isletim sisteminden gelen ham bir adresi Go isaretcisine
// cevirir.
//
// go vet bu donusumu "possible misuse of unsafe.Pointer" diye
// isaretler: uintptr'den isaretciye gecisi genel olarak guvensiz
// sayiyor, cunku arada bir adres hesabi yapilirsa cop toplayici bunu
// goremez. Burada boyle bir hesap yok; adres dogrudan Windows'tan
// geliyor ve gecerliligini o garanti ediyor. Paketteki butun bu tur
// donusumler bu tek fonksiyondan geciyor, boylece uyari da tek kaliyor
// ve digerleri gozden kacmiyor.
func osPointer(addr uintptr) unsafe.Pointer { return unsafe.Pointer(addr) }

// applyMinSize, WM_GETMINMAXINFO'nun tasidigi yapiya en kucuk pencere
// olcusunu yazar. Isleyen bir sinir olmadan pencere kucultuldugunde
// kontroller ust uste biniyor.
func applyMinSize(lparam uintptr, dpi uint32) bool {
	if lparam == 0 {
		return false
	}
	mm := (*minMaxInfo)(osPointer(lparam))
	mm.MinTrackSize.X = Scale(dpi, MinW)
	mm.MinTrackSize.Y = Scale(dpi, MinH)
	return true
}

// lvSelected, secili satirin sirasini verir; secim yoksa -1.
func lvSelected(hwnd uintptr) int {
	r, _, _ := procSendMessage.Call(hwnd, lvmGetNextItem, ^uintptr(0), lvniSelected)
	return int(int32(r))
}

// center, pencereyi sahibinin ortasina yerlestirir. Sahip yoksa
// oldugu yerde birakir.
func center(child, parent uintptr) {
	if parent == 0 {
		return
	}
	var c, p winRect
	procGetWindowRect.Call(child, uintptr(unsafe.Pointer(&c)))
	procGetWindowRect.Call(parent, uintptr(unsafe.Pointer(&p)))
	x := p.Left + ((p.Right-p.Left)-(c.Right-c.Left))/2
	y := p.Top + ((p.Bottom-p.Top)-(c.Bottom-c.Top))/2
	procSetWindowPos.Call(child, 0, uintptr(x), uintptr(y), 0, 0,
		swpNoSize|swpNoZOrder|swpNoActivate)
}

func show(hwnd uintptr) {
	procShowWindow.Call(hwnd, swShow)
	procUpdateWindow.Call(hwnd)
}

func destroy(hwnd uintptr) { procDestroyWindow.Call(hwnd) }

func isWindow(hwnd uintptr) bool {
	r, _, _ := procIsWindow.Call(hwnd)
	return r != 0
}

// trim, biriken bellegi isletim sistemine geri verir.
//
// Diyaloglar kapanirken cagriliyor: QR gorseli ve calisan process
// listesi gecici olarak birkac megabayt tutuyor ve varsayilan ayarlarla
// bu tepe kalici bir tabana donusuyor. Arayuz kullanici bekledigi
// aralarda calistigi icin tam bir toplamanin maliyeti gorunmuyor.
func trim() {
	debug.FreeOSMemory()
	_ = winproc.Trim()
}

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

// info, bilgi kutusu gosterir.
func info(parent uintptr, title, body string) {
	procMessageBox.Call(parent,
		uintptr(unsafe.Pointer(utf16(body))),
		uintptr(unsafe.Pointer(utf16(title))),
		mbIconInfo)
}

// warn, uyari kutusu gosterir.
func warn(parent uintptr, title, body string) {
	procMessageBox.Call(parent,
		uintptr(unsafe.Pointer(utf16(body))),
		uintptr(unsafe.Pointer(utf16(title))),
		mbIconWarn)
}

// confirm, evet/hayir sorar. Geri donusu evet mi.
func confirm(parent uintptr, title, body string) bool {
	r, _, _ := procMessageBox.Call(parent,
		uintptr(unsafe.Pointer(utf16(body))),
		uintptr(unsafe.Pointer(utf16(title))),
		mbYesNo|mbIconWarn)
	return r == idYes
}
