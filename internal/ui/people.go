//go:build windows

package ui

import (
	"strings"

	"github.com/guts/antigame/internal/people"
)

// showPeople, kapiyi acabilen kisileri yonetme diyalogunu acar.
func showPeople(parent uintptr, dir string) {
	m, err := newModal(parent, "Kişiler — anahtar verilenler", 540, 400)
	if err != nil {
		warn(parent, "antigame", "Pencere açılamadı: "+err.Error())
		return
	}

	m.label("Kapıyı bu kişilerin kodları açabilir:", Rect{12, 10, 516, 18})
	list := m.list(Rect{12, 32, 516, 226},
		[]string{"Ad", "İpucu", "Anahtar"}, []int32{170, 240, 90})

	status := m.label("", Rect{12, 266, 516, 52})

	_, addID := m.button("Ekle", Rect{12, 330, 96, 28}, variantSecondary, false)
	_, editID := m.button("Düzenle", Rect{114, 330, 96, 28}, variantSecondary, false)
	_, rotateID := m.button("Anahtar yenile", Rect{216, 330, 118, 28}, variantSecondary, false)
	_, removeID := m.button("Sil", Rect{340, 330, 80, 28}, variantDanger, false)
	_, closeID := m.button("Kapat", Rect{432, 330, 96, 28}, variantSecondary, true)

	var entries []people.Entry

	reload := func() {
		es, err := people.List(dir)
		if err != nil {
			setText(status, "Kişi listesi okunamadı: "+err.Error())
			return
		}
		entries = es
		lvSetRows(list, PeopleRows(es))
	}

	// selected, secili kisiyi verir. Secim yoksa durum satirina yazip
	// false doner; her dugmede ayni kontrolu tekrarlamamak icin.
	selected := func() (people.Entry, bool) {
		i := lvSelected(list)
		if i < 0 || i >= len(entries) {
			setText(status, "Önce listeden bir kişi seçin.")
			return people.Entry{}, false
		}
		return entries[i], true
	}

	m.onCmd = func(id int) {
		switch id {
		case closeID, idCancel, idOK:
			m.close()

		case addID:
			name, hint, ok := askPerson(m.hwnd, "Kişi ekle", "", "")
			if !ok {
				return
			}
			secret, counter, ok := showPair(m.hwnd, name)
			if !ok {
				setText(status, "Eşleştirme tamamlanmadı, kişi eklenmedi.")
				return
			}
			if _, err := people.Add(dir, name, hint, secret, counter); err != nil {
				setText(status, err.Error())
				return
			}
			setText(status, name+" eklendi.")
			reload()

		case editID:
			e, ok := selected()
			if !ok {
				return
			}
			name, hint, ok := askPerson(m.hwnd, "Kişiyi düzenle", e.Name, e.Hint)
			if !ok {
				return
			}
			if err := people.Edit(dir, e.ID, name, hint); err != nil {
				setText(status, err.Error())
				return
			}
			setText(status, "Güncellendi.")
			reload()

		case rotateID:
			e, ok := selected()
			if !ok {
				return
			}
			if !confirm(m.hwnd, "Anahtar yenile",
				e.Name+" için yeni bir anahtar üretilecek. Eski anahtarla üretilen "+
					"kodlar bir daha kapıyı açmayacak. Devam edilsin mi?") {
				return
			}
			secret, counter, ok := showPair(m.hwnd, e.Name)
			if !ok {
				setText(status, "Eşleştirme tamamlanmadı, eski anahtar duruyor.")
				return
			}
			if err := people.Rotate(dir, e.ID, secret, counter); err != nil {
				setText(status, err.Error())
				return
			}
			setText(status, e.Name+" için anahtar yenilendi.")
			reload()

		case removeID:
			e, ok := selected()
			if !ok {
				return
			}
			if !confirm(m.hwnd, "Kişiyi sil",
				e.Name+" silinecek ve anahtarı geçersiz olacak. Devam edilsin mi?") {
				return
			}
			// Son kullanilabilir anahtari silme reddi people.Remove'dan
			// geliyor; burada tekrarlanmiyor.
			if err := people.Remove(dir, e.ID); err != nil {
				setText(status, err.Error())
				return
			}
			setText(status, e.Name+" silindi.")
			reload()
		}
	}

	reload()
	m.run(parent)
}

// askPerson, ad ve ipucu soran kucuk bir diyalog acar. Ad bos
// birakilamaz; ipucu istege bagli.
//
// Ipucu, kisiye nasil ulasilacagini hatirlatir ve kapi penceresinde
// gosterilir: kod isteyecek kisi kimi arayacagini bilmeli.
func askPerson(parent uintptr, title, name, hint string) (string, string, bool) {
	m, err := newModal(parent, title, 380, 190)
	if err != nil {
		warn(parent, "antigame", "Pencere açılamadı: "+err.Error())
		return "", "", false
	}

	m.label("Ad:", Rect{12, 16, 60, 20})
	nameBox := m.edit(name, Rect{76, 12, 288, 26}, 0)

	m.label("İpucu:", Rect{12, 52, 60, 20})
	hintBox := m.edit(hint, Rect{76, 48, 288, 26}, 0)

	m.label("İpucu, kapı penceresinde gösterilir: \"telefon\", \"iş yerinde\" gibi.",
		Rect{12, 82, 352, 32})

	status := m.label("", Rect{12, 116, 352, 18})

	_, okID := m.button("Tamam", Rect{170, 148, 90, 28}, variantPrimary, true)
	_, cancelID := m.button("Vazgeç", Rect{272, 148, 90, 28}, variantSecondary, false)

	var outName, outHint string
	ok := false

	m.onCmd = func(id int) {
		switch id {
		case cancelID, idCancel:
			m.close()
		case okID, idOK:
			outName = strings.TrimSpace(textOf(nameBox))
			if outName == "" {
				setText(status, "Ad boş olamaz.")
				focus(nameBox)
				return
			}
			outHint = strings.TrimSpace(textOf(hintBox))
			ok = true
			m.close()
		}
	}

	focus(nameBox)
	m.run(parent)
	return outName, outHint, ok
}
