// Command antigame, oyun suresi takibi ve MFA kapisi saglar.
// Bu dosya yalnizca alt komut dagitimi yapar, is mantigi icermez.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime/debug"
	"slices"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/windows"

	"github.com/guts/antigame/internal/config"
	"github.com/guts/antigame/internal/gamelist"
	"github.com/guts/antigame/internal/gate"
	"github.com/guts/antigame/internal/menu"
	"github.com/guts/antigame/internal/people"
	"github.com/guts/antigame/internal/report"
	"github.com/guts/antigame/internal/setup"
	"github.com/guts/antigame/internal/single"
	"github.com/guts/antigame/internal/status"
	"github.com/guts/antigame/internal/task"
	"github.com/guts/antigame/internal/telegramwatch"
	"github.com/guts/antigame/internal/tray"
	"github.com/guts/antigame/internal/ui"
	"github.com/guts/antigame/internal/uninstall"
	"github.com/guts/antigame/internal/watch"
	"github.com/guts/antigame/internal/wininput"
	"github.com/guts/antigame/internal/winproc"
)

const usage = `antigame — oyun süresi takibi ve MFA kapısı

Kullanım:
  antigame setup              Kurulum sihirbazı (MFA eşleştirme)
  antigame watch              İzleyiciyi başlat (zamanlanmış görev çalıştırır)
  antigame gate --app <ad>    Kod giriş penceresi
  antigame gate --manual      Oyun açmadan kod gir
  antigame list               Oyun listesini görüntüle / düzenle
  antigame people             Kapıyı açabilen kişileri yönet
  antigame report             Haftalık raporu tarayıcıda aç
  antigame autostart          Başlangıca ekle / çıkar
  antigame uninstall          Kodla doğrulayıp kaldır

Argümansız çalıştırıldığında (ör. çift tıklayarak) menü açılır.
`

func main() {
	// Cift tiklayan kullanici icin arayuz: argumansiz calistirildiginda
	// pencere acilir. Pencere acilamazsa program kullanilamaz hale
	// gelmemeli; eski metin menusu yedek yol olarak duruyor.
	if len(os.Args) < 2 {
		err := ui.Run(config.Dir(), uiDeps())
		if err == nil {
			return
		}
		attachConsole()
		if errors.Is(err, ui.ErrNoGUI) {
			fmt.Fprintf(os.Stderr, "uyarı: %v, metin menüsüne düşülüyor\n", err)
			err = menu.Run(os.Stdin, os.Stdout, menuHeader, menuItems())
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "hata: %v\n", err)
			os.Exit(1)
		}
		return
	}
	attachConsole()

	var err error
	switch os.Args[1] {
	case "gate":
		if slices.Contains(os.Args[2:], "--manual") {
			err = gate.RunManual(config.Dir())
			// Oturum zaten aciksa yapacak bir sey yok: cagiran taraf
			// kalan sureyi zaten soyluyor, ikinci bir uyari gereksiz.
			if errors.Is(err, gate.ErrSessionOpen) {
				err = nil
			}
			break
		}
		app := "Oyun"
		if len(os.Args) >= 4 && os.Args[2] == "--app" {
			app = os.Args[3]
		}
		err = gate.Run(config.Dir(), app)
	case "watch":
		err = runWatch(slices.Contains(os.Args[2:], "--background"))
	case "setup":
		err = setup.Run(config.Dir(), os.Stdin, os.Stdout)
	case "list":
		err = gamelist.Run(config.Dir(), os.Args[2:], os.Stdout)
	case "people":
		err = people.Screen(config.Dir(), os.Stdin, os.Stdout)
	case "uninstall":
		err = uninstall.Run(config.Dir(), os.Stdin, os.Stdout)
	case "autostart":
		err = toggleAutostart()
	case "report":
		var path string
		path, err = report.Run(config.Dir())
		if err == nil {
			fmt.Println("Rapor:", path)
		}
	default:
		fmt.Fprintf(os.Stderr, "bilinmeyen komut: %s\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "hata: %v\n", err)
		os.Exit(1)
	}
}

