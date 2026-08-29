# Telegram Bildirimi Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Kapı açıldığında (kim, ne zaman) onaylı Telegram sohbetlerine anlık bildirim gönder; `/durum` komutuyla günün kullanım özetini isteğe bağlı olarak döndür.

**Architecture:** Yeni bağımsız paket `internal/telegramwatch`, `watch` sürecinde ayrı bir goroutine olarak çalışır; `watch`'un 250ms'lik senkron process-tarama döngüsüne (`Step`) hiç dokunmaz. Kendi döngüsünde event log'u tarayıp yeni `unlock` olaylarını bildirir, ayrı bir döngüde Telegram `getUpdates` uzun-anketini dinleyip `/durum` komutuna ve eşleştirme koduna yanıt verir. `gate` süreci bu özellikten tamamen habersizdir.

**Tech Stack:** Go 1.26, `net/http` (repo'daki ilk gerçek internet bağımlılığı), mevcut `internal/config`/`internal/store` dosya tabanlı kalıcılık, mevcut Win32 UI katmanı (`internal/ui`).

**Spec:** `docs/superpowers/specs/2026-08-23-telegram-bildirim-design.md`

## Global Constraints

- Özellik tamamen opsiyonel: `config.TelegramToken` boşken hiçbir goroutine ağa çıkmaz, sıfır trafik.
- `internal/watch` paketi bu özellik için **değiştirilmez** — Telegram'ın ağ gecikmesi/long-poll'u anti-cheat'in 250ms tarama döngüsünü bloklayamaz.
- `internal/gate` paketi bu özellik için **değiştirilmez** — kısa ömürlü süreç ağa hiç dokunmaz.
- Bot token `config.json`'da düz metin saklanır (kullanıcı kararı, spec §Kararlar).
- Yalnızca kapı açma (`unlock`) olayı anlık bildirim üretir. Oyun başlama/bitişi ve başarısız denemeler bildirim üretmez (kapsam dışı).
- Kullanım özeti yalnızca `/durum` komutuyla, istek üzerine gönderilir — periyodik otomatik push yok.
- Başarısız gönderimde yeniden deneme kuyruğu yok; bir sonraki tarama/tick'te doğal olarak tekrar denenir.
- Tüm kullanıcıya görünen metinler Türkçe, repo genelindeki üslupla tutarlı (kısa, doğrudan).
- Yorumlar repo geleneğine uyar: Türkçe, yalnızca "neden" açıklanır, "ne" değil.

---

## Task 1: Veri modeli — config.json'a bot token ve sohbet listesi

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**Interfaces:**
- Produces: `config.TelegramChat{ID int64, Label string, AddedAt time.Time}`; `Config.TelegramToken string`; `Config.TelegramChats []TelegramChat`

- [ ] **Step 1: Write the failing test**

`internal/config/config_test.go` dosyasının sonuna ekle:

```go
func TestTelegramChatsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	added := time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC)
	c := Default()
	c.TelegramToken = "123:abc"
	c.TelegramChats = []TelegramChat{{ID: 42, Label: "Ebeveyn", AddedAt: added}}

	if err := Save(dir, c); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.TelegramToken != "123:abc" {
		t.Errorf("token korunmadi: %q", got.TelegramToken)
	}
	if len(got.TelegramChats) != 1 || got.TelegramChats[0].ID != 42 ||
		got.TelegramChats[0].Label != "Ebeveyn" || !got.TelegramChats[0].AddedAt.Equal(added) {
		t.Errorf("sohbet listesi korunmadi: %+v", got.TelegramChats)
	}
}

func TestTelegramFieldsDefaultEmpty(t *testing.T) {
	dir := t.TempDir()
	if err := Save(dir, Default()); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.TelegramToken != "" || len(got.TelegramChats) != 0 {
		t.Errorf("varsayilan config'te telegram alanlari bos olmali: %+v", got)
	}
}
```

Dosyanın başındaki `import` bloğuna `"time"` ekli değilse ekle (zaten `config_test.go` başka `time` kullanımı yoksa).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/... -run TestTelegram -v`
Expected: FAIL — `c.TelegramToken undefined (type *Config has no field or method TelegramToken)`

- [ ] **Step 3: Write minimal implementation**

`internal/config/config.go` içinde `Person` struct'ının hemen altına ekle:

```go
// TelegramChat, kapı bildirimlerini almaya onaylanmış bir Telegram
// sohbetidir. Onay, UI'da üretilen tek kullanımlık eşleştirme koduyla
// gerçekleşir (bkz. internal/telegramwatch).
type TelegramChat struct {
	ID    int64  `json:"id"`
	Label string `json:"label,omitempty"`
	// AddedAt, sohbetin ne zaman onaylandığıdır; yalnızca bilgi
	// amaçlıdır, davranışı etkilemez.
	AddedAt time.Time `json:"added_at"`
}
```

`Config` struct'ına, `NextPersonSeq` alanından sonra ekle:

```go
	// TelegramToken bosken bildirim ozelligi tamamen kapalidir: hicbir
	// goroutine aga cikmaz. Duz metin: kullanici bilerek secti (bkz.
	// spec).
	TelegramToken string         `json:"telegram_token,omitempty"`
	TelegramChats []TelegramChat `json:"telegram_chats,omitempty"`
```

Dosyanın importlarına `"time"` ekli değilse ekle.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/... -v`
Expected: PASS (tüm testler, yenileri dahil)

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): Telegram bot token ve onayli sohbet listesi alanlari"
```

---

## Task 2: Veri modeli — state.json'a tarama/offset/eşleştirme alanları

**Files:**
- Modify: `internal/store/state.go`
- Test: `internal/store/state_test.go`

**Interfaces:**
- Produces: `State.TelegramOffset int64`; `State.TelegramLastUnlockTS *time.Time`; `State.TelegramPendingCode string`; `State.TelegramPendingExpiry *time.Time`

- [ ] **Step 1: Write the failing test**

`internal/store/state_test.go` dosyasının sonuna ekle:

```go
func TestTelegramStateFieldsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	ts := time.Date(2026, 8, 23, 9, 30, 0, 0, time.UTC)
	expiry := ts.Add(10 * time.Minute)
	in := &State{
		TelegramOffset:         7,
		TelegramLastUnlockTS:   &ts,
		TelegramPendingCode:    "483920",
		TelegramPendingExpiry:  &expiry,
	}
	if err := SaveState(dir, in); err != nil {
		t.Fatalf("SaveState: %v", err)
	}
	out, err := LoadState(dir)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if out.TelegramOffset != 7 {
		t.Errorf("offset korunmadi: %d", out.TelegramOffset)
	}
	if out.TelegramLastUnlockTS == nil || !out.TelegramLastUnlockTS.Equal(ts) {
		t.Errorf("tarama isareti korunmadi: %+v", out.TelegramLastUnlockTS)
	}
	if out.TelegramPendingCode != "483920" {
		t.Errorf("bekleyen kod korunmadi: %q", out.TelegramPendingCode)
	}
	if out.TelegramPendingExpiry == nil || !out.TelegramPendingExpiry.Equal(expiry) {
		t.Errorf("kod suresi korunmadi: %+v", out.TelegramPendingExpiry)
	}
}

