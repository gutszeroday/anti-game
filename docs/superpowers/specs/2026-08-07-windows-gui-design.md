# Windows arayüzü — tasarım

Tarih: 2026-08-07
Durum: uygulandı. Bu belge, uygulama bittikten sonra gerçekleşen hâle
göre düzeltildi; imzalar ve ölçümler koddan alınmıştır.

Bilinen tek açık nokta: `go vet`, `internal/ui/win.go`'daki `applyMinSize`
için "possible misuse of unsafe.Pointer" uyarısı veriyor. Windows'un
WM_GETMINMAXINFO ile verdiği işaretçiyi okumanın başka yolu yok; dönüşüm
tek bir fonksiyonda tutuldu ki uyarı da tek kalsın.

## Amaç

antigame bugün metin tabanlı çalışıyor: çift tıklandığında numaralı bir menü
açılıyor, her iş bir soru-cevap ekranı. Bu tasarım, aynı işlerin tamamını
gerçek bir Windows penceresinden yapılabilir hale getiriyor.

İkinci bir amaç: programın başka bir Windows 11 makinesine taşınması. Bu
tasarım taşınabilirliği bozmuyor ve taşımanın ne gerektirdiğini yazıya
döküyor.

## Kapsam

GUI'ye taşınan işler:

- Durum görüntüleme (izleyici, anahtar sahipleri, açık oturum, günlük süre)
- Oyun listesi: görüntüle, ekle, çıkar
- Kişi yönetimi: listele, ekle, düzenle, anahtar yenile, sil
- Kurulum sihirbazı (MFA eşleştirme, QR gösterimi)
- Başlangıca ekleme / çıkarma
- Haftalık raporu açma
- İzleyiciyi başlatma
- Kaldırma (kod doğrulamalı)

Kapsam dışı: `gate` (kod giriş penceresi) ve `tray` (tepsi simgesi)
değişmiyor. `watch` (izleyici) değişmiyor.

## Kararlar

### K1 — Ham Win32, GUI kütüphanesi yok

`gate` ve `tray` zaten ham Win32 syscall'larıyla yazılmış; GUI de aynı deseni
izliyor.

Değerlendirilen alternatifler:

| Seçenek | RAM (pencere açıkken) | Exe | Yeni makinede | UI kodu |
|---|---|---|---|---|
| Ham Win32 | ~6 MB | ~5,5 MB | kopyala–çalıştır | ~2000 satır |
| WebView2 | ~150 MB | ~7 MB | Edge'e bağımlı | ~600 satır |
| Tarayıcı + localhost | tarayıcı kadar | ~5,5 MB | kopyala–çalıştır | ~400 satır |

Ham Win32 seçildi. Gerekçe: tek dosya taşınabilirliği koşulsuz korunuyor,
çalışma zamanı bağımlılığı yok, sonuç gerçekten native bir pencere. Bedeli
en çok kod yazmak — kabul edildi.

GUI penceresi ile izleyici ayrı process'ler olduğu için GUI'nin belleği
geçicidir ve 7/24 duran ayak izini etkilemez. Yine de en ucuz seçenek
tercih edildi.

### K2 — Metin ekranları kalıyor

`internal/menu`, `people.Screen`, `gamelist.AddInteractive`,
`gamelist.RemoveInteractive`, `setup.Run`, `uninstall.Run` silinmiyor.

- Argümansız çalıştırma (çift tık) → GUI
- `antigame people`, `antigame list`, `antigame setup` → metin ekranı

Gerekçe: mevcut 225 test dokunulmadan geçerli kalıyor ve GUI açılmazsa
çalışan bir yedek yol var. Bedeli iki kabuk bakmak; çekirdek tek olduğu için
tekrar eden mantık yok.

### K3 — `-H=windowsgui` + `AttachConsole`

Exe `-H=windowsgui` ile derleniyor, böylece çift tıklandığında konsol
penceresi yanıp sönmüyor.

Argümanlı çalıştırıldığında `main()` şunu yapıyor: `GetStdHandle` ile
stdout'un zaten geçerli olup olmadığına bakılıyor (yönlendirme veya boru
varsa geçerlidir). Geçerli değilse `AttachConsole(ATTACH_PARENT_PROCESS)`
çağrılıp `CONIN$`/`CONOUT$` üzerinden stdin/stdout/stderr yeniden
bağlanıyor.

