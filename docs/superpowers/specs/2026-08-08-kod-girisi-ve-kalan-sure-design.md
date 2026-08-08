# Oyunsuz kod girişi ve kalan sürenin tepside gösterimi

Tarih: 2026-08-08

## Sorun

İki eksik var:

1. Kod yalnızca izleyici kapıda bir oyunu durdurduğunda girilebiliyor. Kod
   verecek kişi yanındayken oyun henüz açılmamışsa kodu peşinen girmenin yolu
   yok: önce oyunu açıp öldürülmesini beklemek gerekiyor.
2. Oturumun ne zaman düşeceği yalnızca tepsi menüsündeki "Durum" öğesinin
   içinde, uzun bir metnin ortasında yazıyor. Kullanıcının merak ettiği iki
   an — "oyunu açmak için ne kadar vaktim var" ve "oyunu kapattım, ne zaman
   tekrar kod istenecek" — hızlı bakılacak bir yerde değil.

## Kapsam dışı

- Süre kotası, günlük limit, oturum uzatma. Oturum modeli değişmiyor.
- Kod doğrulama mantığı (`internal/auth`) değişmiyor.
- Rapor ve olay günlüğü biçimi değişmiyor.

## Mevcut model

`session.Open` kod girildiğinde `OpenedAt`, `LastSeen`, `LastGameSeen` alanlarını
şimdiye ayarlar. `session.Remaining` iki sınırın küçüğünü döndürür:

- `LastSeen + grace` (varsayılan 10 dk): listedeki hiçbir şey çalışmıyorken
  oturumun kalan ömrü.
- `LastGameSeen + launcher_window` (varsayılan 10 dk): son gerçek oyundan sonra
  yalnızca başlatıcı çalışırken oturumun kalan ömrü.

`session.Touch(realGame=false)` yalnızca `LastSeen`'i tazeler; başlatıcı
penceresini uzatmaz.

İzleyici oturum kapalıyken her turda `state.json`'ı diskten yeniden okur
(`watch.go` Step). Kapı ayrı bir process olduğu için oturumu o açar ve izleyici
bir sonraki turda görür. Manuel kapı bu yolu aynen kullanır; ek senkron gerekmez.

## Tasarım

### 1. `internal/status`: tek satırlık durum metni

Yeni fonksiyon:

```go
func Brief(dir string, now time.Time) (string, error)
```

Duruma göre tek satır döndürür:

| Durum | Metin |
|---|---|
| Oturum kapalı | `Oturum kapalı — oyun açmak için kod gerekiyor.` |
| Oturum açık, gerçek oyun hiç görülmedi | `Oyunu açmak için 8 dk 40 sn.` |
| Gerçek oyun şu an çalışıyor | `Oyun açık — kapatırsan 10 dk sonra kod istenir.` |
| Gerçek oyun kapandı | `Tekrar kod istenene kadar 6 dk.` |

Ayrımlar:

- **Oyun hiç görülmedi**: `Session.LastGameSeen.Equal(Session.OpenedAt)`.
  `session.Open` ikisini aynı ana ayarlar ve yalnızca gerçek bir oyun görülünce
  `LastGameSeen` ilerler; başlatıcı ilerletmez.
- **Şu an çalışıyor**: `now.Sub(LastGameSeen) <= fresh`, burada
  `fresh = max(3 * poll_ms, 15s)`. İzleyici her turda `LastGameSeen`'i
  tazelediği için taze damga "oyun ayakta" demektir. Üç tur pay bırakmak,
  tek bir yavaş turun "oyun kapandı" gibi görünmesini engeller.

Süre biçimi: 1 dakikanın altında `X sn`, altında dakika varsa `X dk Y sn`,
10 dakikanın üstünde yalnızca `X dk`. Yuvarlama aşağı doğru olur; kalan süreyi
olduğundan uzun göstermek kullanıcıyı yanıltır.

`status.Text` bu hesabı `Brief`'ten devralır: oturum satırı `Brief`'in
döndürdüğü cümle olur, altındaki oyun sayısı ve son kayıt satırları kalır.
Aynı hesabın iki yerde yaşamaması için tek kaynak `Brief`'tir.

### 2. `internal/tray`: sol tık ve canlı tooltip

`Run` imzası seçenek yapısına döner:

```go
type Options struct {
    Tip      string
    TipFunc  func() string // nil ise tooltip sabit kalır
    Items    []Item
    OnClick  func()        // sol tık; nil ise tık yutulur
}

func Run(ctx context.Context, o Options) error
```

