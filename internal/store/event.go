// Package store, olay gunlugunu ve kalici durumu yonetir.
// Veritabani motoru kullanilmaz: gunluk hacim birkac yuz satirdir ve
// izleyicinin bellek tabanini dusuk tutmak oncelikliydi.
package store

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Event, gunluge yazilan tek bir satirdir. Bos alanlar JSON'a yazilmaz.
// Pencere basligi gibi icerik alanlari bilerek yoktur.
type Event struct {
	TS      time.Time `json:"ts"`
	Ev      string    `json:"ev"`
	Exe     string    `json:"exe,omitempty"`
	Name    string    `json:"name,omitempty"`
	PID     int       `json:"pid,omitempty"`
	DurS    int       `json:"dur_s,omitempty"`
	ActiveS int       `json:"active_s,omitempty"`
	Method  string    `json:"method,omitempty"`
	Fails   int       `json:"fails,omitempty"`
	// Who, olayin ait oldugu kisinin ID'sidir: kapiyi acan kisi, ya da
	// oyun bittiginde o oturumu acmis olan kisi. Kapi kurulmadan
	// kaydedilen surelerde bostur.
	Who string `json:"who,omitempty"`
	// From ve To, veri klasoru tasindiginda eski ve yeni yoldur.
	// Tasima kod istemiyor; gunluge dusmesi onu gorunur kiliyor.
	From string `json:"from,omitempty"`
	To   string `json:"to,omitempty"`
}

func monthFile(dir string, t time.Time) string {
	return filepath.Join(dir, fmt.Sprintf("events-%04d-%02d.jsonl", t.Year(), int(t.Month())))
}

// Append, olayi ilgili ayin dosyasina tek satir olarak ekler.
func Append(dir string, e Event) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	e.TS = e.TS.UTC()
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(monthFile(dir, e.TS), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(b, '\n'))
	return err
}

// Read, [from, to] araligindaki olaylari zaman sirasinda dondurur.
// Cozulemeyen satirlar sessizce atlanir: yarim kalmis son satir beklenen
// bir durumdur ve tum gunlugu okunamaz hale getirmemelidir.
func Read(dir string, from, to time.Time) ([]Event, error) {
	var out []Event
	from, to = from.UTC(), to.UTC()
	// Aralikta kalan her ayin dosyasini sirayla gez.
	start := time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, time.UTC)
	for m := start; !m.After(to); m = m.AddDate(0, 1, 0) {
		f, err := os.Open(monthFile(dir, m))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			var e Event
			if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
				continue
			}
			if e.TS.Before(from) || e.TS.After(to) {
				continue
			}
			out = append(out, e)
		}
		err = sc.Err()
		f.Close()
		// Asiri uzun bir satir bozulmaya isarettir: o dosyanin kalanini
		// atlariz ama okunan olaylari ve diger aylari kaybetmeyiz.
		// Bunun disindaki hatalar gercek disk hatalaridir, yutulmamali.
		if err != nil && !errors.Is(err, bufio.ErrTooLong) {
			return nil, err
		}
	}
	return out, nil
}