// watcherRunning, izleyicinin ayakta olup olmadigini kilidi deneyerek
// anlar: kilit alinabiliyorsa calisan bir izleyici yok.
func watcherRunning() bool {
	release, ok := single.Acquire(watchLock)
	if ok {
		release()
		return false
	}
	return true
}

// menuHeader, menunun ustunde o anki durumu gosterir. Her cizimde
// yeniden okunur; menuyu acik birakan kullanici degisiklikleri gorur.
func menuHeader() string {
	dir := config.Dir()
	var b strings.Builder

	if watcherRunning() {
		b.WriteString("İzleyici: çalışıyor\n")
	} else {
		b.WriteString("İzleyici: durdu — 4 ile başlatabilirsiniz\n")
	}

	if !keyReady(dir) {
		b.WriteString("MFA: kurulmadı — kapı devre dışı, yalnızca süre kaydediliyor\n")
		return b.String()
	}
	b.WriteString(people.Summary(dir) + "\n")
	if installed, _ := task.Installed(); installed {
		b.WriteString("Başlangıç: kayıtlı (oturum açılışında başlar)\n")
	} else {
		b.WriteString("Başlangıç: kayıtlı değil — 7 ile ekleyebilirsiniz\n")
	}
	s, err := status.Text(dir, time.Now().UTC())
	if err != nil {
		return b.String()
	}
	b.WriteString(s)
	return b.String()
}

// keyReady, kapiyi acabilecek en az bir kisi olup olmadigini soyler.
// Bir kisinin bozuk anahtar dosyasi digerlerini kapinin disinda
// birakmamali; bu yuzden sayilan, cozulebilen anahtarlardir.
func keyReady(dir string) bool {
	keys, err := people.Keys(dir)
	return err == nil && len(keys) > 0
}

// runningExes, oyun eklerken secenek olarak sunulacak program adlaridir.
// Kullanicinin exe adini bilmesi gerekmesin diye calisan process'lerden
// turetiliyor; tekrarlar ayiklaniyor.
func runningExes() []string {
	procs, err := winproc.List()
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
		out = append(out, p.Exe)
	}
	return out
}

func menuItems() []menu.Item {
	dir := config.Dir()
	return []menu.Item{
		{Key: "1", Label: "Kurulum (MFA eşleştirme)", Run: func() error {
			return setup.Run(dir, os.Stdin, os.Stdout)
		}},
		{Key: "2", Label: "Oyun listesini göster", Run: func() error {
			return gamelist.Run(dir, nil, os.Stdout)
		}},
		{Key: "3", Label: "Haftalık raporu aç", Run: func() error {
			path, err := report.Run(dir)
			if err == nil {
				fmt.Println("Rapor:", path)
			}
			return err
		}},
		{Key: "4", Label: "İzleyiciyi şimdi başlat (arka planda)", Run: func() error {
			return runWatch(false)
		}},
		{Key: "5", Label: "Oyun ekle", Run: func() error {
			return gamelist.AddInteractive(dir, os.Stdin, os.Stdout, runningExes())
		}},
		{Key: "6", Label: "Oyun çıkar", Run: func() error {
			return gamelist.RemoveInteractive(dir, os.Stdin, os.Stdout)
		}},
		{Key: "7", Label: "Başlangıca ekle / çıkar", Run: toggleAutostart},
		{Key: "8", Label: "Kaldır", Run: func() error {
			return uninstall.Run(dir, os.Stdin, os.Stdout)
		}},
		{Key: "9", Label: "Kişileri yönet (anahtar verilenler)", Run: func() error {
			return people.Screen(dir, os.Stdin, os.Stdout)
		}},
	}
}

const watchLock = "antigame-watch"

// uiDeps, arayuzun kendi basina yapamayacagi isleri verir. Tek-ornek
// kilidinin adi ve izleyiciyi konsoldan koparma bayragi bu katmanin
// bilgisi; arayuz bunlari ogrenirse ayni mantik iki yerde yasar.
func uiDeps() ui.Deps {
	return ui.Deps{
		WatcherRunning: watcherRunning,
		StartWatcher:   spawnWatcher,
		ExePath:        os.Executable,
	}
}