Tek çağıran `cmd/antigame/main.go` olduğu için ikinci bir giriş noktası
açılmıyor.

- `WM_LBUTTONUP` → `OnClick` (goroutine'de; mesaj döngüsü bloklanmamalı).
  Çift tık halen varsayılan öğeyi (`Arayüzü aç`) çalıştırır. Sol tık şu an
  boşta olduğu için çakışma yok: çift tık `WM_LBUTTONUP` de üretir, ama
  bilgi kutusu açılıp arayüz de gelmesi kabul edilebilir değil — bu yüzden
  sol tık, `WM_LBUTTONUP` geldiğinde çift tık eşiği (`GetDoubleClickTime`)
  kadar bekleyen bir zamanlayıcıya bağlanır; bu sürede `WM_LBUTTONDBLCLK`
  gelirse tek tık iptal edilir.
- `SetTimer` 60 sn → `TipFunc` çağrılır, dönen metin `NIM_MODIFY` ile
  tooltip'e yazılır. Tooltip 128 karakterle sınırlı; `Brief` tek satır
  olduğu için sığar, yine de kırpılır.

### 3. Oyun açmadan kod girişi

`internal/gate`:

```go
var ErrSessionOpen = errors.New("oturum zaten açık")

func RunManual(dir string) error
```

- `state.json` okunur; `session.Active` ise `ErrSessionOpen` döner ve pencere
  açılmaz. Çağıran kullanıcıya `status.Brief` metnini gösterir.
- Değilse kapı penceresi açılır. `Params.AppName` boş geçilir; boş adda
  pencere başlığı ve üst satır oyun adı yerine "Oyun açmadan kod girişi"
  yazar.
- Kod doğrulanınca akış mevcut kapıyla aynıdır: `session.Open` çağrılır,
  olay günlüğe yazılır.

`cmd/antigame`: `antigame gate --manual` alt komutu `gate.RunManual` çağırır.

Giriş noktaları:

- Tepsi menüsüne `Kod gir…` öğesi.
- Ana pencerenin alt sırasına üçüncü buton `Kod gir…`.

İkisi de kapıyı **ayrı process** olarak başlatır (`exec.Command(exe, "gate",
"--manual")`). Kapının kendi mesaj döngüsü ve tek-örnek kilidi var; çağıranın
döngüsü içine ikinci bir döngü kurmak pencereleri kilitler.

`ErrSessionOpen` ayrı process'te oluştuğu için çağıran onu doğrudan göremez.
Bu yüzden kontrol iki yerde yapılır: çağıran (tepsi/arayüz) tıklamadan önce
`status.Brief` ile bakar ve oturum açıksa bilgi kutusu gösterip kapıyı hiç
başlatmaz; `gate.RunManual` içindeki kontrol ise yarış durumuna karşı ikinci
savunmadır (sessizce çıkar).

### 4. `internal/ui`: üçüncü alt buton

`MainLayout`'a `CodeBtn` eklenir; alt sıra `WatchBtn, ReportBtn, CodeBtn`
olur. Üç buton `3*118 + 2*8 = 370 px`, en küçük pencere genişliği 560 px'e
sığar. `layout_test` üç butonu doğrulayacak şekilde güncellenir.

## Hata durumları

- `state.json` veya `config.json` okunamazsa `Brief` hata döndürür; tepsi
  bilgi kutusunda "Durum okunamadı: …" yazar (mevcut davranışla aynı).
- Kapı process'i başlatılamazsa bilgi kutusuyla bildirilir.
- Tooltip güncellemesi başarısız olursa sessizce geçilir: tooltip bir
  kolaylık, işin kendisi değil.

## Test

- `status`: `Brief` için tablo testi — kapalı oturum, oyun beklenen, oyun
  çalışıyor, oyun kapandı, tazelik sınırının iki yanı, süre biçimi
  (saniye / dk+sn / yalnız dk).
- `gate`: oturum açıkken `RunManual` `ErrSessionOpen` döndürür ve pencere
  açmaz; kapalıyken pencere kurucusuna boş `AppName` gider.
- `ui`: `layout_test` üç alt butonun çakışmadığını ve en küçük genişlikte
  pencere içinde kaldığını doğrular.
- `tray`: `Options` ile `defaultItem` davranışının korunduğu, `TipFunc` nil
  iken zamanlayıcının kurulmadığı.

Win32 mesaj işleme (sol tık / çift tık ayrımı) elle doğrulanır; mevcut tepsi
kodunda da otomatik test yok.
