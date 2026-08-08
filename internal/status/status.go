// Package status, izleyicinin o anki durumunu insan okuyacak bir metne
// cevirir. Tepsi menusundeki "Durum" bunu gosteriyor; arka planda calisan
// izleyicinin konsolu olmadigi icin baska anlatacak yeri yok.
package status

import (
	"fmt"
	"strings"
	"time"

	"github.com/guts/antigame/internal/config"
	"github.com/guts/antigame/internal/session"
	"github.com/guts/antigame/internal/store"
)

// openedBy, oturumu kimin actigini parantez icinde dondurur. Kurtarma
// koduyla ya da kapi kurulmadan acilmis oturumlarda bos doner.
func openedBy(cfg *config.Config, st *store.State) string {
	if st.Session == nil || st.Session.OpenedBy == "" {
		return ""
	}
	if p, ok := cfg.FindPerson(st.Session.OpenedBy); ok {
		return " — " + p.Name + " açtı"
	}
	return ""
}

// freshWindow, "oyun su an calisiyor" saymak icin LastGameSeen'in ne kadar
// taze olmasi gerektigidir. Izleyici her turda tazeliyor; uc tur pay
// birakmak, tek bir yavas turun oyunu kapanmis gostermesini engeller.
func freshWindow(cfg *config.Config) time.Duration {
	return max(3*time.Duration(cfg.PollMS)*time.Millisecond, 15*time.Second)
}

// fmtDur, kalan sureyi kisa ve asagi yuvarlayarak yazar. Yukari
// yuvarlamak kalan sureyi oldugundan uzun gosterir; kullanici ona guvenip
// oyunu gec acardi.
func fmtDur(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	m := int(d / time.Minute)
	s := int((d % time.Minute) / time.Second)
	switch {
	case m == 0:
		return fmt.Sprintf("%d sn", s)
	case s == 0 || m > 10:
		return fmt.Sprintf("%d dk", m)
	default:
		return fmt.Sprintf("%d dk %d sn", m, s)
	}
}

// Brief, kalan sureyi tek satirda anlatir. Tepsi tooltip'i 128 karakterle
// sinirli oldugu icin metin tek satir ve kisa tutuluyor.
func Brief(dir string, now time.Time) (string, error) {
	cfg, err := config.Load(dir)
	if err != nil {
		return "", err
	}
	st, err := store.LoadState(dir)
	if err != nil {
		return "", err
	}
	return briefLine(cfg, st, now), nil
}

// briefLine, kalan sureyi oturumun hangi asamada oldugunu soyleyecek
// bicimde yazar. Ayni sayi uc farkli soru cevapliyor: kod girildi ama oyun
// acilmadi, oyun acik, oyun kapandi.
func briefLine(cfg *config.Config, st *store.State, now time.Time) string {
	grace := time.Duration(cfg.GraceMinutes) * time.Minute
	launcherWindow := time.Duration(cfg.LauncherWindowMinutes) * time.Minute
	if launcherWindow <= 0 {
		launcherWindow = 45 * time.Minute
	}
	left := session.Remaining(st, now, grace, launcherWindow)
	if left <= 0 {
		return "Oturum kapalı — oyun açmak için kod gerekiyor."
	}
	// Gercek bir oyun hic gorulmediyse LastGameSeen, Open'in yazdigi
	// OpenedAt olarak durur; baslatici onu ilerletmez.
	if st.Session.LastGameSeen.Equal(st.Session.OpenedAt) {
		return "Oyunu açmak için " + fmtDur(left) + " var."
	}
	if now.Sub(st.Session.LastGameSeen) <= freshWindow(cfg) {
		return "Oyun açık — kapatırsan " + fmtDur(grace) + " sonra kod istenir."
	}
	return "Tekrar kod istenene kadar " + fmtDur(left) + " var."
}

func Text(dir string, now time.Time) (string, error) {
	cfg, err := config.Load(dir)
	if err != nil {
		return "", err
	}
	st, err := store.LoadState(dir)
	if err != nil {
		return "", err
	}

	grace := time.Duration(cfg.GraceMinutes) * time.Minute
	launcherWindow := time.Duration(cfg.LauncherWindowMinutes) * time.Minute
	if launcherWindow <= 0 {
		launcherWindow = 45 * time.Minute
	}
	var b strings.Builder

	if left := session.Remaining(st, now, grace, launcherWindow); left > 0 {
		// Kalan sure tek yerden yaziliyor: tepside ve burada ayri
		// cumleler kurulunca ikisi birbirinden kayiyordu.
		fmt.Fprintf(&b, "Oturum: açık%s\n%s\n", openedBy(cfg, st), briefLine(cfg, st, now))
	} else {
		b.WriteString("Oturum: kapalı\nListedeki bir oyunu açmak için arkadaşınızdan kod gerekiyor.\n")
	}

	fmt.Fprintf(&b, "\nKapıda durdurulan oyun sayısı: %d\n", len(cfg.Gated))
	if !st.Heartbeat.IsZero() {
		fmt.Fprintf(&b, "Son kayıt: %s\n", st.Heartbeat.Local().Format("02.01.2006 15:04"))
	}
	return b.String(), nil
}
