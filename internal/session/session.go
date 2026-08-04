// Package session, kilit oturumunun yasam dongusunu yonetir.
//
// Sure kotasi olmadigi icin oturumun sabit bir bitis zamani yoktur.
// Oturum iki kosula birden baglidir:
//
//   - Listedeki bir sey (baslatici dahil) son gorulmeden itibaren
//     odemesiz sure kadar gecerli.
//   - Gercek bir oyun son gorulmeden itibaren baslatici penceresi kadar
//     gecerli.
//
// Ikinci kosul olmadan, tepside acik kalan Riot Client oturumu sonsuza
// kadar tazeliyordu: sabah alinan kodla gun boyu girip cikilabiliyordu.
// Birinci kosul olmadan ise mac arasinda yalnizca istemci calisirken
// oturum duser ve yeni mac yuklenirken oyun oldurulurdu.
package session

import (
	"time"

	"github.com/guts/antigame/internal/store"
)

// Active, su anda gecerli bir kilit oturumu olup olmadigini soyler.
func Active(st *store.State, now time.Time, grace, launcherWindow time.Duration) bool {
	if st.Session == nil {
		return false
	}
	// LastSeen bu alan eklenmeden once yazilmis state.json'larda bostur.
	lastSeen := st.Session.LastSeen
	if lastSeen.IsZero() {
		lastSeen = st.Session.LastGameSeen
	}
	return now.Before(lastSeen.Add(grace)) &&
		now.Before(st.Session.LastGameSeen.Add(launcherWindow))
}

// Open, gecerli kod girildiginde yeni bir oturum acar.
// Sayaclar su ana ayarlanir; bu, kullaniciya oyunu baslatmasi icin
// odemesiz sure kadar zaman tanir.
func Open(st *store.State, now time.Time) {
	st.Session = &store.Session{OpenedAt: now, LastGameSeen: now, LastSeen: now}
}

// Touch, izleyici listedeki bir seyi gordugunde cagrilir. realGame false
// ise yalnizca baslatici gorulmustur: oturumun dusmesini erteler ama
// baslatici penceresini uzatmaz. Kapali bir oturumu acmaz.
func Touch(st *store.State, now time.Time, realGame bool) {
	if st.Session == nil {
		return
	}
	st.Session.LastSeen = now
	if realGame {
		st.Session.LastGameSeen = now
	}
}

func Close(st *store.State) { st.Session = nil }
