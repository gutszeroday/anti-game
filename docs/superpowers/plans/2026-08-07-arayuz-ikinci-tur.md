# Arayüz ikinci tur — uygulama planı

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Anahtarı panoya kopyalanabilir yapmak, tepsi simgesinden arayüzü açmak, verilerin nerede saklandığını gösteren bir ekran eklemek.

**Architecture:** Yeni `internal/datainfo` paketi dizini tarayıp açıklamalı bir liste veriyor (saf, test edilir). `internal/ui` bir menü çubuğu ve iki yeni diyalog kazanıyor. `internal/tray` çift tıklama için `Default` alanı alıyor.

**Tech Stack:** Go 1.26.4, `golang.org/x/sys/windows`, ham Win32.

**Spec:** `docs/superpowers/specs/2026-08-07-arayuz-ikinci-tur-design.md`

## Global Constraints

- Her yeni dosya `//go:build windows` ile başlar.
- Yeni Go bağımlılığı yok; `go.mod` değişmemeli.
- Yorumlar Türkçe, ASCII harflerle. Kullanıcıya görünen metinler tam Türkçe.
- `internal/ui` iş mantığı içermez.
- **Konsol uygulaması çalıştıran her yer `CREATE_NO_WINDOW` vermek zorunda** (exe `-H=windowsgui` ile derleniyor). Bkz. `task.command`, `report.open`.
- Her görev sonunda: `go build ./... && go test ./...` temiz geçmeli. `go vet` yalnızca bilinen `applyMinSize` uyarısını vermeli.
- Derleme: `go build -ldflags "-s -w -H=windowsgui" -o bin/antigame.exe ./cmd/antigame`

---

## File Structure

| Dosya | Sorumluluk |
|---|---|
| `internal/datainfo/datainfo.go` (yeni) | Veri dizinini açıklamalı listeye çevirir |
| `internal/tray/tray.go` (değişir) | `Item.Default`, çift tıklama |
| `internal/ui/clipboard.go` (yeni) | Panoya gizli-tutulacak metin yazma |
| `internal/ui/menubar.go` (yeni) | Menü çubuğunun kurulumu |
| `internal/ui/data.go` (yeni) | Veriler diyaloğu |
| `internal/ui/layout.go` (değişir) | Alt sıra iki düğmeye iner |
| `internal/ui/window.go` (değişir) | Menü, kalkan düğmeler, yeni komutlar |
| `internal/ui/pair.go` (değişir) | "Kopyala" düğmesi |
| `cmd/antigame/main.go` (değişir) | Tepsiye "Arayüzü aç" |

---

## Task 1: `internal/datainfo`

**Files:** Create `internal/datainfo/datainfo.go`, `internal/datainfo/datainfo_test.go`

**Interfaces produces:**
- `type Kind int` — `KindConfig`, `KindKey`, `KindState`, `KindEvents`, `KindUnknown`
- `type Entry struct { Name string; Size int64; Kind Kind; Desc string }`
- `func List(dir string, people []config.Person) ([]Entry, error)`

- [ ] **Step 1: Failing test yaz**

Testler: `config.json` → `KindConfig`; `secret-p1.bin` + `Person{ID:"p1",Name:"Ali"}` → `Desc` içinde "Ali"; sahipsiz `secret-p9.bin` → "sahibi yok"; `state.json` → `KindState`; `events-2026-08.jsonl` → `KindEvents`; `rastgele.txt` → `KindUnknown`; boyut okunuyor; olmayan dizin → boş liste + `nil` hata; sıralama kararlı (tür sonra ad).

- [ ] **Step 2: Testin başarısız olduğunu doğrula** — `go test ./internal/datainfo/`

- [ ] **Step 3: `List`'i yaz**

`os.ReadDir`, dizin yoksa `os.IsNotExist` → `nil, nil`. Her giriş için ad eşlemesi:
`config.json`, `state.json`, `secret-p*.bin` (kimlik ad ile eşleştirilir), `events-*.jsonl`, aksi halde bilinmeyen. `DirEntry.Info()` ile boyut.

- [ ] **Step 4: Testleri çalıştır ve commit**

---

## Task 2: `tray` — çift tıklamada varsayılan eylem

