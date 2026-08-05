# Çoklu kişi: birden fazla kişiye anahtar, kişi yönetimi, kişi bazlı rapor

Tarih: 2026-08-05

## Sorun

Bugün tek bir kişi (`friend_name`) ve tek bir TOTP anahtarı (`secret.bin`) var.
Anahtarı tutan kişiye ulaşılamadığında kapı açılmıyor, ikinci bir kişi
eklenemiyor ve raporda "kimin kodu" bilgisi yok.

## Kararlar

- Her kişinin **kendi anahtarı** olur. Kapı hepsini kabul eder, kabul eden
  anahtarın sahibi kayda geçer.
- Kişi ekleme **serbesttir**, yalnızca olay günlüğüne yazılır. Kullanıcı bunu
  bilerek seçti: kapı gönüllü bir engel, kriptografik bir hapis değil.
- Depolama: kişi başına ayrı anahtar dosyası + isimler `config.json` içinde.
- Rapor: kişi başına toplam süre ve kaç kez kapı açtığı.
- Mevcut kişi (Baran) anahtarıyla birlikte taşınır; yeniden eşleştirme yok.

## Veri modeli

### config.json

```go
type Person struct {
    ID   string `json:"id"`             // "p1", "p2" — dosya adında kullanılır
    Name string `json:"name"`
    Hint string `json:"hint,omitempty"`
}

People []Person `json:"people"`
```

`ID` yalnızca `[a-z0-9]` karakterlerinden oluşur ve her zaman program
tarafından üretilir. Dosya adına girdiği için elle yazılmış bir değer yola
sızmamalı; `Load` sırasında doğrulanır, uymayan kayıt yüklenmez.

Yeni id, mevcut kayıtlardaki en büyük numaranın bir fazlasıdır. Silinen bir
id yeniden kullanılmaz — kullanılırsa silinen kişinin geçmiş süresi yeni
kişiye yazılırdı.

`friend_name` / `friend_hint` alanları dosyada kalır ama okunmaz.

### Anahtar dosyaları

`secret-<id>.bin`, bugünkü `vault` biçiminin aynısı (DPAPI ile şifreli).

### state.json

```go
TOTPCounters     map[string]uint64 `json:"totp_counters"`  // id → son kullanılan sayaç
Session.OpenedBy string            `json:"opened_by"`      // kapıyı açan kişi
```

Sayaç kişi başına ayrı tutulmak zorunda: ortak tutulursa bir kişinin sayacı
diğerinin kodunu "kullanılmış" sayıp reddeder.

## Göç

İlk çalıştırmada, `people` boş ve `friend_name` doluysa:

1. `{id:"p1", name:friend_name, hint:friend_hint}` kaydı oluşturulur.
2. `secret.bin` → `secret-p1.bin` kopyalanır, kopya okunup çözülebildiği
   doğrulanır, sonra eski dosya silinir. Yarıda kalırsa iki dosya da durur ve
   göç bir sonraki açılışta tekrar denenir.
3. `last_totp_counter` → `totp_counters["p1"]`.

`secret.bin` yok ama `friend_name` varsa kişi anahtarsız olarak eklenir ve
ekranda öyle işaretlenir.

## Tutarlılık kuralı

`config.json`'da olup `secret-<id>.bin` dosyası olmayan kişi listede
**gösterilir ama kapıyı açamaz**; "anahtarı yok — yenile veya sil" diye
işaretlenir. Sessizce düşürülmez: elle silinmiş bir dosya, kişiyi kullanıcıya
görünmeden yok etmemeli.

Tersi durum (dosya var, kayıt yok) yetim dosyadır: silinmez, kişi ekranında
sayısı uyarı olarak yazılır.

## Kapı doğrulama

`auth.Verifier` tek `Secret` yerine anahtar listesi alır:

```go
type Key struct{ ID string; Secret []byte }
```

`Attempt` sırası: kilit kontrolü → kurtarma kodu → her anahtar için
`totp.Verify(k.Secret, code, now, st.TOTPCounters[k.ID])`.

Döngü ilk eşleşmede değil, **tüm anahtarlar denendikten sonra** karar verir:
bir anahtar "kullanılmış kod" derken başka biri geçerli olabilir; erken çıkış
kullanıcıya yanlış gerekçe gösterir. Karar sırası:

1. herhangi bir `ResultOK` → kabul
2. yoksa herhangi bir `ResultReplay` → "Bu kod daha önce kullanılmış."
3. yoksa → "Kod hatalı."

Kabulde `counters[id]` güncellenir, `session.OpenedBy = id` yazılır ve
`unlock{method:"totp", who:id}` olayı düşülür. Kurtarma kodunda `who` boştur,
`method:"recovery"` kalır.

Hata sayacı ve kilit bugünkü gibi kişiden bağımsız, tek sayaçtır.

Kapı penceresi kişi seçtirmez — tek kod kutusu, kod hangi anahtara uyuyorsa o.
Yazı: `Kodu Baran, Ali veya Ayşe'den isteyin.`, altında iletişim ipuçları gri.
Üçten fazla kişi varsa ilk üçü ve `... ve N kişi daha`.

## İzleyici

`game_end` olayına oturumu açan kişinin id'si (`who`) yazılır. Kapı kapalıyken
çalışan oyunda (MFA kurulmamış, süre yine de kaydediliyor) `who` boş kalır.

## Rapor

```go
type PersonTotal struct{ ID, Name string; DurS, Unlocks int }
Summary.People []PersonTotal
```

Bu haftanın `game_end` olayları (başlatıcılar bugünkü gibi elenir) `who`'ya
toplanır; `unlock` olayları açma sayısını artırır. Süreye göre azalan sıralanır.

