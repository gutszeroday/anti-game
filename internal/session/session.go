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
	return Remaining(st, now, grace, launcherWindow) > 0
}

// Remaining, oturumun dusmesine kalan sureyi dondurur. Oturum kapaliysa
// sifir doner. Iki sinirdan hangisi once doluyorsa o gecerlidir.
//
// Kalan sure Active ile ayni yerden hesaplaniyor: ayri hesaplaninca
// eski bicimli state.json'da anlamsiz sayilar uretiliyordu.
func Remaining(st *store.State, now time.Time, grace, launcherWindow time.Duration) time.Duration {
	if st.Session == nil {
		return 0
	}
	// LastSeen bu alan eklenmeden once yazilmis state.json'larda bostur.
	lastSeen := st.Session.LastSeen
	if lastSeen.IsZero() {
		lastSeen = st.Session.LastGameSeen
	}
	left := min(
		lastSeen.Add(grace).Sub(now),
		st.Session.LastGameSeen.Add(launcherWindow).Sub(now),
	)
	if left < 0 {
		return 0
	}
	return left
}

// Open, gecerli kod girildiginde yeni bir oturum acar. who, kodu veren
// kisinin ID'sidir; kurtarma kodunda bos gecilir.
// Sayaclar su ana ayarlanir; bu, kullaniciya oyunu baslatmasi icin
// odemesiz sure kadar zaman tanir.
func Open(st *store.State, now time.Time, who string) {
	st.Session = &store.Session{OpenedAt: now, LastGameSeen: now, LastSeen: now, OpenedBy: who}
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
