# anti-game — Tasarım Dokümanı

**Tarih:** 2026-08-03
**Durum:** Onaylandı, uygulama planı bekliyor
**Proje kökü:** `C:\Users\guts\Documents\Project04(anti-game)`

---

## 1. Amaç

Oyun oynama süresini ölçmek ve oyuna girişi, kullanıcının kendisinde olmayan bir MFA koduyla kapıya bağlamak. Kod, kullanıcının güvendiği bir arkadaşının telefonundaki authenticator uygulamasında duruyor; oynamak için o kişiden kod istemek gerekiyor.

Öncelikli hedef Riot oyunları (League of Legends, Valorant, TFT, Legends of Runeterra) ama sistem oyuna özel değil, exe adı listesiyle çalışıyor.

**Sistem kısıtı:** İzleyici process'i, oyun oynanırken RAM'de duran tek bileşen ve **5 MB'ı aşmamalı** (hedef 1–3 MB). Bu bir tercih değil, doğrulanan bir kabul kriteri.

## 2. Kapsam dışı

- Süre kotası, günlük limit, otomatik kapatma. Süre **ölçülüyor**, sınırlanmıyor.
- Uzaktan onay, sunucu, bot, internet bağlantısı. Sistem tamamen çevrimdışı.
- Kendini koruma, hosts/firewall müdahalesi, kaldırmayı engelleme.
- Oyunun kullanıcı adına otomatik başlatılması.
- Pencere başlığı, ekran görüntüsü, tuş kaydı gibi hiçbir içerik toplama.

## 3. Tehdit modeli (dürüst çerçeve)

Bu sistem **hız kesicidir, kasa değildir.**

TOTP kodunu doğrulayabilmek için secret'ın doğrulayan makinede bulunması matematiksel bir zorunluluk. Dolayısıyla "sır bende değil" iddiası tam olarak sağlanamıyor. `secret.bin` DPAPI ile kullanıcı kapsamında şifreleniyor — bu, dosyanın başka bir makineye kopyalanmasını ve gelişigüzel okunmasını engelliyor, ama makinenin sahibi olarak kararlı bir çıkarma girişimini engellemiyor.

Aynı şekilde kullanıcı yönetici yetkisine sahip olduğu için zamanlanmış görevi silebilir, izleyiciyi sonlandırabilir, klasörü kaldırabilir.

Sistem bunları engellemeyi hedeflemiyor. Hedefi, oynamaya başlamayı **bilinçli ve gözle görünür bir karar** hâline getirmek: kapıyı açmak için birine yazmak gerekiyor, atlatmak için de kendini kandırdığını fark edeceğin somut bir eylem gerekiyor.

## 4. Alınan kararlar

| Karar | Seçim | Gerekçe |
|---|---|---|
| MFA yöntemi | Çevrimdışı TOTP (RFC 6238) | Sunucu/bot bağımlılığı yok; arkadaş sadece kodu okuyup söylüyor |
| Zorlama sertliği | Sert — process sonlandırma | Uyarı tabanlı çözüm gerçekten durdurmuyor; kendini koruyan sürüm ise fayda sağlamadan risk ekliyor |
| Süre modeli | Sınır yok, sadece takip | Kullanıcının isteği; kota arkadaş üzerinde sosyal baskı yaratıyor |
| Oyun listesi | Manuel | Kütüphane taraması ve sezgisel algılamanın yanlış pozitifi çok |
| Liste dışı uygulamalar | Durdurulmuyor, süresi kaydediliyor | Kör nokta bırakmıyor, oyun sırasında rahatsız etmiyor |
| Dil / çalışma zamanı | Go | En düşük boşta RAM; tek binary; üçüncü parti bağımlılık gerekmiyor |
| IFEO ile kapı | Reddedildi | Genel takip zaten kalıcı izleyici gerektiriyor, IFEO kazanç sağlamıyor; ayrıca Vanguard ve Defender riski |
| Oyunu otomatik başlatma | Reddedildi | Anti-cheat'in oyunu tanımadık üst process'ten görmesi gereksiz risk |