Bilinen bedel: `cmd.exe`'den çalıştırıldığında kabuk hemen prompt'a döner ve
çıktı prompt'un üstüne basar. Yönlendirme ve boru etkilenmez. GUI ana kapı
olduğu için bu takas kabul edildi.

Geri dönüş yolu: `-H=windowsgui` kaldırılır, GUI modunda
`ShowWindow(GetConsoleWindow(), SW_HIDE)` çağrılır. Bu durumda çift tıkta
kısa bir konsol parlaması olur, CLI davranışı kusursuz olur.

**Bu kararın yakalanmayan sonucu (sonradan düzeltildi).** Konsolu olmayan
bir process'ten konsol uygulaması çalıştırılınca Windows *yeni* bir konsol
ve `conhost.exe` penceresi açar. `schtasks` ve `cmd /c start` konsol
uygulaması; önceden mevcut konsolu miras alıyor ve görünmüyorlardı. GUI'ye
geçince her çağrı ekranda siyah bir pencere açıp kapatmaya, üstelik
aktivasyonu çalmaya başladı — kullanıcı bunu donma olarak yaşadı.

Kural: **bu kod tabanında bir konsol uygulaması çalıştıran her yer
`CREATE_NO_WINDOW` vermek zorunda.** Uygulandığı yerler: `task.command`
(üç `schtasks` çağrısı), `report.open`, `pairing.Pair`. `task` paketinde
regresyon testi var.

### K4 — Manifest derlenmiş halde repoda

Manifest'siz ham Win32, Ortak Kontroller v5 (Windows 95 görünümü) kullanır ve
ölçekli ekranlarda bulanık çizer. Manifest şunları bildiriyor:

- `Microsoft.Windows.Common-Controls` v6.0.0.0 — temalı kontroller
- `PerMonitorV2` DPI farkındalığı — ölçekli ekranlarda net çizim
- `asInvoker` — yönetici hakkı istenmiyor (zamanlanmış görev zaten
  `LeastPrivilege` ile kuruluyor)
- `supportedOS` Windows 10/11 GUID'i

Manifest bir kez `rsrc` ile derlenip `cmd/antigame/rsrc_windows_amd64.syso`
olarak repoya konuyor. Amaç: `go build` derleme aleti gerektirmesin.

ARM64 hedefi için ikinci bir `rsrc_windows_arm64.syso` üretilmesi gerekir.

## Mimari

```
                    internal/ui/   (yeni, ham Win32)
                          |
   internal/menu/  -------+------  aynı çekirdek
   (mevcut metin)         |
                          v
   config · people · gamelist · pairing · task · status · report · vault
```

GUI hiçbir iş mantığı içermiyor; yalnızca çekirdeği çağırıyor.

### Doğrudan çağrılabilen çekirdek

Bunlar `io.Reader`/`io.Writer` almıyor, değişiklik gerektirmiyor:

- `config.Load`, `config.Save`, `config.Dir`
- `people.List`, `people.Add`, `people.Edit`, `people.Rotate`,
  `people.Remove`, `people.Keys`, `people.Ensure`, `people.Summary`
- `gamelist.Add`, `gamelist.Remove`, `gamelist.Format`
- `task.Install`, `task.Remove`, `task.Installed`
- `status.Text`
- `report.Run`
- `pairing.NewSecret`, `pairing.OTPAuthURI`, `pairing.GroupKey`
- `winproc.List`
- `single.Acquire`

### Gereken iki çıkarma

**`internal/pairing`** — karar mantığı (kod doğrulama, saat kayması tespiti,
yanlış giriş ayrımı) `confirmPairing`'in soru-cevap döngüsüne gömülüydü.
Döngü değil kararın kendisi dışa açıldı:

- `Check(secret []byte, code string, now time.Time) (counter uint64, ok bool, message string)`
- `QRImage(uri string, size int) (image.Image, error)` — pencere QR'ı kendi
  içinde çizdiği için PNG'ye gerek yok
- `EncodeKey(secret []byte) string` — eskiden unexported `encodeKey`

`confirmPairing` artık `Check`'i döngüye sarıyor; testleri değişmedi.

**`internal/uninstall`** — kod doğrulama, görev kaldırma ve dosya silme sırası
`Run()` gövdesindeydi. İkiye ayrıldı:

- `Verify(dir, code string) (ok bool, message string, err error)` — mesaj
  kullanıcıya gösterilmek zorunda, o yüzden `error` tek başına yetmiyor.
  Yan etkisi var: doğrulama bir oturum açar (kilit ve tekrar kullanım
  korumasını yeniden yazmamak için mevcut yol kullanılıyor).
