//go:build windows

package ui

import (
	"strconv"
	"strings"

	"github.com/guts/antigame/internal/config"
)

// showSettings, kapanma sureleri ve kod-ile-acma anahtarini duzenleme
// diyalogunu acar.
func showSettings(parent uintptr, dir string) {
	m, err := newModal(parent, "Ayarlar", 420, 300)
	if err != nil {
		warn(parent, "antigame", "Pencere açılamadı: "+err.Error())
		return
	}

	m.label("Ödemesiz süre (dk):", Rect{12, 14, 200, 20})
	graceBox := m.edit("", Rect{220, 10, 80, 26}, esNumber)
	m.label("Hiçbir şey çalışmadığında oturumun açık kalacağı süre.",
		Rect{12, 40, 396, 32})

	m.label("Başlatıcı penceresi (dk):", Rect{12, 84, 200, 20})
	launcherBox := m.edit("", Rect{220, 80, 80, 26}, esNumber)
	m.label("Son gerçek oyundan sonra yalnızca başlatıcı çalışırken "+
		"oturumun en fazla yaşayacağı süre.", Rect{12, 110, 396, 32})

	unlockBox := m.checkbox("Kod ile açma kapalı — oyunlar direkt açılsın (süre yine tutulur)",
		Rect{12, 156, 396, 36})

	status := m.label("", Rect{12, 200, 396, 34})

	_, saveID := m.button("Kaydet", Rect{206, 244, 96, 28}, variantPrimary, true)
	_, cancelID := m.button("Vazgeç", Rect{308, 244, 96, 28}, variantSecondary, false)

	cfg, err := config.Load(dir)
	if err != nil {
		setText(status, "Yapılandırma okunamadı: "+err.Error())
	} else {
		setText(graceBox, strconv.Itoa(cfg.GraceMinutes))
		setText(launcherBox, strconv.Itoa(cfg.LauncherWindowMinutes))
		setChecked(unlockBox, cfg.CodeUnlockOff)
	}

	m.onCmd = func(id int) {
		switch id {
		case cancelID, idCancel:
			m.close()

		case saveID, idOK:
			grace, err := strconv.Atoi(strings.TrimSpace(textOf(graceBox)))
			if err != nil || grace <= 0 {
				setText(status, "Ödemesiz süre pozitif bir sayı olmalı.")
				focus(graceBox)
				return
			}
			launcher, err := strconv.Atoi(strings.TrimSpace(textOf(launcherBox)))
			if err != nil || launcher <= 0 {
				setText(status, "Başlatıcı penceresi pozitif bir sayı olmalı.")
				focus(launcherBox)
				return
			}
			cfg, err := config.Load(dir)
			if err != nil {
				setText(status, "Yapılandırma okunamadı: "+err.Error())
				return
			}
			cfg.GraceMinutes = grace
			cfg.LauncherWindowMinutes = launcher
			cfg.CodeUnlockOff = isChecked(unlockBox)
			if err := config.Save(dir, cfg); err != nil {
				setText(status, "Kaydedilemedi: "+err.Error())
				return
			}
			m.close()
		}
	}

	focus(graceBox)
	m.run(parent)
}