## 5. Mimari

Tek binary (`antigame.exe`), alt komutlarla farklı roller. Ayrı process'ler sayesinde arayüz kodu izleyicinin bellek tabanına hiç girmiyor.

```
antigame watch        # kalıcı izleyici — TEK sürekli açık process (~1-3 MB)
antigame gate --app X # kod giriş penceresi — izleyici doğuruyor, kod girilince ölüyor
antigame setup        # kurulum sihirbazı (secret üretimi, QR, kurtarma kodu)
antigame list         # oyun listesi görüntüleme/ekleme/çıkarma
antigame report       # HTML rapor üretip tarayıcıda açma
antigame uninstall    # geçerli kod isteyerek görev + dosyaları kaldırma
```

### 5.1 İzleyici (`watch`)

Tek iş parçacıklı döngü:

- **250 ms** — process listesi taranıyor (`CreateToolhelp32Snapshot`). Listedeki bir exe bulunur ve aktif oturum yoksa `TerminateProcess` çağrılıyor, `blocked` olayı yazılıyor, kapı penceresi doğuruluyor.
- **5 sn** — odaktaki pencerenin exe adı örnekleniyor (`GetForegroundWindow` → `GetWindowThreadProcessId`), AFK durumu `GetLastInputInfo` ile belirleniyor.
- **60 sn** — `state.json`'a nabız yazılıyor; biriken odak sayaçları `usage` satırı olarak boşaltılıyor.
- Her turun sonunda `EmptyWorkingSet` ile çalışma kümesi kırpılıyor.

GUI, HTTP, veritabanı, şablon motoru bu ikilinin çalıştırdığı kod yollarında yer almıyor.

### 5.2 Kapı (`gate`)

Ham Win32 modal diyalog (`user32` çağrıları, GUI kütüphanesi yok). Gösterdiği içerik: engellenen oyunun adı, arkadaşın adı ve iletişim ipucu, 6 haneli giriş alanı, kalan deneme hakkı.

Kod kabul edilirse oturum açılıyor ve pencere kapanıyor. Kullanıcı oyunu **kendisi** yeniden başlatıyor.

Adlandırılmış mutex (`Global\antigame-gate`) ile aynı anda birden fazla kapı penceresi açılması engelleniyor.

### 5.3 Rapor (`report`)

Olay günlüklerini okuyup tek bir bağımsız HTML dosyası üretiyor ve varsayılan tarayıcıda açıyor. Grafikler elle üretilen inline SVG — JavaScript kütüphanesi yok, dosya çevrimdışı açılıyor.

## 6. Veri modeli

