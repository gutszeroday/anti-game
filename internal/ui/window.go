//go:build windows

package ui

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
	"unsafe"

	"github.com/guts/antigame/internal/config"
	"github.com/guts/antigame/internal/gamelist"
	"github.com/guts/antigame/internal/people"
	"github.com/guts/antigame/internal/report"
	"github.com/guts/antigame/internal/status"
	"github.com/guts/antigame/internal/task"
)

const mainClassName = "AntigameMain"

// Kontrol kimlikleri. WM_COMMAND yalnizca sayi tasidigi icin her dugmenin
// sabit bir kimligi olmali.
const (
	idAdd = 200 + iota
	idRemoveGame
	idAutoStart
	idWatch
	idReport
	idCode
	idPeople
	idNotifications
	idUninstall
	idDataInfo
	idOpenFolder
	idAbout
)

// Durum blogunun tazelenme araligi. Iki saniye, izleyici baslatildiktan
// sonra "calisiyor" yazisini beklemeden gormeye yetiyor.
const refreshTimer = 1
const refreshEveryMs = 2000

// Oyun listesinin sutunlari. Genislikler 96 DPI icin; ekran degistiginde
// yeniden olcekleniyor.
var (
	gameColumnTitles = []string{"Ad", "Exe", "Tür"}
	gameColumnWidths = []int32{220, 200, 90}
)

type mainWindow struct {
	dir  string
	deps Deps

	hwnd uintptr
	dpi  uint32
	font uintptr

	status     uintptr
	gamesLabel uintptr
	games      uintptr
	addBtn     uintptr
	removeBtn  uintptr
	autoStart  uintptr
	note       uintptr
	watchBtn   uintptr
	reportBtn  uintptr
	codeBtn    uintptr

	// cfg, son okunan yapilandirmadir; "Cikar" dugmesi secili satirin
	// exe adini buradan bulur.
	cfg *config.Config

	// summary ve hasKey onbellekli: uretmeleri her kisinin anahtar
	// dosyasini cozmeyi gerektiriyor ve bu bilgi kendiliginden
	// degismiyor, yalnizca kullanici kisi ekleyip cikardiginda.
	summary string
	hasKey  bool
}

// Ana pencere durumu paket duzeyinde: WndProc bir C geri cagrimidir ve
// kapali degisken tasiyamaz. Process basina tek ana pencere aciliyor
// (tek-ornek kilidi bunu garanti ediyor), bu yuzden guvenli.
var cur *mainWindow

var mainClass = &windowClass{name: mainClassName, proc: mainProc}

func newMainWindow(dir string, d Deps) (*mainWindow, error) {
	if err := mainClass.register(); err != nil {
		return nil, err
	}

	hwnd, _, err := procCreateWindowEx.Call(0,
		uintptr(unsafe.Pointer(utf16(mainClassName))),
		uintptr(unsafe.Pointer(utf16("antigame"))),
		wsOverlappedWindow|wsClipChildren,
		uintptr(0x80000000), uintptr(0x80000000), // CW_USEDEFAULT
		640, 560, 0, 0, instance(), 0)
	if hwnd == 0 {
		return nil, fmt.Errorf("ana pencere olusturulamadi: %w", err)
	}

	w := &mainWindow{dir: dir, deps: d, hwnd: hwnd}
	w.dpi = dpiOf(hwnd)
	w.font = uiFont(w.dpi)
	cur = w

	// Menu takilamazsa pencereyi acmiyoruz. Kisiler, Kaldir ve Veriler
	// yalnizca menuden ulasilabiliyor; menusuz bir pencere onlara giden
	// tek yolu kapatirdi. Hata ErrNoGUI'ye sarilip metin menusune
	// dusuluyor, orada hepsi duruyor.
	if err := buildMenu(hwnd); err != nil {
		destroy(hwnd)
		return nil, err
	}

	w.build()
	w.relayout()
	w.refresh()

	procSetTimer.Call(hwnd, refreshTimer, refreshEveryMs, 0)
	return w, nil
}

