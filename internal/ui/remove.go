//go:build windows

package ui

import (
	"strings"

	"github.com/guts/antigame/internal/uninstall"
)

// showRemove, kaldirma diyalogunu acar. Kaldirildiysa true doner;
// cagiran pencereyi kapatir.
//
// Kod istenmesi kaldirmayi engellemez: kullanici yonetici yetkisine
// sahiptir ve gorevi elle de silebilir. Amac, kaldirmayi kaza eseri
// degil bilincli bir karar haline getirmek.
func showRemove(parent uintptr, dir string) bool {
	m, err := newModal(parent, "antigame'i kaldır", 460, 300)
	if err != nil {
		warn(parent, "antigame", "Pencere açılamadı: "+err.Error())
		return false
	}

	m.label("Kaldırmak için anahtarı olan birinden kod isteyin.\n\n"+
		"Zamanlanmış görev silinecek ve izleyici bir daha oturum açılışında "+
		"başlamayacak. Çalışan izleyici varsa bilgisayar yeniden başlatılana "+
		"kadar çalışmaya devam eder.",
		Rect{12, 12, 436, 84})

	m.label("Kod:", Rect{12, 108, 40, 20})
	code := m.edit("", Rect{56, 104, 100, 26}, esNumber|esCenter)

	wipe := m.checkbox("Kayıtlı süre verileri de silinsin (geri alınamaz)",
		Rect{12, 142, 436, 22})

	status := m.label("", Rect{12, 172, 436, 70})

	_, okID := m.button("Kaldır", Rect{252, 254, 90, 28}, false)
	_, cancelID := m.button("Vazgeç", Rect{354, 254, 90, 28}, true)

	done := false
	m.onCmd = func(id int) {
		switch id {
		case cancelID, idCancel:
			m.close()
		case okID, idOK:
			entered := strings.TrimSpace(textOf(code))
			if entered == "" {
				setText(status, "Kod girilmedi.")
				focus(code)
				return
			}
			ok, msg, err := uninstall.Verify(dir, entered)
			if err != nil {
				setText(status, err.Error())
				return
			}
			if !ok {
				setText(status, "Kaldırma reddedildi: "+msg)
				focus(code)
				selectAll(code)
				return
			}
			if err := uninstall.Purge(dir, isChecked(wipe)); err != nil {
				setText(status, err.Error())
				return
			}
			done = true
			m.close()
		}
	}

	focus(code)
	m.run(parent)

	if done {
		info(parent, "antigame", "Kaldırıldı. Bu pencere kapanacak.")
	}
	return done
}