Tüm dosyalar `%LOCALAPPDATA%\antigame\` altında. Veritabanı motoru yok; günlük hacim birkaç yüz satır.

### 6.1 `events-YYYY-MM.jsonl`

Sadece sona eklenen olay günlüğü. Zaman damgaları UTC / RFC3339. Yarım kalmış son satırı okuyucu atlıyor.

```jsonc
{"ts":"2026-08-03T18:04:11Z","ev":"game_start","exe":"VALORANT-Win64-Shipping.exe","name":"Valorant","pid":8842}
{"ts":"2026-08-03T19:14:21Z","ev":"game_end","exe":"VALORANT-Win64-Shipping.exe","pid":8842,"dur_s":4210,"active_s":3980}
{"ts":"2026-08-03T19:15:00Z","ev":"usage","exe":"chrome.exe","dur_s":60,"active_s":45}
{"ts":"2026-08-03T17:58:02Z","ev":"blocked","exe":"RiotClientServices.exe","name":"Riot Client"}
{"ts":"2026-08-03T18:03:40Z","ev":"unlock","method":"totp"}
{"ts":"2026-08-03T18:02:11Z","ev":"unlock_fail","fails":3}
{"ts":"2026-08-03T09:00:04Z","ev":"watch_start"}
```

`usage` satırları odak değişiminde veya 60 saniyede bir (hangisi önce gelirse) boşaltılıyor; her 5 saniyelik örnek için satır yazılmıyor.

### 6.2 `config.json`

```jsonc
{
  "friend_name": "Ahmet",
  "friend_hint": "WhatsApp'tan yaz",
  "gated": [
    {"name": "Riot Client",            "exe": "RiotClientServices.exe"},
    {"name": "League of Legends",      "exe": "LeagueClient.exe"},
    {"name": "League of Legends (Oyun)","exe": "League of Legends.exe"},
    {"name": "Valorant Başlatıcı",     "exe": "VALORANT.exe"},
    {"name": "Valorant",               "exe": "VALORANT-Win64-Shipping.exe"},
    {"name": "Legends of Runeterra",   "exe": "LoR.exe"}
  ],
  "grace_minutes": 10,
  "poll_ms": 250,
  "focus_sample_s": 5,
  "idle_threshold_s": 300
}
```

TFT ayrı bir çalıştırılabilir dosya değil, `LeagueClient.exe` üzerinden açılıyor; ayrı girdi gerekmiyor.

Eşleştirme varsayılan olarak exe adına göre, büyük/küçük harf duyarsız. Genel isimli bir exe için `"path"` alanı verilerek tam yola sabitlenebiliyor; `path` varsa hem ad hem yol eşleşmek zorunda.

### 6.3 `state.json`

```jsonc
{
  "last_totp_counter": 58291837,
  "fail_count": 0,
  "lock_until": null,
  "session": {
    "opened_at": "2026-08-03T18:03:40Z",
    "last_game_seen": "2026-08-03T19:14:21Z"
  },
  "heartbeat": "2026-08-03T19:20:00Z",
  "recovery_hash": "…",
  "recovery_used": false
}
```

Yazma işlemleri geçici dosyaya yazıp `MoveFileEx` ile yer değiştirme yoluyla atomik yapılıyor.

### 6.4 `secret.bin`

Base32 TOTP secret'ının `CryptProtectData` (kullanıcı kapsamı) ile şifrelenmiş hâli. Kurulum dışında hiçbir yerde düz metin olarak diske yazılmıyor.

## 7. MFA akışı

### 7.1 Kurulum

1. `antigame setup` 160 bit rastgele secret üretiyor.
2. `otpauth://totp/anti-game:<kullanıcı>?secret=…&issuer=anti-game` URI'si tek kullanımlık bir HTML sayfasında QR olarak gösterilip tarayıcıda açılıyor.
3. Arkadaş authenticator uygulamasıyla okutuyor ve doğrulama için bir kod söylüyor; setup bu kodu doğrulayarak eşleşmeyi teyit ediyor. Bu kodun adım sayacı da `last_totp_counter`'a yazılıyor — yani kurulumda kullanılan kod sonradan kapıyı açmak için kullanılamıyor.
4. Kullanıcı onaylıyor; geçici HTML dosyası siliniyor, secret DPAPI ile şifrelenip `secret.bin`'e yazılıyor.
5. Tek kullanımlık kurtarma kodu üretiliyor, ekranda gösteriliyor, tuzlanmış SHA-256 özeti `state.json`'a yazılıyor. Kullanıcıya bunu **ikinci bir kişiye** vermesi söyleniyor.
6. Oturum açılışına bağlı zamanlanmış görev kuruluyor.

### 7.2 Doğrulama

RFC 6238: HMAC-SHA-1, 6 hane, 30 saniyelik adım, **±1 adım** tolerans.

İki ek koruma:

- **Tekrar kullanım engeli.** Kabul edilen kodun adım sayacı `last_totp_counter`'a yazılıyor. Sayacı bu değerden küçük veya eşit hiçbir kod kabul edilmiyor. Bu, aynı kodun ikinci kez kullanılmasını ve sistem saatinin geri alınarak eski kodun tekrar girilmesini kapatıyor.
- **Kaba kuvvet engeli.** 5 hatalı denemeden sonra kademeli kilit: 15 dk → 30 dk → 60 dk. `lock_until` diske yazıldığı için uygulamayı yeniden başlatmak kilidi sıfırlamıyor.