func TestTelegramStateFieldsDefaultEmpty(t *testing.T) {
	st, err := LoadState(t.TempDir())
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if st.TelegramOffset != 0 || st.TelegramLastUnlockTS != nil ||
		st.TelegramPendingCode != "" || st.TelegramPendingExpiry != nil {
		t.Errorf("bos durumda telegram alanlari sifir olmali: %+v", st)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/... -run TestTelegram -v`
Expected: FAIL — `unknown field TelegramOffset in struct literal`

- [ ] **Step 3: Write minimal implementation**

`internal/store/state.go`'daki `State` struct'ına, `RecoveryUsed` alanından sonra ekle:

```go
	// TelegramOffset, getUpdates'te son islenen guncellemenin bir
	// fazlasidir; ayni mesaji tekrar islememek icin.
	TelegramOffset int64 `json:"telegram_offset,omitempty"`
	// TelegramLastUnlockTS, event log taramasinin nereye kadar geldigini
	// isaretler. Bos ise (ilk calisma) gecmis taranmaz.
	TelegramLastUnlockTS *time.Time `json:"telegram_last_unlock_ts,omitempty"`
	// TelegramPendingCode, "Sohbet ekle" ile uretilen tek kullanimlik
	// eslestirme kodudur; suresi TelegramPendingExpiry'de.
	TelegramPendingCode   string     `json:"telegram_pending_code,omitempty"`
	TelegramPendingExpiry *time.Time `json:"telegram_pending_expiry,omitempty"`
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/store/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/store/state.go internal/store/state_test.go
git commit -m "feat(store): Telegram tarama/offset/eslestirme durum alanlari"
```

---

## Task 3: internal/telegram — Telegram Bot API istemcisi

**Files:**
- Create: `internal/telegram/telegram.go`
- Test: `internal/telegram/telegram_test.go`

**Interfaces:**
- Consumes: nothing (standalone, only `net/http`, `encoding/json`)
- Produces: `telegram.Client{Token string, HTTPClient *http.Client, BaseURL string}`; `(c Client) SendMessage(chatID int64, text string) error`; `(c Client) GetUpdates(offset int64, timeoutS int) ([]Update, error)`; `telegram.Update{UpdateID int64, Chat int64, Text string}`

- [ ] **Step 1: Write the failing test**

Create `internal/telegram/telegram_test.go`:

```go
package telegram

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSendMessagePostsChatAndText(t *testing.T) {
	var gotChat, gotText, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := r.ParseForm(); err != nil {
			t.Fatalf("form ayristirilamadi: %v", err)
		}
		gotChat = r.FormValue("chat_id")
		gotText = r.FormValue("text")
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": true})
	}))
	defer srv.Close()

	c := Client{Token: "TESTTOKEN", HTTPClient: srv.Client(), BaseURL: srv.URL}
	if err := c.SendMessage(42, "Kapı açıldı"); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if gotChat != "42" || gotText != "Kapı açıldı" {
		t.Errorf("beklenmeyen istek: chat=%q text=%q", gotChat, gotText)
	}
	if !strings.HasSuffix(gotPath, "/sendMessage") {
		t.Errorf("beklenmeyen yol: %q", gotPath)
	}
}

func TestSendMessageReturnsErrorOnAPIFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"ok": false, "description": "Forbidden: bot was blocked"})
	}))
	defer srv.Close()

	c := Client{Token: "t", HTTPClient: srv.Client(), BaseURL: srv.URL}
	err := c.SendMessage(1, "x")
	if err == nil || !strings.Contains(err.Error(), "Forbidden") {
		t.Fatalf("beklenen hata gelmedi: %v", err)
	}
}

func TestGetUpdatesSendsOffsetAndTimeout(t *testing.T) {
	var gotOffset, gotTimeout string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotOffset = r.URL.Query().Get("offset")
		gotTimeout = r.URL.Query().Get("timeout")
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": []any{}})
	}))
	defer srv.Close()

	c := Client{Token: "t", HTTPClient: srv.Client(), BaseURL: srv.URL}
	if _, err := c.GetUpdates(99, 25); err != nil {
		t.Fatalf("GetUpdates: %v", err)
	}
	if gotOffset != "99" || gotTimeout != "25" {
		t.Errorf("beklenmeyen sorgu: offset=%q timeout=%q", gotOffset, gotTimeout)
	}
}

func TestGetUpdatesKeepsUpdateIDForNonMessageUpdates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ok":true,"result":[
			{"update_id":10,"message":{"chat":{"id":555},"text":"/durum"}},
			{"update_id":11}
		]}`))
	}))
	defer srv.Close()

	c := Client{Token: "t", HTTPClient: srv.Client(), BaseURL: srv.URL}
	updates, err := c.GetUpdates(0, 25)
	if err != nil {
		t.Fatalf("GetUpdates: %v", err)
	}
	if len(updates) != 2 {
		t.Fatalf("2 guncelleme bekleniyordu, %d geldi", len(updates))
	}
	if updates[0].UpdateID != 10 || updates[0].Chat != 555 || updates[0].Text != "/durum" {
		t.Errorf("ilk guncelleme yanlis ayristi: %+v", updates[0])
	}
	if updates[1].UpdateID != 11 || updates[1].Chat != 0 {
		t.Errorf("mesajsiz guncellemenin UpdateID'si korunmali, Chat sifir olmali: %+v", updates[1])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/telegram/... -v`
Expected: FAIL — `package internal/telegram: no Go files in ...` (paket henüz yok)

- [ ] **Step 3: Write minimal implementation**

Create `internal/telegram/telegram.go`:

```go
// Package telegram, Telegram Bot API'sine ince bir istemcidir. Yalnizca
// bildirim gonderme (sendMessage) ve komut/eslestirme dinleme
// (getUpdates, uzun anket) icin gereken iki cagriyi kapsar. Bu, repodaki
// ilk gercek internet bagimliligidir; her cagri istege bagli ve
// hatalari yutulacak sekilde tasarlanmistir (bkz. internal/telegramwatch).
package telegram

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Client, tek bir bot token'ina bagli ince bir HTTP istemcisidir.
type Client struct {
	Token string
	// HTTPClient testte sahte sunucuya yonlendirmek icin degistirilir.
	// Nil birakilirsa varsayilan istemci (30s zaman asimi) kullanilir.
	HTTPClient *http.Client
	// BaseURL testte sahte sunucuya yonlendirmek icin degistirilir.
	// Bos birakilirsa gercek Telegram API'si kullanilir.
	BaseURL string
}

// Update, getUpdates'ten donen tek bir olaydir. Chat, mesaj disi
// guncellemelerde (ornegin edited_message) sifir kalir; cagiran bu
// durumda guncellemeyi yok saymali ama UpdateID'yi yine de offset'i
// ilerletmek icin kullanmalidir — aksi halde Telegram ayni guncellemeyi
// tekrar tekrar gonderir.
type Update struct {
	UpdateID int64
	Chat     int64
	Text     string
}

func (c Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func (c Client) baseURL() string {
	if c.BaseURL != "" {
		return c.BaseURL
	}
	return "https://api.telegram.org/bot" + c.Token
}

type apiResponse struct {
	OK          bool            `json:"ok"`
	Description string          `json:"description"`
	Result      json.RawMessage `json:"result"`
}

// SendMessage, verilen sohbete duz metin gonderir.
func (c Client) SendMessage(chatID int64, text string) error {
	form := url.Values{
		"chat_id": {strconv.FormatInt(chatID, 10)},
		"text":    {text},
	}
	resp, err := c.httpClient().PostForm(c.baseURL()+"/sendMessage", form)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var out apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return err
	}
	if !out.OK {
		return fmt.Errorf("telegram: %s", out.Description)
	}
	return nil
}

type tgUpdate struct {
	UpdateID int64 `json:"update_id"`
	Message  *struct {
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		Text string `json:"text"`
	} `json:"message"`
}

// GetUpdates, offset'ten sonraki guncellemeleri sorar. timeoutS,
// Telegram'in sunucu tarafi uzun anket suresidir (saniye); istemci
// zaman asimi bundan uzun tutulmalidir (bkz. httpClient).
func (c Client) GetUpdates(offset int64, timeoutS int) ([]Update, error) {
	q := url.Values{
		"offset":  {strconv.FormatInt(offset, 10)},
		"timeout": {strconv.Itoa(timeoutS)},
	}
	resp, err := c.httpClient().Get(c.baseURL() + "/getUpdates?" + q.Encode())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if !out.OK {
		return nil, fmt.Errorf("telegram: %s", out.Description)
	}
	var raw []tgUpdate
	if err := json.Unmarshal(out.Result, &raw); err != nil {
		return nil, err
	}
	updates := make([]Update, 0, len(raw))
	for _, u := range raw {
		up := Update{UpdateID: u.UpdateID}
		if u.Message != nil {
			up.Chat = u.Message.Chat.ID
			up.Text = u.Message.Text
		}
		updates = append(updates, up)
	}
	return updates, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/telegram/... -v`