İsim `config.json`'dan çözülür. Kişi silinmişse `Ali (silinmiş)` yazılır —
geçmiş süre kaybolmaz. `who` boş olanlar `Kapı yokken` satırında toplanır.

HTML raporda yeni bölüm: kişi, süre, kaç kez açtı.

## Terminal görünümü

Yeni `internal/term` paketi renk kararını tek yerden verir:

- Windows'ta `SetConsoleMode` ile VT açılır; açılmazsa renk kapalıdır.
- Çıktı terminale değil boruya/dosyaya gidiyorsa renk kapalıdır. Testler
  `bytes.Buffer`'a yazdığı için ANSI kaçışı görmez.
- `NO_COLOR` ortam değişkeni varsa kapalıdır.
- Aynı yerde `SetConsoleOutputCP(CP_UTF8)` çağrılır. Bu gereklilik: kutu
  çizgileri ve Türkçe harfler konsol kod sayfası 857 kalırsa bozuk çıkar.
  Başarısız olursa kutu çizgileri ASCII'ye (`-`, `|`) düşer.

Palet beş anlam taşır, süs yoktur:

| Rol | Renk |
|---|---|
| Başlık | cyan + kalın |
| Menü tuşu | sarı + kalın |
| İyi durum (oturum açık, izleyici çalışıyor, anahtar var) | yeşil |
| Uyarı (MFA yok, anahtarı yok) | sarı |
| Hata / kilitli | kırmızı |
| İkincil (ipucu, tarih) | gri |

Kural: renk bilgiyi tekrarlar, taşımaz. Renk kapalıyken hiçbir anlam
kaybolmaz — durum kelimesi her zaman yazılır.

Menü her çizimde ekranı temizler (`\x1b[2J\x1b[H`); renk kapalıysa yalnızca
boş satır bırakılır.

## Kişi ekranı

Ana menü bugün 1–8 arası dolu; kişi ekranı `9) Kişileri yönet` olarak eklenir.
Mevcut tuşlar yerinde kalır — numaraları kaydırmak alışkanlığı bozar.
`antigame people` komutu da aynı ekranı açar.

```
────────────────────────────────────────────────────
 antigame — Kişiler
────────────────────────────────────────────────────

  1. Baran      WhatsApp           ✓ anahtar var
  2. Ali        Telegram @ali      ✓ anahtar var
  3. Ayşe       —                  ! anahtarı yok

  [e] Ekle   [d] Düzenle   [s] Sil   [y] Anahtar yenile
  [0] Geri

Seçiminiz:
```

İşlemler:

- **Ekle**: isim ve iletişim sorulur, yeni anahtar üretilir, QR sayfası açılır,
  kod doğrulanır, kayıt yazılır. `setup` içindeki eşleştirme akışı yeniden
  kullanılır.
- **Düzenle**: isim ve iletişim değişir, anahtar aynı kalır.
- **Sil**: onay istenir, anahtar dosyası ve kayıt silinir.
- **Anahtar yenile**: kişi kalır, anahtarı değişir (telefon kaybı).

Her işlem olay günlüğüne yazılır: `person_add`, `person_edit`, `person_remove`,
`person_rotate`.

## Hata durumları

Yazma sırası her işlemde aynı: **önce kod doğrula, sonra anahtar dosyasını yaz,
en son `config.Save`**. Ters sırada yarıda kesilme "ismi var, anahtarı yok"
bırakır.

- **Kilitlenme koruması**: silme sonrası anahtarı çalışan kişi sayısı sıfıra
  düşecekse silme reddedilir. Yalnızca "son kişi" kontrolü yetmez; anahtarsız
  iki kişi kalması da kapıyı açılmaz yapar.
- **Ekleme yarıda kesilirse**: hiçbir şey yazılmaz.
- **Anahtar yenileme**: yeni anahtar `secret-<id>.bin.tmp`'ye yazılır, kod
  doğrulanınca `os.Rename` ile yerine geçer. İptalde tmp silinir, eski anahtar
  çalışmaya devam eder. Sayaç sıfırlanır — eski yüksek sayaç yeni anahtarın
  kodlarını reddederdi.
- **Silme**: önce anahtar dosyası, sonra config ve sayaç. Dosya silinemezse
  (kilitli) config'e dokunulmaz, hata gösterilir.
- **DPAPI çözemezse** (Windows profili değişmiş) o kişi "anahtarı okunamıyor"
  diye işaretlenir, diğer kişiler çalışmaya devam eder.
- **Hiç çalışan anahtar yoksa**: bugünkü "MFA kurulmamış" davranışı sürer —
  kapı devre dışı, süre kaydedilmeye devam eder.

## Test planı

| Paket | Test |
|---|---|
| config | `friend_name` → `people` göçü; `people` doluyken göç yapılmaz; geçersiz id reddi; yeni id üretimi silinen id'yi tekrar vermez |
| vault | kişi başına yaz/oku/sil; `secret.bin` → `secret-p1.bin` göçü; göç yarıda kalırsa tekrar çalışır |
| auth | ikinci kişinin kodu kabul edilir; bir kişinin kullanılmış kodu diğerini etkilemez; replay ile hatalı kod mesajı ayrışır; kilit kişiden bağımsız |
| report | kişi bazlı süre ve açma sayısı; silinmiş kişi adı; `who` boş satırı; başlatıcı elenmesi bozulmadı |
| people ekranı | son çalışan anahtar silinemez; ekleme iptalinde config değişmez; yenileme iptalinde eski anahtar durur |
| term | renk kapalıyken çıktıda ANSI kaçışı yok; `NO_COLOR` uyulur |

## Kapsam dışı

- Kişi başına ayrı kilit/hata sayacı.
- Kişi başına oyun kırılımı (kişi × oyun tablosu).
- Kişi eklemeyi mevcut bir kişinin onayına bağlamak.