Kurtarma kodu aynı kapıdan giriliyor; özeti tutuyorsa oturum açılıyor, `recovery_used` işaretleniyor ve kod bir daha kabul edilmiyor.

### 7.3 Oturum semantiği

Oturum, kota olmadığı için süreye değil oyunun çalışmasına bağlı:

- Kod kabul edildiğinde `session` açılıyor ve `last_game_seen` o an olarak ayarlanıyor. Bu, kullanıcıya oyunu başlatmak için `grace_minutes` kadar süre veriyor.
- İzleyici her turda listedeki bir oyunu görürse `last_game_seen`'i güncelliyor.
- Listedeki hiçbir oyun çalışmıyorsa ve `now > last_game_seen + grace_minutes` ise oturum kapanıyor.

Sonuç: oyun çöktüğünde veya kısa süre kapatıldığında yeni kod istenmiyor; akşam tekrar oturulduğunda isteniyor.

## 8. Takip

**Kapıdaki oyunlar** için process doğumundan ölümüne kadar duvar saati ölçülüyor (`dur_s`). Aynı anda `GetLastInputInfo` ile 5 dakikadan uzun girdisiz süreler düşülerek `active_s` hesaplanıyor. Rapor ikisini de gösteriyor: 3 saat açık kalan lobi ile 3 saat oynanan maç ayrışıyor.

**Liste dışı uygulamalar** için 5 saniyede bir odaktaki exe örnekleniyor, aynı mantıkla aktif/AFK ayrımı tutuluyor, `usage` satırı olarak toplu yazılıyor.

**Pencere başlıkları kaydedilmiyor.** Başlıklar tarayıcı sekmesi, dosya adı, sohbet ismi sızdırır. Sadece exe adı tutuluyor ve hiçbir veri `%LOCALAPPDATA%` dışına çıkmıyor.

## 9. Rapor içeriği

- Bu haftanın toplam oyun süresi; geçen haftaya göre fark.
- Oyun bazında dağılım (süre ve aktif süre ayrı).
- Günlük çubuk grafik.
- Gün içi saat dağılımı — hangi saatlerde oynandığı.
- Son 4 haftanın trendi.
- **İzleyicinin kapalı olduğu aralıklar**, nabız boşluklarından çıkarılıyor (2 dakikadan uzun boşluklar).
- Listede olmayıp çok vakit alan uygulamalar ve bunlar için `antigame list add <exe>` önerisi.

## 10. Hata durumları

| Durum | Davranış |
|---|---|
| İzleyici çöküyor | Zamanlanmış görev 1'er dakika arayla 3 kez yeniden başlatıyor; boşluk raporda görünür kalıyor |
| Kapı penceresi zaten açık | Adlandırılmış mutex ikinci pencereyi engelliyor |
| Bozuk/yarım son günlük satırı | Okuyucu atlıyor, gerisini işliyor |
| DPAPI çözülemiyor | Net hata mesajı + `setup`'a yönlendirme |
| Yaz saati / saat dilimi | Kayıtlar UTC, gösterim yerel |
| Genel isimli exe çakışması | `config.json`'da `path` ile tam yola sabitleme |
| `state.json` yazımı yarıda kesiliyor | Geçici dosya + atomik yer değiştirme |
| Oyun listesi boş | `watch` uyarı yazıp sadece takip modunda çalışıyor |

## 11. Teknoloji ve yapı

Go 1.26, `GOOS=windows`. İki üçüncü parti bağımlılık:

- `golang.org/x/sys/windows` — Win32 çağrıları.
- Bir QR kodlayıcı (ör. `github.com/skip2/go-qrcode`) — yalnızca `setup` yolunda. QR kodlayıcıyı elle yazmak birkaç yüz satırlık gereksiz iş; buna karşılık izleyici bu kod yoluna hiç girmediği için kalıcı bellek tabanına etkisi yok, sadece ikili boyutu birkaç yüz KB artıyor.

