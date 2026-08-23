package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// TelegramState, internal/telegramwatch'in calisma zamani durumudur:
// getUpdates offset'i, unlock tarama isareti ve bekleyen eslestirme
// kodu. state.json'dan AYRI bir dosyada tutulur: internal/watch,
// state.json'u kendi bellek ici kopyasindan (w.st) her 30 saniyede bir
// kosulsuz geri yazar (bkz. watch.go persist()), ve bir oturum acikken
// (tam da bir kapi acildiktan sonraki durum) o kopyayi diskten hic
// yenilemez (bkz. watch.go Reload() cagrisi, yalnizca oturum kapaliyken
// calisir). Bu alanlar state.json icinde olsaydi, watch'in eski
// kopyasi telegramwatch'in yazdigi degerleri sessizce geri alirdi —
// tekrar tekrar bildirim, kirilan eslestirme, hatta gate'in
// FailCount/LockUntil alanlarinin eski degere donmesi (kilit atlatma
// riski). internal/watch'a asla dokunulmadigi icin (spec §Global
// Constraints) tek cozum bu alanlari watch'in hic bilmedigi ayri bir
// dosyaya tasimakti.
type TelegramState struct {
	// Offset, getUpdates'te son islenen guncellemenin bir fazlasidir;
	// ayni mesaji tekrar islememek icin.
	Offset int64 `json:"offset,omitempty"`
	// LastUnlockTS, event log taramasinin nereye kadar geldigini
	// isaretler. Bos ise (ilk calisma) gecmis taranmaz.
	LastUnlockTS *time.Time `json:"last_unlock_ts,omitempty"`
	// PendingCode, "Sohbet ekle" ile uretilen tek kullanimlik
	// eslestirme kodudur; suresi PendingExpiry'de.
	PendingCode   string     `json:"pending_code,omitempty"`
	PendingExpiry *time.Time `json:"pending_expiry,omitempty"`
}

const telegramStateFile = "telegram_state.json"

// LoadTelegramState, dosya yoksa bos bir durum dondurur (ilk calistirma
// kurulum gerektirmesin diye).
func LoadTelegramState(dir string) (*TelegramState, error) {
	b, err := os.ReadFile(filepath.Join(dir, telegramStateFile))
	if os.IsNotExist(err) {
		return &TelegramState{}, nil
	}
	if err != nil {
		return nil, err
	}
	var s TelegramState
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// SaveTelegramState, SaveState ile ayni desende atomik yazar: once
// gecici dosya, sonra yer degistirme.
func SaveTelegramState(dir string, s *TelegramState) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(dir, telegramStateFile+".tmp")
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, filepath.Join(dir, telegramStateFile)); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