// toggleAutostart, oturum acilisinda baslatan zamanlanmis gorevi kurar
// veya kaldirir. Kurulum sihirbazi bunu zaten yapiyor; buradaki secenek
// gorevi sonradan kaldirip geri eklemek icin.
func toggleAutostart() error {
	installed, err := task.Installed()
	if err != nil {
		return err
	}
	if installed {
		if err := task.Remove(); err != nil {
			return err
		}
		fmt.Println("Başlangıçtan çıkarıldı. İzleyici oturum açılışında başlamayacak.")
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if err := task.Install(exe); err != nil {
		return err
	}
	fmt.Printf("Başlangıca eklendi: %s\n", exe)
	if !keyReady(config.Dir()) {
		fmt.Println("Uyarı: MFA kurulumu yapılmadığı için kapı devre dışı;" +
			" izleyici yalnızca süre kaydeder. Kurulumu 1 ile tamamlayın.")
	}
	return nil
}

// spawnWatcher, izleyiciyi ayri ve konsoldan bagimsiz bir process olarak
// baslatir. Terminalden baslatildiginda pencere kapaninca izleyicinin de
// olmesi bekleniyordu; artik olmuyor.
func spawnWatcher() error {
	if release, ok := single.Acquire(watchLock); ok {
		release()
	} else {
		fmt.Println("İzleyici zaten çalışıyor.")
		return nil
	}

	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, "watch", "--background")
	// DETACHED_PROCESS: cocuk kendi konsolsuz process'i olur ve ust
	// process'in penceresi kapaninca etkilenmez.
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.DETACHED_PROCESS}
	if err := cmd.Start(); err != nil {
		return err
	}
	// Beklemiyoruz: amac zaten baglantiyi koparmak.
	fmt.Printf("İzleyici arka planda başlatıldı (PID %d). Bu pencereyi kapatabilirsiniz.\n",
		cmd.Process.Pid)
	return nil
}