// build, kontrolleri olusturur. Yerlesim relayout'ta yapiliyor; burada
// verilen sifir dikdortgenler yalnizca yer tutuyor.
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

func (w *mainWindow) relayout() {
	cw, ch := clientSize(w.hwnd)
	l := Main(cw, ch, w.dpi)
	for h, r := range map[uintptr]Rect{
		w.status: l.Status, w.gamesLabel: l.GamesLabel, w.games: l.Games,
		w.addBtn: l.AddBtn, w.removeBtn: l.RemoveBtn,
		w.autoStart: l.AutoStart, w.note: l.Note,
		w.watchBtn: l.WatchBtn, w.reportBtn: l.ReportBtn, w.codeBtn: l.CodeBtn,
	} {
		move(h, r)
	}
}

// header, durum blogunun metnini kurar. Metin menusundeki basligin
// aynisi: kullanici hangi kabugu kullanirsa kullansin ayni seyi gormeli.
//
// Yalnizca ucuz kaynaklardan besleniyor; kisi ozeti onbellekten geliyor.
func (w *mainWindow) header() string {
	var b strings.Builder

	if w.watcherRunning() {
		b.WriteString("İzleyici: çalışıyor\n")
	} else {
		b.WriteString("İzleyici: durdu — aşağıdan başlatabilirsiniz\n")
	}

	if !w.hasKey {
		b.WriteString("MFA: kurulmadı — kapı devre dışı, yalnızca süre kaydediliyor\n")
		b.WriteString("Kişiler penceresinden anahtar verin.")
		return b.String()
	}
	b.WriteString(w.summary)
	b.WriteString("\n")

	s, err := status.Text(w.dir, time.Now().UTC())
	if err != nil {
		return b.String()
	}
	b.WriteString(s)
	return b.String()
}

func (w *mainWindow) watcherRunning() bool {
	return w.deps.WatcherRunning != nil && w.deps.WatcherRunning()
}

// keyReady, kapiyi acabilecek en az bir kisi var mi soyler. Bir kisinin
// bozuk anahtar dosyasi digerlerini kapinin disinda birakmamali; bu
// yuzden sayilan, cozulebilen anahtarlardir.
func keyReady(dir string) bool {
	keys, err := people.Keys(dir)
	return err == nil && len(keys) > 0
}

// tick, zamanlayicidan cagrilir. Yalnizca kendiliginden degisebilecek
// seyleri okur.
//
// Burada pahali is yapilmamali. Onceden her tik task.Installed()
// cagiriyordu; o bir schtasks process'i doguruyor ve arayuzun konsolu
// olmadigi icin ekranda iki saniyede bir konsol penceresi acilip
// kapaniyordu. Liste de her tikta bosaltilip yeniden doldurulunca
// kullanicinin secimi silinip "Cikar" dugmesi kullanilamaz hale
// geliyordu.
func (w *mainWindow) tick() {
	setText(w.status, w.header())
	enable(w.watchBtn, !w.watcherRunning())
}

// refresh, degismis olabilecek her seyi yeniden okur. Acilista ve
// kullanici bir sey degistirdikten sonra cagrilir, zamanlayicidan degil.
func (w *mainWindow) refresh() {
	// Not satiri sifirlaniyor: "oyun listesi bos" uyarisi oyun
	// eklendikten sonra ekranda kalmamali. Eylem sonrasi mesajlar
	// refresh'ten sonra yaziliyor, onlar etkilenmiyor.
	setText(w.note, "")

	w.hasKey = keyReady(w.dir)
	w.summary = people.Summary(w.dir)

	if installed, err := task.Installed(); err == nil {
		setChecked(w.autoStart, installed)
	}

	cfg, err := config.Load(w.dir)
	if err != nil {
		w.cfg = nil
		lvSetRows(w.games, nil)
		setText(w.note, "Yapılandırma okunamadı: "+err.Error())
		w.tick()
		return
	}
	w.cfg = cfg
	lvSetRows(w.games, GameRows(cfg))

	// Bos liste sessizce gecilmemeli: kullanici korumasiz oldugunu
	// bilmeli.
	if len(cfg.Gated) == 0 {
		setText(w.note, "Oyun listesi boş — hiçbir oyun kapıda durdurulmayacak, "+
			"yalnızca süre kaydedilecek.")
	}

	w.tick()
}

