// Command antigame, oyun suresi takibi ve MFA kapisi saglar.
// Bu dosya yalnizca alt komut dagitimi yapar, is mantigi icermez.
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime/debug"
	"slices"
	"syscall"

	"github.com/guts/antigame/internal/config"
	"github.com/guts/antigame/internal/gamelist"
	"github.com/guts/antigame/internal/gate"
	"github.com/guts/antigame/internal/menu"
	"github.com/guts/antigame/internal/report"
	"github.com/guts/antigame/internal/setup"
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
  antigame list               Oyun listesini görüntüle / düzenle
  antigame report             Haftalık raporu tarayıcıda aç
  antigame uninstall          Kodla doğrulayıp kaldır

Argümansız çalıştırıldığında (ör. çift tıklayarak) menü açılır.
`

func main() {
	// Cift tiklayan kullanici icin menu: argumansiz calistirildiginda
	// kullanim metnini yazip kapanmak, ekranda hicbir sey gostermiyordu.
	if len(os.Args) < 2 {
		if err := menu.Run(os.Stdin, os.Stdout, menuItems()); err != nil {
			fmt.Fprintf(os.Stderr, "hata: %v\n", err)
			os.Exit(1)
		}
		return
	}
	var err error
	switch os.Args[1] {
	case "gate":
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
	case "uninstall":
		err = uninstall.Run(config.Dir(), os.Stdin, os.Stdout)
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
		{Key: "4", Label: "İzleyiciyi şimdi başlat (Ctrl+C ile durur)", Run: func() error {
			return runWatch(false)
		}},
		{Key: "5", Label: "Kaldır", Run: func() error {
			return uninstall.Run(dir, os.Stdin, os.Stdout)
		}},
	}
}

func runWatch(background bool) error {
	// Izleyici cok az ayirma yapar; varsayilan %100 yerine daha sik ve
	// daha kucuk toplama, kalici bellek tabanini asagi ceker.
	debug.SetGCPercent(20)

	// Zamanlanmis gorev bu modu kullanir: konsol penceresi ekranda kalmasin
	// ve kullanici onu kapatinca izleyici olmesin. Menuden baslatilan izleyici
	// bu yola girmez, kullanicinin kendi penceresinde Ctrl+C ile durur.
	if background {
		if err := winproc.DetachConsole(); err != nil {
			return err
		}
	}

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
		Dir:           dir,
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
	})
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return w.Run(ctx)
}