TOTP, SVG ve HTML üretimi dahil geri kalan her şey standart kütüphaneyle yazılıyor.

Derleme: `-ldflags="-s -w"`, izleyicide `GOGC` düşürülüp her turda `EmptyWorkingSet`.

```
cmd/antigame/          alt komut dağıtımı
internal/config/       config.json okuma/yazma, exe eşleştirme
internal/store/        JSONL ekleme ve okuma, atomik state yazımı
internal/totp/         RFC 6238 üretim ve doğrulama, sayaç kontrolü
internal/dpapi/        CryptProtectData / CryptUnprotectData sarmalayıcı
internal/winproc/      process listeleme, sonlandırma, çalışma kümesi kırpma
internal/wininput/     GetLastInputInfo, GetForegroundWindow
internal/session/      oturum açma/kapama, ödemesiz süre mantığı
internal/gate/         Win32 modal diyalog
internal/report/       toplama, SVG ve HTML üretimi
internal/task/         zamanlanmış görev kurma/kaldırma
```

## 12. Test planı

**Birim testleri**

- RFC 6238 resmî test vektörleriyle TOTP doğrulama.
- Tekrar kullanım reddi: aynı kod ikinci kez reddediliyor; geri alınmış saatle üretilmiş kod reddediliyor.
- ±1 adım toleransı sınırları: ±30 sn kabul, ±60 sn ret.
- Kilit kademeleri ve kilidin yeniden başlatmaya dayanması.
- Kurtarma kodu: bir kez kabul, ikinci kez ret.
- Oturum mantığı: ödemesiz süre içinde ve dışında yeniden başlatma.
- Sentetik JSONL üzerinden toplama; yarım satır, saat dilimi ve ay sınırı durumları.
- AFK eşiği hesabı.
- Exe eşleştirme: harf duyarsızlığı, `path` sabitlemesi, benzer isimli exe'nin eşleşmemesi.

**Entegrasyon testleri**

- `fakegame.exe` üretilip listeye ekleniyor; çalıştırıldığında sonlandırılıyor mu, `blocked` olayı yazılıyor mu, kapı doğuruluyor mu.
- Geçerli kod girildikten sonra aynı binary'nin sonlandırılmadığı doğrulanıyor.
- Ödemesiz süre dolduktan sonra tekrar sonlandırıldığı doğrulanıyor.

**Bütçe testi**

- İzleyici 1 saat çalıştırılıp çalışma kümesi ölçülüyor; **5 MB üzeri başarısızlık.**

**Elle kontrol listesi**

- Gerçek Riot Client ile kapı denemesi.
- `Get-Process` ile RAM ölçümü ve tasarım hedefiyle karşılaştırma.
- Kurulum akışının arkadaşın telefonuyla uçtan uca denenmesi.

## 13. Bilinen sınırlar

- Kurulum anında secret kullanıcının makinesinden geçiyor; sistem o an kopyalanmamasına güveniyor.
- Yönetici yetkisiyle zamanlanmış görev silinebilir, izleyici sonlandırılabilir. Rapordaki nabız boşlukları bunu görünür kılıyor ama engellemiyor.
- Process sonlandırma, oyun kendi başlatıcısı tarafından yeniden doğurulursa döngüye girebilir; kapı penceresi açıkken tekrar doğurma denemeleri sessizce sonlandırılıyor.
- 250 ms'lik tarama aralığı nedeniyle oyun en fazla çeyrek saniye çalışmış oluyor.
- `GetLastInputInfo` klavye ve fareyi görüyor ama **oyun kumandası girdisini görmüyor**. Yalnızca kumandayla oynanan bir oyunda AFK sayacı yanlış çalışır; `dur_s` doğru kalır, `active_s` olduğundan düşük çıkar. Kumandayla oynama alışkanlığı ortaya çıkarsa XInput yoklaması eklenerek çözülebilir, ilk sürümde kapsam dışı.