Expected: PASS (4 test)

- [ ] **Step 5: Commit**

```bash
git add internal/telegram
git commit -m "feat(telegram): Bot API istemcisi (sendMessage, getUpdates)"
```

---

## Task 4: internal/telegramwatch — unlock tarayıcı

**Files:**
- Create: `internal/telegramwatch/unlock.go`
- Test: `internal/telegramwatch/unlock_test.go`

**Interfaces:**
- Consumes: `config.Config{TelegramChats}`, `config.Config.FindPerson`, `store.State.TelegramLastUnlockTS`, `store.LoadState`/`SaveState`/`Read`/`Append`, `store.Event{TS, Ev, Who, Method}`
- Produces: `sender` interface (`SendMessage(chatID int64, text string) error`) — Task 6 ve Task 7 bunu kullanır; `scanUnlocks(dir string, cfg *config.Config, client sender, now time.Time) error`; `formatUnlock(e store.Event, cfg *config.Config) string`

- [ ] **Step 1: Write the failing test**

Create `internal/telegramwatch/unlock_test.go`:

```go
package telegramwatch

import (
	"errors"
	"testing"
	"time"

	"github.com/guts/antigame/internal/config"
	"github.com/guts/antigame/internal/store"
)

// fakeSender, gercek aga cikmadan gonderimleri kaydeder. Diger test
// dosyalarinda (command_test.go) da kullanilir.
type fakeSender struct {
	sent []sentMsg
	// failChat, gonderimi kasten basarisiz kilinacak sohbet ID'sidir.
	failChat int64
}

type sentMsg struct {
	chat int64
	text string
}

func (f *fakeSender) SendMessage(chatID int64, text string) error {
	if chatID == f.failChat {
		return errors.New("gonderim basarisiz")
	}
	f.sent = append(f.sent, sentMsg{chatID, text})
	return nil
}

func TestScanUnlocksFirstRunSetsBookmarkWithoutSending(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC)
	if err := store.Append(dir, store.Event{TS: now.Add(-time.Hour), Ev: "unlock", Who: "p1"}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	cfg := &config.Config{TelegramChats: []config.TelegramChat{{ID: 1}}}
	fs := &fakeSender{}

	if err := scanUnlocks(dir, cfg, fs, now); err != nil {
		t.Fatalf("scanUnlocks: %v", err)
	}
	if len(fs.sent) != 0 {
		t.Fatalf("ilk taramada gecmis bildirilmemeli, %d mesaj gonderildi", len(fs.sent))
	}
	st, err := store.LoadState(dir)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if st.TelegramLastUnlockTS == nil || !st.TelegramLastUnlockTS.Equal(now) {
		t.Fatalf("isaret simdiye kurulmadi: %+v", st.TelegramLastUnlockTS)
	}
}

func TestScanUnlocksSendsNewUnlocksToAllChats(t *testing.T) {
	dir := t.TempDir()
	base := time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC)
	if err := store.SaveState(dir, &store.State{TelegramLastUnlockTS: &base}); err != nil {
		t.Fatalf("SaveState: %v", err)
	}
	evTS := base.Add(5 * time.Minute)
	if err := store.Append(dir, store.Event{TS: evTS, Ev: "unlock", Who: "p1"}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	cfg := &config.Config{
		People:        []config.Person{{ID: "p1", Name: "Baran"}},
		TelegramChats: []config.TelegramChat{{ID: 1}, {ID: 2}},
	}
	fs := &fakeSender{}
	now := evTS.Add(time.Minute)

	if err := scanUnlocks(dir, cfg, fs, now); err != nil {
		t.Fatalf("scanUnlocks: %v", err)
	}
	if len(fs.sent) != 2 {
		t.Fatalf("2 sohbete gonderim bekleniyordu, %d geldi", len(fs.sent))
	}
	want := "Kapı açıldı: Baran, " + evTS.Local().Format("15:04")
	for _, m := range fs.sent {
		if m.text != want {
			t.Errorf("beklenmeyen mesaj: got %q want %q", m.text, want)
		}
	}
	st, err := store.LoadState(dir)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if !st.TelegramLastUnlockTS.Equal(evTS) {
		t.Fatalf("isaret ilerlemedi: %+v", st.TelegramLastUnlockTS)
	}
}

func TestScanUnlocksOneChatFailureDoesNotBlockOthersOrBookmark(t *testing.T) {
	dir := t.TempDir()
	base := time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC)
	if err := store.SaveState(dir, &store.State{TelegramLastUnlockTS: &base}); err != nil {
		t.Fatalf("SaveState: %v", err)
	}
	evTS := base.Add(time.Minute)
	if err := store.Append(dir, store.Event{TS: evTS, Ev: "unlock", Who: ""}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	cfg := &config.Config{TelegramChats: []config.TelegramChat{{ID: 1}, {ID: 2}}}
	fs := &fakeSender{failChat: 1}

	if err := scanUnlocks(dir, cfg, fs, evTS.Add(time.Second)); err != nil {
		t.Fatalf("scanUnlocks: %v", err)
	}
	if len(fs.sent) != 1 || fs.sent[0].chat != 2 {
		t.Fatalf("basarisiz sohbet digerini engellememeli: %+v", fs.sent)
	}
	st, err := store.LoadState(dir)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if !st.TelegramLastUnlockTS.Equal(evTS) {
		t.Fatalf("basarisiz gonderim isareti ilerletmeyi engellememeli: %+v", st.TelegramLastUnlockTS)
	}
}

func TestFormatUnlockUsesRecoveryLabel(t *testing.T) {
	e := store.Event{TS: time.Date(2026, 8, 23, 14, 32, 0, 0, time.UTC), Method: "recovery"}
	got := formatUnlock(e, &config.Config{})
	want := "Kapı açıldı: Kurtarma kodu, " + e.TS.Local().Format("15:04")
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestFormatUnlockFallsBackToIDWhenPersonUnknown(t *testing.T) {
	e := store.Event{TS: time.Date(2026, 8, 23, 14, 32, 0, 0, time.UTC), Who: "p9"}
	got := formatUnlock(e, &config.Config{})
	want := "Kapı açıldı: p9, " + e.TS.Local().Format("15:04")
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/telegramwatch/... -v`
Expected: FAIL — `package internal/telegramwatch: no Go files in ...`

