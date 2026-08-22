# Telegram bildirimi: kapı açma anlık, kullanım özeti komutla

Tarih: 2026-08-23

## Sorun

Denetleyen kişi (ör. ebeveyn) kapının ne zaman açıldığını ve ne kadar
kullanıldığını görmek için `antigame report` çalıştırmak ya da makineye
gitmek zorunda. Uzaktan, anlık ve isteğe bağlı görünürlük yok.

## Kararlar

- **Kapı açma → anlık push.** Biri kod girip kapıyı açtığında, kim ve ne
  zaman bilgisi tüm onaylı Telegram sohbetlerine gider. Oyun başlama/bitişi
  ve başarısız denemeler bu sürümde bildirim üretmez (gürültüyü azaltmak
  için — kapsam dışına bakınız).
- **Kullanım özeti komutla, otomatik değil.** Periyodik push yok. Onaylı
  bir sohbet `/durum` yazınca watcher bugünün özetini hesaplayıp geri
  yollar.
- **Watcher, event log'u tarayarak bildirir — `gate` süreci ağa hiç
  dokunmaz.** `gate` kısa ömürlü: başarılı kodda pencere hemen kapanır ve
  süreç çıkar, arka planda başlatılan bir ağ çağrısı süreç çıkışıyla
  yarışır ve kaybolabilir. `watch` sürekli çalışan tek süreç; `unlock`
  olayını diskten (zaten yazılıyor) birkaç saniye içinde görüp gönderir.
  Bu, `gate`'in kapı açma akışına gecikme veya ağ hatası riski eklemesini
  de engeller.
- **Birden fazla onaylı sohbet** desteklenir (tek ebeveyn şart değil).
- **Onay, eşleştirme koduyla.** Bota mesaj atan herkes otomatik onaylanmaz;
  UI'da üretilen tek kullanımlık kodu bota yazan sohbet onaylanır. Mevcut
  `internal/pairing` (TOTP eşleştirme) ile aynı desen.
- **Bot token `config.json`'da düz metin.** Kullanıcı bilerek seçti;
  mevcut config zaten kişi adları gibi hassas olmayan/az hassas bilgiyi
  düz tutuyor. Anahtar dosyaları (vault, DPAPI şifreli) bundan ayrı kalır
  ve etkilenmez.
- **Özellik tamamen opsiyonel.** Token/sohbet boşsa watcher'da ek goroutine
  hiç başlamaz, sıfır ağ trafiği. Anti-cheat çekirdek işlevi (kapı açma,
  oyun engelleme) bu özellikten bağımsız çalışmaya devam eder.

## Veri modeli

### config.json

```go
type TelegramChat struct {
    ID      int64     `json:"id"`
    Label   string    `json:"label,omitempty"`
    AddedAt time.Time `json:"added_at"`
}

// Config'e eklenir:
TelegramToken string          `json:"telegram_token,omitempty"`
TelegramChats []TelegramChat  `json:"telegram_chats,omitempty"`
```

`Label` isteğe bağlı, eşleştirme sırasında UI'da girilebilir; boşsa
listede `Sohbet <ID>` gösterilir.

### state.json

```go
// State'e eklenir:
TelegramOffset         int64      `json:"telegram_offset,omitempty"`          // getUpdates dedup
TelegramLastUnlockTS   *time.Time `json:"telegram_last_unlock_ts,omitempty"`  // tarama işareti
// TelegramPendingCode, UI'da "Sohbet ekle" ile üretilen tek kullanımlık
// eşleştirme kodudur. telegramwatch bunu okuyup eşleşen mesajı onaylar.
TelegramPendingCode   string     `json:"telegram_pending_code,omitempty"`
TelegramPendingExpiry *time.Time `json:"telegram_pending_expiry,omitempty"`
```

Bu alanlar çalışma zamanı durumudur (Counter, LockUntil gibi), kullanıcı
ayarı değildir — bu yüzden config değil state'e girer.

`TelegramLastUnlockTS` boşsa (ilk çalıştırma) watcher taramaya "şu an"dan
başlar: geçmiş `unlock` olayları için geriye dönük bildirim atılmaz.

## Yeni paket: internal/telegram

`gate` gibi kritik yollardan izole, yalnızca `internal/telegramwatch`
tarafından import edilir.

```go
type Client struct {
    Token      string
    HTTPClient *http.Client // testte enjekte edilir
}

func (c Client) SendMessage(chatID int64, text string) error
func (c Client) GetUpdates(offset int64, timeoutS int) ([]Update, error)

type Update struct {
    UpdateID int64
    Chat     int64  // 0 ise mesaj değil, yok sayılır
    Text     string
}
```

