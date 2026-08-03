// Command antigame, oyun suresi takibi ve MFA kapisi saglar.
// Bu dosya yalnizca alt komut dagitimi yapar, is mantigi icermez.
package main

import (
	"fmt"
	"os"

	"github.com/guts/antigame/internal/config"
	"github.com/guts/antigame/internal/gate"
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
	default:
		fmt.Fprintf(os.Stderr, "bilinmeyen komut: %s\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "hata: %v\n", err)
		os.Exit(1)
	}
}