- [ ] **Step 3: Write minimal implementation**

Create `internal/telegramwatch/unlock.go`:

```go
// Package telegramwatch, kapı olaylarını Telegram'a bildirir ve
// /durum komutuna yanıt verir. watch ve gate paketlerinden tamamen
// bağımsız çalışır: ağ çağrıları hiçbir zaman anti-cheat'in kritik
// döngüsünü bloklamaz (bkz. spec, "Watcher entegrasyonu").
package telegramwatch

import (
	"time"

	"github.com/guts/antigame/internal/config"
	"github.com/guts/antigame/internal/store"
)

// sender, telegram.Client'in bu paketin kullandigi tek metodudur.
// Arayuz olmasi testte gercek aga cikmadan sahte bir gonderici
// takilabilmesini saglar.
type sender interface {
	SendMessage(chatID int64, text string) error
}

// scanUnlocks, son taramadan bu yana yazilan "unlock" olaylarini
// onayli sohbetlere bildirir ve tarama isaretini ilerletir.
//
// Ilk cagrida (state.json'da isaret yoksa) gecmis taranmaz, isaret
// yalnizca "simdi"ye kurulur: kurulumdan once acilmis kapilar icin
// geriye donuk bildirim atilmaz.
func scanUnlocks(dir string, cfg *config.Config, client sender, now time.Time) error {
	st, err := store.LoadState(dir)
	if err != nil {
		return err
	}
	if st.TelegramLastUnlockTS == nil {
		st.TelegramLastUnlockTS = &now
		return store.SaveState(dir, st)
	}

	from := st.TelegramLastUnlockTS.Add(time.Nanosecond)
	if from.After(now) {
		return nil
	}
	events, err := store.Read(dir, from, now)
	if err != nil {
		return err
	}

	last := *st.TelegramLastUnlockTS
	for _, e := range events {
		if e.Ev != "unlock" {
			continue
		}
		msg := formatUnlock(e, cfg)
		for _, chat := range cfg.TelegramChats {
			// Gonderim hatasi yutulur: bir sohbetin engellenmesi
			// digerlerini veya bir sonraki taramayi kilitlememeli.
			_ = client.SendMessage(chat.ID, msg)
		}
		if e.TS.After(last) {
			last = e.TS
		}
	}
	if !last.After(*st.TelegramLastUnlockTS) {
		return nil
	}
	st.TelegramLastUnlockTS = &last
	return store.SaveState(dir, st)
}

// formatUnlock, kapi acma bildirim metnini uretir.
func formatUnlock(e store.Event, cfg *config.Config) string {
	who := e.Who
	switch {
	case e.Method == "recovery":
		who = "Kurtarma kodu"
	case who == "":
		who = "Bilinmeyen"
	default:
		if p, ok := cfg.FindPerson(who); ok {
			who = p.Name
		}
	}
	return "Kapı açıldı: " + who + ", " + e.TS.Local().Format("15:04")
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/telegramwatch/... -v`
Expected: PASS (5 test)

- [ ] **Step 5: Commit**

```bash
git add internal/telegramwatch
git commit -m "feat(telegramwatch): unlock olaylarini tarayip bildirme"
```

---

## Task 5: internal/telegramwatch — günlük özet (/durum içeriği)

**Files:**
- Create: `internal/telegramwatch/summary.go`
- Test: `internal/telegramwatch/summary_test.go`

**Interfaces:**
- Consumes: `config.Config.Match`, `config.Config.FindPerson`, `store.Read`, `store.Event{TS, Ev, Who, Exe, DurS, Method}`
- Produces: `dailySummary(dir string, cfg *config.Config, now time.Time) (string, error)`; `formatDur(s int) string` — Task 6 `dailySummary`'yi kullanır

- [ ] **Step 1: Write the failing test**

Create `internal/telegramwatch/summary_test.go`:

```go
package telegramwatch

import (
	"strings"
	"testing"
	"time"

	"github.com/guts/antigame/internal/config"
	"github.com/guts/antigame/internal/store"
)

func TestDailySummaryNoEventsToday(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	got, err := dailySummary(dir, &config.Config{}, now)
	if err != nil {
		t.Fatalf("dailySummary: %v", err)
	}
	if got != "Bugün henüz hareket yok." {
		t.Errorf("beklenmeyen ozet: %q", got)
	}
}

func TestDailySummarySumsDurationAndUnlocksPerPerson(t *testing.T) {
	dir := t.TempDir()
	day := time.Date(2026, 8, 23, 9, 0, 0, 0, time.UTC)
	events := []store.Event{
		{TS: day.Add(time.Hour), Ev: "unlock", Who: "p1"},
		{TS: day.Add(2 * time.Hour), Ev: "game_end", Who: "p1", Exe: "VALORANT.exe", DurS: 5400},
		{TS: day.Add(3 * time.Hour), Ev: "game_end", Who: "p1", Exe: "VALORANT.exe", DurS: 3000},
	}
	for _, e := range events {
		if err := store.Append(dir, e); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}
	cfg := &config.Config{People: []config.Person{{ID: "p1", Name: "Baran"}}}
	now := day.Add(4 * time.Hour)

	got, err := dailySummary(dir, cfg, now)
	if err != nil {
		t.Fatalf("dailySummary: %v", err)
	}
	if !strings.Contains(got, "Baran — 2s 20dk, kapı 1 kez açıldı") {
		t.Errorf("beklenen satiri icermiyor: %q", got)
	}
}

func TestDailySummaryExcludesLauncherDuration(t *testing.T) {
	dir := t.TempDir()
	day := time.Date(2026, 8, 23, 9, 0, 0, 0, time.UTC)
	if err := store.Append(dir, store.Event{
		TS: day.Add(time.Hour), Ev: "game_end", Who: "p1", Exe: "RiotClientServices.exe", DurS: 999,
	}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	cfg := &config.Config{Gated: []config.Game{{Exe: "RiotClientServices.exe", Launcher: true}}}
	got, err := dailySummary(dir, cfg, day.Add(2*time.Hour))
	if err != nil {
		t.Fatalf("dailySummary: %v", err)
	}
	if got != "Bugün henüz hareket yok." {
		t.Errorf("baslatici suresi ozete sizmis: %q", got)
	}
}

func TestFormatDurUnderHour(t *testing.T) {
	if got := formatDur(600); got != "10dk" {
		t.Errorf("got %q want 10dk", got)
	}
}

func TestFormatDurOverHour(t *testing.T) {
	if got := formatDur(8400); got != "2s 20dk" {
		t.Errorf("got %q want 2s 20dk", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/telegramwatch/... -run TestDailySummary -v`
Expected: FAIL — `undefined: dailySummary`

- [ ] **Step 3: Write minimal implementation**

Create `internal/telegramwatch/summary.go`:

```go
package telegramwatch

import (
	"fmt"
	"strings"
	"time"

	"github.com/guts/antigame/internal/config"
	"github.com/guts/antigame/internal/store"
)

// dailySummary, bugunun (yerel takvim gunu) kisi basina oyun suresini
// ve kapi acma sayisini duz metin olarak uretir.
//
// report.Aggregate kullanilmaz: o haftalik pencereye (weekStart) sabit,
// gunluk bir aralik kabul etmiyor.
func dailySummary(dir string, cfg *config.Config, now time.Time) (string, error) {
	loc := now.Location()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	events, err := store.Read(dir, dayStart, now)
	if err != nil {
		return "", err
	}

	type totals struct {
		durS    int
		unlocks int
	}
	per := map[string]*totals{}
	var order []string
	get := func(who string) *totals {
		t, ok := per[who]
		if !ok {
			t = &totals{}
			per[who] = t
			order = append(order, who)
		}
		return t
	}

	for _, e := range events {
		switch e.Ev {
		case "game_end":
			// Baslaticilar rapor/aggregate.go'daki ayni kuralla elenir:
			// istemci hem baslatici hem oyun olarak sayilirsa sure katlanir.
			if g, gated := cfg.Match(e.Exe, ""); gated && g.Launcher {
				continue
			}
			get(e.Who).durS += e.DurS
		case "unlock":
			if e.Method == "recovery" {
				continue
			}
			get(e.Who).unlocks++
		}
	}

	if len(order) == 0 {
		return "Bugün henüz hareket yok.", nil
	}
	var b strings.Builder
	b.WriteString("Bugün:\n")
	for _, who := range order {
		name := who
		if who == "" {
			name = "Kapı yokken"
		} else if p, ok := cfg.FindPerson(who); ok {
			name = p.Name
		}
		t := per[who]
		fmt.Fprintf(&b, "  %s — %s, kapı %d kez açıldı\n", name, formatDur(t.durS), t.unlocks)
	}
	return b.String(), nil
}

// formatDur, saniyeyi "2s 14dk" ya da saat yoksa "14dk" bicimine cevirir.
func formatDur(s int) string {
	h := s / 3600
	m := (s % 3600) / 60
	if h > 0 {
		return fmt.Sprintf("%ds %ddk", h, m)
	}
	return fmt.Sprintf("%ddk", m)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/telegramwatch/... -v`
Expected: PASS (10 test)

- [ ] **Step 5: Commit**

```bash
git add internal/telegramwatch/summary.go internal/telegramwatch/summary_test.go
git commit -m "feat(telegramwatch): /durum icin gunluk kullanim ozeti"
```

---

## Task 6: internal/telegramwatch — komut yönlendirme ve eşleştirme onayı

**Files:**
- Create: `internal/telegramwatch/command.go`
- Test: `internal/telegramwatch/command_test.go`

**Interfaces:**
- Consumes: `telegram.Update{UpdateID, Chat, Text}` (Task 3), `sender` (Task 4), `dailySummary` (Task 5), `config.Save`, `store.SaveState`, `config.TelegramChat`
- Produces: `handleUpdate(dir string, cfg *config.Config, st *store.State, u telegram.Update, client sender, now time.Time) error`; `approvedChat(cfg *config.Config, chatID int64) bool` — Task 7 bu ikisini kullanır

- [ ] **Step 1: Write the failing test**

Create `internal/telegramwatch/command_test.go`:

```go
package telegramwatch

import (
	"testing"
	"time"

	"github.com/guts/antigame/internal/config"
	"github.com/guts/antigame/internal/store"
	"github.com/guts/antigame/internal/telegram"
)

func TestHandleUpdateApprovedChatDurumRepliesWithSummary(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{TelegramChats: []config.TelegramChat{{ID: 7}}}
	st := &store.State{}
	fs := &fakeSender{}
	now := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)

	if err := handleUpdate(dir, cfg, st, telegram.Update{Chat: 7, Text: "/durum"}, fs, now); err != nil {
		t.Fatalf("handleUpdate: %v", err)
	}
	if len(fs.sent) != 1 || fs.sent[0].chat != 7 {
		t.Fatalf("ozet yaniti beklenen sohbete gitmedi: %+v", fs.sent)
	}
}

func TestHandleUpdateUnapprovedChatCommandIsIgnored(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{}
	st := &store.State{}
	fs := &fakeSender{}

	if err := handleUpdate(dir, cfg, st, telegram.Update{Chat: 99, Text: "/durum"}, fs, time.Now()); err != nil {
		t.Fatalf("handleUpdate: %v", err)
	}
	if len(fs.sent) != 0 {
		t.Fatalf("onaysiz sohbete yanit gitmemeli: %+v", fs.sent)
	}
}

func TestHandleUpdateMatchingPairingCodeApprovesChat(t *testing.T) {
	dir := t.TempDir()
	if err := config.Save(dir, &config.Config{}); err != nil {
		t.Fatalf("config.Save: %v", err)
	}
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	expiry := time.Now().Add(10 * time.Minute)
	st := &store.State{TelegramPendingCode: "483920", TelegramPendingExpiry: &expiry}
	fs := &fakeSender{}

	if err := handleUpdate(dir, cfg, st, telegram.Update{Chat: 42, Text: "483920"}, fs, time.Now()); err != nil {
		t.Fatalf("handleUpdate: %v", err)
	}
	if !approvedChat(cfg, 42) {
		t.Fatalf("sohbet onaylanmadi: %+v", cfg.TelegramChats)
	}
	if st.TelegramPendingCode != "" || st.TelegramPendingExpiry != nil {
		t.Fatalf("bekleyen kod temizlenmedi: %+v", st)
	}
	onDisk, err := config.Load(dir)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if !approvedChat(onDisk, 42) {
		t.Fatalf("onay diske yazilmadi")
	}
	if len(fs.sent) != 1 || fs.sent[0].chat != 42 {
		t.Fatalf("onay mesaji gonderilmedi: %+v", fs.sent)
	}
}

func TestHandleUpdateExpiredPairingCodeIsIgnored(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{}
	expiry := time.Now().Add(-time.Minute)
	st := &store.State{TelegramPendingCode: "111111", TelegramPendingExpiry: &expiry}
	fs := &fakeSender{}

	if err := handleUpdate(dir, cfg, st, telegram.Update{Chat: 42, Text: "111111"}, fs, time.Now()); err != nil {
		t.Fatalf("handleUpdate: %v", err)
	}
	if approvedChat(cfg, 42) {
		t.Fatalf("suresi dolmus kod sohbeti onaylamamali")
	}
}

func TestHandleUpdateWrongTextDoesNotApprove(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{}
	expiry := time.Now().Add(10 * time.Minute)
	st := &store.State{TelegramPendingCode: "483920", TelegramPendingExpiry: &expiry}
	fs := &fakeSender{}

	if err := handleUpdate(dir, cfg, st, telegram.Update{Chat: 42, Text: "yanlış"}, fs, time.Now()); err != nil {
		t.Fatalf("handleUpdate: %v", err)
	}
	if approvedChat(cfg, 42) {
		t.Fatalf("yanlis metin sohbeti onaylamamali")
	}
}

func TestHandleUpdateNonMessageUpdateIsIgnored(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{}
	st := &store.State{}
	fs := &fakeSender{}

	if err := handleUpdate(dir, cfg, st, telegram.Update{UpdateID: 1, Chat: 0}, fs, time.Now()); err != nil {
		t.Fatalf("handleUpdate: %v", err)
	}
	if len(fs.sent) != 0 {
		t.Fatalf("mesajsiz guncelleme yanit uretmemeli: %+v", fs.sent)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/telegramwatch/... -run TestHandleUpdate -v`
