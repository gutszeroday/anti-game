//go:build windows

// Package uninstall, gecerli bir kod karsiliginda zamanlanmis gorevi
// kaldirir ve istege bagli olarak verileri siler.
//
// Not: bu, kaldirmayi engellemez. Kullanici yonetici yetkisine sahiptir ve
// gorevi elle de silebilir. Amac, kaldirmayi kaza eseri degil bilincli bir
// karar haline getirmektir.
package uninstall

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/guts/antigame/internal/auth"
	"github.com/guts/antigame/internal/config"
	"github.com/guts/antigame/internal/task"
	"github.com/guts/antigame/internal/vault"
)

type verifyFunc func(code string) (ok bool, message string, err error)

// run, disaridan verilen dogrulayici ve kaldirici ile calisir; boylece
// gercek bir zamanlanmis gorev olusturmadan test edilebilir.
func run(dir string, in io.Reader, out io.Writer, verify verifyFunc, remove func() error) error {
	r := bufio.NewReader(in)

	fmt.Fprint(out, "Kaldırmak için arkadaşınızdan kod isteyin.\nKod: ")
	code, _ := r.ReadString('\n')
	code = strings.TrimSpace(code)
	if code == "" {
		return errors.New("kod girilmedi, kaldırma iptal edildi")
	}

	ok, msg, err := verify(code)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("kaldırma reddedildi: %s", msg)
	}

	if err := remove(); err != nil {
		return err
	}
	fmt.Fprintln(out, "Zamanlanmış görev kaldırıldı. İzleyici bir daha otomatik başlamayacak.")

	fmt.Fprint(out, "Kayıtlı süre verileri de silinsin mi? (e/h): ")
	ans, _ := r.ReadString('\n')
	if strings.EqualFold(strings.TrimSpace(ans), "e") {
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
		fmt.Fprintln(out, "Veriler silindi.")
	} else {
		fmt.Fprintf(out, "Veriler duruyor: %s\n", dir)
	}
	return nil
}

// Run, cmd tarafindan cagrilan ust seviye giristir.
func Run(dir string, in io.Reader, out io.Writer) error {
	cfg, err := config.Load(dir)
	if err != nil {
		return err
	}
	secret, err := vault.Load(dir)
	if err != nil {
		return err
	}
	v := auth.Verifier{
		Dir:    dir,
		Secret: secret,
		Grace:  time.Duration(cfg.GraceMinutes) * time.Minute,
	}
	verify := func(code string) (bool, string, error) {
		// Attempt basarida bir oturum acar; kaldirma sirasinda bunun
		// pratik bir etkisi yok, kilit ve tekrar kullanim korumasini
		// yeniden yazmamak icin ayni yol kullaniliyor.
		o, err := v.Attempt(code)
		if err != nil {
			return false, "", err
		}
		return o.OK, o.Message, nil
	}
	return run(dir, in, out, verify, task.Remove)
}
