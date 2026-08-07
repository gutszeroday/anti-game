# Arayüz ikinci tur — tasarım

Tarih: 2026-08-07
Durum: onaylandı, uygulanacak

Önceki tasarım: `2026-08-07-windows-gui-design.md`

## Amaç

Pencere kullanılmaya başlandıktan sonra çıkan üç eksik:

1. Eşleştirme sırasında gösterilen anahtar kopyalanamıyor — elle dikte
   etmek gerekiyor.
2. İzleyici çalışırken arayüze ulaşmanın yolu yok; tepsi simgesi var ama
   pencereyi açmıyor.
3. Neyin nereye kaydedildiği hiçbir yerde yazmıyor.

## Kapsam

Bu belge birinci adımı kapsıyor: **görüntüleme**. Veri klasörünü
değiştirmek ikinci adım ve kendi spec'i olacak — ayarın nerede duracağı,
izleyicinin ve zamanlanmış görevin yeni yeri nasıl bulacağı, taşıma
sırasında çalışan izleyicinin ne olacağı ayrı kararlar gerektiriyor.

Kapsam dışı: yedek alma, dışa/içe aktarma.

## K1 — Menü çubuğu

Alt düğme sırası dolmuştu: beşinci düğme 560 px'lik en küçük pencere
boyutunda taşıyordu. Değerlendirilen alternatifler iki sıra düğme ve
sekmeli pencereydi. Menü çubuğu seçildi: düğme sırası ileride yine
büyüyecek ve menü klavye kısayollarını bedavaya veriyor.

```
+- antigame ---------------------------------+
| Yönet    Veriler    Yardım                 |
+--------------------------------------------+
| İzleyici: çalışıyor                        |
| Kişiler: 3 kişi kapıyı açabiliyor          |
| Oturum: kapalı                             |
+--------------------------------------------+
| Korunan oyunlar                            |
| [ liste ]                                  |
|                        [Ekle...]  [Çıkar]  |
| [x] Windows açılışında başlat              |
| [İzleyiciyi başlat]  [Haftalık rapor]      |
+--------------------------------------------+
```

Menü içeriği:

| Menü | Öğeler |
|---|---|
| Yönet | Oyun ekle… / Oyun çıkar / ─ / Kişiler… / ─ / Kaldır… |
| Veriler | Nerede saklanıyor… / Klasörü aç |
| Yardım | Hakkında |

`Kişiler…` ve `Kaldır…` alt sıradan menüye taşınıyor. `Ekle…` ve `Çıkar`
listenin yanında kalıyor: ikisi de seçili satıra bağlı, menüye taşımak
bağlamlarını koparırdı.

`MainLayout`'tan `PeopleBtn` ve `RemoveAppBtn` çıkıyor; yerleşim testleri
buna göre güncelleniyor. Menü çubuğunun yüksekliği istemci alanından
düşülüyor — `GetClientRect` bunu zaten hesaba katıyor, yerleşim
matematiği değişmiyor.

Aynı eylemler hem menüden hem düğmeden gelebildiği için komut kimlikleri
paylaşılıyor: `WM_COMMAND` her ikisinde de aynı sayıyı taşıyor, işleyici
tek.

## K2 — Anahtarı panoya kopyalama

Eşleştirme diyaloğunda "Anahtarı göster"e basılınca yanındaki
**"Kopyala"** düğmesi etkinleşiyor. Anahtar gösterilmeden düğme pasif:
kopyalanacak bir şey yokken tıklanabilir olması yanıltıcı olurdu.

Panoya yazarken iki Windows bayrağı ekleniyor:

- `ExcludeClipboardContentFromMonitorProcessing`
- `CanIncludeInClipboardHistory` = 0

İkisi de `RegisterClipboardFormatW` ile kayıtlı özel biçimler. Olmadan
TOTP anahtarı Win+V pano geçmişine ve bulut pano senkronuna düşer.
Kapıyı açan bir kimlik bilgisi için bu gerçek bir sızıntı; anahtarın
diskte DPAPI ile şifrelenmesinin anlamı kalmazdı.

**Otomatik temizleme yok.** Kopyalamanın amacı anahtarı bir mesajla
göndermek; diyalog kapanınca panoyu boşaltmak özelliği işe yaramaz hale
getirirdi. Zamanlayıcıyla temizlemek de kullanıcının araya koyduğu başka
bir kopyalamayı ezme riski taşıyor. Bunun yerine diyalogda ne olduğu
yazıyor: pano geçmişine yazılmadı, gönderdikten sonra panoyu temizleyin.

## K3 — Tepsiden arayüzü açma

Tepsi menüsünün başına **"Arayüzü aç"** geliyor. Ayrıca simgeye **çift
tıklamak** aynı şeyi yapıyor: Windows'ta tepsi simgesinin beklenen
hareketi bu.

`tray.Item`'a `Default bool` alanı ekleniyor. Çift tıklama bu alanı
taşıyan ilk öğeyi çalıştırıyor. Alternatif — "listenin ilk öğesi
varsayılandır" — aynı sonucu verirdi ama sıralama değiştiğinde sessizce
başka bir eylemi çalıştırırdı.

