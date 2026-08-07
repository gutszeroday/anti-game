//go:build windows

package ui

import (
	"fmt"
	"time"

	"github.com/guts/antigame/internal/datadir"
	"github.com/guts/antigame/internal/store"
)

// Veri klasorunu tasima akisi.
//
// Sira onemli ve her adim bir oncekinin basarisina bagli:
//
//	1. dogrula      hedef kullanilabilir mi
//	2. kopyala      dosyalar yeni klasore
//	3. dogrula      kopya birebir mi
//	4. olay yaz     data_moved -> YENI klasore
//	5. ayar         kayit defterine yeni yol
//	6. bekle        izleyici yeni klasore gecti mi
//	7. sil          eski dosyalar
//
// 1-3 arasinda bir sey patlarsa hicbir sey degismez: ayar hala eski
// klasoru gosterir ve hedefteki yarim kopyaya kimse bakmaz.

// watcherSwitchTimeout, izleyicinin yeni klasore gecmesi icin taninan
// suredir. Izleyici klasoru bes saniyede bir kontrol ediyor; ucu bir
// kere kacirilmis kontrole pay.
var watcherSwitchTimeout = 15 * time.Second

// setWatcherSwitchTimeout, testlerin beklemeyi kisaltmasi icin.
func setWatcherSwitchTimeout(d time.Duration) { watcherSwitchTimeout = d }

// moveResult, tasimanin sonucunu anlatir.
type moveResult struct {
	// OldKept, eski klasorun silinmediginı soyler: izleyici yeni yere
	// gecmediyse eski veriyi silmek onun altindan zemini cekerdi.
	OldKept bool
	Note    string
}

// moveData, veri klasorunu tasir. progress her adimda cagrilir.
//
// setDir disaridan aliniyor: gercek surumu kayit defterine yaziyor ve
// testlerin kullanicinin makinesindeki ayari degistirmemesi gerekiyor.
func moveData(from, to string, watcherRunning bool, setDir func(string) error, progress func(string)) (moveResult, error) {
	var res moveResult

	progress("Hedef klasör kontrol ediliyor…")
	if err := datadir.Validate(from, to); err != nil {
		return res, err
	}

	progress("Dosyalar kopyalanıyor…")
	if err := datadir.Copy(from, to); err != nil {
		return res, err
	}

	progress("Kopya doğrulanıyor…")
	if err := datadir.Verify(from, to); err != nil {
		return res, err
	}

	// Olay yeni klasore yaziliyor: gecmis orada devam edecek.
	if err := store.Append(to, store.Event{
		TS: time.Now().UTC(), Ev: "data_moved", From: from, To: to,
	}); err != nil {
		return res, fmt.Errorf("taşıma günlüğe yazılamadı: %w", err)
	}

	progress("Ayar kaydediliyor…")
	if err := setDir(to); err != nil {
		// Kopya hedefte duruyor ama kimse ona bakmiyor; eski klasor
		// aktif kaldigi icin veri kaybi yok.
		return res, fmt.Errorf("yeni klasör kaydedilemedi, eski klasör kullanılmaya devam ediyor: %w", err)
	}

	if watcherRunning {
		progress("İzleyicinin yeni klasöre geçmesi bekleniyor…")
		if !waitForWatcher(to, watcherSwitchTimeout) {
			res.OldKept = true
			res.Note = "İzleyici yeni klasöre geçmedi, eski klasör silinmedi: " + from
			return res, nil
		}
	}

	progress("Eski dosyalar siliniyor…")
	if err := datadir.RemoveContents(from); err != nil {
		res.OldKept = true
		res.Note = "Taşındı, ama eski dosyalar silinemedi: " + err.Error()
		return res, nil
	}

	res.Note = "Taşındı: " + to
	return res, nil
}

// waitForWatcher, izleyicinin yeni klasore gectigine dair kaniti bekler.
//
// Kor bir sleep yerine kanit bekleniyor: kanit gelmediyse eski veriyi
// silmek, hala eski klasore yazan izleyicinin altindan zemini cekmek
// olurdu.
func waitForWatcher(dir string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		now := time.Now().UTC()
		ev, err := store.Read(dir, now.AddDate(0, 0, -1), now.Add(time.Minute))
		if err == nil {
			for _, e := range ev {
				if e.Ev == "data_dir_changed" && e.To == dir {
					return true
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false
}
