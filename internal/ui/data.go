//go:build windows

package ui

import (
	"fmt"
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"

	"github.com/guts/antigame/internal/config"
	"github.com/guts/antigame/internal/datainfo"
)

// showData, verilerin nerede saklandigini gosteren diyalogu acar.
//
// Salt okunur: kisiler ve oyunlar kendi ekranlarindan degistiriliyor,
// dosyalari buradan duzenlemek ayni isi iki yoldan yapmak olurdu.
func showData(parent uintptr, dir string) {
	m, err := newModal(parent, "Veriler", 580, 430)
	if err != nil {
		warn(parent, "antigame", "Pencere açılamadı: "+err.Error())
		return
	}

	m.label("Her şey bu klasörde:", Rect{12, 10, 556, 18})
	// Salt okunur metin alani: yol secilip kopyalanabilsin diye etiket
	// degil EDIT kullaniliyor.
	path := m.edit(dir, Rect{12, 30, 556, 24}, esReadOnly)

	list := m.list(Rect{12, 64, 556, 250},
		[]string{"Dosya", "Boyut", "İçerik"}, []int32{180, 70, 290})

	m.label("Anahtar dosyaları DPAPI ile şifreli: yalnızca bu makinede ve bu "+
		"Windows hesabında açılabilir. Başka bir bilgisayara kopyalamak işe yaramaz.",
		Rect{12, 322, 556, 40})

	status := m.label("", Rect{12, 366, 350, 18})

	_, openID := m.button("Klasörü aç", Rect{366, 390, 100, 28}, false)
	_, closeID := m.button("Kapat", Rect{478, 390, 90, 28}, true)

	var people []config.Person
	if cfg, err := config.Load(dir); err == nil {
		people = cfg.People
	}

	entries, err := datainfo.List(dir, people)
	switch {
	case err != nil:
		setText(status, "Klasör okunamadı: "+err.Error())
	case len(entries) == 0:
		setText(status, "Klasör henüz oluşmamış — kurulum yapılmamış olabilir.")
	default:
		lvSetRows(list, dataRows(entries))
	}

	m.onCmd = func(id int) {
		switch id {
		case closeID, idCancel, idOK:
			m.close()
		case openID:
			if err := openFolder(dir); err != nil {
				setText(status, "Klasör açılamadı: "+err.Error())
			}
		}
	}

	focus(path)
	m.run(parent)
}

func dataRows(es []datainfo.Entry) []Row {
	rows := make([]Row, 0, len(es))
	for _, e := range es {
		rows = append(rows, Row{Cells: []string{e.Name, size(e.Size), e.Desc}})
	}
	return rows
}

// size, bayt sayisini okunabilir hale getirir. Kesinlik gerekmiyor;
// kullanicinin merak ettigi buyukluk mertebesi.
func size(n int64) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%d B", n)
	case n < 1024*1024:
		return fmt.Sprintf("%d KB", n/1024)
	default:
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	}
}

// openFolder, veri klasorunu Dosya Gezgini'nde acar.
//
// explorer.exe basarili durumda bile sifir disi cikis kodu dondurebiliyor,
// bu yuzden Run degil Start kullaniliyor: cikis kodunu beklemek yanlis
// hata mesaji uretirdi.
func openFolder(dir string) error {
	c := exec.Command("explorer", dir)
	c.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW,
	}
	return c.Start()
}
