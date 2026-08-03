// Package session, kilit oturumunun yasam dongusunu yonetir.
//
// Sure kotasi olmadigi icin oturumun sabit bir bitis zamani yoktur.
// Oturum, listedeki bir oyun calistigi surece (izleyici Touch cagirir) ve
// son gorulmeden itibaren odemesiz sure boyunca gecerlidir. Boylece oyun
// coktugunde veya kisa sureligine kapatildiginda yeni kod istenmez, ama
// aksam yeniden oturuldugunda istenir.
package session

import (
	"time"

	"github.com/guts/antigame/internal/store"
)

// Active, su anda gecerli bir kilit oturumu olup olmadigini soyler.
func Active(st *store.State, now time.Time, grace time.Duration) bool {
	if st.Session == nil {
		return false
	}
	return now.Before(st.Session.LastGameSeen.Add(grace))
}

// Open, gecerli kod girildiginde yeni bir oturum acar.
// LastGameSeen su ana ayarlanir; bu, kullaniciya oyunu baslatmasi icin
// odemesiz sure kadar zaman tanir.
func Open(st *store.State, now time.Time) {
	st.Session = &store.Session{OpenedAt: now, LastGameSeen: now}
}

// Touch, izleyici listedeki bir oyunu gordugunde cagrilir ve oturumun
// dusmesini erteler. Kapali bir oturumu acmaz.
func Touch(st *store.State, now time.Time) {
	if st.Session == nil {
		return
	}
	st.Session.LastGameSeen = now
}

func Close(st *store.State) { st.Session = nil }
