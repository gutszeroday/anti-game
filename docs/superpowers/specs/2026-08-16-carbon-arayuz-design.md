# Carbon Design Language ile Arayüz Yenileme

## Amaç

`internal/ui` altındaki ham Win32 arayüzü (ana pencere + diyaloglar) IBM
Carbon Design System'in görsel diline (renk, tipografi, düz/köşeli
bileşenler) yaklaştırmak. Kapsam yalnızca GUI; konsol menü (`internal/menu`)
ve kapı penceresi (`internal/gate`) değişmiyor.

Carbon'un kendisi (React/CSS) kullanılmıyor — WebView2 gibi ağır bir
bağımlılık eklemeden, mevcut native Win32 kontrollerini Carbon'un renk
paleti, tipografi ölçeği ve düz-köşeli bileşen diline göre elle çiziyoruz
(owner-draw). Yeni bağımlılık yok, tek dosya taşınabilirlik korunuyor.

Davranış değişmiyor — yalnızca görsel katman. Hiçbir iş mantığı
(`config`, `gamelist`, `people`, `task`, ...) dokunulmuyor.

## Tasarım Tokenleri

### Renk (Gray 10 tema, sabit — sistem temasına göre değişmiyor)

| Token | Hex | Kullanım |
|---|---|---|
| `background` | `#F4F4F4` | Ana pencere / diyalog zemini |
| `layer01` | `#FFFFFF` | Liste, input zemini |
| `textPrimary` | `#161616` | Gövde metni |
| `textSecondary` | `#525252` | Not/yardımcı metin |
| `borderSubtle` | `#E0E0E0` | Liste satır ayırıcı |
| `borderStrong` | `#8D8D8D` | Input çerçevesi (odaksız) |
| `interactive` | `#0F62FE` | Primary buton zemin, focus çerçevesi, seçili satır vurgusu |
| `hoverPrimary` | `#0353E9` | Primary buton hover |
| `activePrimary` | `#002D9C` | Primary buton basılı |
| `secondaryBg` | `#393939` | Secondary buton zemin |
| `hoverSecondary` | `#4C4C4C` | Secondary buton hover |
| `dangerBg` | `#DA1E28` | Danger buton zemin (Kaldır/Sil) |
| `hoverDanger` | `#BA1B23` | Danger buton hover |
| `disabledBg` | `#C6C6C6` | Devre dışı buton zemin |
| `disabledText` | `#8D8D8D` | Devre dışı buton metni |
| `selectedRow` | `#E0E7FF` | Liste seçili satır zemini |
| `onColor` | `#FFFFFF` | Primary/secondary/danger buton üstü metin |

### Tipografi (Segoe UI, IBM Plex yerine — Windows'ta hazır yüklü)

| Rol | Boyut | Ağırlık |
|---|---|---|
| Diyalog/pencere başlığı | 16px | Semibold (600) |
| Gövde metni | 14px | Regular (400) |
| Buton metni | 14px | Semibold (600) |
| Not / ipucu | 12px | Regular (400) |

DPI başına önbelleklenir (mevcut `fontCache` desenine ek, dört varyant).

## Bileşen Tedavileri

### Pencere / diyalog zemini

`WM_ERASEBKGND` işlenerek `background` (#F4F4F4) solid brush ile
dolduruluyor. Şu an sınıf arkaplanı `COLOR_BTNFACE` sistem fırçası —
bunun yerine paket düzeyinde önbelleklenen bir `CreateSolidBrush` kullanılır.

### Buton (owner-draw)

`BS_OWNERDRAW` stiliyle oluşturuluyor, çizim `WM_DRAWITEM` içinde
yapılıyor. Üç varyant: **primary** (mavi zemin), **secondary** (koyu gri
zemin), **danger** (kırmızı zemin, yalnızca Kaldır/Sil gibi yıkıcı
eylemlerde). Varyant, kontrolün `GWLP_USERDATA`'sında saklanıyor (owner-draw
mesajı yalnızca kontrol tutamacını taşıyor, hangi varyant olduğunu bilmek
için).

