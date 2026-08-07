# Windows arayüzü — uygulama planı

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** antigame'in bütün metin ekranlarını, aynı çekirdeği çağıran gerçek bir Win32 penceresiyle değiştirmek; CLI'yı yedek yol olarak bırakmak.

**Architecture:** Yeni `internal/ui` paketi ham Win32 syscall'larıyla pencereleri çiziyor, iş mantığını mevcut çekirdek paketlerden (`config`, `people`, `gamelist`, `pairing`, `task`, `status`, `report`, `uninstall`) çağırıyor. İki paketten saf fonksiyon çıkarılıyor; geri kalan çekirdek olduğu gibi kullanılıyor. `gate`, `tray` ve `watch` hiç değişmiyor.

**Tech Stack:** Go 1.26.4, `golang.org/x/sys/windows` (LazyDLL/LazyProc), `github.com/skip2/go-qrcode`, stok Win32 kontrolleri (`BUTTON`, `EDIT`, `STATIC`, `SysListView32`).

**Spec:** `docs/superpowers/specs/2026-08-07-windows-gui-design.md`

## Global Constraints

- Hedef platform yalnızca Windows. Her yeni dosya `//go:build windows` ile başlar.
- GUI kütüphanesi eklenmeyecek. Yeni Go bağımlılığı **yok**; `go.mod` değişmemeli.
- `internal/ui` iş mantığı içermez. Karar veren her satır çekirdek pakete ait.
- Yorumlar Türkçe, ASCII harflerle (mevcut kod tabanının kuralı: `//` yorumlarında Türkçe karakter kullanılmıyor). Kullanıcıya görünen metinler tam Türkçe.
- Mevcut 225 testin hiçbirinin iddiası değişmeyecek. Test sayısı yalnızca artabilir.
- `gate`, `tray`, `watch` paketlerine dokunulmayacak.
- Derleme komutu: `go build -ldflags "-s -w -H=windowsgui" -o bin/antigame.exe ./cmd/antigame`
- Her görev sonunda: `go build ./... && go vet ./... && go test ./...` temiz geçmeli.

---

## File Structure

| Dosya | Sorumluluk |
|---|---|
| `internal/pairing/pairing.go` (değişir) | `Check` ve `QRImage` dışa açılır; `confirmPairing` `Check`'i çağırır |
| `internal/uninstall/uninstall.go` (değişir) | `Verify` ve `Purge` dışa açılır; `run` bunları çağırır |
| `internal/ui/layout.go` (yeni) | Saf yerleşim hesabı: DPI ölçeği, ana pencere kontrol dikdörtgenleri |
| `internal/ui/rows.go` (yeni) | Saf dönüşüm: config → oyun satırları, people → kişi satırları, hata metni |
| `internal/ui/win.go` (yeni) | Win32 sarmalayıcıları: sınıf kaydı, kontrol oluşturma, font, DPI, mesaj kutusu |
| `internal/ui/ui.go` (yeni) | `Run(dir)` — tek örnek kilidi, ortak kontroller, geri düşme |
| `internal/ui/window.go` (yeni) | Ana pencere: durum bloğu, oyun listesi, başlangıç kutusu, düğmeler |
| `internal/ui/games.go` (yeni) | Oyun ekle diyaloğu |
| `internal/ui/qr.go` (yeni) | `image.Image` → HBITMAP çizimi |
| `internal/ui/pair.go` (yeni) | Eşleştirme diyaloğu (kurulum, kişi ekle, anahtar yenile ortak) |
| `internal/ui/people.go` (yeni) | Kişiler diyaloğu |
| `internal/ui/remove.go` (yeni) | Kaldırma onay diyaloğu |
| `cmd/antigame/main.go` (değişir) | Argümansız çağrı `ui.Run`'a gider; konsol bağlama |
| `cmd/antigame/console.go` (yeni) | `AttachConsole` ile terminale geri bağlanma |
| `cmd/antigame/antigame.manifest` (yeni) | Ortak Kontroller v6, PerMonitorV2, asInvoker |
| `cmd/antigame/rsrc_windows_amd64.syso` (yeni, ikili) | Derlenmiş manifest kaynağı |

---

## Task 1: `pairing` — tek atışlık kod doğrulama ve QR görüntüsü

Metin döngüsü GUI'de kullanılamaz: GUI kod alanından tek bir kod alıp tek bir cevap vermek ister. Döngünün içindeki karar mantığı saf bir fonksiyona çıkarılıyor, döngü onu çağırır hale geliyor.

**Files:**
- Modify: `internal/pairing/pairing.go:122-165` (`confirmPairing`)
- Test: `internal/pairing/pairing_test.go`

**Interfaces:**
- Produces:
  - `func Check(secret []byte, code string, now time.Time) (counter uint64, ok bool, message string)` — `ok` true ise `counter` kabul edilen kodun sayacı. `ok` false ise `message` kullanıcıya gösterilecek Türkçe açıklama (saat kayması, yanlış kayıt).
  - `func QRImage(uri string, size int) (image.Image, error)`
  - `func EncodeKey(secret []byte) string` — mevcut `encodeKey`'in dışa açılmışı; GUI anahtarı `GroupKey` ile göstermek için ham base32'ye ihtiyaç duyuyor.

- [ ] **Step 1: Failing test yaz**

`internal/pairing/pairing_test.go` sonuna:

```go
func TestCheckAcceptsValidCode(t *testing.T) {
	secret := []byte("12345678901234567890")
	now := time.Unix(1_700_000_000, 0).UTC()
	code := totp.Code(secret, now)

	counter, ok, msg := Check(secret, code, now)
	if !ok {
		t.Fatalf("gecerli kod reddedildi: %s", msg)
	}
	if counter == 0 {
		t.Error("sayac dondurulmedi")
	}
}

func TestCheckExplainsClockSkew(t *testing.T) {
	secret := []byte("12345678901234567890")
	now := time.Unix(1_700_000_000, 0).UTC()
	code := totp.Code(secret, now.Add(5*time.Minute))

	_, ok, msg := Check(secret, code, now)
	if ok {
		t.Fatal("kaymis kod kabul edildi")
	}
	if !strings.Contains(msg, "saati") {
		t.Errorf("mesaj saat kaymasindan bahsetmiyor: %q", msg)
	}
}

func TestCheckExplainsWrongEntry(t *testing.T) {
	secret := []byte("12345678901234567890")
	now := time.Unix(1_700_000_000, 0).UTC()

	_, ok, msg := Check(secret, "000000", now)
	if ok {
		t.Fatal("alakasiz kod kabul edildi")
	}
	if !strings.Contains(msg, "eşleşmiyor") {
		t.Errorf("mesaj yanlis kaydi anlatmiyor: %q", msg)
	}
}

func TestQRImageIsSquareAndNonEmpty(t *testing.T) {
	img, err := QRImage("otpauth://totp/test?secret=AAAA", 256)
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	if b.Dx() != b.Dy() || b.Dx() == 0 {
		t.Errorf("kare olmayan veya bos gorsel: %v", b)
	}
}
```

- [ ] **Step 2: Testin başarısız olduğunu doğrula**

Run: `go test ./internal/pairing/ -run 'TestCheck|TestQRImage' -v`
Expected: FAIL — `undefined: Check`, `undefined: QRImage`

- [ ] **Step 3: `Check`, `QRImage`, `EncodeKey` yaz**

`encodeKey` → `EncodeKey` olarak yeniden adlandır, paket içi çağrıları güncelle. Sonra:

```go
// Check, tek bir kodu dogrular. Metin dongusunun ve GUI'nin ortak karar
// noktasi: hangi mesajin gosterilecegini burasi soyler, kabuklar yalnizca
// gosterir.
func Check(secret []byte, code string, now time.Time) (uint64, bool, string) {
	counter, res := totp.Verify(secret, code, now, 0)
	if res == totp.ResultOK {
		return counter, true, ""
	}
	// Kod dogru anahtardan uretilmis ama pencereye girmiyorsa sorun
	// saatlerin uyusmamasidir; bunu miktariyla soylemek gerekiyor.
	if skew, ok := totp.FindSkew(secret, code, now); ok {
		yon := "ileri"
		if skew < 0 {
			yon, skew = "geri", -skew
		}
		return 0, false, fmt.Sprintf(
			"Anahtar doğru, ama kodu üreten cihazın saati %d dakika %s. "+
				"Telefonda saati otomatik ayara alın (Google Authenticator: "+
				"Ayarlar > Zaman düzeltmesi > Kodlar için saati eşitle), sonra yeni kodu girin.",
			int(skew.Round(time.Minute).Minutes()), yon)
	}
	return 0, false, "Bu kod anahtarla eşleşmiyor. Uygulamada \"anti-game\" " +
		"kaydını seçtiğinizden emin olun; başka bir hesabın kodu girilmiş olabilir."
}

// QRImage, eslestirme URI'sini kare bir gorsele cevirir. go-qrcode
// yalnizca bu paketten cagrilir (bkz. paket yorumu).
func QRImage(uri string, size int) (image.Image, error) {
	q, err := qrcode.New(uri, qrcode.Medium)
	if err != nil {
		return nil, err
	}
	return q.Image(size), nil
}
```

- [ ] **Step 4: `confirmPairing`'i `Check` üzerinden yeniden yaz**

Gövde, döngü + `Check` çağrısı + mesajı `out`'a basma haline gelir. Mevcut testlerin gördüğü davranış (yanlış kodda pes etmeme, boş satırda iptal, saat kayması mesajı) aynı kalır:

```go
func confirmPairing(r *bufio.Reader, out io.Writer, secret []byte, now func() time.Time, onReveal func() error) (uint64, error) {
	for {
		code, err := readCode(r, out, EncodeKey(secret), onReveal)
		if err != nil && code == "" {
			return 0, err
		}
		if code == "" {
			return 0, errors.New("kod girilmedi, eşleştirme iptal edildi")
		}
		counter, ok, msg := Check(secret, code, now())
		if ok {
			return counter, nil
		}
		fmt.Fprintf(out, "\n%s Çıkmak için boş bırakıp Enter.\n\n", msg)
	}
}
```

- [ ] **Step 5: Tüm pairing testlerini çalıştır**

Run: `go test ./internal/pairing/ -v`
Expected: PASS — yeni 4 test dahil hepsi. `TestConfirmPairingReportsClockSkew` ve `TestConfirmPairingReportsWrongEntry` hâlâ geçmeli.

- [ ] **Step 6: Commit**

```bash
git add internal/pairing/
git commit -m "refactor: let one function decide what a pairing code means"
```

---

## Task 2: `uninstall` — doğrulama ve silme ayrımı

**Files:**
- Modify: `internal/uninstall/uninstall.go:31-96`
- Test: `internal/uninstall/uninstall_test.go`

**Interfaces:**
- Produces:
  - `func Verify(dir, code string) (ok bool, message string, err error)`
  - `func Purge(dir string, deleteData bool) error` — görevi kaldırır; `deleteData` ise veri dizinini de siler.

- [ ] **Step 1: Failing test yaz**

`internal/uninstall/uninstall_test.go` sonuna:

```go
func TestPurgeKeepsDataWhenNotAsked(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "iz.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := purge(dir, false, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "iz.txt")); err != nil {
		t.Error("veri silinmemeliydi")
	}
}

func TestPurgeDeletesDataWhenAsked(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "iz.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := purge(dir, true, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("veri dizini silinmeliydi")
	}
}

func TestPurgeKeepsDataWhenTaskRemovalFails(t *testing.T) {
	dir := t.TempDir()
	boom := errors.New("gorev kaldirilamadi")
	if err := purge(dir, true, func() error { return boom }); !errors.Is(err, boom) {
		t.Fatalf("hata yutuldu: %v", err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Error("gorev kaldirilamadiysa veri silinmemeliydi")
	}
}
```

- [ ] **Step 2: Testin başarısız olduğunu doğrula**

Run: `go test ./internal/uninstall/ -run TestPurge -v`
Expected: FAIL — `undefined: purge`

- [ ] **Step 3: `purge`, `Purge`, `Verify` yaz**

```go
// purge, gorev kaldiriciyi disaridan alir; boylece gercek bir zamanlanmis
// gorev olusturmadan test edilebilir.
//
// Silme sirasi onemli: gorev kaldirilamadiysa veri durmali. Aksi halde
// izleyici acilista yeniden baslar ama gecmisi yok olmus olur.
func purge(dir string, deleteData bool, remove func() error) error {
	if err := remove(); err != nil {
		return err
	}
	if !deleteData {
		return nil
	}
	return os.RemoveAll(dir)
}

// Purge, zamanlanmis gorevi kaldirir ve istenirse veri dizinini siler.
func Purge(dir string, deleteData bool) error {
	return purge(dir, deleteData, task.Remove)
}

// Verify, kaldirma kodunu dogrular. Attempt basarida bir oturum acar;
// kaldirma sirasinda pratik etkisi yok, kilit ve tekrar kullanim
// korumasini yeniden yazmamak icin ayni yol kullaniliyor.
func Verify(dir, code string) (bool, string, error) {
	cfg, err := config.Load(dir)
	if err != nil {
		return false, "", err
	}
	keys, err := people.Keys(dir)
	if err != nil {
		return false, "", err
	}
	if len(keys) == 0 {
		return false, "", vault.ErrNoSecret
	}
	v := auth.Verifier{
		Dir:   dir,
		Keys:  keys,
		Grace: time.Duration(cfg.GraceMinutes) * time.Minute,
	}
	o, err := v.Attempt(code)
	if err != nil {
		return false, "", err
	}
	return o.OK, o.Message, nil
}
```

`Run` ise `run(dir, in, out, func(c string) (bool, string, error) { return Verify(dir, c) }, task.Remove)` haline gelir; `run` içindeki `os.RemoveAll(dir)` çağrısı `purge(dir, true, func() error { return nil })` ile değiştirilir (görev zaten yukarıda kaldırılmış oluyor).

- [ ] **Step 4: Testleri çalıştır**

Run: `go test ./internal/uninstall/ -v`
Expected: PASS — mevcut 3 test + yeni 3 test.

- [ ] **Step 5: Commit**

```bash
git add internal/uninstall/
git commit -m "refactor: split uninstall into verify and purge"
```

---

## Task 3: `internal/ui` saf katman — yerleşim ve satırlar

Win32 kodu başsız test edilemez. Test edilebilir olan her şey bu iki dosyada toplanıyor, pencereler yalnızca çağırıyor.

**Files:**
- Create: `internal/ui/layout.go`, `internal/ui/rows.go`
- Test: `internal/ui/layout_test.go`, `internal/ui/rows_test.go`

