//go:build windows

package ui

import "testing"

func TestScaleIsIdentityAt96DPI(t *testing.T) {
	if got := Scale(96, 100); got != 100 {
		t.Errorf("Scale(96,100) = %d, istenen 100", got)
	}
}

func TestScaleGrowsWithDPI(t *testing.T) {
	if got := Scale(144, 100); got != 150 {
		t.Errorf("Scale(144,100) = %d, istenen 150", got)
	}
}

func TestScaleTreatsZeroDPIAs96(t *testing.T) {
	// GetDpiForWindow basarisiz olursa 0 doner; olculer sifira cokmemeli.
	if got := Scale(0, 100); got != 100 {
		t.Errorf("Scale(0,100) = %d, istenen 100", got)
	}
}

func TestMainStacksControlsWithoutOverlap(t *testing.T) {
	l := Main(640, 520, 96)
	if l.Status.Y+l.Status.H > l.GamesLabel.Y {
		t.Error("durum blogu oyun basligiyla cakisiyor")
	}
	if l.GamesLabel.Y+l.GamesLabel.H > l.Games.Y {
		t.Error("baslik listeyle cakisiyor")
	}
	if l.Games.Y+l.Games.H > l.AddBtn.Y {
		t.Error("liste ekle/cikar dugmeleriyle cakisiyor")
	}
	if l.AddBtn.Y+l.AddBtn.H > l.AutoStart.Y {
		t.Error("ekle/cikar baslangic kutusuyla cakisiyor")
	}
	if l.AutoStart.Y+l.AutoStart.H > l.Note.Y {
		t.Error("baslangic kutusu not satiriyla cakisiyor")
	}
	if l.Note.Y+l.Note.H > l.WatchBtn.Y {
		t.Error("not satiri alt dugmelerle cakisiyor")
	}
}

func TestMainKeepsEverythingInsideTheWindow(t *testing.T) {
	const w, h int32 = 640, 520
	l := Main(w, h, 96)
	for name, r := range map[string]Rect{
		"status": l.Status, "games": l.Games,
		"add": l.AddBtn, "remove": l.RemoveBtn,
		"watch": l.WatchBtn, "report": l.ReportBtn,
		"people": l.PeopleBtn, "removeApp": l.RemoveAppBtn,
	} {
		if r.X < 0 || r.Y < 0 || r.X+r.W > w || r.Y+r.H > h {
			t.Errorf("%s pencere disinda: %+v", name, r)
		}
	}
}

func TestMainButtonsDoNotOverlapHorizontally(t *testing.T) {
	l := Main(640, 520, 96)
	row := []Rect{l.WatchBtn, l.ReportBtn, l.PeopleBtn, l.RemoveAppBtn}
	for i := 1; i < len(row); i++ {
		if row[i-1].X+row[i-1].W > row[i].X {
			t.Errorf("%d. ve %d. dugme cakisiyor", i-1, i)
		}
	}
	if l.AddBtn.X+l.AddBtn.W > l.RemoveBtn.X {
		t.Error("ekle ve cikar dugmeleri cakisiyor")
	}
}

func TestMainGivesExtraHeightToTheList(t *testing.T) {
	small := Main(640, 520, 96)
	big := Main(640, 800, 96)
	if big.Games.H <= small.Games.H {
		t.Error("pencere buyudugunde liste buyumeli")
	}
	if big.WatchBtn.H != small.WatchBtn.H {
		t.Error("dugme yuksekligi pencereyle degismemeli")
	}
}

func TestMainPinsButtonsToTheBottom(t *testing.T) {
	const h int32 = 800
	l := Main(640, h, 96)
	if gap := h - (l.WatchBtn.Y + l.WatchBtn.H); gap != pad {
		t.Errorf("dugmeler alta sabitlenmemis, alt bosluk %d", gap)
	}
}

func TestMainSurvivesTheMinimumSize(t *testing.T) {
	l := Main(MinW, MinH, 96)
	if l.Games.H <= 0 {
		t.Errorf("en kucuk boyutta liste yok oldu: %+v", l.Games)
	}
	if l.Games.Y+l.Games.H > l.AddBtn.Y {
		t.Error("en kucuk boyutta liste dugmelerle cakisiyor")
	}
}

func TestMainScalesWithDPI(t *testing.T) {
	lo := Main(640, 520, 96)
	hi := Main(960, 780, 144)
	if hi.WatchBtn.H <= lo.WatchBtn.H {
		t.Error("dugme yuksekligi DPI ile buyumeli")
	}
}