// setNote, alt bilgi satirini degistirir. Bir sonraki refresh onu
// silmesin diye ayri tutuluyor: hata mesajlari kullanici baska bir sey
// yapana kadar durmali.
func (w *mainWindow) setNote(s string) { setText(w.note, s) }

func (w *mainWindow) onCommand(id int) {
	switch id {
	case idAdd:
		if showAddGame(w.hwnd, w.dir) {
			w.refresh()
		}

	case idRemoveGame:
		i := lvSelected(w.games)
		if w.cfg == nil || i < 0 || i >= len(w.cfg.Gated) {
			w.setNote("Önce listeden çıkarılacak oyunu seçin.")
			return
		}
		g := w.cfg.Gated[i]
		if !confirm(w.hwnd, "Oyunu çıkar",
			g.Name+" listeden çıkarılacak ve bir daha kapıda durdurulmayacak. "+
				"Devam edilsin mi?") {
			return
		}
		if err := gamelist.Remove(w.dir, g.Exe); err != nil {
			w.setNote(err.Error())
			return
		}
		w.refresh()
		w.setNote(g.Name + " listeden çıkarıldı.")

	case idAutoStart:
		w.toggleAutoStart()

	case idWatch:
		if w.deps.StartWatcher == nil {
			return
		}
		if err := w.deps.StartWatcher(); err != nil {
			w.setNote("İzleyici başlatılamadı: " + err.Error())
			return
		}
		w.refresh()
		w.setNote("İzleyici arka planda başlatıldı.")

	case idReport:
		path, err := report.Run(w.dir)
		if err != nil {
			w.setNote("Rapor açılamadı: " + err.Error())
			return
		}
		w.setNote("Rapor tarayıcıda açıldı: " + path)

	case idCode:
		w.openManualGate()

	case idPeople:
		showPeople(w.hwnd, w.dir)
		w.refresh()

	case idNotifications:
		showNotifications(w.hwnd, w.dir)
		w.refresh()

	case idUninstall:
		if showRemove(w.hwnd, w.dir) {
			destroy(w.hwnd)
		}

	case idDataInfo:
		showData(w.hwnd, w.dir)

	case idOpenFolder:
		if err := openFolder(w.dir); err != nil {
			w.setNote("Klasör açılamadı: " + err.Error())
		}

	case idAbout:
		info(w.hwnd, "antigame",
			"antigame — oyun süresi takibi ve MFA kapısı\n\n"+
				"Korunan bir oyunu açmak için anahtarı olan birinden kod gerekiyor.\n"+
				"Süre kayıtları haftalık raporda toplanıyor.\n\n"+
				"Veri klasörü:\n"+w.dir)
	}
}

// openManualGate, oyun acmadan kod girme penceresini acar. Kapi ayri bir
// process olarak baslatiliyor: kendi mesaj dongusu var ve bu pencerenin
// dongusu icine ikincisini kurmak ikisini de kilitlerdi.
//
// Oturum zaten aciksa kapi hic acilmiyor; kullanicinin kod istemesi
// gereksiz, kalan sureyi soylemek yeterli.
func (w *mainWindow) openManualGate() {
	now := time.Now().UTC()
	if open, err := status.SessionOpen(w.dir, now); err == nil && open {
		s, err := status.Brief(w.dir, now)
		if err != nil {
			s = "Oturum zaten açık."
		}
		w.setNote(s)
		return
	}
	if w.deps.ExePath == nil {
		return
	}
	exe, err := w.deps.ExePath()
	if err != nil {
		w.setNote("Program yolu bulunamadı: " + err.Error())
		return
	}
	if err := exec.Command(exe, "gate", "--manual").Start(); err != nil {
		w.setNote("Kod penceresi açılamadı: " + err.Error())
		return
	}
	w.setNote("Kod penceresi açıldı.")
}