**Interfaces:**
- Produces:
  - `type Rect struct{ X, Y, W, H int32 }`
  - `func Scale(dpi uint32, v int32) int32` — 96 DPI'daki `v`'yi verilen DPI'ya çevirir.
  - `type MainLayout struct{ Status, Games, GamesLabel, AddBtn, RemoveBtn, AutoStart, Note, WatchBtn, ReportBtn, PeopleBtn, RemoveAppBtn Rect }`
  - `func Main(w, h int32, dpi uint32) MainLayout` — pencere istemci alanına göre kontrol dikdörtgenleri.
  - `const MinW, MinH int32` — 560, 440 (96 DPI'da).
  - `type Row struct{ Cells []string }`
  - `func GameRows(c *config.Config) []Row` — sütunlar: Ad, Exe, Tür.
  - `func PeopleRows(es []people.Entry) []Row` — sütunlar: Ad, İpucu, Anahtar.

- [ ] **Step 1: Failing test yaz — `internal/ui/layout_test.go`**

```go
//go:build windows

package ui

import "testing"

func TestScaleIsIdentityAt96DPI(t *testing.T) {
	if got := Scale(96, 100); got != 100 {
		t.Errorf("Scale(96,100) = %d, istenen 100", got)
	}
}

func TestScaleGrowsWithDPI(t *testing.T) {
	if got := Scale(144, 100); got != 150 {
		t.Errorf("Scale(144,100) = %d, istenen 150", got)
	}
}

func TestScaleHandlesZeroDPI(t *testing.T) {
	// GetDpiForWindow basarisiz olursa 0 doner; 96 varsayilmali.
	if got := Scale(0, 100); got != 100 {
		t.Errorf("Scale(0,100) = %d, istenen 100", got)
	}
}

func TestMainStacksControlsWithoutOverlap(t *testing.T) {
	l := Main(640, 520, 96)
	if l.Status.Y+l.Status.H > l.GamesLabel.Y {
		t.Error("durum blogu oyun basligiyla cakisiyor")
	}
	if l.Games.Y+l.Games.H > l.AddBtn.Y {
		t.Error("liste dugmelerle cakisiyor")
	}
	if l.AutoStart.Y+l.AutoStart.H > l.WatchBtn.Y {
		t.Error("baslangic kutusu alt dugmelerle cakisiyor")
	}
}

func TestMainKeepsButtonsInsideWindow(t *testing.T) {
	l := Main(640, 520, 96)
	for name, r := range map[string]Rect{
		"watch": l.WatchBtn, "report": l.ReportBtn,
		"people": l.PeopleBtn, "removeApp": l.RemoveAppBtn,
	} {
		if r.X+r.W > 640 || r.Y+r.H > 520 {
			t.Errorf("%s dugmesi pencere disinda: %+v", name, r)
		}
	}
}

func TestMainGrowsListWhenWindowGrows(t *testing.T) {
	small := Main(640, 520, 96)
	big := Main(640, 800, 96)
	if big.Games.H <= small.Games.H {
		t.Error("pencere buyudugunde liste buyumeli")
	}
}

func TestMainPinsButtonsToBottomWhenWindowGrows(t *testing.T) {
	big := Main(640, 800, 96)
	if big.WatchBtn.Y < 700 {
		t.Errorf("dugmeler alta sabitlenmemis: Y=%d", big.WatchBtn.Y)
	}
}
```

- [ ] **Step 2: Failing test yaz — `internal/ui/rows_test.go`**

```go
//go:build windows

package ui

import (
	"strings"
	"testing"

	"github.com/guts/antigame/internal/config"
	"github.com/guts/antigame/internal/people"
)

func TestGameRowsSeparatesLaunchersFromGames(t *testing.T) {
	c := &config.Config{Gated: []config.Game{
		{Name: "Riot Client", Exe: "RiotClientServices.exe", Launcher: true},
		{Name: "Valorant", Exe: "VALORANT.exe"},
	}}
	rows := GameRows(c)
	if len(rows) != 2 {
		t.Fatalf("%d satir, istenen 2", len(rows))
	}
	if rows[0].Cells[2] != "Başlatıcı" {
		t.Errorf("baslatici isaretlenmemis: %q", rows[0].Cells[2])
	}
	if rows[1].Cells[2] != "Oyun" {
		t.Errorf("oyun yanlis etiketlenmis: %q", rows[1].Cells[2])
	}
}

func TestGameRowsKeepsNameAndExe(t *testing.T) {
	c := &config.Config{Gated: []config.Game{{Name: "Valorant", Exe: "VALORANT.exe"}}}
	rows := GameRows(c)
	if rows[0].Cells[0] != "Valorant" || rows[0].Cells[1] != "VALORANT.exe" {
		t.Errorf("beklenmeyen satir: %v", rows[0].Cells)
	}
}

func TestGameRowsEmptyListIsEmptyNotNil(t *testing.T) {
	rows := GameRows(&config.Config{})
	if rows == nil {
		t.Error("bos liste nil degil bos dilim olmali")
	}
}

func TestPeopleRowsMarksMissingKey(t *testing.T) {
	rows := PeopleRows([]people.Entry{
		{Person: config.Person{Name: "Ali", Hint: "telefon"}, HasKey: true},
		{Person: config.Person{Name: "Ayşe"}, HasKey: false},
	})
	if rows[0].Cells[2] != "var" {
		t.Errorf("anahtar var isaretlenmemis: %q", rows[0].Cells[2])
	}
	if !strings.Contains(rows[1].Cells[2], "yok") {
		t.Errorf("eksik anahtar isaretlenmemis: %q", rows[1].Cells[2])
	}
}

func TestPeopleRowsShowsDashForEmptyHint(t *testing.T) {
	rows := PeopleRows([]people.Entry{{Person: config.Person{Name: "Ali"}, HasKey: true}})
	if rows[0].Cells[1] != "—" {
		t.Errorf("bos ipucu icin tire bekleniyordu: %q", rows[0].Cells[1])
	}
}
```

Not: `people.Entry`'nin gerçek alan adları koda bakılarak doğrulanacak; `Person`/`HasKey` varsayımı tutmuyorsa test o adlara uyarlanacak (davranış aynı kalır).

- [ ] **Step 3: Testlerin başarısız olduğunu doğrula**

Run: `go test ./internal/ui/ -v`
Expected: FAIL — paket yok / tanımsız semboller.

- [ ] **Step 4: `layout.go` ve `rows.go` yaz**

`Scale`: `if dpi == 0 { dpi = 96 }; return v * int32(dpi) / 96`.

`Main`: yukarıdan aşağı yığar — durum bloğu (4 satır yüksekliği), "Korunan oyunlar" etiketi, liste (kalan alanı yer), Ekle/Çıkar düğmeleri listenin altında sağa yaslı, başlangıç kutusu, not satırı, en altta dört düğme sıraya dizili. Kenar boşluğu 12, düğme 110×26, satır aralığı 8 — hepsi `Scale`'den geçirilir. Düğmeler `h`'den geriye doğru hesaplanır (alta sabit), liste kalan boşluğu alır.

`GameRows` / `PeopleRows`: yukarıdaki testlerin dayattığı düz dönüşüm.

- [ ] **Step 5: Testleri çalıştır**

Run: `go test ./internal/ui/ -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/ui/
git commit -m "feat: add testable layout and row math for the window"
```

---

## Task 4: `win.go` — Win32 sarmalayıcıları

Bu dosya pencereler arasında paylaşılan sıkıcı işi tek yerde toplar. Kendi testi yok (Win32 çağrıları), doğrulaması Task 5'in ekranda açılmasıdır.

**Files:**
- Create: `internal/ui/win.go`

**Interfaces:**
- Consumes: `internal/ui/layout.go` (`Scale`)
- Produces:
  - `func registerClass(name string, proc any) error` — sınıfı bir kez kaydeder (`sync.Once` ile, `tray.go`'daki desen).
  - `func create(class, text string, style uint32, r Rect, parent, id uintptr) uintptr` — kontrol oluşturur ve UI fontunu uygular.
  - `func uiFont() uintptr` — `SPI_GETNONCLIENTMETRICS` → `lfMessageFont` → `CreateFontIndirectW`, `sync.Once` ile önbellekli.
  - `func dpiOf(hwnd uintptr) uint32` — `GetDpiForWindow`, başarısızlıkta 96.
  - `func setText(hwnd uintptr, s string)` — metin değişmediyse `SetWindowTextW` çağırmaz (titreme önleme).
  - `func text(hwnd uintptr) string` — `WM_GETTEXT`.
  - `func enable(hwnd uintptr, on bool)`, `func check(hwnd uintptr, on bool)`, `func checked(hwnd uintptr) bool`
  - `func initCommonControls() error` — `ICC_LISTVIEW_CLASSES`.
  - `func lvSetColumns(hwnd uintptr, titles []string, widths []int32)`
  - `func lvSetRows(hwnd uintptr, rows []Row)` — listeyi temizler ve yeniden doldurur.
  - `func lvSelected(hwnd uintptr) int` — seçili satır, yoksa -1.
  - `func center(child, parent uintptr)` — diyaloğu sahibinin ortasına yerleştirir.

Kullanılan Win32 çağrıları: `RegisterClassExW`, `CreateWindowExW`, `DestroyWindow`, `DefWindowProcW`, `GetMessageW`, `TranslateMessage`, `DispatchMessageW`, `PostQuitMessage`, `SendMessageW`, `SetWindowTextW`, `GetWindowTextW`, `EnableWindow`, `ShowWindow`, `UpdateWindow`, `GetClientRect`, `GetWindowRect`, `SetWindowPos`, `MoveWindow`, `SetForegroundWindow`, `EnableWindow`, `GetDpiForWindow`, `SystemParametersInfoW`, `CreateFontIndirectW`, `DeleteObject`, `SetTimer`, `KillTimer`, `InitCommonControlsEx`, `FindWindowW`.

- [ ] **Step 1: `win.go`'yu yaz**

`tray.go`'daki `LazyDLL`/`LazyProc` ve `utf16()` desenini birebir izle. `wndClassEx` ve `msgStruct` yapıları `tray.go`'dakiyle aynı; `internal/ui` ayrı paket olduğu için kopyalanıyor (iki paket arasında Win32 tip paketi kurmak, iki kullanım için erken soyutlama).

- [ ] **Step 2: Derlendiğini doğrula**

Run: `go build ./... && go vet ./...`
Expected: hata yok

- [ ] **Step 3: Commit**

```bash
git add internal/ui/win.go
git commit -m "feat: wrap the Win32 calls the window needs"
```

---

## Task 5: Ana pencere

**Files:**
- Create: `internal/ui/ui.go`, `internal/ui/window.go`

**Interfaces:**
- Consumes: `layout.Main`, `rows.GameRows`, `win.go`'nun tamamı, `config.Load`, `status.Text`, `people.Summary`, `people.Keys`, `gamelist.Remove`, `task.Installed/Install/Remove`, `report.Run`, `single.Acquire`
- Produces:
  - `func Run(dir string) error` — GUI'nin tek giriş noktası. Win32 kurulumu başarısız olursa `ErrNoGUI` döner.
  - `var ErrNoGUI = errors.New("arayüz açılamadı")`

- [ ] **Step 1: `ui.go` yaz**

`Run(dir)`:
1. `single.Acquire("antigame-ui")` — başarısızsa `FindWindowW(className, nil)` ile mevcut pencereyi bul, `SetForegroundWindow`, `nil` dön.
2. `initCommonControls()` — hata → `ErrNoGUI`
3. `registerClass` — hata → `ErrNoGUI`
4. Ana pencereyi oluştur — hata → `ErrNoGUI`
5. `runtime.LockOSThread()`, mesaj döngüsü (`tray.Run`'daki desen).

- [ ] **Step 2: `window.go` yaz**

Durum: paket düzeyinde `var cur *mainWindow` (WndProc C geri çağrımıdır, kapalı değişken taşıyamaz — `tray.go`'daki aynı gerekçe). Process başına tek pencere olduğu için güvenli.

`mainWindow` alanları: `dir string`, `hwnd`, `status`, `gamesLabel`, `games`, `addBtn`, `removeBtn`, `autoStart`, `note`, `watchBtn`, `reportBtn`, `peopleBtn`, `removeAppBtn uintptr`, `lastStatus string`.

Mesaj işleme:
- `WM_CREATE`: kontrolleri oluştur, liste sütunlarını kur (`Ad` 220, `Exe` 200, `Tür` 90), `refresh()`, `SetTimer(hwnd, 1, 2000, 0)`
- `WM_TIMER`: `refresh()`
- `WM_SIZE`, `WM_DPICHANGED`: `relayout()`
- `WM_GETMINMAXINFO`: `MinW`/`MinH`'yi `Scale` ile ölçekleyip uygula
- `WM_COMMAND`: düğme kimliğine göre dallan
- `WM_DESTROY`: `KillTimer`, `PostQuitMessage`

`refresh()`:
```
cfg, err := config.Load(dir)   -> hata: note'a yaz, listeyi bosalt
setText(status, header())      -> setText degismeyeni atlar
lvSetRows(games, GameRows(cfg))
check(autoStart, installed)
enable(watchBtn, !watcherRunning())
```

`header()` `menuHeader()`'ın aynısı: izleyici durumu, `people.Summary`, başlangıç durumu, `status.Text`. `watcherRunning()` `single.Acquire(watchLock)` denemesiyle — `cmd/antigame/main.go`'daki mantık `ui` paketine kopyalanmıyor, `Run`'a parametre olarak geçiliyor:

```go
type Deps struct {
	WatcherRunning func() bool
	StartWatcher   func() error
	ExePath        func() (string, error)
}
func Run(dir string, d Deps) error
```

Böylece `ui` paketi `watchLock` sabitini ve `DETACHED_PROCESS` mantığını bilmiyor; `cmd` katmanı veriyor.

Düğme eylemleri:
- `İzleyiciyi başlat` → `d.StartWatcher()`, sonra `refresh()`
- `Rapor` → `report.Run(dir)`, hata mesaj kutusunda
- `Kişiler…` → `showPeople(hwnd, dir)` (Task 8)
- `Kaldır…` → `showRemove(hwnd, dir)` (Task 9)
- `Ekle…` → `showAddGame(hwnd, dir)` (Task 6)
- `Çıkar` → seçili satırın exe'siyle `gamelist.Remove(dir, exe)`, seçim yoksa uyarı
- Başlangıç kutusu → `task.Install(exe)` / `task.Remove()`; hata olursa `check(autoStart, !yeni)` ile geri al ve `note`'a hatayı yaz. MFA kurulu değilken kurulduysa `note`'a uyarı.

- [ ] **Step 3: Derle ve elle aç**

Run: `go build -ldflags "-H=windowsgui" -o bin/antigame-test.exe ./cmd/antigame` — henüz `main.go` bağlanmadığı için bu adımda geçici bir `cmd/uitest` yerine doğrudan Task 10'a kadar `go build ./...` ile derleme kontrolü yeterli.

Run: `go build ./... && go vet ./... && go test ./...`
Expected: hepsi geçer

- [ ] **Step 4: Commit**

```bash
git add internal/ui/
git commit -m "feat: draw the main window"
```

---

## Task 6: Oyun ekle diyaloğu

**Files:**
- Create: `internal/ui/games.go`

**Interfaces:**
- Consumes: `win.go`, `winproc.List`, `config.Load`, `gamelist.Add`
- Produces: `func showAddGame(parent uintptr, dir string) bool` — eklendiyse true (çağıran `refresh()` çağırır).

- [ ] **Step 1: `games.go` yaz**

Modal diyalog (`WS_POPUPWINDOW|WS_CAPTION`, sahibi `parent`, `EnableWindow(parent,false)` … `true`). İçerik:
- Liste: çalışan program adları. `winproc.List()` → küçük harfe göre tekilleştir → `config.Load(dir).Gated` içinde olanları ayıkla → alfabetik sırala.
- Metin alanı: "veya exe adını yazın"
- Kutu: "Bu bir başlatıcı (oyun süresi sayılmaz)"
- Düğmeler: Ekle, Vazgeç
- Durum satırı: hata mesajı

Ekle: listede seçim varsa onun adı, yoksa metin alanı. Boşsa "exe adı girin" yaz. `gamelist.Add(dir, name, exe, "")` — `name` exe'nin uzantısız hali. Hata varsa (yinelenen exe) durum satırına yaz, diyaloğu kapatma.

Not: `gamelist.Add` imzası `Add(dir, name, exe, path string) error`. Başlatıcı bayrağı `Add` üzerinden geçmiyor — ekledikten sonra `config.Load` → ilgili oyunun `Launcher` alanını ayarla → `config.Save`. (`gamelist.AddInteractive` de aynısını yapıyor; oradaki yol örnek alınacak.)

- [ ] **Step 2: Derle**

Run: `go build ./... && go vet ./... && go test ./...`

- [ ] **Step 3: Commit**

```bash
git add internal/ui/games.go
git commit -m "feat: add games from a window"
```

---

## Task 7: QR çizimi ve eşleştirme diyaloğu

**Files:**
- Create: `internal/ui/qr.go`, `internal/ui/pair.go`
- Test: `internal/ui/qr_test.go`

**Interfaces:**
- Consumes: `pairing.NewSecret`, `pairing.OTPAuthURI`, `pairing.QRImage`, `pairing.EncodeKey`, `pairing.GroupKey`, `pairing.Check`
- Produces:
  - `func dibFromImage(img image.Image) ([]byte, int32, int32)` — 32-bit BGRA piksel dizisi, genişlik, yükseklik. **Saf, test edilir.**
  - `func drawImage(hdc uintptr, img image.Image, r Rect)` — `StretchDIBits`.
  - `func showPair(parent uintptr, account string) (secret []byte, counter uint64, ok bool)` — kullanıcı vazgeçerse `ok=false`.

- [ ] **Step 1: Failing test yaz — `internal/ui/qr_test.go`**

```go
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
	img.Set(0, 0, color.RGBA{R: 0xff, A: 0xff})
	img.Set(0, 1, color.RGBA{B: 0xff, A: 0xff})
	buf, _, _ := dibFromImage(img)
	if buf[0] != 0xff {
		t.Errorf("ilk satir alt satir olmali, mavi bekleniyordu: %v", buf[:4])
	}
}
```

- [ ] **Step 2: Testin başarısız olduğunu doğrula**

Run: `go test ./internal/ui/ -run TestDIB -v`
Expected: FAIL — `undefined: dibFromImage`

- [ ] **Step 3: `qr.go` yaz**

`dibFromImage`: `img.Bounds()` üzerinde gez, her piksel `color.RGBA` olarak alınıp `B,G,R,A` sırasıyla yazılır, satırlar alttan yukarı.

`drawImage`: `BITMAPINFOHEADER` doldur (`biBitCount=32`, `biCompression=BI_RGB`, `biHeight` pozitif = alttan yukarı), `StretchDIBits(hdc, ..., DIB_RGB_COLORS, SRCCOPY)`.

- [ ] **Step 4: Testleri çalıştır**

Run: `go test ./internal/ui/ -v`
Expected: PASS

- [ ] **Step 5: `pair.go` yaz**

`showPair(parent, account)`:
1. `pairing.NewSecret()` → `pairing.OTPAuthURI(secret, account)` → `pairing.QRImage(uri, 260)`
2. Modal pencere: QR alanı (`WM_PAINT` içinde `drawImage`), açıklama metni, kod alanı (`ES_NUMBER`, 6 hane), "Anahtarı göster" düğmesi, durum satırı, Onayla / Vazgeç.
3. Onayla → `pairing.Check(secret, kod, time.Now().UTC())`. `ok` ise kapan ve döndür; değilse `msg`'i durum satırına yaz, kod alanını seç.
4. "Anahtarı göster" → uyarı metni + `pairing.GroupKey(pairing.EncodeKey(secret))` durum alanında gösterilir. (Metin kabuğundaki `onReveal` kaydı GUI'de yok; kayıt tutulmuyorsa davranış metin akışından ayrılır — bu yüzden düğme yalnızca uyarıyı gösterir ve anahtarı açar, aynı `DİKKAT` metniyle.)
5. Anahtar yalnızca doğrulandıktan sonra çağırana verilir; diyalog iptal edilirse hiçbir şey diske yazılmaz.

- [ ] **Step 6: Derle**

Run: `go build ./... && go vet ./... && go test ./...`

- [ ] **Step 7: Commit**

```bash
git add internal/ui/qr.go internal/ui/pair.go internal/ui/qr_test.go
git commit -m "feat: pair a key without leaving the window"
```

---

## Task 8: Kişiler diyaloğu

**Files:**
- Create: `internal/ui/people.go`

**Interfaces:**
- Consumes: `people.List/Add/Edit/Rotate/Remove`, `showPair`, `PeopleRows`
- Produces: `func showPeople(parent uintptr, dir string)`

- [ ] **Step 1: `people.go` yaz**

Modal pencere. Liste (`Ad` 180, `İpucu` 200, `Anahtar` 80) + düğmeler: Ekle, Düzenle, Anahtar yenile, Sil, Kapat. Durum satırı hatalar için.

- **Ekle**: ad ve ipucu soran küçük bir alt diyalog (iki `EDIT` + Tamam/Vazgeç) → `showPair(hwnd, ad)` → `people.Add(dir, ad, ipucu, secret, counter)`
- **Düzenle**: seçili kişinin ad/ipucu alanları dolu gelen aynı alt diyalog → `people.Edit(dir, id, ad, ipucu)`
- **Anahtar yenile**: onay → `showPair(hwnd, ad)` → `people.Rotate(dir, id, secret, counter)`
- **Sil**: onay kutusu → `people.Remove(dir, id)`. Son kullanılabilir anahtarı silme reddi `people.Remove`'dan geliyor; hata metni durum satırında gösteriliyor.

Her işlemden sonra `people.List(dir)` yeniden okunup liste tazeleniyor. Seçim yoksa düğmeler pasif.

- [ ] **Step 2: Derle**

Run: `go build ./... && go vet ./... && go test ./...`

- [ ] **Step 3: Commit**

```bash
git add internal/ui/people.go
git commit -m "feat: manage keyholders from a window"
```

---

## Task 9: Kaldırma diyaloğu

**Files:**
- Create: `internal/ui/remove.go`

**Interfaces:**
- Consumes: `uninstall.Verify`, `uninstall.Purge`
- Produces: `func showRemove(parent uintptr, dir string) bool` — kaldırıldıysa true (çağıran pencereyi kapatır).

- [ ] **Step 1: `remove.go` yaz**

Modal pencere: neyin silineceğini anlatan metin, kod alanı, "Kayıtlı süre verileri de silinsin" kutusu (varsayılan kapalı), Kaldır / Vazgeç, durum satırı.

Kaldır → `uninstall.Verify(dir, kod)`. `ok` değilse `message`'ı durum satırına yaz, diyaloğu kapatma. `ok` ise `uninstall.Purge(dir, kutuİşaretli)` → başarılıysa bilgi kutusu göster ve `true` dön.

- [ ] **Step 2: Derle**

Run: `go build ./... && go vet ./... && go test ./...`

- [ ] **Step 3: Commit**

```bash
git add internal/ui/remove.go
git commit -m "feat: uninstall from a window"
```

---

## Task 10: Bağlama — manifest, konsol, `main.go`

**Files:**
- Create: `cmd/antigame/antigame.manifest`, `cmd/antigame/rsrc_windows_amd64.syso`, `cmd/antigame/console.go`
- Modify: `cmd/antigame/main.go:51-95`

**Interfaces:**
- Consumes: `ui.Run`, `ui.Deps`
- Produces: `func attachConsole()` — argümanlı çalıştırmada terminale geri bağlanır.

- [ ] **Step 1: Manifest yaz**

`cmd/antigame/antigame.manifest`:
```xml
<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<assembly xmlns="urn:schemas-microsoft-com:asm.v1" manifestVersion="1.0">
  <assemblyIdentity type="win32" name="antigame" version="1.0.0.0"
                    processorArchitecture="*"/>
  <dependency>
    <dependentAssembly>
      <assemblyIdentity type="win32"
        name="Microsoft.Windows.Common-Controls" version="6.0.0.0"
        processorArchitecture="*" publicKeyToken="6595b64144ccf1df"
        language="*"/>
    </dependentAssembly>
  </dependency>
  <trustInfo xmlns="urn:schemas-microsoft-com:asm.v3">
    <security><requestedPrivileges>
      <requestedExecutionLevel level="asInvoker" uiAccess="false"/>
    </requestedPrivileges></security>
  </trustInfo>
  <application xmlns="urn:schemas-microsoft-com:asm.v3">
    <windowsSettings>
      <dpiAwareness xmlns="http://schemas.microsoft.com/SMI/2016/WindowsSettings">PerMonitorV2</dpiAwareness>
      <dpiAware xmlns="http://schemas.microsoft.com/SMI/2005/WindowsSettings">true/pm</dpiAware>
    </windowsSettings>
  </application>
  <compatibility xmlns="urn:schemas-microsoft-com:compatibility.v1">
    <application>
      <supportedOS Id="{8e0f7a12-bfb3-4fe8-b9a5-48fd50a15a9a}"/>
    </application>
  </compatibility>
</assembly>
```

- [ ] **Step 2: `.syso` üret ve repoya koy**

```bash
go run github.com/akavel/rsrc@latest -manifest cmd/antigame/antigame.manifest -arch amd64 -o cmd/antigame/rsrc_windows_amd64.syso
```

`go run ...@latest` `go.mod`'a bağımlılık eklemez. Üretilen `.syso` commit edilir; `go.sum` ve `go.mod` değişmemeli — commit öncesi `git diff go.mod go.sum` boş olmalı.

- [ ] **Step 3: `console.go` yaz**

```go
//go:build windows

package main

// attachConsole, -H=windowsgui ile derlenen exe'yi cagiran terminale
// geri baglar. Yonlendirme veya boru varsa stdout zaten gecerlidir;
// o durumda dokunmuyoruz, aksi halde dosyaya yazilan cikti konsola kacar.
func attachConsole() {
	if h, err := windows.GetStdHandle(windows.STD_OUTPUT_HANDLE); err == nil && h != 0 {
		return
	}
	if err := procAttachConsole.Call(attachParentProcess); ... {
		return
	}
	reopen(&os.Stdout, "CONOUT$", os.O_WRONLY)
	reopen(&os.Stderr, "CONOUT$", os.O_WRONLY)
	reopen(&os.Stdin, "CONIN$", os.O_RDONLY)
}
```

`ATTACH_PARENT_PROCESS` = `^uintptr(0)` (yani `DWORD(-1)`).

- [ ] **Step 4: `main.go`'yu bağla**

```go
func main() {
	if len(os.Args) < 2 {
		if err := ui.Run(config.Dir(), uiDeps()); err == nil {
			return
		}
		// Arayuz acilamadiysa program kullanilamaz hale gelmemeli:
		// eski metin menusu yedek yol olarak duruyor.
		attachConsole()
		if err := menu.Run(os.Stdin, os.Stdout, menuHeader, menuItems()); err != nil {
			fmt.Fprintf(os.Stderr, "hata: %v\n", err)
			os.Exit(1)
		}
		return
	}
	attachConsole()
	... (mevcut switch aynen)
}

func uiDeps() ui.Deps {
	return ui.Deps{
		WatcherRunning: watcherRunning,
		StartWatcher:   func() error { return runWatch(false) },
		ExePath:        os.Executable,
	}
}
```

- [ ] **Step 5: Derle ve testleri çalıştır**

```bash
go build -ldflags "-s -w -H=windowsgui" -o bin/antigame.exe ./cmd/antigame
go vet ./...
go test ./...
git diff --exit-code go.mod go.sum
```
Expected: hepsi temiz, `go.mod`/`go.sum` değişmemiş.

- [ ] **Step 6: Commit**

```bash
git add cmd/antigame/
git commit -m "feat: open the window when antigame is double-clicked"
```

---

## Task 11: Duman testi ve belgeler

**Files:**
- Modify: `docs/superpowers/specs/2026-08-07-windows-gui-design.md` (spec'teki `pairing.Confirm` adı `pairing.Check` olarak düzeltilir)

- [ ] **Step 1: Elle duman testi**

Spec'teki 16 maddelik listeyi sırayla yürüt. Her madde için sonucu not al.

- [ ] **Step 2: Bulunan kusurları düzelt**

Her düzeltme kendi commit'i.

- [ ] **Step 3: Spec'i gerçeğe uydur**

`pairing.Confirm` → `pairing.Check`; `uninstall.Verify/Purge` imzaları gerçekleşen halleriyle yazılır.

- [ ] **Step 4: Commit**

```bash
git add docs/
git commit -m "docs: match the spec to what shipped"
```

---

## Self-Review Notları

- **Spec kapsamı:** K1 → Task 3-9 (ham Win32). K2 → Task 10 (menü yedek yol olarak duruyor). K3 → Task 10 (`console.go`, build bayrağı). K4 → Task 10 (manifest + `.syso`). Mimari/çıkarmalar → Task 1-2. Pencereler → Task 5-9. Hata yönetimi → Task 5 (`ErrNoGUI` geri düşmesi), her diyalogda durum satırı. Test → Task 3, 7 (birim), Task 11 (duman). Taşınabilirlik → Task 10 build komutu, Task 11 belge.
- **Sapma:** Spec `pairing.Confirm` diyordu; plan `pairing.Check` kullanıyor. Gerekçe: GUI'nin ihtiyacı döngü değil tek atışlık karar. Task 11 spec'i düzeltiyor.
- **Sapma:** Spec `uninstall.Verify(dir, code) error` diyordu; plan `(bool, string, error)` kullanıyor — mevcut `verifyFunc` tipi zaten bu şekilde ve mesaj kullanıcıya gösterilmeli.
- **Sapma:** `ui.Run` spec'te `Run(dir)` idi; plan `Run(dir, Deps)` kullanıyor, böylece `watchLock` ve `DETACHED_PROCESS` bilgisi `cmd` katmanında kalıyor.
