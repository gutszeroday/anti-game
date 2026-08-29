# Riot süreç ailesi: kapı bayrakları, aile sonlandırma ve rapor sayımı

Tarih: 2026-08-04
Durum: onaylandı, uygulanmayı bekliyor

## Sorun

Kullanıcı League of Legends'a girip çıktı, üzerinden üç saat geçti ve hâlâ
MFA kodu sorulmadan girip çıkabiliyordu.

### Kök neden

Diskteki `config.json` içinde hiçbir girdide `launcher` alanı yok:

```json
{"name":"Riot Client","exe":"RiotClientServices.exe"}
{"name":"League of Legends","exe":"LeagueClient.exe"}
```

`watch.go` her turda şunu çağırıyor:

```go
session.Touch(w.st, now, !g.Launcher)
```

`Launcher` false olduğu için Riot Client **gerçek oyun** sayılıyor. Tepside
açık durduğu sürece `LastGameSeen` tazeleniyor, başlatıcı penceresi hiç
dolmuyor, oturum süresiz açık kalıyor. `internal/session/session.go` başlık
yorumu bu senaryoyu tarif ediyor — koruma yazılmış, ama bayrak kayıp olduğu
için hiç devreye girmemiş.

Ölçülen kanıt, `state.json`:

```json
"opened_at":      "2026-08-04T15:11:19Z"
"last_game_seen": "2026-08-04T20:13:37Z"
```

Hiçbir oyun çalışmazken `last_game_seen` beş saat boyunca tazelenmiş.

Bayrakların kaybolma sebebi ac1512d'de düzeltilen `config.Load` alan miras
hatası. O düzeltme kodu onardı, zaten bozulmuş `config.json`'u onarmadı.
`Launcher` alanı `omitempty` taşıdığı için "yok" ile "false" ayırt
edilemiyor; dosya kendi kendini düzeltemez.

### İkinci sorun: eksik süreç ailesi

Riot ailesinin yarısı listede değil. Ölçüm anında çalışanlar:

| Süreç | Kopya | Listede |
|---|---|---|
| `RiotClientServices.exe` | 1 | var |
| `Riot Client.exe` | 6 | **yok** |
| `RiotClientCrashHandler.exe` | 1 | **yok** |
| `LeagueClientUx.exe` | — | **yok** (günlükte 92 olay) |

Süre dolup `RiotClientServices.exe` öldürülse bile ayakta kalan Electron
arayüz süreçleri servisi yeniden doğurabiliyor.

### Üçüncü sorun: rapor süreyi şişiriyor

Başlatıcılar da `game_start`/`game_end` yazdığı için haftalık rapor onları
oyun süresi sayıyor:

| Exe | Oturum | Süre |
|---|---|---|
| `RiotClientServices.exe` | 2 | 4.91 sa |
| `League of Legends.exe` | 12 | 3.83 sa |
| `LeagueClient.exe` | 4 | 1.92 sa |
| **Raporun toplamı** | | **10.65 sa** |
| **Gerçek oyun** | | **3.83 sa** |

Rapor 6.8 saat fazla sayıyor. Aileye `LeagueClientUx.exe` eklenince şişme
büyür; rapor düzeltmesi config değişikliğinin zorunlu eşlikçisi.

## Tasarım

### Ayrı bir aile mekanizması eklenmiyor

Oturum düştüğünde `watch.go` zaten listedeki her eşleşen süreci
sonlandırıyor. Aileyi birlikte kapatmak için gereken tek şey ailenin
tamamının listede olması. Ayrı bir `family` alanı ve ayrı sonlandırma yolu
YAGNI olur.

### 1. `config.Default()` aileyi eksiksiz tanımlar

| Exe | launcher | Gerekçe |
|---|---|---|
| `RiotClientServices.exe` | evet | servis |
| `Riot Client.exe` | evet | Electron arayüz (yeni) |
| `RiotClientCrashHandler.exe` | evet | çökme kutusu çıkmasın (yeni) |
| `LeagueClient.exe` | evet | istemci |
| `LeagueClientUx.exe` | evet | LoL arayüzü (yeni) |
| `League of Legends.exe` | hayır | gerçek oyun |
| `VALORANT.exe` | evet | başlatıcı |
| `VALORANT-Win64-Shipping.exe` | hayır | gerçek oyun |
| `LoR.exe` | hayır | gerçek oyun |