Expected: FAIL — `undefined: handleUpdate`

- [ ] **Step 3: Write minimal implementation**

Create `internal/telegramwatch/command.go`:

```go
package telegramwatch

import (
	"strings"
	"time"

	"github.com/guts/antigame/internal/config"
	"github.com/guts/antigame/internal/store"
	"github.com/guts/antigame/internal/telegram"
)

// handleUpdate, tek bir Telegram guncellemesini isler:
//   - onayli sohbetten "/durum" -> gunun ozeti yollanir
//   - onaysiz sohbetten, bekleyen eslestirme koduyla birebir eslesen
//     metin -> sohbet onaylanir ve config'e yazilir
//   - diger her sey yok sayilir (botun varligini yabancilara sizdirmamak icin)
func handleUpdate(dir string, cfg *config.Config, st *store.State, u telegram.Update, client sender, now time.Time) error {
	if u.Chat == 0 {
		return nil
	}
	text := strings.TrimSpace(u.Text)

	if approvedChat(cfg, u.Chat) {
		if text == "/durum" {
			summary, err := dailySummary(dir, cfg, now)
			if err != nil {
				return err
			}
			return client.SendMessage(u.Chat, summary)
		}
		return nil
	}

	if st.TelegramPendingCode == "" || st.TelegramPendingExpiry == nil || now.After(*st.TelegramPendingExpiry) {
		return nil
	}
	if text != st.TelegramPendingCode {
		return nil
	}

	cfg.TelegramChats = append(cfg.TelegramChats, config.TelegramChat{ID: u.Chat, AddedAt: now})
	if err := config.Save(dir, cfg); err != nil {
		return err
	}
	st.TelegramPendingCode = ""
	st.TelegramPendingExpiry = nil
	if err := store.SaveState(dir, st); err != nil {
		return err
	}
	return client.SendMessage(u.Chat, "Kaydınız tamamlandı.")
}

func approvedChat(cfg *config.Config, chatID int64) bool {
	for _, c := range cfg.TelegramChats {
		if c.ID == chatID {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/telegramwatch/... -v`
Expected: PASS (16 test)

- [ ] **Step 5: Commit**

```bash
git add internal/telegramwatch/command.go internal/telegramwatch/command_test.go
git commit -m "feat(telegramwatch): /durum komutu ve eslestirme kodu onayi"
```

---

## Task 7: internal/telegramwatch — Run orkestrasyonu ve watch sürecine bağlama

**Files:**
- Create: `internal/telegramwatch/run.go`
- Test: `internal/telegramwatch/run_test.go`
- Modify: `cmd/antigame/main.go:358-365` (bkz. Step 6)

**Interfaces:**
- Consumes: `scanUnlocks`, `handleUpdate` (Task 4, Task 6), `telegram.Client` (Task 3)
- Produces: `telegramwatch.Run(ctx context.Context, dirFunc func() string) error`

- [ ] **Step 1: Write the failing test**

Create `internal/telegramwatch/run_test.go`:

```go
package telegramwatch

import (
	"context"
	"testing"
	"time"
)

func TestRunReturnsPromptlyWhenContextCancelled(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	doneCh := make(chan error, 1)
	go func() { doneCh <- Run(ctx, func() string { return dir }) }()

	select {
	case err := <-doneCh:
		if err != nil {
			t.Fatalf("Run hata dondu: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run, context iptalinden sonra donmedi")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/telegramwatch/... -run TestRun -v`
Expected: FAIL — `undefined: Run`

- [ ] **Step 3: Write minimal implementation**

Create `internal/telegramwatch/run.go`:

```go
package telegramwatch

import (
	"context"
	"time"

	"github.com/guts/antigame/internal/config"
	"github.com/guts/antigame/internal/store"
	"github.com/guts/antigame/internal/telegram"
)

const (
	unlockScanEvery  = 10 * time.Second
	updatesLongPollS = 25
	updatesRetryWait = 5 * time.Second
	idleRecheckEvery = 10 * time.Second
)

// Run, Telegram entegrasyonunu baslatir ve ctx iptal edilene kadar
// calisir. Iki bagimsiz dongu yurutur: biri kapi acma olaylarini
// tarayip bildirir, digeri komut/eslestirme mesajlarini dinler. Ikisi
// de yalnizca cfg.TelegramToken doluyken gercek ag cagrisi yapar; bos
// tokenla sifir trafik uretirler.
//
// watch paketiyle bellek ici hicbir sey paylasilmaz: ikisi de kendi
// config.Load/store.LoadState dongusunu yurutur, tipki gate ve watch
// sureclerinin bugun de state.json/config.json uzerinden kilitsiz
// haberlestigi gibi.
func Run(ctx context.Context, dirFunc func() string) error {
	done := make(chan struct{}, 2)
	go func() { runUnlockScanner(ctx, dirFunc); done <- struct{}{} }()
	go func() { runCommandListener(ctx, dirFunc); done <- struct{}{} }()
	<-done
	<-done
	return nil
}

func runUnlockScanner(ctx context.Context, dirFunc func() string) {
	t := time.NewTicker(unlockScanEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			dir := dirFunc()
			cfg, err := config.Load(dir)
			if err != nil || cfg.TelegramToken == "" || len(cfg.TelegramChats) == 0 {
				continue
			}
			client := telegram.Client{Token: cfg.TelegramToken}
			_ = scanUnlocks(dir, cfg, client, time.Now().UTC())
		}
	}
}

func runCommandListener(ctx context.Context, dirFunc func() string) {
	for {
		if ctx.Err() != nil {
			return
		}
		dir := dirFunc()
		cfg, err := config.Load(dir)
		if err != nil || cfg.TelegramToken == "" {
			if !sleepCtx(ctx, idleRecheckEvery) {
				return
			}
			continue
		}
		client := telegram.Client{Token: cfg.TelegramToken}
		st, err := store.LoadState(dir)
		if err != nil {
			if !sleepCtx(ctx, updatesRetryWait) {
				return
			}
			continue
		}
		updates, err := client.GetUpdates(st.TelegramOffset, updatesLongPollS)
		if err != nil {
			if !sleepCtx(ctx, updatesRetryWait) {
				return
			}
			continue
		}
		if len(updates) == 0 {
			continue
		}
		for _, u := range updates {
			_ = handleUpdate(dir, cfg, st, u, client, time.Now().UTC())
			st.TelegramOffset = u.UpdateID + 1
		}
		_ = store.SaveState(dir, st)
	}
}

// sleepCtx, ctx iptal olana ya da sure dolana kadar bekler. ctx iptal
// olursa false doner; cagiran bunu "artik calismaya devam etme" olarak
// okur.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/telegramwatch/... -v`
Expected: PASS (17 test)

- [ ] **Step 5: Wire into `cmd/antigame/main.go`**

`cmd/antigame/main.go` içindeki import bloğuna ekle (alfabetik sıra korunur):

```go
	"github.com/guts/antigame/internal/task"
	"github.com/guts/antigame/internal/telegramwatch"
	"github.com/guts/antigame/internal/tray"
```

`runWatch` içinde şu bloğu bul:

```go
	watcher := make(chan error, 1)
	go func() { watcher <- w.Run(ctx) }()

	if err := tray.Run(ctx, tray.Options{
```

Şununla değiştir:

```go
	watcher := make(chan error, 1)
	go func() { watcher <- w.Run(ctx) }()

	// Telegram bildirimi izleyiciden tamamen bagimsiz calisir: ag
	// cagrisi anti-cheat'in kritik dongusunu hicbir zaman bloklamamali
	// (bkz. internal/telegramwatch paket belgesi).
	go func() { _ = telegramwatch.Run(ctx, config.Dir) }()

	if err := tray.Run(ctx, tray.Options{
```

- [ ] **Step 6: Verify wiring compiles**

Run: `go build ./...`
Expected: derleme hatasız biter.

- [ ] **Step 7: Commit**

```bash
git add internal/telegramwatch/run.go internal/telegramwatch/run_test.go cmd/antigame/main.go
git commit -m "feat(telegramwatch): Run orkestrasyonu, izleyiciye bagimsiz goroutine olarak baglama"
```

---

## Task 8: UI — onaylı sohbetleri listeleme satırları

**Files:**
- Modify: `internal/ui/rows.go`
- Test: `internal/ui/rows_test.go`

**Interfaces:**
- Consumes: `config.TelegramChat{ID, Label, AddedAt}` (Task 1)
- Produces: `ChatRows(chats []config.TelegramChat) []Row` — Task 9 kullanır

- [ ] **Step 1: Write the failing test**

`internal/ui/rows_test.go` dosyasının sonuna ekle:

```go
func TestChatRowsUsesLabelWhenSet(t *testing.T) {
	added := time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC)
	rows := ChatRows([]config.TelegramChat{{ID: 1, Label: "Ebeveyn", AddedAt: added}})
	if rows[0].Cells[0] != "Ebeveyn" {
		t.Errorf("etiket kullanilmadi: %q", rows[0].Cells[0])
	}
}

func TestChatRowsFallsBackToChatID(t *testing.T) {
	rows := ChatRows([]config.TelegramChat{{ID: 42}})
	if rows[0].Cells[0] != "Sohbet 42" {
		t.Errorf("beklenen varsayilan etiket degil: %q", rows[0].Cells[0])
	}
}

func TestChatRowsEmptyListIsEmptyNotNil(t *testing.T) {
	if rows := ChatRows(nil); rows == nil {
		t.Error("bos liste nil degil bos dilim olmali")
	}
}
```

