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
}

type State struct {
	LastTOTPCounter uint64     `json:"last_totp_counter"`
	FailCount       int        `json:"fail_count"`
	LockUntil       *time.Time `json:"lock_until"`
	Session         *Session   `json:"session"`
	Heartbeat       time.Time  `json:"heartbeat"`
	RecoveryHash    string     `json:"recovery_hash"`
	RecoverySalt    string     `json:"recovery_salt"`
	RecoveryUsed    bool       `json:"recovery_used"`
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
	return &s, nil
}

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
