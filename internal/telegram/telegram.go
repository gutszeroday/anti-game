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