### 2. `config.Load` bayrağı dayatır

`Default()`'ta adı geçen bir exe için dosyadaki `launcher` değeri
yok sayılır, varsayılan dayatılır. Bozuk dosya okunduğu anda düzelir; disk
yazımı gerekmez, `Load` saf kalır. Kullanıcının kendi eklediği oyunlara
dokunulmaz.

Bu aynı zamanda güvenlik özelliği: `RiotClientServices.exe`'nin başlatıcı
olduğu artık `config.json` düzenlenerek kaldırılamaz.

### 3. Başlatıcı penceresi 45 → 10 dakika

`Default().LauncherWindowMinutes` ve kullanıcının `config.json`'u.

Bilinen ödünleşim, kullanıcıya bildirildi ve kabul edildi: `League of
Legends.exe` yalnızca maç yüklenirken doğar. Kuyruk ve şampiyon seçimi on
dakikayı aşarsa oturum seçimin ortasında düşer ve istemci kapanır.

### 4. Rapor başlatıcıları saymaz

`report.Aggregate`, bir `game_end` olayını toplamlara katmadan önce
`cfg.Match` ile sorar. Eşleşiyor **ve** `Launcher` ise olay atlanır: toplam
süre, oyun tablosu, günlük dağılım ve saat dağılımı dışında kalır.

Listeyle eşleşmeyen exe (sonradan silinmiş bir oyun) oyun sayılmaya devam
eder; aksi halde geçmiş kayıtlar rapordan silinirdi.

## Beklenen davranış

- Maç sürerken: `League of Legends.exe` çalışır, `Touch(realGame=true)`, her
  iki sayaç da tazelenir, oturum yaşar.
- Maçtan çıkınca: yalnızca başlatıcılar çalışır, `Touch(realGame=false)`,
  `LastSeen` tazelenir ama `LastGameSeen` donar.
- Son maçtan 10 dakika sonra: oturum düşer, Riot ailesinin tamamı
  sonlandırılır, kapı penceresi MFA kodu sorar.

## Testler

- `config`: bozuk config (launcher alanı yok) okunduğunda bilinen
  başlatıcıların bayrağı geri gelir.
- `config`: `Default()` Riot ailesini eksiksiz içerir.
- `config`: kullanıcının eklediği oyunun bayrağı migration'dan etkilenmez.
- `watch`: yalnızca başlatıcı çalışırken oturum başlatıcı penceresi kadar
  sonra düşer.
- `watch`: oturum düştüğünde ailenin tamamı sonlandırılır.
- `report`: başlatıcı `game_end` olayları toplam süreye, oyun tablosuna,
  gün ve saat dağılımına girmez.
- `report`: listede olmayan exe oyun sayılmaya devam eder.

## Kapsam dışı

Bu tasarım sırasında bulunan, ayrı ele alınacak konular:

- `report.Run` tarayıcı açma hatasını yutuyor (`render.go`, `Start()`
  hatasında `nil` dönüyor); açılamayan rapor sessiz kalıyor.
- `findGaps` 35 günlük olayı tarıyor ama rapor başlığı "Bu hafta" diyor.
- `watch/budget_test.go` gerçek binary'i geçici bir `LOCALAPPDATA` ile
  başlatıyor. O izleyici boş dizinden okuduğu için oturumu yok sayar ve
  `Default()` listesindeki her şeyi sonlandırır — makinede gerçekten
  çalışan Riot süreçleri dahil. Yani `go test ./...` kullanıcının oyununu
  kapatır. Değişiklikten önce de böyleydi; aile büyüdüğü için etkisi arttı.
  Testin sonlandırmayı devre dışı bırakan bir bayrakla çalışması gerekir.
