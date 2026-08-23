//go:build windows

package ui

import (
	"strings"
	"time"

	"github.com/guts/antigame/internal/config"
	"github.com/guts/antigame/internal/store"
)

// pairingCodeTTL, uretilen eslestirme kodunun gecerlilik suresidir.
const pairingCodeTTL = 10 * time.Minute

const notificationsHelpText = `Telegram'dan bildirim almak için önce kendi botunuzu oluşturun:

  1. Telegram'da @BotFather'ı açın.
  2. /newbot yazın, botunuza bir isim verin.
  3. BotFather'ın verdiği token'ı aşağıya yapıştırıp Kaydet'e basın.

Token girilince bildirimler otomatik açılır. Sonra "Sohbet ekle" ile
kendi sohbetinizi eşleştirebilirsiniz.`

// showNotifications, Telegram bot token'ini ve onayli sohbetleri
// yonetme diyalogunu acar.
func showNotifications(parent uintptr, dir string) {
	m, err := newModal(parent, "Bildirimler — Telegram", 540, 440)
	if err != nil {
		warn(parent, "antigame", "Pencere açılamadı: "+err.Error())
		return
	}

	m.label("Bot token:", Rect{12, 12, 80, 20})
	tokenBox := m.edit("", Rect{96, 8, 340, 26}, 0)
	_, saveTokenID := m.button("Kaydet", Rect{444, 8, 84, 26}, variantSecondary, false)

	help := m.label(notificationsHelpText, Rect{12, 42, 516, 92})

	list := m.list(Rect{12, 140, 516, 156},
		[]string{"Sohbet", "Eklenme"}, []int32{300, 200})

	status := m.label("", Rect{12, 300, 516, 36})

	_, addID := m.button("Sohbet ekle", Rect{12, 344, 110, 28}, variantSecondary, false)
	_, removeID := m.button("Kaldır", Rect{130, 344, 90, 28}, variantDanger, false)
	_, closeID := m.button("Kapat", Rect{438, 344, 90, 28}, variantSecondary, true)

	var chats []config.TelegramChat
	var tokenSet bool

	reload := func() {
		c, err := config.Load(dir)
		if err != nil {
			setText(status, "Yapılandırma okunamadı: "+err.Error())
			return
		}
		chats = c.TelegramChats
		tokenSet = c.TelegramToken != ""
		setText(tokenBox, c.TelegramToken)
		if tokenSet {
			setText(help, "Bot bağlandı. Aşağıdan sohbet ekleyip kaldırabilirsiniz.")
		} else {
			setText(help, notificationsHelpText)
		}
		lvSetRows(list, ChatRows(chats))
	}

	selected := func() (config.TelegramChat, bool) {
		i := lvSelected(list)
		if i < 0 || i >= len(chats) {
			setText(status, "Önce listeden bir sohbet seçin.")
			return config.TelegramChat{}, false
		}
		return chats[i], true
	}

	m.onCmd = func(id int) {
		switch id {
		case closeID, idCancel, idOK:
			m.close()

		case saveTokenID:
			c, err := config.Load(dir)
			if err != nil {
				setText(status, "Yapılandırma okunamadı: "+err.Error())
				return
			}
			c.TelegramToken = strings.TrimSpace(textOf(tokenBox))
			if err := config.Save(dir, c); err != nil {
				setText(status, "Kaydedilemedi: "+err.Error())
				return
			}
			setText(status, "Token kaydedildi.")
			reload()

		case addID:
			if !tokenSet {
				setText(status, "Önce bot token girip kaydedin.")
				return
			}
			code, err := newPairingCode()
			if err != nil {
				setText(status, "Kod üretilemedi: "+err.Error())
				return
			}
			ts, err := store.LoadTelegramState(dir)
			if err != nil {
				setText(status, "Durum okunamadı: "+err.Error())
				return
			}
			expiry := time.Now().Add(pairingCodeTTL)
			ts.PendingCode = code
			ts.PendingExpiry = &expiry
			if err := store.SaveTelegramState(dir, ts); err != nil {
				setText(status, "Durum yazılamadı: "+err.Error())
				return
			}
			showPairingCode(m.hwnd, code)
			setText(status, "Kod onaylandıktan sonra listeyi görmek için pencereyi kapatıp tekrar açın.")

		case removeID:
			chat, ok := selected()
			if !ok {
				return
			}
			c, err := config.Load(dir)
			if err != nil {
				setText(status, "Yapılandırma okunamadı: "+err.Error())
				return
			}
			kept := c.TelegramChats[:0]
			for _, ch := range c.TelegramChats {
				if ch.ID != chat.ID {
					kept = append(kept, ch)
				}
			}
			c.TelegramChats = kept
			if err := config.Save(dir, c); err != nil {
				setText(status, "Kaydedilemedi: "+err.Error())
				return
			}
			setText(status, "Sohbet kaldırıldı.")
			reload()
		}
	}

	reload()
	m.run(parent)
}

// showPairingCode, uretilen eslestirme kodunu ve bota nasil
// gonderilecegini gosterir. Onay watcher'da arka planda gerceklesir;
// bu diyalog onu beklemez, kullanici kapatabilir.
func showPairingCode(parent uintptr, code string) {
	m, err := newModal(parent, "Sohbet ekle", 380, 200)
	if err != nil {
		warn(parent, "antigame", "Pencere açılamadı: "+err.Error())
		return
	}
	m.label("Bu kodu botunuza mesaj olarak gönderin:", Rect{12, 12, 352, 20})
	m.label(code, Rect{12, 40, 200, 32})
	m.label("Kod 10 dakika geçerlidir.", Rect{12, 80, 352, 20})
	status := m.label("", Rect{12, 112, 352, 20})

	_, copyID := m.button("Kopyala", Rect{12, 148, 90, 28}, variantSecondary, false)
	_, closeID := m.button("Kapat", Rect{272, 148, 90, 28}, variantPrimary, true)

	m.onCmd = func(id int) {
		switch id {
		case copyID:
			if err := copySecret(m.hwnd, code); err != nil {
				setText(status, "Kopyalanamadı: "+err.Error())
				return
			}
			setText(status, "Kod panoya kopyalandı.")
		case closeID, idCancel, idOK:
			m.close()
		}
	}
	m.run(parent)
}
