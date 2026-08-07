# Veri klasörünü değiştirme — tasarım

Tarih: 2026-08-07
Durum: onaylandı, uygulanacak

Önceki: `2026-08-07-arayuz-ikinci-tur-design.md` (Adım 1 — görüntüleme)

## Amaç

Verilerin yaşadığı klasörü kullanıcı seçebilsin.

## Çözülmesi gereken çatlak

`watch.Options.Dir` izleyiciye açılışta bir kez veriliyor ve bir daha
sorulmuyor. Kapı ise ayrı process olarak doğduğu için her çalıştığında
`config.Dir()`'i yeniden okuyor.

Klasör izleyici çalışırken değişirse: kapı yeni klasöre oturum açar,
izleyici eski klasöre bakmayı sürdürür, oturumu hiç görmez ve oyunu
öldürmeye devam eder. Özellik bu düzeltilmeden çalışmaz.

## K1 — Ayar kayıt defterinde

`HKCU\Software\antigame`, `DataDir` (REG_SZ).

Alternatif olarak varsayılan klasöre bir işaretçi dosya konabilirdi; ama o
dosya tam da taşımanın sildiği yerde yaşardı. Kayıt defteri klasör silinse
de kalıyor ve `HKCU` yönetici hakkı gerektirmiyor.

`config.Dir()` artık kayıt defterini okuyor: değer varsa ve mutlak yolsa
onu, aksi halde `%LOCALAPPDATA%\antigame`. **Önbelleğe alınmıyor** —
izleyicinin değişikliği fark etmesi buna bağlı. Kayıt defteri okuması
mikrosaniyeler sürüyor, döngüde sorun değil.

Yeni: `config.DefaultDir()`, `config.SetDir(path)`, `config.ClearDir()`.

Zamanlanmış görev değişmiyor: aynı exe'yi çalıştırıyor, o da kayıt
defterini okuyor.

## K2 — İzleyici klasörünü yeniden okuyor

`watch` döngüsünde `configCheckEvery` (5 sn) yanına klasör kontrolü
giriyor. Ayar değiştiyse izleyici durumu ve yapılandırmayı yeni klasörden
okuyup oraya yazmaya başlıyor ve `data_dir_changed` olayını oraya düşüyor.

`Options.Dir` yerine `Options.DirFunc func() string` geliyor; testler
klasörü değiştirilebilir yapabilsin diye. `w.o.Dir` kullanan her yer
`w.dir()` çağırıyor; `w.dir` alanı yalnızca kontrol anında güncelleniyor,
böylece tek bir tur içinde klasör ortadan değişmiyor.

**Klasörü kaybolan izleyici ölmemeli.** Bugün `store.Append` hata verirse
`Run` dönüyor ve koruma duruyor. Klasör yoksa izleyici ayarı yeniden okuyup
yeni yere geçiyor. Bu, taşımadan bağımsız olarak da doğru: birisi klasörü
elle silerse koruma çökmemeli.

## K3 — Taşıma sırası

```
1. Doğrula        hedef mutlak mı, kaynağın içinde mi, kaynağın
                  kendisi mi, zaten antigame verisi var mı
2. Kopyala        bütün dosyalar yeni klasöre
3. Doğrula        dosya sayısı ve boyutları birebir mi
4. Olay yaz       data_moved  ->  YENİ klasöre
5. Kayıt defteri  DataDir = yeni yol
6. Bekle          izleyici çalışıyorsa yeni klasörde data_dir_changed
                  olayı görünene kadar, en fazla 15 sn
7. Sil            eski dosyalar
```

1–3 arasında bir şey patlarsa hiçbir şey değişmiyor; eski klasör aktif
kalıyor ve yarım kopya hedefte kalsa bile kimse ona bakmıyor.

6. adım zaman aşımına uğrarsa **silme yapılmıyor** ve kullanıcıya eski
klasörün durduğu söyleniyor. Kör bir `sleep` yerine kanıt bekleniyor: kanıt
gelmediyse eski veriyi silmek, izleyicinin altından zemini çekmek olurdu.

Hedefte zaten `config.json` varsa taşıma reddediliyor. İki veri kümesini
birleştirmek anahtarlar ve sayaçlar açısından belirsiz: hangi kişinin
sayacı geçerli sorusunun doğru cevabı yok.

