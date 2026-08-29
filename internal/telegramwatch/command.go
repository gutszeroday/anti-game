package telegramwatch

import (
	"strings"
	"time"

	"github.com/guts/antigame/internal/config"
	"github.com/guts/antigame/internal/telegram"
	"github.com/guts/antigame/internal/totp"
	"github.com/guts/antigame/internal/vault"
)

const helpText = "Kullanılabilir komutlar:\n" +
	"/durum - Bu haftanın özeti\n" +
	"/kapanis_bildirimi - İzleyici kapandığında bildirim al/kapat\n" +
	"/help - Bu mesaj"

// handleUpdate, tek bir Telegram guncellemesini isler:
//   - onayli sohbetten "/durum" -> bu haftanin ozeti yollanir
//   - onayli sohbetten "/help" -> komut listesi yollanir
//   - onaysiz sohbetten, kapidaki kisilerden birinin o anki kapi koduyla
//     birebir eslesen metin -> sohbet o kisinin adiyla onaylanir
//   - diger her sey yok sayilir (botun varligini yabancilara sizdirmamak icin)
func handleUpdate(dir string, cfg *config.Config, u telegram.Update, client sender, now time.Time) error {
	if u.Chat == 0 {
		return nil
	}
	text := strings.TrimSpace(u.Text)

	if approvedChat(cfg, u.Chat) {
		switch text {
		case "/durum":
			summary, err := weekSummary(dir, cfg, personIDForChat(cfg, u.Chat), now)
			if err != nil {
				return err
			}
			return client.SendMessage(u.Chat, summary)
		case "/help":
			return client.SendMessage(u.Chat, helpText)
		case "/kapanis_bildirimi":
			return toggleNotifyOnClose(dir, cfg, u.Chat, client)
		}
		return nil
	}

	person := matchPersonCode(dir, cfg, text, now)
	if person == nil {
		return nil
	}

	cfg.TelegramChats = append(cfg.TelegramChats, config.TelegramChat{ID: u.Chat, Label: person.Name, PersonID: person.ID, AddedAt: now})
	if err := config.Save(dir, cfg); err != nil {
		return err
	}
	return client.SendMessage(u.Chat, "Kaydınız tamamlandı, "+person.Name+".")
}

// matchPersonCode, metnin kapidaki kisilerden birinin o anki (+-1 adim
// toleransli) kapi koduyla eslesip eslesmedigine bakar, eslesirse
// kisiyi dondurur.
//
// Kapinin kendi tekrar-kullanim korumasina (state.json'daki
// TOTPCounters) kasten dokunulmaz: paylasilan bir sayac, botla
// eslesmek icin kod yazan kisiyi kapinin onunde "kod kullanilmis"
// diye kilitlerdi. Bu yuzden ayni kod Telegram'a birden fazla kez
// yazilabilir.
func matchPersonCode(dir string, cfg *config.Config, text string, now time.Time) *config.Person {
	for i := range cfg.People {
		p := &cfg.People[i]
		secret, err := vault.LoadPerson(dir, p.ID)
		if err != nil {
			continue
		}
		if _, result := totp.Verify(secret, text, now, 0); result == totp.ResultOK {
			return p
		}
	}
	return nil
}

func approvedChat(cfg *config.Config, chatID int64) bool {
	for _, c := range cfg.TelegramChats {
		if c.ID == chatID {
			return true
		}
	}
	return false
}

// toggleNotifyOnClose, izleyici kapandiginda bu sohbete bildirim
// gidip gitmeyecegini ters cevirir ve diske yazar. Bilerek yalnizca
// buradan (Telegram komutuyla) degisir; istemci arayuzunde bu alana
// hic dokunulmuyor.
func toggleNotifyOnClose(dir string, cfg *config.Config, chatID int64, client sender) error {
	for i := range cfg.TelegramChats {
		if cfg.TelegramChats[i].ID != chatID {
			continue
		}
		cfg.TelegramChats[i].NotifyOnClose = !cfg.TelegramChats[i].NotifyOnClose
		if err := config.Save(dir, cfg); err != nil {
			return err
		}
		if cfg.TelegramChats[i].NotifyOnClose {
			return client.SendMessage(chatID, "Kapanış bildirimi açıldı.")
		}
		return client.SendMessage(chatID, "Kapanış bildirimi kapandı.")
	}
	return nil
}

// personIDForChat, onayli bir sohbetin hangi kisiye ait oldugunu dondurur.
func personIDForChat(cfg *config.Config, chatID int64) string {
	for _, c := range cfg.TelegramChats {
		if c.ID == chatID {
			return c.PersonID
		}
	}
	return ""
}
