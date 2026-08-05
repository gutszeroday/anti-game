//go:build windows

// Package setup, ilk kurulumu yapar: ilk kisiyi eslestirir, kurtarma
// kodunu uretir ve zamanlanmis gorevi kurar.
//
// Kisi ekleme/duzenleme akislari burada degil people paketindedir;
// kurulum yalnizca "hicbir sey yokken" calisan yoldur.
package setup

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/guts/antigame/internal/auth"
	"github.com/guts/antigame/internal/config"
	"github.com/guts/antigame/internal/pairing"
	"github.com/guts/antigame/internal/people"
	"github.com/guts/antigame/internal/store"
	"github.com/guts/antigame/internal/task"
	"github.com/guts/antigame/internal/vault"
)

// Run, etkilesimli kurulum sihirbazidir.
func Run(dir string, in io.Reader, out io.Writer) error {
	r := bufio.NewReader(in)
	ask := func(q string) (string, error) {
		fmt.Fprint(out, q)
		s, err := r.ReadString('\n')
		return strings.TrimSpace(s), err
	}

	cfg, err := people.Ensure(dir)
	if err != nil {
		return err
	}
	if len(cfg.People) > 0 {
		var names []string
		for _, p := range cfg.People {
			names = append(names, p.Name)
		}
		fmt.Fprintf(out, "Zaten kurulu. Kayıtlı kişiler: %s\n", strings.Join(names, ", "))
		a, _ := ask("Baştan kurulsun mu? Kayıtlı kişilerin hepsi silinir (e/h): ")
		if !strings.EqualFold(a, "e") {
			fmt.Fprintln(out, "Kurulum iptal edildi. Kişileri menüdeki \"Kişileri yönet\" ekranından düzenleyebilirsiniz.")
			return nil
		}
		for _, p := range cfg.People {
			if err := vault.RemovePerson(dir, p.ID); err != nil {
				return err
			}
		}
		cfg.People = nil
		if err := config.Save(dir, cfg); err != nil {
			return err
		}
	}

	name, err := ask("MFA'yı tutacak arkadaşınızın adı: ")
	if err != nil && name == "" {
		return err
	}
	hint, err := ask("Ona nasıl ulaşacaksınız (kapıda gösterilir, ör. WhatsApp): ")
	if err != nil && hint == "" {
		return err
	}

	secret, counter, err := pairing.Pair(r, out, name, func() error {
		return store.Append(dir, store.Event{TS: time.Now().UTC(), Ev: "pairing_manual"})
	})
	if err != nil {
		return err
	}
	if _, err := people.Add(dir, name, hint, secret, counter); err != nil {
		return err
	}

	st, err := store.LoadState(dir)
	if err != nil {
		return err
	}
	st.FailCount = 0
	st.LockUntil = nil

	recovery, salt, hash, err := auth.NewRecoveryCode()
	if err != nil {
		return err
	}
	st.RecoverySalt, st.RecoveryHash, st.RecoveryUsed = salt, hash, false
	if err := store.SaveState(dir, st); err != nil {
		return err
	}

	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if err := task.Install(exe); err != nil {
		return err
	}

	fmt.Fprintf(out, `
Kurulum tamamlandı.

KURTARMA KODU: %s

Bu kodu SİZ saklamayın — arkadaşınız telefonunu kaybederse diye
ikinci bir kişiye verin. Tek kullanımlıktır.

Başka kişilere de anahtar vermek için menüdeki "Kişileri yönet"
ekranını kullanın.

İzleyici oturum açılışında otomatik başlayacak. Şimdi başlatmak için:
  antigame watch
`, recovery)
	return nil
}