- `Purge(dir string, deleteData bool) error` — görevi kaldırır, `deleteData`
  ise veri dizinini de siler. Sıra önemli: görev kaldırılamazsa veri duruyor,
  aksi halde izleyici açılışta yeniden başlar ama geçmişi yok olmuş olur.

`Run()` bu ikisini çağıran ince kabuk olarak kalıyor. Anahtar yokluğunu kod
sorulmadan önce kontrol ediyor: girilmesi imkânsız bir kod istemek boşuna.

**`internal/gamelist`** — `AddLauncher(dir, name, exe, path string) error`
eklendi. `Add`'in başlatıcı bayrağı yoktu ve pencerede o kutunun karşılığı
gerekiyordu; mevcut `Add` imzası dokunulmadan kaldı.

`setup.Run` olduğu gibi kalıyor. GUI sihirbazı aynı adımları
(`pairing` → `people.Add` → `task.Install`) kendi sırasıyla çağırıyor.
Ortak bir "kurulum akışı" soyutlaması yazılmıyor: iki kabuk için erken
soyutlama olur, her ikisi de üç çağrılık zincir.

## Pencereler

Yalnızca stok Win32 kontrolleri kullanılıyor (`BUTTON`, `EDIT`, `STATIC`,
`SysListView32`). Özel çizim yok; tema, klavye gezinme ve ekran okuyucu
desteği kontrollerden geliyor.

### Ana pencere