Arayüz ayrı bir process olarak başlatılıyor: kendi mesaj döngüsü ve
tek-örnek kilidi gerekiyor. Arayüz zaten açıksa `single.Acquire`
başarısız oluyor ve mevcut pencere öne getiriliyor — ikinci pencere
açılmıyor.

Tepsi izleyicinin içinde yaşıyor; çalıştırılabilir yolu `cmd` katmanı
veriyor (`trayItems`), `tray` paketi öğrenmiyor.

## K4 — Veriler diyaloğu

```
+- Veriler -------------------------------------------+
| C:\Users\guts\AppData\Local\antigame                |
| +------------------+--------+--------------------+  |
| | Dosya            | Boyut  | İçerik             |  |
| | config.json      |   2 KB | oyun listesi,      |  |
| |                  |        | kişiler            |  |
| | secret-p1.bin    |  248 B | Ali'nin anahtarı   |  |
| | secret-p2.bin    |  248 B | Ayşe'nin anahtarı  |  |
| | state.json       |   1 KB | açık oturum,       |  |
| |                  |        | sayaçlar           |  |
| | events-2026-08…  |  84 KB | süre kayıtları     |  |
| +------------------+--------+--------------------+  |
| Anahtar dosyaları DPAPI ile şifreli: yalnızca bu    |
| makinede ve bu Windows hesabında açılır.            |
|                         [Klasörü aç]     [Kapat]    |
+-----------------------------------------------------+
```

Yeni paket `internal/datainfo`:

```go
type Kind int
const (
	KindConfig Kind = iota  // config.json
	KindKey                 // secret-p<id>.bin
	KindState               // state.json
	KindEvents              // events-YYYY-MM.jsonl
	KindUnknown             // taninmayan dosya
)

type Entry struct {
	Name   string // dosya adi
	Size   int64  // bayt
	Kind   Kind
	Desc   string // Turkce aciklama, kisi adi cozulmus halde
}

// List, dizindeki dosyalari aciklamalariyla listeler. Dizin yoksa
// bos dilim doner, hata degil.
func List(dir string, people []config.Person) ([]Entry, error)
```

Saf fonksiyon, `[]config.Person` dışarıdan geliyor — anahtar dosyasının
hangi kişiye ait olduğunu çözmek için gerekli, ama paketin `config`
okumasına gerek yok. Böylece test edilebilir.

Tanınmayan dosyalar da listeleniyor (`KindUnknown`): dizine elle bir şey
kopyalanmışsa kullanıcı görebilmeli.

Sahipsiz anahtar dosyaları — kişi listesinde karşılığı olmayan
`secret-p*.bin` — `KindKey` olarak listeleniyor ama açıklaması "sahibi
yok" oluyor. `people.Orphans` bunları zaten sayıyordu; burada
görünürlük kazanıyorlar.

`ui/data.go` yalnızca çiziyor. "Klasörü aç" `explorer.exe <dir>`
çalıştırıyor.

## Hata yönetimi

- `datainfo.List` dizin okunamıyorsa hata döndürüyor; diyalog listeyi boş
  bırakıp durum satırında sebebi gösteriyor.
- Panoya yazma başarısızsa (başka bir uygulama panoyu kilitlemişse)
  diyalogun durum satırında söyleniyor; sessizce başarılı gösterilmiyor.
- "Arayüzü aç" başarısızsa tepsi bir bilgi kutusu gösteriyor.

## Test

**Birim testi yazılanlar:**

- `datainfo.List` — her dosya türünün tanınması; kişi kimliğinin ada
  çevrilmesi; sahipsiz anahtarın "sahibi yok" olarak işaretlenmesi;
  tanınmayan dosyanın listelenmesi; boyutun okunması; olmayan dizinin
  hata değil boş liste vermesi
- `ui` yerleşimi — menü çubuğu sonrası iki düğmeli alt sıra, en küçük
  boyutta çakışma yok
- `tray` — `Default` işaretli öğenin bulunması, hiçbiri işaretli değilse
  çift tıklamanın bir şey yapmaması

**Elle duman testi:**

1. Menü çubuğu görünüyor, üç menü de açılıyor
2. Yönet → Kişiler… kişiler penceresini açıyor
3. Yönet → Oyun ekle… ekleme penceresini açıyor
4. Veriler → Nerede saklanıyor… dosyaları listeliyor, boyutlar doğru
5. Veriler → Klasörü aç Dosya Gezgini'ni açıyor
6. Eşleştirmede "Anahtarı göster" → "Kopyala" etkinleşiyor
7. Kopyalanan anahtar bir metin alanına yapıştırılabiliyor
8. Win+V pano geçmişinde anahtar **görünmüyor**
9. Tepsi menüsünde "Arayüzü aç" var ve pencereyi açıyor
10. Tepsi simgesine çift tık pencereyi açıyor
11. Pencere açıkken tepsiden tekrar açmak ikinci pencere açmıyor
12. 560×440'ta alt düğmeler ve menü çakışmıyor

## Kapsam dışı

- Veri klasörünü değiştirme (ikinci adım, ayrı spec)
- Yedek alma / dışa aktarma
- Dosyaların içeriğini pencerede düzenleme — kişiler ve oyunlar zaten
  kendi ekranlarından değiştiriliyor
