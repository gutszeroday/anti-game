# Oyunsuz Kod Girişi ve Kalan Süre Göstergesi — Uygulama Planı

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Kullanıcı oyunu açmadan kod girebilsin; oturumun ne zaman düşeceği tepsi simgesine sol tıkla ve tooltip'te görünsün.

**Architecture:** Üç katman. (1) `internal/status` yeni `Brief` fonksiyonuyla tek satırlık, duruma duyarlı kalan-süre metni üretir — hesabın tek kaynağı burasıdır. (2) `internal/tray` seçenek yapısına döner: sol tık geri çağrımı ve 60 saniyede bir tooltip tazeleyen zamanlayıcı. (3) `internal/gate` oyun adı olmadan da açılabilen `RunManual` kazanır; tepsi menüsü ve ana pencere onu ayrı process olarak (`antigame gate --manual`) başlatır.

**Tech Stack:** Go 1.x, ham Win32 (`golang.org/x/sys/windows`), GUI kütüphanesi yok.

## Global Constraints

- Spec: `docs/superpowers/specs/2026-08-08-kod-girisi-ve-kalan-sure-design.md`
- Kod yorumları ve kullanıcıya görünen metinler Türkçe; kaynak kodda ASCII yorum (Türkçe karakter yalnızca kullanıcıya görünen string'lerde).
- GUI kütüphanesi eklenmez. Yeni bağımlılık eklenmez.
- Bellek bütçesi önemlidir: yeni goroutine/timer sayısı asgari tutulur.
- Win32 kodu `//go:build windows` etiketli dosyalarda kalır; saf mantık (süre biçimi, durum seçimi) etiketsiz ve test edilebilir olmalıdır.
- Testler: `go test ./...` tamamı geçmeli. `go vet ./...` mevcut tek `unsafe.Pointer` uyarısının ötesine geçmemeli.
- Commit mesajları Conventional Commits, İngilizce, sonunda `Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>`.

---

### Task 1: `status.Brief` — tek satırlık kalan süre metni

**Files:**
- Modify: `internal/status/status.go`
- Test: `internal/status/status_test.go`

**Interfaces:**
- Consumes: `session.Remaining(st, now, grace, launcherWindow)`, `config.Load`, `store.LoadState`
- Produces:
  - `func Brief(dir string, now time.Time) (string, error)`
  - `func fmtDur(d time.Duration) string` (paket içi)

- [ ] **Step 1: Write the failing tests**

`internal/status/status_test.go` sonuna:

```go
func brief(t *testing.T, mutate func(*store.State)) string {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Default()
	cfg.GraceMinutes = 10
	cfg.LauncherWindowMinutes = 10
	if err := config.Save(dir, cfg); err != nil {
		t.Fatal(err)
	}
	st, _ := store.LoadState(dir)
	if mutate != nil {
		mutate(st)
	}
	if err := store.SaveState(dir, st); err != nil {
		t.Fatal(err)
	}
	s, err := Brief(dir, t0)
	if err != nil {
		t.Fatalf("Brief: %v", err)
	}
	return s
}

func TestBriefClosedSession(t *testing.T) {
	if s := brief(t, nil); !strings.Contains(s, "kod gerekiyor") {
		t.Errorf("kapali oturum icin kod istenmeli: %q", s)
	}
}

func TestBriefWaitsForTheGameToStart(t *testing.T) {
	// Kod 2 dakika once girildi, oyun hic acilmadi: 8 dakika kaldi.
	s := brief(t, func(st *store.State) {
		session.Open(st, t0.Add(-2*time.Minute), "")
	})
	if !strings.Contains(s, "Oyunu açmak için") || !strings.Contains(s, "8 dk") {
		t.Errorf("oyun bekleniyor metni yanlis: %q", s)
	}
}

func TestBriefSaysGameIsRunning(t *testing.T) {
	// LastGameSeen taze: izleyici oyunu bu tur gordu.
	s := brief(t, func(st *store.State) {
		session.Open(st, t0.Add(-30*time.Minute), "")
		st.Session.LastSeen = t0
		st.Session.LastGameSeen = t0
	})
	if !strings.Contains(s, "Oyun açık") {
		t.Errorf("oyun calisiyor metni yanlis: %q", s)
	}
}

func TestBriefCountsDownAfterTheGameCloses(t *testing.T) {
	// Oyun 4 dakika once son gorulmus: 6 dakika kaldi.
	s := brief(t, func(st *store.State) {
		session.Open(st, t0.Add(-30*time.Minute), "")
		st.Session.LastSeen = t0.Add(-4 * time.Minute)
		st.Session.LastGameSeen = t0.Add(-4 * time.Minute)
	})
	if !strings.Contains(s, "Tekrar kod istenene kadar") || !strings.Contains(s, "6 dk") {
		t.Errorf("oyun kapandi metni yanlis: %q", s)
	}
}

func TestFmtDurShrinksTheUnitWithTheRemainder(t *testing.T) {
	for _, c := range []struct {
		d    time.Duration
		want string
	}{
		{40 * time.Second, "40 sn"},
		{2*time.Minute + 30*time.Second, "2 dk 30 sn"},
		{11 * time.Minute, "11 dk"},
	} {
		if got := fmtDur(c.d); got != c.want {
			t.Errorf("fmtDur(%v) = %q, istenen %q", c.d, got, c.want)
		}
	}
}
```

- [ ] **Step 2: Run the tests and watch them fail**

Run: `go test ./internal/status/`
Expected: FAIL — `undefined: Brief`, `undefined: fmtDur`

- [ ] **Step 3: Implement `Brief` and `fmtDur`**

`internal/status/status.go` içine (mevcut `Text`'in üstüne):

```go
// freshWindow, "oyun su an calisiyor" saymak icin LastGameSeen'in ne kadar
// taze olmasi gerektigidir. Izleyici her turda tazeliyor; uc tur pay
// birakmak, tek bir yavas turun oyunu kapanmis gostermesini engeller.
func freshWindow(cfg *config.Config) time.Duration {
	d := 3 * time.Duration(cfg.PollMS) * time.Millisecond
	if d < 15*time.Second {
		d = 15 * time.Second
	}
	return d
}

// fmtDur, kalan sureyi kisa ve asagi yuvarlayarak yazar. Yukari
// yuvarlamak kalan sureyi oldugundan uzun gosterir; kullanici ona
// guvenip oyunu gec acardi.
func fmtDur(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	m := int(d / time.Minute)
	s := int((d % time.Minute) / time.Second)
	switch {
	case m == 0:
		return fmt.Sprintf("%d sn", s)
	case m > 10:
		return fmt.Sprintf("%d dk", m)
	default:
		return fmt.Sprintf("%d dk %d sn", m, s)
	}
}

// Brief, kalan sureyi tek satirda anlatir. Tepsi tooltip'i 128 karakterle
// sinirli oldugu icin metin tek satir ve kisa tutuluyor.
func Brief(dir string, now time.Time) (string, error) {
	cfg, err := config.Load(dir)
	if err != nil {
		return "", err
	}
	st, err := store.LoadState(dir)
	if err != nil {
		return "", err
	}
	return brief(cfg, st, now), nil
}

func brief(cfg *config.Config, st *store.State, now time.Time) string {
	grace := time.Duration(cfg.GraceMinutes) * time.Minute
	launcherWindow := time.Duration(cfg.LauncherWindowMinutes) * time.Minute
	left := session.Remaining(st, now, grace, launcherWindow)
	if left <= 0 {
		return "Oturum kapalı — oyun açmak için kod gerekiyor."
	}
	// Gercek bir oyun hic gorulmediyse LastGameSeen, Open'in yazdigi
	// OpenedAt olarak durur; baslatici onu ilerletmez.
	if st.Session.LastGameSeen.Equal(st.Session.OpenedAt) {
		return "Oyunu açmak için " + fmtDur(left) + "."
	}
	if now.Sub(st.Session.LastGameSeen) <= freshWindow(cfg) {
		return "Oyun açık — kapatırsan " + fmtDur(grace) + " sonra kod istenir."
	}
	return "Tekrar kod istenene kadar " + fmtDur(left) + "."
}
```

`Text` içindeki oturum bloğu aynı kaynağı kullanacak şekilde değişir:

```go
	if left := session.Remaining(st, now, grace, launcherWindow); left > 0 {
		fmt.Fprintf(&b, "Oturum: açık%s\n%s\n", openedBy(cfg, st), brief(cfg, st, now))
	} else {
		b.WriteString("Oturum: kapalı\nListedeki bir oyunu açmak için arkadaşınızdan kod gerekiyor.\n")
	}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/status/`
Expected: PASS. Mevcut `Text` testleri de geçmeli; "dakika sonra düşer" metnini arayan eski bir test varsa yeni cümleye göre güncellenir.

- [ ] **Step 5: Commit**

```bash
git add internal/status
git commit -m "feat: say in one line when the session drops"
```

---

### Task 2: `gate.RunManual` — oyun açmadan kod girişi

**Files:**
- Modify: `internal/gate/gate.go`
- Modify: `cmd/antigame/main.go`
- Test: `internal/gate/gate_test.go`

**Interfaces:**
- Consumes: `session.Active`, `store.LoadState`, `config.Load`, `people.Keys`, `gate.Show(Params)`
- Produces:
  - `var ErrSessionOpen = errors.New("oturum zaten açık")`
  - `func RunManual(dir string) error`
  - `func title(app string) string`, `func prompt(app string) string` (paket içi, boş `AppName` için metin üretir)

- [ ] **Step 1: Write the failing tests**

`internal/gate/gate_test.go` sonuna:

```go
func TestRunManualRefusesWhenTheSessionIsOpen(t *testing.T) {
	dir := t.TempDir()
	if err := config.Save(dir, config.Default()); err != nil {
		t.Fatal(err)
	}
	st, _ := store.LoadState(dir)
	session.Open(st, time.Now().UTC(), "")
	if err := store.SaveState(dir, st); err != nil {
		t.Fatal(err)
	}
	if err := RunManual(dir); !errors.Is(err, ErrSessionOpen) {
		t.Errorf("acik oturumda RunManual = %v, istenen ErrSessionOpen", err)
	}
}

func TestManualTextsDoNotNameAGame(t *testing.T) {
	if got := title(""); !strings.Contains(got, "Kod") {
		t.Errorf("manuel baslik yanlis: %q", got)
	}
	if strings.Contains(prompt(""), "  ") {
		t.Errorf("bos oyun adi metinde bosluk birakiyor: %q", prompt(""))
	}
	if got := title("Valorant"); !strings.Contains(got, "Valorant") {
		t.Errorf("oyun adli baslik yanlis: %q", got)
	}
}
```

- [ ] **Step 2: Run the tests and watch them fail**

Run: `go test ./internal/gate/`
Expected: FAIL — `undefined: RunManual`, `undefined: title`

- [ ] **Step 3: Implement**

`internal/gate/gate.go`:

```go
// ErrSessionOpen, oturum zaten acikken manuel kapinin acilmadigini
// soyler. Acik oturumda kod istemek kullaniciyi bosuna arkadasina
// gonderirdi.
var ErrSessionOpen = errors.New("oturum zaten açık")

// title, pencere basligini uretir. Manuel giriste oyun adi yoktur.
func title(app string) string {
	if app == "" {
		return "Kod girişi"
	}
	return fmt.Sprintf("%s kapıda durduruldu", app)
}

// prompt, ust satiri uretir.
func prompt(app string) string {
	if app == "" {
		return "Oyun açmadan kod girebilirsiniz."
	}
	return fmt.Sprintf("%s açılmadan önce MFA kodu gerekiyor.", app)
}

// RunManual, oyun acilmadan kod girmek icin kapiyi acar. Kod gecerliyse
// oturum acilir ve kullanicinin oyunu baslatmak icin odemesiz sure kadar
// vakti olur.
func RunManual(dir string) error {
	st, err := store.LoadState(dir)
	if err != nil {
		return err
	}
	cfg, err := config.Load(dir)
	if err != nil {
		return err
	}
	grace := time.Duration(cfg.GraceMinutes) * time.Minute
	launcherWindow := time.Duration(cfg.LauncherWindowMinutes) * time.Minute
	if session.Active(st, time.Now().UTC(), grace, launcherWindow) {
		return ErrSessionOpen
	}
	return Run(dir, "")
}
```

`Show` içindeki iki satır fonksiyonlara devredilir:

```go
	title := title(p.AppName)
	...
	prompt := prompt(p.AppName)
```

(Yerel değişken adı fonksiyon adını gölgeliyorsa yerel adı `titleText` / `promptText` yap.)

`cmd/antigame/main.go` içindeki `case "gate":` bloğu:

```go
	case "gate":
		if slices.Contains(os.Args[2:], "--manual") {
			err = gate.RunManual(config.Dir())
			// Oturum zaten aciksa yapacak bir sey yok; kullaniciya
			// bunu cagiran taraf zaten soyluyor.
			if errors.Is(err, gate.ErrSessionOpen) {
				err = nil
			}
			break
		}
		app := "Oyun"
		if len(os.Args) >= 4 && os.Args[2] == "--app" {
			app = os.Args[3]
		}
		err = gate.Run(config.Dir(), app)
```

`usage` metnine satır eklenir:

```
  antigame gate --manual      Oyun açmadan kod gir
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/gate/ && go build ./...`
Expected: PASS, derleme temiz.

- [ ] **Step 5: Commit**

```bash
git add internal/gate cmd/antigame
git commit -m "feat: let the code go in before the game does"
```

---

### Task 3: `tray.Options` — sol tık ve canlı tooltip

**Files:**
- Modify: `internal/tray/tray.go`
- Modify: `cmd/antigame/main.go`
- Test: `internal/tray/tray_test.go`

**Interfaces:**
- Consumes: `status.Brief`, `gate` alt komutu (`antigame gate --manual`)
- Produces: `type Options struct { Tip string; TipFunc func() string; Items []Item; OnClick func() }`, `func Run(ctx context.Context, o Options) error`

- [ ] **Step 1: Write the failing test**

`internal/tray/tray_test.go` sonuna:

```go
func TestTipTextPrefersTipFunc(t *testing.T) {
	o := Options{Tip: "sabit", TipFunc: func() string { return "canlı" }}
	if got := o.tipText(); got != "canlı" {
		t.Errorf("tipText = %q, istenen \"canlı\"", got)
	}
	o.TipFunc = nil
	if got := o.tipText(); got != "sabit" {
		t.Errorf("TipFunc yokken tipText = %q, istenen \"sabit\"", got)
	}
}

func TestTipTextTrimsToTheTooltipLimit(t *testing.T) {
	long := strings.Repeat("a", 300)
	o := Options{TipFunc: func() string { return long }}
	if n := len([]rune(o.tipText())); n > 127 {
		t.Errorf("tooltip %d karakter, en fazla 127 olmali", n)
	}
}
```

- [ ] **Step 2: Run the test and watch it fail**

Run: `go test ./internal/tray/`
Expected: FAIL — `undefined: Options`

- [ ] **Step 3: Implement**

`internal/tray/tray.go`:

```go
// Options, tepsi simgesinin davranisidir.
type Options struct {
	// Tip, TipFunc yokken kullanilan sabit tooltip metnidir.
	Tip string
	// TipFunc, tooltip'i tazelemek icin dakikada bir cagrilir. nil ise
	// zamanlayici hic kurulmaz: bos bir simge icin saniye saydirmak
	// bellek ve uyanma butcesini bosa harcar.
	TipFunc func() string
	Items   []Item
	// OnClick, sol tikta calisir. Cift tik varsayilan ogeyi actigi icin
	// tek tik, cift tik esigi kadar geciktirilir.
	OnClick func()
}

// tooltipMax, Shell_NotifyIcon'in SzTip alanidir: 128 uint16, sonuncusu
// sonlandirici.
const tooltipMax = 127

func (o Options) tipText() string {
	s := o.Tip
	if o.TipFunc != nil {
		s = o.TipFunc()
	}
	r := []rune(s)
	if len(r) > tooltipMax {
		r = r[:tooltipMax]
	}
	return string(r)
}
```

Paket düzeyi durum `curItems` yanına `curOpts Options` eklenir; `showMenu` ve `wndProc` `curOpts.Items` üstünden çalışır.

Yeni Win32 çağrıları:

```go
	procSetTimer           = user32.NewProc("SetTimer")
	procKillTimer          = user32.NewProc("KillTimer")
	procGetDoubleClickTime = user32.NewProc("GetDoubleClickTime")
```

Sabitler:

```go
	wmTimer      = 0x0113
	nimModify    = 0x0001
	idTipTimer   = 1 // tooltip tazeleme
	idClickTimer = 2 // tek tik / cift tik ayrimi
	tipPeriodMS  = 60_000
```

`wndProc` içinde:

```go
	case wmTrayMessage:
		switch uint32(lparam) {
		case wmRButtonUp:
			showMenu(hwnd)
			return 0
		case wmLButtonUp:
			// Cift tik once WM_LBUTTONUP uretir. Tek tikla cift tiki
			// ayirmanin tek yolu esik kadar beklemek; beklemeden
			// calistirinca hem bilgi kutusu hem arayuz aciliyordu.
			if curOpts.OnClick != nil {
				d, _, _ := procGetDoubleClickTime.Call()
				procSetTimer.Call(hwnd, idClickTimer, d, 0)
			}
			return 0
		case wmLButtonDbl:
			procKillTimer.Call(hwnd, idClickTimer)
			if i := defaultItem(curOpts.Items); i >= 0 && curOpts.Items[i].Run != nil {
				go curOpts.Items[i].Run()
			}
			return 0
		}
	case wmTimer:
		switch wparam {
		case idClickTimer:
			procKillTimer.Call(hwnd, idClickTimer)
			if curOpts.OnClick != nil {
				go curOpts.OnClick()
			}
			return 0
		case idTipTimer:
			refreshTip(hwnd)
			return 0
		}
```

`refreshTip`, `NIM_MODIFY` ile yalnızca `NIF_TIP` gönderir:

```go
// refreshTip, tooltip metnini tazeler. Basarisizlik sessizce gecilir:
// tooltip bir kolaylik, isin kendisi degil.
func refreshTip(hwnd uintptr) {
	nid := notifyIconData{
		HWnd:   windows.Handle(hwnd),
		UID:    1,
		UFlags: nifTip,
	}
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	copy(nid.SzTip[:len(nid.SzTip)-1], windows.StringToUTF16(curOpts.tipText()))
	procShellNotifyIcon.Call(nimModify, uintptr(unsafe.Pointer(&nid)))
}
```

`Run` imzası değişir; simge eklendikten sonra `TipFunc` varsa zamanlayıcı kurulur ve çıkarken `KillTimer` çağrılır:

```go
func Run(ctx context.Context, o Options) error {
	...
	curOpts = o
	...
	copy(nid.SzTip[:len(nid.SzTip)-1], windows.StringToUTF16(o.tipText()))
	...
	if o.TipFunc != nil {
		procSetTimer.Call(hwnd, idTipTimer, tipPeriodMS, 0)
		defer procKillTimer.Call(hwnd, idTipTimer)
	}
```

`cmd/antigame/main.go` çağrısı:

```go
	if err := tray.Run(ctx, tray.Options{
		Tip:     "antigame — izleyici çalışıyor",
		TipFunc: func() string { return trayTip(dir) },
		Items:   trayItems(dir),
		OnClick: func() { tray.Info("antigame", briefText(dir)) },
	}); err != nil {
```

Yardımcılar:

```go
// briefText, kalan sureyi tek satirda verir; okunamazsa nedenini soyler.
func briefText(dir string) string {
	s, err := status.Brief(dir, time.Now().UTC())
	if err != nil {
		return "Durum okunamadı: " + err.Error()
	}
	return s
}

func trayTip(dir string) string {
	return "antigame — " + briefText(dir)
}
```

`trayItems` listesine "Kod gir…" öğesi eklenir (varsayılan "Arayüzü aç" öğesinden sonra):

```go
		{Label: "Kod gir…", Run: func() {
			exe, err := os.Executable()
			if err != nil {
				tray.Info("antigame", "Program yolu bulunamadı: "+err.Error())
				return
			}
			// Oturum aciksa kapiyi hic acmiyoruz: kullanicinin kod
			// istemesi gerekmiyor, kalan sureyi soylemek yeterli.
			if s, err := status.Brief(dir, time.Now().UTC()); err == nil &&
				!strings.Contains(s, "kod gerekiyor") {
				tray.Info("antigame", "Oturum zaten açık. "+s)
				return
			}
			if err := exec.Command(exe, "gate", "--manual").Start(); err != nil {
				tray.Info("antigame", "Kod penceresi açılamadı: "+err.Error())
			}
		}},
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/tray/ ./cmd/... && go build ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tray cmd/antigame
git commit -m "feat: show the countdown from the tray icon"
```

---

### Task 4: Ana pencereye "Kod gir…" butonu

**Files:**
- Modify: `internal/ui/layout.go`
- Modify: `internal/ui/window.go`
- Test: `internal/ui/layout_test.go`

**Interfaces:**
- Consumes: `Scale`, `Main(w, h, dpi)`, `status.Brief`, `Deps.ExePath`
- Produces: `MainLayout.CodeBtn Rect`

- [ ] **Step 1: Write the failing tests**

`internal/ui/layout_test.go` içinde alt sıra testlerini üç butona genişlet:

```go
func TestMainBottomRowHasThreeButtons(t *testing.T) {
	l := Main(640, 520, 96)
	row := []Rect{l.WatchBtn, l.ReportBtn, l.CodeBtn}
	for i := 1; i < len(row); i++ {
		if row[i-1].X+row[i-1].W > row[i].X {
			t.Errorf("%d. ve %d. dugme cakisiyor", i-1, i)
		}
		if row[i].Y != row[0].Y {
			t.Errorf("%d. dugme ayni siradan cikti", i)
		}
	}
}

func TestMainBottomRowFitsAtTheMinimumWidth(t *testing.T) {
	l := Main(MinW, MinH, 96)
	if l.CodeBtn.X+l.CodeBtn.W > MinW-pad {
		t.Errorf("alt sira en kucuk genislige sigmiyor: %+v", l.CodeBtn)
	}
}
```

`TestMainKeepsEverythingInsideTheWindow` haritasına `"code": l.CodeBtn` eklenir.

- [ ] **Step 2: Run the tests and watch them fail**

Run: `go test ./internal/ui/`
Expected: FAIL — `l.CodeBtn undefined`

- [ ] **Step 3: Implement**

`internal/ui/layout.go`: `MainLayout`'a `CodeBtn Rect` eklenir ve döngü genişler:

```go
	for i, r := range []*Rect{&l.WatchBtn, &l.ReportBtn, &l.CodeBtn} {
		*r = Rect{left + int32(i)*(bw+g), btnY, bw, bh}
	}
```

`internal/ui/window.go`:
- Kimlik listesine `idCode` eklenir.
- `build()` içinde buton oluşturulur:

```go
	w.codeBtn = create("BUTTON", "Kod gir…", bsPushButton|wsTabStop, 0, z, w.hwnd, idCode, w.font)
```

- `relayout()` haritasına `w.codeBtn: l.CodeBtn` eklenir.
- `onCommand` içine:

```go
	case idCode:
		w.openManualGate()
```

- Yeni metot:

```go
// openManualGate, oyun acmadan kod girme penceresini ayri process olarak
// baslatir. Kapinin kendi mesaj dongusu var; bu pencerenin dongusu icine
// ikincisini kurmak ikisini de kilitlerdi.
func (w *mainWindow) openManualGate() {
	s, err := status.Brief(w.dir, time.Now().UTC())
	if err == nil && !strings.Contains(s, "kod gerekiyor") {
		w.setNote("Oturum zaten açık. " + s)
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
```

(Alan adları mevcut pencere yapısına göre uyarlanır: `w.dir`, `w.deps` adları `newMainWindow` içinde ne ise o kullanılır.)

- [ ] **Step 4: Run the tests**

Run: `go test ./... && go build ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ui
git commit -m "feat: add a code button to the main window"
```

---

### Task 5: Bütünsel doğrulama

**Files:**
- Modify: yalnızca test/derleme çıktısı gerektirirse

- [ ] **Step 1: Run everything**

Run: `go test ./... && go vet ./... && go build -o bin/antigame.exe ./cmd/antigame`
Expected: testler geçer, `vet` yalnızca bilinen `unsafe.Pointer` uyarısını verir.

- [ ] **Step 2: Manual check list**

- `bin\antigame.exe` çift tıkla açılır, alt sırada üç buton görünür.
- "Kod gir…" oyun adı yazmayan kapı penceresini açar.
- Geçerli kod girilir; ana pencere durumu "Oyunu açmak için ~10 dk" der.
- İzleyici çalışırken tepsi simgesine sol tık aynı satırı kutuda gösterir; fare üstünde tooltip süreyi yazar.
- Oyun açılır: satır "Oyun açık — kapatırsan 10 dk sonra kod istenir." olur.
- Oyun kapatılır: satır "Tekrar kod istenene kadar N dk." olarak sayar.
- Oturum açıkken "Kod gir…" kapıyı açmaz, kalan süreyi söyler.

- [ ] **Step 3: Commit any fixes**

```bash
git add -A
git commit -m "fix: <what the manual pass turned up>"
```

## Self-Review

- Spec kapsamı: §1 → Task 1; §2 → Task 3; §3 → Task 2 + Task 3 (tepsi) + Task 4 (pencere); §4 → Task 4; test bölümü → her görevin kendi adımları + Task 5.
- Yer tutucu yok; her adımda çalıştırılacak komut ve yazılacak kod var.
- Tip tutarlılığı: `Brief(dir, now) (string, error)` Task 1'de tanımlanır, Task 3 ve 4'te aynı imzayla çağrılır. `tray.Options` Task 3'te tanımlanır, tek çağıran `main.go` aynı görevde güncellenir. `gate.ErrSessionOpen` Task 2'de tanımlanır, `main.go` aynı görevde kullanır.