Başlangıç boyutu 640×520 (96 DPI'da), yeniden boyutlandırılabilir, en küçük
560×440.

```
+- antigame ---------------------------------------+
| İzleyici: çalışıyor                              |
| Anahtar: 2 kişide (Ali, Ayşe)                    |
| Oturum: açık — 38 dk kaldı (Ali açtı)            |
| Bugün: 1s 42dk                                   |
+--------------------------------------------------+
| Korunan oyunlar                                  |
| +----------------------+-------------+---------+ |
| | Ad                   | Exe         | Tür     | |
| | Riot Client          | RiotClien.. | Başlt.  | |
| | League of Legends    | LeagueCli.. | Başlt.  | |
| | Valorant             | VALORANT.e. | Oyun    | |
| +----------------------+-------------+---------+ |
|                            [Ekle...]  [Çıkar]    |
+--------------------------------------------------+
| [x] Windows açılışında başlat                    |
+--------------------------------------------------+
| [İzleyiciyi başlat] [Rapor] [Kişiler...] [Kaldır]|
+--------------------------------------------------+
```

**Canlı tazeleme.** `WM_TIMER` 2 saniyede bir tetikleniyor; durum bloğu
`status.Text` + `people.Summary` + izleyici kilidi denemesiyle yeniden
kuruluyor. Mevcut `menuHeader()` mantığının aynısı. Metin değişmediyse
`SetWindowText` çağrılmıyor (gereksiz titremeyi önlemek için).

**Başlangıç kutusu.** Açılışta `task.Installed()` okunuyor. Tıklamada
`task.Install(exePath)` veya `task.Remove()` çağrılıyor. Hata dönerse kutu
eski durumuna geri alınıyor ve altındaki durum satırında hata gösteriliyor —
kutu asla yanlış durumu göstermiyor.

MFA kurulu değilken başlangıca eklenirse, altta uyarı çıkıyor: kapı devre
dışı, yalnızca süre kaydediliyor. (Mevcut `toggleAutostart` davranışıyla
aynı.)

**"İzleyiciyi durdur" düğmesi yok.** `tray.go`'daki gerekçe burada da
geçerli: tek tıkla kapatılabilen bir izleyici, atlatmayı fark edilir bir
eylem olmaktan çıkarır.

**İzleyiciyi başlat** düğmesi `watch --background` process'ini
`DETACHED_PROCESS` ile başlatıyor (mevcut `spawnWatcher` mantığı). İzleyici
zaten çalışıyorsa düğme pasif.

### Diyaloglar

Hepsi modal, ana pencereye sahipli.

**Oyun ekle.** Çalışan program adlarının listesi (`winproc.List`, tekrarlar
ve halihazırda listedekiler ayıklanmış) + elle exe adı yazma alanı +
"Başlatıcı" kutusu. `gamelist.Add` çağrılıyor.

**Kişiler.** Liste: Ad / İpucu / Anahtar durumu. Düğmeler: Ekle, Düzenle,
Anahtar yenile, Sil. Ekle ve Anahtar yenile eşleştirme diyaloğunu açıyor.
Sil, `people.Remove`'un son kullanılabilir anahtarı silmeyi reddetmesine
güveniyor; hata mesajı diyalogda gösteriliyor.

**Eşleştirme.** Tek parça, üç yerden çağrılıyor: ilk kurulum, yeni kişi,
anahtar yenileme. İçerik: QR resmi, gruplanmış anahtar metni (elle girmek
isteyen için), altı haneli kod alanı, durum satırı. `pairing.Check` yanlış
kod, saat kayması ve yanlış giriş durumlarını ayırt ediyor; üçü de ayrı
mesajla gösteriliyor.

QR tarayıcıda açılmıyor. `go-qrcode` `image.Image` döndürüyor; `qr.go` bunu
32-bit DIB'e çevirip `StretchDIBits` ile çiziyor.

**Kaldırma.** Ne silineceğini anlatan uyarı metni + kod alanı.
`uninstall.Verify` başarılıysa `uninstall.Purge` çağrılıyor ve program
kapanıyor.

### Dosya yerleşimi

```
internal/ui/
  ui.go       Run(dir, Deps) — tek örnek kilidi, mesaj döngüsü, geri düşme
  modal.go    diyalogların ortak iskeleti
  win.go      Win32 sarmalayıcıları: sınıf, kontrol, font, DPI
  layout.go   saf yerleşim hesabı (test edilir)
  rows.go     config/people -> liste satırları (test edilir)
  window.go   ana pencere
  games.go    oyun ekle diyaloğu
  people.go   kişiler diyaloğu
  pair.go     eşleştirme diyaloğu
  remove.go   kaldırma onayı
  qr.go       QR -> HBITMAP
```

`win.go` iki şeyi merkezîleştiriyor:

- **DPI ölçeği.** Yerleşim elle hesaplanıyor (Go'da kaynak dosyası derlemek
  alet gerektirir), bu yüzden her koordinat `GetDpiForWindow`'dan gelen
  ölçekle çarpılıyor. `WM_DPICHANGED` geldiğinde yeniden yerleşim yapılıyor.
- **Font.** `SPI_GETNONCLIENTMETRICS` → `lfMessageFont` →
  `CreateFontIndirect`, sonra her kontrole `WM_SETFONT`. Yapılmazsa Windows
  varsayılan bitmap fontunu kullanır.

`InitCommonControlsEx` `ICC_LISTVIEW_CLASSES` ile bir kez çağrılıyor.

### Tek örnek

`single.Acquire("antigame-ui")` başarısız olursa yeni pencere açılmıyor;
`FindWindow` ile mevcut pencere bulunup `SetForegroundWindow` ile öne
getiriliyor ve process çıkıyor.

## Hata yönetimi

- Çekirdek çağrısı hata verirse diyalog kapanmıyor; hata diyalogun kendi
  durum satırında gösteriliyor.
- Açılışta Win32 çağrısı başarısız olursa (pencere sınıfı kaydı, ortak
  kontroller, pencere oluşturma) GUI vazgeçiyor ve `menu.Run` çağrılıyor.
  Exe hiçbir koşulda kullanılamaz hale gelmiyor.
- `schtasks` hataları ham çıktısıyla gösteriliyor; `task` paketi zaten
  `CombinedOutput`'u hataya sarıyor.

## Test

`wndProc` başsız test edilemez. Bu yüzden `internal/ui` bilerek ince
tutuluyor: mantık yok, yalnızca çağrı ve yerleşim.

**Birim testi yazılanlar:**

- `layout.go` — DPI ölçek hesabı, yeniden boyutlandırmada kontrol
  koordinatları, en küçük boyut sınırı
- `rows.go` — `config.Config` → oyun liste satırları (başlatıcı/oyun
  ayrımı, uzun adların kısaltılması); `[]people.Entry` → kişi liste satırları
  (anahtarsız kişi işaretleniyor)
- `uninstall.Verify` ve `uninstall.Purge` — çıkarmadan sonra doğrudan
- `pairing.Check` — kabul, saat kayması ve yanlış kayıt için ayrı ayrı

**Elle duman testi kontrol listesi:**

1. Çift tık → pencere açılıyor, konsol parlaması yok
2. Durum bloğu izleyici başlatılınca 2 sn içinde güncelleniyor
3. Oyun ekle → liste tazeleniyor → `config.json`'da görünüyor
4. Oyun çıkar → liste tazeleniyor
5. Başlangıç kutusu işaretle → Görev Zamanlayıcı'da `antigame-watch` var
6. Başlangıç kutusu kaldır → görev silinmiş
7. Kişi ekle → QR okunuyor → kod kabul ediliyor → kişi listede
8. Son anahtarı olan kişiyi sil → reddediliyor, mesaj görünüyor
9. Anahtar yenile → eski kod reddediliyor, yeni kod kabul ediliyor
10. Rapor → tarayıcıda açılıyor
11. %150 ölçekli ekranda yazı net, kontroller kesilmemiş
12. Pencere yeniden boyutlandırılınca liste büyüyor, düğmeler altta kalıyor
13. İkinci kez çift tık → yeni pencere açılmıyor, mevcut öne geliyor
14. `antigame list` hâlâ terminalde çalışıyor
15. `antigame report > out.txt` yönlendirmesi dosyaya yazıyor
16. Kaldırma → yanlış kod reddediliyor; doğru kod her şeyi siliyor

**Bellek.** GUI process'i geçici. Hedef: çalışma kümesi 15 MB altı. Mevcut
izleyici bellek bütçesi testi değişmiyor; GUI için ayrı bir bütçe testi
yazılmıyor (process kısa ömürlü, ölçüm gürültülü olur).

Varsayılan ayarlarla ölçülen 52 MB hedefin çok üstündeydi. İzleyicideki aynı
çare uygulandı: `debug.SetGCPercent(20)` ve her diyalog kapanışında
`debug.FreeOSMemory()` + `winproc.Trim()`. Diyaloglar (QR görseli, çalışan
process listesi) tepe yapıyor ve varsayılan ayarlarla o tepe kalıcı bir
tabana dönüşüyordu. Ölçülen sonuç: **5,6 MB çalışma kümesi**. Özel bellek
47 MB'ta kalıyor — bu Go çalışma zamanının ayırdığı adres alanı ve
izleyicinin profiliyle aynı.

**Gerileme sınırı.** Mevcut 225 testin hiçbirinin iddiası değişmiyor. İki
dosyada yalnızca çağrılan ad güncelleniyor (`confirmPairing` → `Check`,
`uninstall.Run`'un içi `Verify`/`Purge`'e ayrıldığı için testin kurduğu
senaryo aynı kalıyor). Test sayısı yalnızca artabilir.

## Taşınabilirlik

Bu tasarım tek dosya taşınabilirliğini korumak üzere kuruldu. Başka bir
Windows 11 makinesine taşımak için:

**Yeniden derlemek gerekmiyor.** `bin/antigame.exe` statik bir Go binary'si;
hedef makinede Go, DLL veya .NET gerekmiyor. Tek istisna ARM64 makineler
(`GOARCH=arm64` ile yeniden derleme ve ikinci bir `.syso` gerekir).

**Taşınamayan iki şey:**

- **Anahtar dosyaları** (`secret-p*.bin`). `internal/dpapi` bunları DPAPI ile
  şifreliyor; çözülmesi yalnızca aynı makinede ve aynı Windows kullanıcı
  hesabında mümkün. Bu kasıtlı bir tasarım: dosyanın kopyalanıp okunmasını
  engelliyor. Yeni makinede kurulum sihirbazı çalıştırılıp kişiler yeniden
  eşleştirilmeli.
- **Zamanlanmış görev.** `antigame-watch` görevi o makinenin Görev
  Zamanlayıcı'sına yazılıyor. Yeni makinede başlangıç kutusu bir kez
  işaretlenmeli.

**Taşınabilen:** `%LOCALAPPDATA%\antigame\config.json` (oyun listesi).
BOM'suz UTF-8 olarak kopyalanmalı — BOM'lu dosya sessizce yok sayılır ve
oyunlar korumasız kalır.

**Derleme komutu:**

```
go build -ldflags "-s -w -H=windowsgui" -o bin/antigame.exe ./cmd/antigame
```

## Kapsam dışı

- `gate` ve `tray` penceleri değişmiyor
- `watch` izleyicisi değişmiyor
- Tepsi menüsünün zenginleştirilmesi (ayrı iş)
- Rapor HTML'inin GUI'ye taşınması (tarayıcı bu iş için yeterli)
- Koyu tema desteği (stok kontroller sistem temasını izliyor; özel koyu tema
  ayrı iş)
