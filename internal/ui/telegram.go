//go:build windows

package ui

import (
	"strings"

	"github.com/guts/antigame/internal/config"
	"github.com/guts/antigame/internal/store"
)

// sentListLimit, "Gönderilenler" listesinde gösterilecek en yeni
// bildirim sayısıdır. Pencere kaydırma sunmuyor; makul bir üst sınır
// yeterli.
const sentListLimit = 50

const notificationsHelpText = `Telegram'dan bildirim almak için önce kendi botunuzu oluşturun:

  1. Telegram'da @BotFather'ı açın.
  2. /newbot yazın, botunuza bir isim verin.
  3. BotFather'ın verdiği token'ı aşağıya yapıştırıp Kaydet'e basın.

Token girilince bildirimler otomatik açılır. Sonra her kişi kendi kapı
kodunu (6 haneli) bota mesaj olarak gönderip kendi adıyla eşleşebilir.`

// showNotifications, Telegram bot token'ini ve onayli sohbetleri
// yonetme diyalogunu acar.
func showNotifications(parent uintptr, dir string) {
	m, err := newModal(parent, "Bildirimler — Telegram", 540, 680)
	if err != nil {
		warn(parent, "antigame", "Pencere açılamadı: "+err.Error())
		return
	}

	m.label("Bot token:", Rect{12, 12, 80, 20})
	tokenBox := m.edit("", Rect{96, 8, 340, 26}, 0)
	_, saveTokenID := m.button("Kaydet", Rect{444, 8, 84, 26}, variantSecondary, false)

	help := m.label(notificationsHelpText, Rect{12, 42, 516, 92})

	list := m.list(Rect{12, 140, 516, 140},
		[]string{"Sohbet", "Eklenme"}, []int32{300, 200})

	m.label("Gönderilenler:", Rect{12, 286, 200, 18})
	sentList := m.list(Rect{12, 306, 516, 220},
		[]string{"Zaman", "Sohbet", "Mesaj"}, []int32{130, 120, 250})

	status := m.label("", Rect{12, 532, 516, 36})

	_, removeID := m.button("Kaldır", Rect{12, 576, 90, 28}, variantDanger, false)
	_, closeID := m.button("Kapat", Rect{438, 576, 90, 28}, variantSecondary, true)

	var chats []config.TelegramChat

	reload := func() {
		c, err := config.Load(dir)
		if err != nil {
			setText(status, "Yapılandırma okunamadı: "+err.Error())
			return
		}
		chats = c.TelegramChats
		tokenSet := c.TelegramToken != ""
		setText(tokenBox, c.TelegramToken)
		if tokenSet {
			setText(help, "Bot bağlandı. Kişiler kapı kodlarını bota gönderip eşleştikçe listede görünür.")
		} else {
			setText(help, notificationsHelpText)
		}
		lvSetRows(list, ChatRows(chats))

		sent, err := store.ReadSent(dir, sentListLimit)
		if err != nil {
			setText(status, "Gönderilen bildirimler okunamadı: "+err.Error())
			return
		}
		lvSetRows(sentList, SentRows(sent))
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