func runWatch(background bool) error {
	if !background {
		return spawnWatcher()
	}

	// Ayni anda iki izleyici gunluge cift kayit yazar ve kapida iki
	// pencere acar; ikincisi sessizce cikmali.
	release, ok := single.Acquire(watchLock)
	if !ok {
		return nil
	}
	defer release()

	// Izleyici cok az ayirma yapar; varsayilan %100 yerine daha sik ve
	// daha kucuk toplama, kalici bellek tabanini asagi ceker.
	debug.SetGCPercent(20)

	// Konsol yoksa (DETACHED_PROCESS ile baslatildiysa) bu zaten basarisiz
	// olur; gorev uzerinden gelindiginde pencereyi kapatir. Iki durumda da
	// izleyicinin calismasini engellememeli.
	_ = winproc.DetachConsole()

	dir := config.Dir()
	cfg, err := config.Load(dir)
	if err != nil {
		return err
	}
	// Spec §10: liste bossa kapida durdurma yapilmaz, yalnizca sure tutulur.
	// Bu sessizce olmamali; kullanici korumasiz oldugunu bilmeli.
	if len(cfg.Gated) == 0 {
		fmt.Fprintln(os.Stderr,
			"uyarı: oyun listesi boş — hiçbir oyun kapıda durdurulmayacak, yalnızca süre kaydedilecek")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	w, err := watch.New(watch.Options{
		Dir: dir,
		// Kullanici veri klasorunu tasidiginda izleyici oraya gecmeli;
		// yoksa kapi yeni klasore oturum acar, izleyici eskiye bakar ve
		// oyunu oldurmeyi surdurur.
		DirFunc:       config.Dir,
		Cfg:           cfg,
		List:          winproc.List,
		Path:          winproc.Path,
		Terminate:     winproc.Terminate,
		Trim:          winproc.Trim,
		Idle:          wininput.IdleSeconds,
		ForegroundPID: wininput.ForegroundPID,
		SpawnGate: func(app string) error {
			return exec.Command(exe, "gate", "--app", app).Start()
		},
		// Eslestirme yoksa kapi acilamaz; izleyici oyunu oldurup
		// kullaniciyi kod girecek yeri olmadan kilitlememeli.
		SecretReady: func() bool { return keyReady(dir) },
	})
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Izleyici arka planda, tepsi mesaj dongusu ise cagiran thread'de calisir:
	// pencere mesajlari yalnizca pencereyi olusturan thread'e teslim edilir.
	watcher := make(chan error, 1)
	go func() { watcher <- w.Run(ctx) }()

	// Telegram bildirimi izleyiciden tamamen bagimsiz calisir: ag
	// cagrisi anti-cheat'in kritik dongusunu hicbir zaman bloklamamali
	// (bkz. internal/telegramwatch paket belgesi).
	go func() { _ = telegramwatch.Run(ctx, config.Dir) }()

	if err := tray.Run(ctx, tray.Options{
		Tip: "antigame — izleyici çalışıyor",
		// Tooltip ve sol tik ayni cumleyi gosteriyor: kullanicinin
		// sordugu tek sey ne kadar suresi kaldigi.
		TipFunc: func() string { return "antigame — " + briefText(dir) },
		Items:   trayItems(dir),
		OnClick: func() { tray.Info("antigame", briefText(dir)) },
	}); err != nil {
		// Tepsi acilamazsa izleyici yine de calismali; simge bir kolaylik,
		// isin kendisi degil.
		fmt.Fprintf(os.Stderr, "uyarı: tepsi simgesi açılamadı: %v\n", err)
		return <-watcher
	}
	stop()
	return <-watcher
}

// briefText, kalan sureyi tek satirda verir. Okunamazsa nedeni yaziliyor:
// arka planda calisan izleyicinin hatayi soyleyecek baska yeri yok.
func briefText(dir string) string {
	s, err := status.Brief(dir, time.Now().UTC())
	if err != nil {
		return "Durum okunamadı: " + err.Error()
	}
	return s
}

// openManualGate, oyun acmadan kod girme penceresini ayri process olarak
// baslatir. Kapinin kendi mesaj dongusu var; cagiranin dongusu icine
// ikincisini kurmak ikisini de kilitlerdi.
func openManualGate(dir string) {
	if open, err := status.SessionOpen(dir, time.Now().UTC()); err == nil && open {
		tray.Info("antigame", "Oturum zaten açık. "+briefText(dir))
		return
	}
	exe, err := os.Executable()
	if err != nil {
		tray.Info("antigame", "Program yolu bulunamadı: "+err.Error())
		return
	}
	if err := exec.Command(exe, "gate", "--manual").Start(); err != nil {
		tray.Info("antigame", "Kod penceresi açılamadı: "+err.Error())
	}
}

func trayItems(dir string) []tray.Item {
	return []tray.Item{
		// Varsayilan oge: simgeye cift tiklayinca bu calisir.
		{Label: "Arayüzü aç", Default: true, Run: func() {
			exe, err := os.Executable()
			if err != nil {
				tray.Info("antigame", "Program yolu bulunamadı: "+err.Error())
				return
			}
			// Arayuz ayri process olarak aciliyor: kendi mesaj dongusu
			// ve tek-ornek kilidi var. Zaten aciksa kilidi alamaz ve
			// mevcut pencereyi one getirip cikar.
			if err := exec.Command(exe).Start(); err != nil {
				tray.Info("antigame", "Arayüz açılamadı: "+err.Error())
			}
		}},
		{Label: "Kod gir…", Run: func() { openManualGate(dir) }},
		{Label: "Haftalık raporu aç", Run: func() {
			if _, err := report.Run(dir); err != nil {
				tray.Info("antigame", "Rapor açılamadı: "+err.Error())
			}
		}},
		{Label: "Oyun listesi", Run: func() {
			cfg, err := config.Load(dir)
			if err != nil {
				tray.Info("antigame", "Liste okunamadı: "+err.Error())
				return
			}
			tray.Info("antigame — oyun listesi", gamelist.Format(cfg))
		}},
		{Label: "Durum", Run: func() {
			s, err := status.Text(dir, time.Now().UTC())
			if err != nil {
				tray.Info("antigame", "Durum okunamadı: "+err.Error())
				return
			}
			tray.Info("antigame — durum", s)
		}},
	}
}
