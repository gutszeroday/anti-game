package store

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// SentNotification, telegramwatch'in proaktif olarak gonderdigi tek bir
// bildirimdir (kapi acma, izleyici kapanisi). Komut yanitlari
// (/durum, /help, kayit onayi) buraya yazilmaz: onlari kullanici zaten
// kendi Telegram'inda goruyor, burasi istemcide "giden bildirimler"i
// gormek icin.
type SentNotification struct {
	TS     time.Time `json:"ts"`
	ChatID int64     `json:"chat_id"`
	Label  string    `json:"label,omitempty"`
	Text   string    `json:"text"`
}

const telegramSentFile = "telegram-sent.jsonl"

// ponytail: tek dosya, ay bazli bolme veya donderme yok — hacim haftada
// birkac satir; binlerce satiri gecerse events-*.jsonl'daki ay bazli
// bolme deseni buraya da eklenebilir.

// AppendSent, bildirimi gunluk dosyasina tek satir olarak ekler.
func AppendSent(dir string, n SentNotification) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	n.TS = n.TS.UTC()
	b, err := json.Marshal(n)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, telegramSentFile), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(b, '\n'))
	return err
}

// ReadSent, en yeni limit kadar bildirimi en yeniden en eskiye siralı
// dondurur. Dosya yoksa bos liste doner.
func ReadSent(dir string, limit int) ([]SentNotification, error) {
	f, err := os.Open(filepath.Join(dir, telegramSentFile))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var all []SentNotification
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		var n SentNotification
		if err := json.Unmarshal(sc.Bytes(), &n); err != nil {
			continue
		}
		all = append(all, n)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}

	if len(all) > limit {
		all = all[len(all)-limit:]
	}
	// En yeni ilk once: dosya ekleme sirasinda (en eski once), burada ters cevriliyor.
	for i, j := 0, len(all)-1; i < j; i, j = i+1, j-1 {
		all[i], all[j] = all[j], all[i]
	}
	return all, nil
}