Dosyanın `import` bloğuna `"time"` ekli değilse ekle.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ui/... -run TestChatRows -v`
Expected: FAIL — `undefined: ChatRows`

- [ ] **Step 3: Write minimal implementation**

`internal/ui/rows.go`'daki import bloğuna `"fmt"` ekle, dosyanın sonuna ekle:

```go
// ChatRows, onayli Telegram sohbetlerini goruntulenecek satirlara
// cevirir. Sutunlar: Sohbet, Eklenme.
func ChatRows(chats []config.TelegramChat) []Row {
	rows := make([]Row, 0, len(chats))
	for _, c := range chats {
		label := c.Label
		if label == "" {
			label = fmt.Sprintf("Sohbet %d", c.ID)
		}
		rows = append(rows, Row{Cells: []string{label, c.AddedAt.Local().Format("2006-01-02 15:04")}})
	}
	return rows
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ui/... -run "TestChatRows|TestGameRows|TestPeopleRows" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ui/rows.go internal/ui/rows_test.go
git commit -m "feat(ui): onayli Telegram sohbetleri icin liste satirlari"
```

---

## Task 9: UI — Bildirimler ekranı ve menü bağlantısı

**Files:**
- Create: `internal/ui/pairingcode.go`
- Test: `internal/ui/pairingcode_test.go`
- Create: `internal/ui/telegram.go`
- Modify: `internal/ui/window.go` (komut kimlikleri + `onCommand` switch)
- Modify: `internal/ui/menubar.go` (menü girdisi)

**Interfaces:**
- Consumes: `ChatRows` (Task 8), `config.TelegramChat`/`TelegramToken`/`TelegramChats` (Task 1), `store.State.TelegramPendingCode`/`TelegramPendingExpiry` (Task 2), mevcut modal yardımcıları (`newModal`, `m.label`, `m.edit`, `m.button`, `m.list`, `lvSetRows`, `lvSelected`, `setText`, `textOf`, `warn`, `copySecret`)
- Produces: `showNotifications(parent uintptr, dir string)`; `newPairingCode() (string, error)`; `idNotifications` sabiti

- [ ] **Step 1: Write the failing test**

Create `internal/ui/pairingcode_test.go`:

```go
//go:build windows

package ui

import (
	"strconv"
	"testing"
)

func TestNewPairingCodeIsSixNumericDigits(t *testing.T) {
	for i := 0; i < 20; i++ {
		code, err := newPairingCode()
		if err != nil {
			t.Fatalf("newPairingCode: %v", err)
		}
		if len(code) != 6 {
			t.Fatalf("kod 6 haneli degil: %q", code)
		}
		if _, err := strconv.Atoi(code); err != nil {
			t.Fatalf("kod sayisal degil: %q (%v)", code, err)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ui/... -run TestNewPairingCode -v`
Expected: FAIL — `undefined: newPairingCode`

- [ ] **Step 3: Write minimal implementation**

Create `internal/ui/pairingcode.go`:

```go
//go:build windows

package ui

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

// newPairingCode, "Sohbet ekle" akisinda kullanicinin bota gonderecegi
// 6 haneli rastgele kodu uretir.
func newPairingCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ui/... -run TestNewPairingCode -v`
Expected: PASS

- [ ] **Step 5: Create the Bildirimler dialog**

Create `internal/ui/telegram.go`:

```go
//go:build windows

package ui

import (
	"strings"
	"time"

	"github.com/guts/antigame/internal/config"
	"github.com/guts/antigame/internal/store"
)

// pairingCodeTTL, uretilen eslestirme kodunun gecerlilik suresidir.
const pairingCodeTTL = 10 * time.Minute

const notificationsHelpText = `Telegram'dan bildirim almak için önce kendi botunuzu oluşturun:

  1. Telegram'da @BotFather'ı açın.
  2. /newbot yazın, botunuza bir isim verin.
  3. BotFather'ın verdiği token'ı aşağıya yapıştırıp Kaydet'e basın.

Token girilince bildirimler otomatik açılır. Sonra "Sohbet ekle" ile
kendi sohbetinizi eşleştirebilirsiniz.`

// showNotifications, Telegram bot token'ini ve onayli sohbetleri
// yonetme diyalogunu acar.
func showNotifications(parent uintptr, dir string) {
	m, err := newModal(parent, "Bildirimler — Telegram", 540, 440)
	if err != nil {
		warn(parent, "antigame", "Pencere açılamadı: "+err.Error())
		return
	}

	m.label("Bot token:", Rect{12, 12, 80, 20})
	tokenBox := m.edit("", Rect{96, 8, 340, 26}, 0)
	_, saveTokenID := m.button("Kaydet", Rect{444, 8, 84, 26}, variantSecondary, false)

	help := m.label(notificationsHelpText, Rect{12, 42, 516, 92})

	list := m.list(Rect{12, 140, 516, 156},
		[]string{"Sohbet", "Eklenme"}, []int32{300, 200})

	status := m.label("", Rect{12, 300, 516, 36})

	_, addID := m.button("Sohbet ekle", Rect{12, 344, 110, 28}, variantSecondary, false)
	_, removeID := m.button("Kaldır", Rect{130, 344, 90, 28}, variantDanger, false)
	_, closeID := m.button("Kapat", Rect{438, 344, 90, 28}, variantSecondary, true)

	var chats []config.TelegramChat
	var tokenSet bool

	reload := func() {
		c, err := config.Load(dir)
		if err != nil {
			setText(status, "Yapılandırma okunamadı: "+err.Error())
			return
		}
		chats = c.TelegramChats
		tokenSet = c.TelegramToken != ""
		setText(tokenBox, c.TelegramToken)
		if tokenSet {
			setText(help, "Bot bağlandı. Aşağıdan sohbet ekleyip kaldırabilirsiniz.")
		} else {
			setText(help, notificationsHelpText)
		}
		lvSetRows(list, ChatRows(chats))
	}

	selected := func() (config.TelegramChat, bool) {
		i := lvSelected(list)
		if i < 0 || i >= len(chats) {
			setText(status, "Önce listeden bir sohbet seçin.")
			return config.TelegramChat{}, false
		}
		return chats[i], true
	}

	m.onCmd = func(id int) {
		switch id {
		case closeID, idCancel, idOK:
			m.close()

		case saveTokenID:
			c, err := config.Load(dir)
			if err != nil {
				setText(status, "Yapılandırma okunamadı: "+err.Error())
				return
			}
			c.TelegramToken = strings.TrimSpace(textOf(tokenBox))
			if err := config.Save(dir, c); err != nil {
				setText(status, "Kaydedilemedi: "+err.Error())
				return
			}
			setText(status, "Token kaydedildi.")
			reload()

		case addID:
			if !tokenSet {
				setText(status, "Önce bot token girip kaydedin.")
				return
			}
			code, err := newPairingCode()
			if err != nil {
				setText(status, "Kod üretilemedi: "+err.Error())
				return
			}
			st, err := store.LoadState(dir)
			if err != nil {
				setText(status, "Durum okunamadı: "+err.Error())
				return
			}
			expiry := time.Now().Add(pairingCodeTTL)
			st.TelegramPendingCode = code
			st.TelegramPendingExpiry = &expiry
			if err := store.SaveState(dir, st); err != nil {
				setText(status, "Durum yazılamadı: "+err.Error())
				return
			}
			showPairingCode(m.hwnd, code)
			setText(status, "Kod onaylandıktan sonra listeyi görmek için pencereyi kapatıp tekrar açın.")

		case removeID:
			chat, ok := selected()
			if !ok {
				return
			}
			c, err := config.Load(dir)
			if err != nil {
				setText(status, "Yapılandırma okunamadı: "+err.Error())
				return
			}
			kept := c.TelegramChats[:0]
			for _, ch := range c.TelegramChats {
				if ch.ID != chat.ID {
					kept = append(kept, ch)
				}
			}
			c.TelegramChats = kept
			if err := config.Save(dir, c); err != nil {
				setText(status, "Kaydedilemedi: "+err.Error())
				return
			}
			setText(status, "Sohbet kaldırıldı.")
			reload()
		}
	}

	reload()
	m.run(parent)
}

// showPairingCode, uretilen eslestirme kodunu ve bota nasil
// gonderilecegini gosterir. Onay watcher'da arka planda gerceklesir;
// bu diyalog onu beklemez, kullanici kapatabilir.
func showPairingCode(parent uintptr, code string) {
	m, err := newModal(parent, "Sohbet ekle", 380, 200)
	if err != nil {
		warn(parent, "antigame", "Pencere açılamadı: "+err.Error())
		return
	}
	m.label("Bu kodu botunuza mesaj olarak gönderin:", Rect{12, 12, 352, 20})
	m.label(code, Rect{12, 40, 200, 32})
	m.label("Kod 10 dakika geçerlidir.", Rect{12, 80, 352, 20})
	status := m.label("", Rect{12, 112, 352, 20})

	_, copyID := m.button("Kopyala", Rect{12, 148, 90, 28}, variantSecondary, false)
	_, closeID := m.button("Kapat", Rect{272, 148, 90, 28}, variantPrimary, true)

	m.onCmd = func(id int) {
		switch id {
		case copyID:
			if err := copySecret(m.hwnd, code); err != nil {
				setText(status, "Kopyalanamadı: "+err.Error())
				return
			}
			setText(status, "Kod panoya kopyalandı.")
		case closeID, idCancel, idOK:
			m.close()
		}
	}
	m.run(parent)
}
```

- [ ] **Step 6: Wire the menu command**

`internal/ui/window.go`'daki komut kimlikleri bloğunu bul:

```go
const (
	idAdd = 200 + iota
	idRemoveGame
	idAutoStart
	idWatch
	idReport
	idCode
	idPeople
	idUninstall
	idDataInfo
	idOpenFolder
	idAbout
)
```

Şununla değiştir (`idNotifications`, `idPeople`'dan hemen sonra eklendi):

```go
const (
	idAdd = 200 + iota
	idRemoveGame
	idAutoStart
	idWatch
	idReport
	idCode
	idPeople
	idNotifications
	idUninstall
	idDataInfo
	idOpenFolder
	idAbout
)
```

Aynı dosyada `onCommand` içindeki `case idPeople:` bloğunu bul:

```go
	case idPeople:
		showPeople(w.hwnd, w.dir)
		w.refresh()

	case idUninstall:
```

Şununla değiştir:

```go
	case idPeople:
		showPeople(w.hwnd, w.dir)
		w.refresh()

	case idNotifications:
		showNotifications(w.hwnd, w.dir)
		w.refresh()

	case idUninstall:
```

`internal/ui/menubar.go`'daki `&Yönet` girdilerini bul:

```go
		{"&Kişiler…", idPeople},
		separator,
		{"Kal&dır…", idUninstall},
```

Şununla değiştir:

```go
		{"&Kişiler…", idPeople},
		{"&Bildirimler…", idNotifications},
		separator,
		{"Kal&dır…", idUninstall},
```

- [ ] **Step 7: Verify it builds and all UI tests pass**

Run: `go build ./... && go test ./internal/ui/... -v`
Expected: derleme hatasız biter, tüm testler PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/ui/pairingcode.go internal/ui/pairingcode_test.go internal/ui/telegram.go internal/ui/window.go internal/ui/menubar.go
git commit -m "feat(ui): Bildirimler ekrani, sohbet eslestirme akisi, menu baglantisi"
```

---

## Final Verification

Tüm görevler bittikten sonra:

- [ ] Run: `go build ./...` — tüm paketler hatasız derlenir
- [ ] Run: `go vet ./...` — uyarı çıkmaz
- [ ] Run: `go test ./...` — tüm paketler (yeni `internal/telegram`, `internal/telegramwatch` dahil) PASS
- [ ] Manuel duman testi: gerçek bir Telegram bot token'ıyla (BotFather'dan alınmış), `antigame` ana ekranından "Bildirimler" aç → token kaydet → "Sohbet ekle" → kodu bota gönder → birkaç saniye içinde "Kaydınız tamamlandı." mesajı gelir → bota `/durum` yaz → günün özeti gelir → kapıyı bir kez aç → birkaç saniye içinde "Kapı açıldı: ..." bildirimi gelir.