**DPAPI anahtarları taşımadan etkilenmiyor.** Şifreleme yola değil, makine
ve Windows hesabına bağlı; aynı makinede kaldığı sürece çözülmeye devam
ediyor.

## K4 — Kod istenmiyor, olay günlüğüne yazılıyor

Taşımak için MFA kodu istenmiyor. Kod koruma eklemezdi: `config.json`'ı
elle düzenlemek zaten mümkün ve proje bunu engellemeyi değil, atlatmayı
fark edilir kılmayı hedefliyor (spec §3).

Bunun yerine taşıma olay günlüğüne düşüyor:

```json
{"ts":"...","ev":"data_moved","from":"C:\\...\\antigame","to":"D:\\antigame"}
```

`store.Event`'e `From` ve `To` alanları ekleniyor (`omitempty`, başka olay
türlerini etkilemiyor).

## Yeni paket

`internal/datadir` — saf, test edilir, kayıt defterine dokunmaz:

```go
// Validate, tasimanin guvenli olup olmadigini soyler.
func Validate(from, to string) error

// Copy, kaynaktaki dosyalari hedefe kopyalar. Alt dizinler yok sayilir:
// antigame duz bir dizin kullaniyor.
func Copy(from, to string) error

// Verify, hedefin kaynakla ayni dosyalari ayni boyutlarda icerdigini
// dogrular.
func Verify(from, to string) error

// RemoveContents, kaynaktaki dosyalari siler. Dizinin kendisi kaliyor:
// silmek Dosya Gezgini'nde acik duran bir pencereyi bozardi ve bos
// dizin zarar vermiyor.
func RemoveContents(dir string) error
```

Kayıt defteri erişimi `config` paketinde
(`golang.org/x/sys/windows/registry` — `x/sys` zaten bağımlılık, yeni
modül yok).

## Arayüz

Veriler diyaloğuna **"Taşı…"** düğmesi. Klasör seçici `SHBrowseForFolder`;
COM tabanlı modern diyalog bu pencerede kazanç sağlamadan çok daha fazla
kod isterdi.

Taşıma sırasında durum satırı adımları yazıyor. Taşıma bitince liste ve
yol alanı yeni klasörü gösteriyor.

## Hata yönetimi

- Doğrulama hatası durum satırında, taşıma başlamıyor.
- Kopyalama yarıda kalırsa hedefteki yarım dosyalar siliniyor ve kaynak
  aktif kalıyor.
- Kayıt defterine yazılamazsa (nadiren, politika) taşıma geri alınmıyor
  ama ayar değişmiyor: eski klasör aktif kalıyor, hedefteki kopya
  kullanılmıyor. Kullanıcıya söyleniyor.
- İzleyici 15 sn içinde geçmezse eski klasör silinmiyor.

## Test

**Birim testi:**

- `datadir.Validate` — hedef kaynağın içinde; hedef kaynağın kendisi;
  göreli yol; hedefte zaten `config.json`; geçerli hedef
- `datadir.Copy` + `Verify` — dosyalar taşınıyor, boyutlar eşleşiyor;
  alt dizin atlanıyor; eksik dosyada `Verify` hata veriyor
- `datadir.RemoveContents` — dosyalar gidiyor, dizin kalıyor
- `config.Dir` — kayıt defteri boşken varsayılan; doluyken o değer;
  göreli yol yazılıysa varsayılana düşüyor
- `watch` — klasör değişince izleyici yeni klasöre yazıyor; klasör
  silinince ölmüyor, ayarı yeniden okuyor

**Elle duman testi:**

1. Veriler → Taşı… klasör seçici açılıyor
2. Boş bir klasöre taşıma: dosyalar yeni yerde, eski klasör boş
3. Taşımadan sonra kapı hâlâ açılıyor (DPAPI anahtarları çalışıyor)
4. Taşımadan sonra rapor geçmişi görünüyor
5. İzleyici çalışırken taşıma: 5 sn içinde yeni klasöre yazmaya başlıyor
6. Hedefte veri varsa taşıma reddediliyor
7. Taşımadan sonra makine yeniden başlatılınca izleyici yeni klasörü
   kullanıyor

## Kapsam dışı

- Klasörü varsayılana geri döndürme düğmesi — aynı taşıma akışıyla
  varsayılan klasör seçilerek yapılabiliyor
- Ağ sürücüsüne veya senkron klasöre taşımaya özel uyarı
- Yedek alma