// toggleAutoStart, oturum acilisinda baslatan zamanlanmis gorevi kurar
// veya kaldirir.
//
// Kutu kendi kendine isaretleniyor (BS_AUTOCHECKBOX), bu yuzden islem
// basarisiz olursa geri alinmali: yanlis durum gosteren bir kutu,
// korumanin kurulu oldugunu sanmaya yol acar.
func (w *mainWindow) toggleAutoStart() {
	want := isChecked(w.autoStart)

	if !want {
		if err := task.Remove(); err != nil {
			setChecked(w.autoStart, true)
			w.setNote("Başlangıçtan çıkarılamadı: " + err.Error())
			return
		}
		w.setNote("Başlangıçtan çıkarıldı. İzleyici oturum açılışında başlamayacak.")
		return
	}

	exe, err := w.deps.ExePath()
	if err != nil {
		setChecked(w.autoStart, false)
		w.setNote("Program yolu bulunamadı: " + err.Error())
		return
	}
	if err := task.Install(exe); err != nil {
		setChecked(w.autoStart, false)
		w.setNote("Başlangıca eklenemedi: " + err.Error())
		return
	}
	if !keyReady(w.dir) {
		w.setNote("Başlangıca eklendi. Uyarı: MFA kurulmadığı için kapı devre dışı; " +
			"izleyici yalnızca süre kaydeder.")
		return
	}
	w.setNote("Başlangıca eklendi: " + exe)
}

func mainProc(hwnd, msg, wparam, lparam uintptr) uintptr {
	w := cur
	switch msg {
	case wmCommand:
		if w != nil && w.hwnd == hwnd {
			w.onCommand(int(uint16(wparam)))
			return 0
		}

	case wmTimer:
		if w != nil && w.hwnd == hwnd && wparam == refreshTimer {
			w.tick()
			return 0
		}

	case wmSize:
		if w != nil && w.hwnd == hwnd {
			w.relayout()
			return 0
		}

	case wmDpiChanged:
		if w != nil && w.hwnd == hwnd {
			w.dpi = uint32(uint16(wparam))
			w.font = uiFont(w.dpi)
			for _, h := range []uintptr{
				w.status, w.gamesLabel, w.games, w.addBtn, w.removeBtn,
				w.autoStart, w.note, w.watchBtn, w.reportBtn,
			} {
				setFont(h, w.font)
			}
			lvSetColumnWidths(w.games, gameColumnWidths, w.dpi)
			if hdr, _, _ := procSendMessage.Call(w.games, lvmGetHeader, 0, 0); hdr != 0 {
				setFont(hdr, semiboldFont(w.dpi))
			}
			// Pencereyi yeni ekrana tasiyip olceklemeyi varsayilan isleyici
			// yapiyor; ardindan gelen WM_SIZE ile yerlesim yenileniyor.
			// lparam'daki dikdortgeni elle uygulamak ayni isi tekrarlardi.
			break
		}

	case wmDrawItem:
		di := (*drawItemStruct)(osPointer(lparam))
		if di.CtlType == odtButton {
			drawButton(di)
			return 1
		}

	case wmEraseBkgnd:
		paintBackground(wparam, hwnd)
		return 1

	case wmCtlColorStatic:
		procSetBkMode.Call(wparam, transparentBkMode)
		procSetTextColor.Call(wparam, uintptr(clrTextPrimary))
		return ensureBackgroundBrush()

	case wmGetMinMaxInfo:
		if applyMinSize(lparam, dpiOf(hwnd)) {
			return 0
		}

	case wmDestroy:
		procKillTimer.Call(hwnd, refreshTimer)
		procPostQuitMessage.Call(0)
		return 0
	}

	r, _, _ := procDefWindowProc.Call(hwnd, msg, wparam, lparam)
	return r
}