**Files:** Modify `internal/tray/tray.go`, `internal/tray/tray_test.go`

**Interfaces produces:** `Item.Default bool`, `func defaultItem(items []Item) int` (paket içi, -1 = yok)

- [ ] **Step 1: Failing test yaz**

```go
func TestDefaultItemFindsTheMarkedOne(t *testing.T) {
	items := []Item{{Label: "a"}, {Label: "b", Default: true}, {Label: "c"}}
	if got := defaultItem(items); got != 1 {
		t.Errorf("defaultItem = %d, istenen 1", got)
	}
}

func TestDefaultItemReportsNoneWhenUnmarked(t *testing.T) {
	if got := defaultItem([]Item{{Label: "a"}}); got != -1 {
		t.Errorf("defaultItem = %d, istenen -1", got)
	}
}

func TestDefaultItemTakesTheFirstMark(t *testing.T) {
	items := []Item{{Label: "a", Default: true}, {Label: "b", Default: true}}
	if got := defaultItem(items); got != 0 {
		t.Errorf("defaultItem = %d, istenen 0", got)
	}
}

func TestDefaultItemOnEmptyList(t *testing.T) {
	if got := defaultItem(nil); got != -1 {
		t.Errorf("defaultItem = %d, istenen -1", got)
	}
}
```

- [ ] **Step 2: Testin başarısız olduğunu doğrula**

- [ ] **Step 3: Uygula**

`Item`'a `Default bool`. `defaultItem` yaz. `wndProc`'a `wmLButtonDblClk = 0x0203` ekle: `defaultItem(curItems)` -1 değilse `go curItems[i].Run()`.

Çift tıklama tek tıklamayı da üretir; menü açılıp hemen kapanmasın diye `wmLButtonUp` artık menü açmıyor, menü yalnızca sağ tıkla açılıyor. Bu Windows'ta zaten beklenen davranış.

- [ ] **Step 4: Testleri çalıştır ve commit**

---

## Task 3: `ui/clipboard.go`

**Files:** Create `internal/ui/clipboard.go`

**Interfaces produces:** `func copySecret(owner uintptr, s string) error`

- [ ] **Step 1: Uygula**

`OpenClipboard(owner)` → `EmptyClipboard` → `GlobalAlloc(GMEM_MOVEABLE)` + `GlobalLock` ile UTF-16 kopyala → `SetClipboardData(CF_UNICODETEXT, h)`.

Ardından iki özel biçim, `RegisterClipboardFormatW` ile:
`ExcludeClipboardContentFromMonitorProcessing` (boş 1 baytlık blok yeterli) ve
`CanIncludeInClipboardHistory` (`DWORD 0`). Bunlar olmadan anahtar Win+V geçmişine ve bulut panoya düşer.

`SetClipboardData` başarılıysa bellek panoya devredilir, `GlobalFree` **çağrılmaz**.

- [ ] **Step 2: Derle, commit**

---

## Task 4: Menü çubuğu ve küçülen düğme sırası

**Files:** Create `internal/ui/menubar.go`; modify `internal/ui/layout.go`, `internal/ui/layout_test.go`, `internal/ui/window.go`

**Interfaces produces:** `func buildMenu(hwnd uintptr) error`

- [ ] **Step 1: Yerleşim testini güncelle**

`MainLayout`'tan `PeopleBtn` ve `RemoveAppBtn` çıkar. `TestMainKeepsEverythingInsideTheWindow` ve `TestMainButtonsDoNotOverlapHorizontally` iki düğmeye göre güncellenir. `TestMainSurvivesTheMinimumSize` korunur.

- [ ] **Step 2: Testin başarısız olduğunu doğrula**

- [ ] **Step 3: `layout.go`'yu güncelle** — alt sıra `WatchBtn`, `ReportBtn`.

- [ ] **Step 4: `menubar.go` yaz**

`CreateMenu`, `CreatePopupMenu`, `AppendMenuW(MF_POPUP/MF_STRING/MF_SEPARATOR)`, `SetMenu`. Öğe kimlikleri `window.go`'daki mevcut sabitlerle **aynı** (`idAdd`, `idRemoveGame`, `idPeople`, `idUninstall`) artı yeni `idDataInfo`, `idOpenFolder`, `idAbout`.

