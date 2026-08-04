// Command antigame, oyun suresi takibi ve MFA kapisi saglar.
// Bu dosya yalnizca alt komut dagitimi yapar, is mantigi icermez.
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/guts/antigame/internal/config"
	"github.com/guts/antigame/internal/gate"
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
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
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
		err = runWatch()
	default:
		fmt.Fprintf(os.Stderr, "bilinmeyen komut: %s\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "hata: %v\n", err)
		os.Exit(1)
	}
}

func runWatch() error {
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
