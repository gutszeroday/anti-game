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
	"github.com/guts/antigame/internal/people"
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
	if !strings.EqualFold(strings.TrimSpace(ans), "e") {
		fmt.Fprintf(out, "Veriler duruyor: %s\n", dir)
		return nil
	}
	// Gorev yukarida kaldirildi; burada yalnizca veri silinsin diye
	// kaldirici yerine bos bir islev veriliyor.
	if err := purge(dir, true, func() error { return nil }); err != nil {
		return err
	}
	fmt.Fprintln(out, "Veriler silindi.")
	return nil
}

// purge, gorev kaldiriciyi disaridan alir; boylece gercek bir zamanlanmis
// gorev olusturmadan test edilebilir.
//
// Sira onemli: gorev kaldirilamadiysa veri durmali. Aksi halde izleyici
// oturum acilisinda yeniden baslar ama gecmisi yok olmus olur.
func purge(dir string, deleteData bool, remove func() error) error {
	if err := remove(); err != nil {
		return err
	}
	if !deleteData {
		return nil
	}
	return os.RemoveAll(dir)
}

// Purge, zamanlanmis gorevi kaldirir ve istenirse veri dizinini siler.
// Kodun dogrulanmasi cagirana ait: once Verify, sonra Purge.
func Purge(dir string, deleteData bool) error {
	return purge(dir, deleteData, task.Remove)
}

// Verify, kaldirma kodunu dogrular ve reddedilme sebebini metin olarak
// dondurur. Yan etkisi var: Attempt basarida bir oturum acar. Kaldirma
// sirasinda bunun pratik etkisi yok, kilit ve tekrar kullanim korumasini
// yeniden yazmamak icin ayni yol kullaniliyor.
func Verify(dir, code string) (bool, string, error) {
	cfg, err := config.Load(dir)
	if err != nil {
		return false, "", err
	}
	keys, err := people.Keys(dir)
	if err != nil {
		return false, "", err
	}
	if len(keys) == 0 {
		return false, "", vault.ErrNoSecret
	}
	v := auth.Verifier{
		Dir:   dir,
		Keys:  keys,
		Grace: time.Duration(cfg.GraceMinutes) * time.Minute,
	}
	o, err := v.Attempt(code)
	if err != nil {
		return false, "", err
	}
	return o.OK, o.Message, nil
}

// Run, cmd tarafindan cagrilan ust seviye giristir.
//
// Anahtar yoklugu kod sorulmadan once kontrol ediliyor: girilmesi imkansiz
// bir kod istemek kullaniciyi bosuna ugrastirirdi.
func Run(dir string, in io.Reader, out io.Writer) error {
	keys, err := people.Keys(dir)
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		return vault.ErrNoSecret
	}
	verify := func(code string) (bool, string, error) { return Verify(dir, code) }
	return run(dir, in, out, verify, task.Remove)
}