`net/http` ile `https://api.telegram.org/bot<token>/sendMessage` ve
`/getUpdates?offset=..&timeout=..`. `GetUpdates` timeout'u Telegram'ın
kendi long-poll parametresi (`timeout=25`); istemci tarafında ek bir
`http.Client.Timeout` bundan büyük tutulur (ör. 30s) yoksa istek erken
kesilir.

## Watcher entegrasyonu: yeni paket internal/telegramwatch

`internal/watch`'un `Step(now)` döngüsü 250ms'de bir çalışıyor ve
senkron: `Run` içinde `Step` doğrudan çağrılıyor, ağ çağrısı için ayrılmış
bir goroutine yok (bkz. `watch.go` — "Dongu Step olarak disari aciliyor,
testler zamani elle surebiliyor"). Telegram çağrıları (özellikle
`GetUpdates`'in 25 saniyelik long-poll'u) bu döngünün içine konursa oyun
tarama/durdurma turu 25 saniyeye kadar donar — anti-cheat'in temel
işlevini bozar. Bu yüzden Telegram mantığı **`watch` paketine hiç
dokunmaz**; ayrı bir paket ve ayrı goroutine'ler olarak yaşar.

Yeni paket **`internal/telegramwatch`**, tek giriş noktası:

```go
func Run(ctx context.Context, dirFunc func() string) error
```

`cmd/antigame/main.go`'daki `runWatch`'a, mevcut `w.Run(ctx)` çağrısının
yanına üçüncü bir goroutine olarak eklenir:

```go
watcher := make(chan error, 1)
go func() { watcher <- w.Run(ctx) }()

telegram := make(chan error, 1)
go func() { telegram <- telegramwatch.Run(ctx, config.Dir) }()
```

`telegramwatch.Run`, `Watcher` ile hiçbir bellek içi durum paylaşmaz;
kendi döngüsünde `config.Load`/`store.LoadState` ile okur,
`config.Save`/`store.SaveState` ile yazar — tıpkı `gate` ve `watch`
süreçlerinin bugün de aynı `state.json`/`config.json` üzerinden, kilitsiz,
"son yazan kazanır" tutarlılığıyla haberleştiği gibi (bkz. `state.go`,
atomik `.tmp`+rename yazım). Bu, yeni bir eşzamanlılık riski değil,
mevcut dosya tabanlı koordinasyon modelinin süreç içi bir uzantısıdır.

`telegramwatch.Run` içinde, `ctx` iptal olana kadar süren iki bağımsız
goroutine — yalnızca `cfg.TelegramToken != ""` ise başlar:

**1. Unlock tarayıcı** (~10sn ticker)

```
lastTS := st.TelegramLastUnlockTS (yoksa now)
tick:
  events := store.Read(dir, lastTS.Add(1ns), now)
  for e in events where e.Ev == "unlock":
      msg := formatUnlock(e)  // "Kapı açıldı: Baran, 14:32"
      for chat in cfg.TelegramChats:
          client.SendMessage(chat.ID, msg)  // hata yut, sonraki tick dener
      lastTS = e.TS
  st.TelegramLastUnlockTS = &lastTS
  store.SaveState(dir, st)
```

Bir chat'e gönderim başarısız olursa diğer chat'ler etkilenmez, hata
sessizce loglanır (ya da yutulur) ve `lastTS` yine ilerler — bir sohbetin
engellemesi diğerlerini kilitlemez. (Retry yok: basitlik tercih edildi,
kapsam dışına bakınız.)

**2. Komut / eşleştirme dinleyici** (`GetUpdates` long-poll, sürekli döngü)

```
tick:
  updates := client.GetUpdates(st.TelegramOffset, 25)
  for u in updates:
      st.TelegramOffset = u.UpdateID + 1
      if chatOnaylı(u.Chat):
          if u.Text == "/durum":
              reply(bugünün özeti)
      else if pendingPairingCode != "" and u.Text == pendingPairingCode:
          cfg.TelegramChats = append(..., TelegramChat{ID: u.Chat, AddedAt: now})
          config.Save(dir, cfg)
          pendingPairingCode = ""
          client.SendMessage(u.Chat, "Kaydınız tamamlandı.")
      // başka her şey: yok sayılır
  store.SaveState(dir, st)  // offset kalıcı, tekrar işlenmez
```

Onaysız sohbetlerden gelen komutlara (eşleştirme kodu dışında) yanıt
verilmez — botun varlığını ve komut setini yabancılara sızdırmamak için.

`/durum` yanıtı `report.Aggregate` kullanır, ama haftalık değil günlük
pencere: `now.Truncate(gün başı)` ile `now` arası, düz metin formatında
(HTML render kullanılmaz).

## UI — yeni "Bildirimler" ekranı (internal/ui/telegram.go)

`internal/ui/people.go` ile aynı desende, `showPeople` yanına eklenir.

Token alanı boşken, ekranın üstünde sabit bir açıklama metni gösterilir —
token nereden alınır, kullanıcı bunu bilmeden ekranı açamaz:

```
Telegram'dan bildirim almak için önce kendi botunuzu oluşturun:

  1. Telegram'da @BotFather'ı açın.
  2. /newbot yazın, botunuza bir isim verin.
  3. BotFather'ın verdiği token'ı aşağıya yapıştırıp Kaydet'e basın.

Token girilince bildirimler otomatik açılır. Sonra "Sohbet ekle" ile
kendi sohbetinizi eşleştirebilirsiniz.
```

Token doluyken bu metin gizlenir, yerine sohbet listesi ve "Sohbet ekle"
akışı gösterilir. Ayrı bir aç/kapa anahtarı yok: token'ın varlığı
özelliğin açık olduğu anlamına gelir (bkz. Kararlar) — token'ı silip
Kaydet'e basmak özelliği kapatır (watcher goroutine'leri bir sonraki
yeniden başlatmada durur).

- Bot token alanı: göster/düzenle/kaydet
- Onaylı sohbet listesi: etiket, eklenme tarihi
- **Sohbet ekle**: 6 haneli rastgele kod üretir (10 dakika geçerli),
  "Bu kodu bota gönderin: 483920" gösterir. Watcher'ın eşleştirme
  goroutine'i `pendingPairingCode`'u okuyabilmesi için UI, kodu
  `state.json`'a geçici bir alanla yazar (`TelegramPendingCode`,
  `TelegramPendingExpiry`) — böylece watcher ve UI aynı süreç olmak
  zorunda kalmaz.
- **Kaldır**: onay istenmeden anında siler (kişi silmenin aksine, kapıyı
  kilitleme riski yok — geri dönüşü kolay, token varsa yeniden eşleştirme
  bir dakika sürer).

Menüye `antigame` ana ekranından erişim: mevcut "Kişiler" düğmesinin
yanına "Bildirimler" düğmesi.

## Hata durumları

- **Token boş/geçersiz**: goroutine'ler başlamaz ya da `SendMessage`/
  `GetUpdates` 401 döner → sessizce atlanır, watcher'ın ana işlevini
  etkilemez. UI'da son hata durumu gösterilebilir (ör. "Token geçersiz
  görünüyor") ama bu bloklayıcı değildir.
- **Ağ erişimi yok**: `GetUpdates`/`SendMessage` timeout/hata → tick
  atlanır, bir sonrakinde tekrar denenir. `TelegramOffset` ve
  `TelegramLastUnlockTS` yalnızca başarılı işlemden sonra ilerletilir —
  başarısız gönderim veri kaybına yol açmaz, bir sonraki tick'te tekrar
  denenir.
- **Eşleştirme kodu süresi dolar**: `TelegramPendingExpiry` geçmişse
  watcher kodu görmezden gelir, UI "süresi doldu" gösterir.
- **Aynı anda iki eşleştirme**: yeni kod üretimi eskisinin yerine geçer,
  eski kod artık eşleşmez.
- **`gate` süreci**: bu özellikten habersizdir, hiçbir değişiklik almaz.

## Test planı

| Paket | Test |
|---|---|
| telegram | `SendMessage`/`GetUpdates` istek biçimi ve yanıt ayrıştırma, `httptest.Server` ile; hata durumunda anlamlı `error` |
| telegramwatch (unlock tarayıcı) | sahte event log'dan yalnızca yeni `unlock` olaylarının seçilmesi; `lastTS` ilerlemesi; bir chat'in hatası diğerini engellemiyor; token boşken goroutine hiç ağ çağrısı yapmıyor |
| telegramwatch (komut dinleyici) | onaylı chat'ten `/durum` doğru özeti döndürüyor; onaysız chat'ten komut yanıtsız kalıyor; doğru eşleştirme kodu chat'i onaylıyor; yanlış/süresi dolmuş kod onaylamıyor; offset tekrar işlemi önlüyor |
| watch | değişmiyor — Telegram entegrasyonu bu pakete hiç dokunmadığının regresyon kanıtı olarak mevcut testler aynen geçmeye devam eder |
| config | `TelegramChats` serileştirme/yükleme; eski config'lerde alan yoksa boş liste |
| ui | eşleştirme kodu üretimi ve state'e yazımı; kaldırmanın anında etkili olması |

## Kapsam dışı

- Oyun başlama/bitişi ve başarısız deneme bildirimleri.
- Periyodik (günlük/haftalık) otomatik özet push.
- Başarısız gönderimde yeniden deneme kuyruğu / üstel geri çekilme.
- Birden fazla bot/token desteği (tek token, çoklu sohbet yeterli).
- Sohbet başına yetki kısıtlaması (ör. bazı sohbetler yalnızca bildirim
  alır, komut kullanamaz) — tüm onaylı sohbetler eşit yetkili.