- [ ] **Step 5: `window.go`'yu güncelle**

`build()`'den iki düğme kalkar, `buildMenu(hwnd)` çağrılır. `relayout()` ve `wmDpiChanged` listelerinden iki tutamaç çıkar. `onCommand`'a `idDataInfo`, `idOpenFolder`, `idAbout` eklenir.

- [ ] **Step 6: Derle, test et, commit**

---

## Task 5: Veriler diyaloğu

**Files:** Create `internal/ui/data.go`

**Interfaces produces:** `func showData(parent uintptr, dir string)`, `func openFolder(dir string) error`

- [ ] **Step 1: `data.go` yaz**

Modal 560×420. Üstte dizin yolu (salt okunur `EDIT`, seçilip kopyalanabilsin). Liste: Dosya 190 / Boyut 80 / İçerik 250. Altta DPAPI notu. Düğmeler: Klasörü aç, Kapat.

`config.Load(dir)` ile kişi listesi alınır, `datainfo.List(dir, cfg.People)` çağrılır. Hata durum satırında.

`openFolder` → `exec.Command("explorer", dir)`; `explorer` GUI uygulaması ama tutarlılık için `CREATE_NO_WINDOW` verilir. `explorer.exe` sıfırdan farklı çıkış kodu döndürebiliyor, bu hata sayılmaz — `Start()` kullanılır.

Boyut biçimi: `< 1024` → "N B", `< 1024*1024` → "N KB", aksi halde "N,N MB".

- [ ] **Step 2: Derle, commit**

---

## Task 6: "Kopyala" düğmesi

**Files:** Modify `internal/ui/pair.go`

- [ ] **Step 1: Uygula**

"Anahtarı göster" yanına "Kopyala" düğmesi, başlangıçta pasif. "Anahtarı göster"e basılınca `enable(copyBtn, true)`.

"Kopyala" → `copySecret(m.hwnd, pairing.GroupKey(pairing.EncodeKey(s)))`. Başarılıysa durum satırına "Anahtar panoya kopyalandı. Pano geçmişine yazılmadı; gönderdikten sonra panoyu temizleyin." Hata varsa hatanın kendisi.

Yerleşim genişletilir: diyalog 470→500 px, düğmeler sığsın.

- [ ] **Step 2: Derle, commit**

---

## Task 7: Tepsiden arayüzü açma

**Files:** Modify `cmd/antigame/main.go`

- [ ] **Step 1: Uygula**

`trayItems(dir)`'in başına:

```go
{Label: "Arayüzü aç", Default: true, Run: func() {
	exe, err := os.Executable()
	if err != nil {
		tray.Info("antigame", "Program yolu bulunamadı: "+err.Error())
		return
	}
	// Arayuz ayri process: kendi mesaj dongusu ve tek-ornek kilidi var.
	// Zaten aciksa kilit alinamaz ve mevcut pencere one gelir.
	if err := exec.Command(exe).Start(); err != nil {
		tray.Info("antigame", "Arayüz açılamadı: "+err.Error())
	}
}},
```

- [ ] **Step 2: Derle, test et, commit**

---

## Task 8: Duman testi

- [ ] **Step 1:** Spec'teki 12 maddelik listeyi yürüt, otomatik doğrulanabilenleri ölç.
- [ ] **Step 2:** Bulunan kusurları düzelt, her biri kendi commit'i.
- [ ] **Step 3:** `bin/antigame.exe`'yi yeniden derle.

---

## Self-Review Notları

- **Spec kapsamı:** K1 → Task 4. K2 → Task 3 + 6. K3 → Task 2 + 7. K4 → Task 1 + 5. Hata yönetimi → her diyalogda durum satırı, tepside bilgi kutusu. Test → Task 1, 2, 4 (birim), Task 8 (duman).
- **Bağımlılık sırası:** Task 3 (pano) Task 6'dan önce; Task 1 (datainfo) Task 5'ten önce; Task 4 (yerleşim) Task 5'in düğmesini bağladığı için ondan önce.
- **Sapma yok:** tüm imzalar spec'te yazdığı gibi.
