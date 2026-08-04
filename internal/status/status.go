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
		fmt.Fprintf(&b, "Oturum: açık\nOyun kapalıyken %d dakika sonra düşer.\n",
			int(left.Round(time.Minute).Minutes()))
	} else {
		b.WriteString("Oturum: kapalı\nListedeki bir oyunu açmak için arkadaşınızdan kod gerekiyor.\n")
	}

	fmt.Fprintf(&b, "\nKapıda durdurulan oyun sayısı: %d\n", len(cfg.Gated))
	if !st.Heartbeat.IsZero() {
		fmt.Fprintf(&b, "Son kayıt: %s\n", st.Heartbeat.Local().Format("02.01.2006 15:04"))
	}
	return b.String(), nil
}
