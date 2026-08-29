//go:build windows

package ui

import (
	"fmt"

	"github.com/guts/antigame/internal/config"
	"github.com/guts/antigame/internal/people"
	"github.com/guts/antigame/internal/store"
)

// Row, liste kontrolundeki bir satirdir. Hucreler sutun sirasindadir.
type Row struct{ Cells []string }

// GameRows, oyun listesini goruntulenecek satirlara cevirir.
// Sutunlar: Ad, Exe, Tur.
//
// Baslatici ayrimi kullaniciya gosteriliyor cunku baslaticinin suresi
// oyun suresi olarak sayilmiyor; listede ayni gorunselerdi rapordaki
// eksik sure aciklanamaz olurdu.
func GameRows(c *config.Config) []Row {
	rows := make([]Row, 0, len(c.Gated))
	for _, g := range c.Gated {
		tur := "Oyun"
		if g.Launcher {
			tur = "Başlatıcı"
		}
		rows = append(rows, Row{Cells: []string{g.Name, g.Exe, tur}})
	}
	return rows
}

// PeopleRows, kisi listesini goruntulenecek satirlara cevirir.
// Sutunlar: Ad, Ipucu, Anahtar.
//
// Anahtari cozulemeyen kisi "anahtar yok"tan ayri gosteriliyor: birincisi
// bozuk bir dosya (Windows profili degismis olabilir), ikincisi hic
// eslestirilmemis bir kayit. Yapilacak sey farkli.
func PeopleRows(es []people.Entry) []Row {
	rows := make([]Row, 0, len(es))
	for _, e := range es {
		hint := e.Hint
		if hint == "" {
			hint = "—"
		}
		var key string
		switch {
		case e.Usable():
			key = "var"
		case e.KeyErr != nil:
			key = "okunamıyor"
		default:
			key = "yok"
		}
		rows = append(rows, Row{Cells: []string{e.Name, hint, key}})
	}
	return rows
}

// ChatRows, onayli Telegram sohbetlerini goruntulenecek satirlara
// cevirir. Sutunlar: Sohbet, Eklenme.
func ChatRows(chats []config.TelegramChat) []Row {
	rows := make([]Row, 0, len(chats))
	for _, c := range chats {
		label := c.Label
		if label == "" {
			label = fmt.Sprintf("Sohbet %d", c.ID)
		}
		rows = append(rows, Row{Cells: []string{label, c.AddedAt.Local().Format("2006-01-02 15:04")}})
	}
	return rows
}

// SentRows, giden Telegram bildirimlerini goruntulenecek satirlara
// cevirir. Sutunlar: Zaman, Sohbet, Mesaj.
func SentRows(sent []store.SentNotification) []Row {
	rows := make([]Row, 0, len(sent))
	for _, n := range sent {
		label := n.Label
		if label == "" {
			label = fmt.Sprintf("Sohbet %d", n.ChatID)
		}
		rows = append(rows, Row{Cells: []string{n.TS.Local().Format("2006-01-02 15:04"), label, n.Text}})
	}
	return rows
}