Durumlar: normal / hover / basılı / disabled / focus. Köşeler keskin
(radius yok — Carbon'un ayırt edici özelliği). Focus'ta 2px `interactive`
renginde iç çerçeve.

Hover/basılı durumları için `WM_MOUSEMOVE` (mouse enter/leave izleme,
`TrackMouseEvent`) ve `WM_LBUTTONDOWN`/`WM_LBUTTONUP` üzerinden kontrolün
durumu tutuluyor ve `InvalidateRect` ile yeniden çizim tetikleniyor.

Tek çizim fonksiyonu (`drawButton`) `internal/ui/theme.go`'da yaşıyor;
hem ana pencere hem modal `WM_DRAWITEM`'ı buna yönlendiriyor — iki yerde
aynı mantık yaşamasın diye.

### Input (EDIT)

Şu anki gömük 3D kenar (`WS_EX_CLIENTEDGE`) kaldırılıyor. Düz `WS_BORDER`
ile oluşturuluyor; zemin `WM_CTLCOLOREDIT` içinde beyaz (`layer01`) olarak
boyanıyor. Kenar rengi `SetWindowSubclass` (comctl32) ile `WM_NCPAINT`
yakalanarak elle çiziliyor: odaksızken `borderStrong` (1px), odaklıyken
`interactive` (2px). Odak değişimi `WM_SETFOCUS`/`WM_KILLFOCUS`'ta
`RedrawWindow` ile tetikleniyor.

### Checkbox

Native bırakılıyor — Carbon'un checkbox'ı zaten sade bir kare/onay
işareti, owner-draw'a değecek görsel fark yok. Kapsam dışı.

### Liste (SysListView32 — oyun/kişi listeleri)

`LVS_EX_GRIDLINES` kaldırılıyor (Carbon tablosunda tam ızgara yok, yalnızca
yatay satır ayırıcı). Header zemini beyaz, satır ayırıcı `borderSubtle`.
Seçili satır zemini `selectedRow`. Bunlar `LVM_SETTEXTBKCOLOR` /
`LVM_SETBKCOLOR` / header alt sınıflandırması ile ayarlanıyor; tam
owner-draw'a gerek yok (liste kontrolü bu API'leri destekliyor).

### Menü çubuğu

Native Win32 menü — OS çiziyor, dokunulmuyor. Kapsam dışı.

## Dosya Değişiklikleri

- **Yeni: `internal/ui/theme.go`** — renk/font sabitleri, brush/font
  önbellekleri, `drawButton`, buton hover/basılı durum takibi, edit
  subclass fonksiyonu, liste renklendirme yardımcıları.
- `internal/ui/win.go` — `create()` çağrılarına owner-draw buton ve
  subclass edit için yeni yardımcılar (`createButton(variant)`,
  `createEdit()`); yeni Win32 proc bildirimleri (`SetWindowSubclass`,
  `TrackMouseEvent`, `CreateSolidBrush`, `CreatePen`, `FillRect`,
  `DrawText` sabitleri).
- `internal/ui/window.go`, `internal/ui/modal.go` — `WM_DRAWITEM`,
  `WM_CTLCOLOREDIT`, `WM_ERASEBKGND` işleyicileri eklendi;
  `WM_MOUSEMOVE`/`WM_LBUTTONDOWN`/`WM_LBUTTONUP` butonlara yönlendiriliyor.
- `internal/ui/layout.go` — dokunulmuyor (mevcut ölçüler Carbon'un 8px
  ızgarasıyla zaten uyumlu: pad=12, gap=8).
- `internal/ui/games.go`, `people.go`, `remove.go`, `pair.go`, `qr.go`,
  `folder.go`, `data.go`, `move.go` — buton/edit oluşturma çağrıları yeni
  yardımcılara geçiyor (`m.button` → varyant parametreli), davranış
  değişmiyor.

## Test Planı

Boyama kodu (`WM_DRAWITEM`, `WM_NCPAINT`) gerçek HWND gerektirdiği için
birim testi yazılamaz — mevcut pakette de bu tür kod test edilmiyor
(pattern: `layout_test.go`, `rows_test.go` yalnızca saf mantığı test
ediyor). Yeni saf fonksiyonlar (durum → renk eşleşmesi, örn.
`buttonColors(variant, state) (bg, fg, border)`) test edilebilir ve test
ediliyor.

Görsel doğrulama: `run` skill ile uygulama başlatılıp her ekran elle
kontrol ediliyor (ana pencere, Oyun ekle, Kişiler, Kaldır, Kod eşleştirme,
QR, Klasör taşı) — DPI ölçekleme ve odak/hover durumları dahil.

## Hata Yönetimi

Mevcut desenle aynı: `CreateSolidBrush`/`CreateFontIndirect` gibi
çağrılar başarısız olursa (çok düşük ihtimal, kaynak tükenmesi dışında)
sıfır tutamaç döner ve çağıran sistem varsayılanına düşer (ör. font
oluşturulamazsa `setFont` zaten sıfırı yok sayıyor). Yeni kod aynı
"sessizce varsayılana düş" kuralını izliyor — uygulama görsel bir
bileşen yüzünden çökmemeli.

## Kapsam Dışı

İkon seti, toggle-switch stili, koyu tema / sistem temasına göre otomatik
geçiş, hareket/animasyon eğrileri, IBM Plex font gömme, konsol menü,
kapı penceresi (`internal/gate`), tepsi simgesi (`internal/tray`).
