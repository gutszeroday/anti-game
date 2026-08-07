//go:build windows

package ui

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/guts/antigame/internal/config"
	"github.com/guts/antigame/internal/gamelist"
	"github.com/guts/antigame/internal/winproc"
)

// showAddGame, oyun ekleme diyalogunu acar. Eklendiyse true doner;
// cagiran listeyi tazeler.
func showAddGame(parent uintptr, dir string) bool {
	m, err := newModal(parent, "Oyun ekle", 430, 372)
	if err != nil {
		warn(parent, "antigame", "Pencere açılamadı: "+err.Error())
		return false
	}

	options := runningCandidates(dir)

	m.label("Şu anda çalışan programlar:", Rect{12, 10, 406, 18})
	list := m.list(Rect{12, 32, 406, 176}, []string{"Program"}, []int32{380})
	lvSetRows(list, exeRows(options))

	m.label("veya exe adını yazın (ör. Palworld.exe):", Rect{12, 216, 406, 18})
	entry := m.edit("", Rect{12, 236, 406, 24}, 0)

	launcher := m.checkbox("Bu bir başlatıcı — kapıda durdurulur, süresi oyun süresi sayılmaz",
		Rect{12, 268, 406, 22})

	status := m.label("", Rect{12, 296, 406, 34})

	addBtn, addID := m.button("Ekle", Rect{224, 334, 90, 28}, true)
	_, cancelID := m.button("Vazgeç", Rect{326, 334, 90, 28}, false)
	_ = addBtn

	added := false
	m.onCmd = func(id int) {
		switch id {
		case cancelID, idCancel:
			m.close()
		case addID, idOK:
			exe := strings.TrimSpace(textOf(entry))
			if exe == "" {
				if i := lvSelected(list); i >= 0 && i < len(options) {
					exe = options[i]
				}
			}
			if exe == "" {
				setText(status, "Listeden bir program seçin veya exe adını yazın.")
				focus(entry)
				return
			}
			name := strings.TrimSuffix(exe, filepath.Ext(exe))
			add := gamelist.Add
			if isChecked(launcher) {
				add = gamelist.AddLauncher
			}
			if err := add(dir, name, exe, ""); err != nil {
				setText(status, err.Error())
				return
			}
			added = true
			m.close()
		}
	}

	focus(entry)
	m.run(parent)
	return added
}

// runningCandidates, eklenebilecek program adlaridir: o anda calisanlar,
// tekrarlar ve zaten listede olanlar ayiklanmis, alfabetik.
//
// Kullanicinin exe adini ezberlemesi gerekmesin diye calisan
// process'lerden turetiliyor; zaten listede olani gostermek kafa
// karistirirdi.
func runningCandidates(dir string) []string {
	procs, err := winproc.List()
	if err != nil {
		return nil
	}
	c, err := config.Load(dir)
	if err != nil {
		return nil
	}

	seen := make(map[string]bool, len(procs))
	var out []string
	for _, p := range procs {
		key := strings.ToLower(p.Exe)
		if p.Exe == "" || seen[key] {
			continue
		}
		seen[key] = true
		if _, gated := c.Match(p.Exe, ""); gated {
			continue
		}
		out = append(out, p.Exe)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i]) < strings.ToLower(out[j])
	})
	return out
}

func exeRows(exes []string) []Row {
	rows := make([]Row, 0, len(exes))
	for _, e := range exes {
		rows = append(rows, Row{Cells: []string{e}})
	}
	return rows
}
