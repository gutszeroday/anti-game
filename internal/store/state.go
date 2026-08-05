package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Session, acik bir kilit oturumudur. Sure kotasi olmadigi icin bitis
// zamani yoktur; oturum, oyun calistigi surece ve son goruldukten sonraki
// odemesiz sure boyunca gecerlidir.
type Session struct {
	OpenedAt time.Time `json:"opened_at"`
	// LastGameSeen yalnizca gercek oyunlarla guncellenir. Baslatici
	// (Riot Client gibi) tek basina oturumu sonsuza kadar tazeleyemesin
	// diye ayri tutuluyor.
	LastGameSeen time.Time `json:"last_game_seen"`
	// LastSeen, baslatici dahil listedeki herhangi bir seyin en son
	// gorulme anidir.
	LastSeen time.Time `json:"last_seen"`
	// OpenedBy, oturumu acan kisinin ID'sidir. Kurtarma koduyla acilan
	// oturumda bostur. Oyun sureleri bu kisiye yazilir.
	OpenedBy string `json:"opened_by,omitempty"`
}

type State struct {
	// LastTOTPCounter, tek kisilik donemden kalir; LoadState onu
	// TOTPCounters'a tasir ve sifirlar.
	LastTOTPCounter uint64 `json:"last_totp_counter,omitempty"`
	// TOTPCounters, kisi basina en son kullanilan sayaci tutar. Ortak tek
	// sayac, bir kisinin kullandigi kodun digerinin kodunu "kullanilmis"
	// saymasina yol acardi.
	TOTPCounters map[string]uint64 `json:"totp_counters,omitempty"`
	FailCount    int               `json:"fail_count"`
	LockUntil    *time.Time        `json:"lock_until"`
	Session      *Session          `json:"session"`
	Heartbeat    time.Time         `json:"heartbeat"`
	RecoveryHash string            `json:"recovery_hash"`
	RecoverySalt string            `json:"recovery_salt"`
	RecoveryUsed bool              `json:"recovery_used"`
}

const stateFile = "state.json"

func LoadState(dir string) (*State, error) {
	b, err := os.ReadFile(filepath.Join(dir, stateFile))
	if os.IsNotExist(err) {
		return &State{}, nil
	}
	if err != nil {
		return nil, err
	}
	var s State
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	// Tek kisilik donemin sayaci ilk kisiye devredilir. Devirden sonra
	// eski alan sifirlanir; yoksa her okuma, ilerlemis sayacin uzerine
	// eski degeri yazar ve kullanilmis bir kod yeniden gecerli olurdu.
	if len(s.TOTPCounters) == 0 && s.LastTOTPCounter > 0 {
		s.TOTPCounters = map[string]uint64{"p1": s.LastTOTPCounter}
	}
	s.LastTOTPCounter = 0
	return &s, nil
}

// Counter, kisinin en son kullandigi TOTP sayacini dondurur.
func (s *State) Counter(id string) uint64 { return s.TOTPCounters[id] }

// SetCounter, kisinin sayacini gunceller.
func (s *State) SetCounter(id string, v uint64) {
	if s.TOTPCounters == nil {
		s.TOTPCounters = map[string]uint64{}
	}
	s.TOTPCounters[id] = v
}

// ClearCounter, kisinin sayacini siler. Anahtar yenilendiginde cagrilir:
// eski yuksek sayac, yeni anahtarin uretecegi kodlari reddederdi.
func (s *State) ClearCounter(id string) { delete(s.TOTPCounters, id) }

// SaveState, once gecici dosyaya yazip yer degistirerek atomik yazar.
// os.Rename Windows'ta MOVEFILE_REPLACE_EXISTING kullanir, yani yarim
// yazilmis bir state.json olusamaz.
func SaveState(dir string, s *State) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(dir, stateFile+".tmp")
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, filepath.Join(dir, stateFile)); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
