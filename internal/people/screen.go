//go:build windows

package people

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/guts/antigame/internal/pairing"
	"github.com/guts/antigame/internal/store"
	"github.com/guts/antigame/internal/term"
)

// nameWidth ve hintWidth, liste sutunlarinin genisligidir. Uzun degerler
// kirpilir; satirin kaymasi listeyi okunmaz yapiyordu.
const (
	nameWidth = 14
	hintWidth = 20
)

// Screen, kisi yonetim ekranini secim yapilana kadar cizer.
func Screen(dir string, in io.Reader, out io.Writer) error {
	r := bufio.NewReader(in)
	th := term.New(out)

	for {
		entries, err := List(dir)
		if err != nil {
			return err
		}
		fmt.Fprint(out, th.Clear())
		fmt.Fprint(out, th.Banner("antigame — Kişiler"))
		fmt.Fprint(out, renderList(th, entries))

		if n, err := Orphans(dir); err == nil && n > 0 {
			fmt.Fprintf(out, "\n%s\n", th.Warn(fmt.Sprintf(
				"Kimseye ait olmayan %d anahtar dosyası var; silinmedi.", n)))
		}

		fmt.Fprintf(out, "\n  %s Ekle   %s Düzenle   %s Sil   %s Anahtar yenile\n",
			th.Key("[e]"), th.Key("[d]"), th.Key("[s]"), th.Key("[y]"))
		fmt.Fprintf(out, "  %s Geri\n\nSeçiminiz: ", th.Key("[0]"))

		line, err := r.ReadString('\n')
		choice := strings.ToLower(strings.TrimSpace(line))
		if err != nil && choice == "" {
			return nil
		}

		var opErr error
		switch choice {
		case "0", "":
			return nil
		case "e":
			opErr = addFlow(dir, r, out, th)
		case "d":
			opErr = editFlow(dir, r, out, th, entries)
		case "s":
			opErr = removeFlow(dir, r, out, th, entries)
		case "y":
			opErr = rotateFlow(dir, r, out, th, entries)
		default:
			opErr = fmt.Errorf("geçersiz seçim: %q", choice)
		}
		if opErr != nil {
			fmt.Fprintf(out, "\n%s %v\n", th.Bad("hata:"), opErr)
		}
		pause(r, out, th)
	}
}

// renderList, kisi tablosunu dondurur.
func renderList(th *term.Theme, entries []Entry) string {
	if len(entries) == 0 {
		return "\n  " + th.Warn("Kayıtlı kişi yok. [e] ile ekleyin.") + "\n"
	}
	var b strings.Builder
	b.WriteString("\n")
	for i, e := range entries {
		hint := e.Hint
		if hint == "" {
			hint = "—"
		}
		fmt.Fprintf(&b, "  %s %s %s %s\n",
			th.Key(fmt.Sprintf("%d.", i+1)),
			pad(e.Name, nameWidth),
			pad(hint, hintWidth),
			th.Mark(e.Usable())+" "+statusText(th, e))
	}
	return b.String()
}

func statusText(th *term.Theme, e Entry) string {
	switch {
	case e.KeyErr != nil:
		return th.Bad("anahtarı okunamıyor")
	case !e.HasKey:
		return th.Warn("anahtarı yok")
	default:
		return th.Good("anahtar var")
	}
}

// pad, metni sabit genislige getirir. Genislik rune sayisina gore
// olculur: Turkce harfler bayt sayarken sutunlari kaydiriyordu.
func pad(s string, w int) string {
	rs := []rune(s)
	if len(rs) > w {
		return string(rs[:w-1]) + "…"
	}
	return s + strings.Repeat(" ", w-len(rs))
}

func ask(r *bufio.Reader, out io.Writer, q string) (string, error) {
	fmt.Fprint(out, q)
	s, err := r.ReadString('\n')
	return strings.TrimSpace(s), err
}

func pause(r *bufio.Reader, out io.Writer, th *term.Theme) {
	fmt.Fprint(out, "\n"+th.Dim("Devam etmek için Enter..."))
	r.ReadString('\n')
}

// pick, listeden sira numarasiyla kisi sectirir.
func pick(r *bufio.Reader, out io.Writer, entries []Entry, q string) (Entry, error) {
	if len(entries) == 0 {
		return Entry{}, errors.New("kayıtlı kişi yok")
	}
	s, err := ask(r, out, q)
	if err != nil && s == "" {
		return Entry{}, err
	}
	n, convErr := strconv.Atoi(s)
	if convErr != nil || n < 1 || n > len(entries) {
		return Entry{}, fmt.Errorf("geçersiz sıra numarası: %q", s)
	}
	return entries[n-1], nil
}

func addFlow(dir string, r *bufio.Reader, out io.Writer, th *term.Theme) error {
	name, err := ask(r, out, "\nKişinin adı: ")
	if err != nil && name == "" {
		return err
	}
	if name == "" {
		return errors.New("ad boş bırakıldı, ekleme iptal edildi")
	}
	hint, err := ask(r, out, "Ona nasıl ulaşacaksınız (kapıda gösterilir): ")
	if err != nil && hint == "" {
		return err
	}

	secret, counter, err := pairing.Pair(r, out, name, func() error {
		return store.Append(dir, store.Event{TS: time.Now().UTC(), Ev: "pairing_manual"})
	})
	if err != nil {
		return err
	}
	p, err := Add(dir, name, hint, secret, counter)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "\n%s %s artık kapıyı açabilir.\n", th.Good("Eklendi:"), p.Name)
	return nil
}

func editFlow(dir string, r *bufio.Reader, out io.Writer, th *term.Theme, entries []Entry) error {
	e, err := pick(r, out, entries, "\nDüzenlenecek kişinin sıra numarası: ")
	if err != nil {
		return err
	}
	name, err := ask(r, out, fmt.Sprintf("Ad [%s]: ", e.Name))
	if err != nil && name == "" {
		return err
	}
	hint, err := ask(r, out, fmt.Sprintf("İletişim [%s]: ", e.Hint))
	if err != nil && hint == "" {
		return err
	}
	// Bos birakilan alan degistirilmez; kullanici yalnizca birini
	// duzeltmek icin digerini yeniden yazmak zorunda kalmamali.
	if name == "" {
		name = e.Name
	}
	if hint == "" {
		hint = e.Hint
	}
	if err := Edit(dir, e.ID, name, hint); err != nil {
		return err
	}
	fmt.Fprintf(out, "\n%s\n", th.Good("Güncellendi."))
	return nil
}

func removeFlow(dir string, r *bufio.Reader, out io.Writer, th *term.Theme, entries []Entry) error {
	e, err := pick(r, out, entries, "\nSilinecek kişinin sıra numarası: ")
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "\n%s\n", th.Warn(fmt.Sprintf(
		"%s siliniyor. Anahtarı geçersiz olur, geçmiş süresi raporda kalır.", e.Name)))
	a, err := ask(r, out, "Silinsin mi? (e/h): ")
	if err != nil && a == "" {
		return err
	}
	if !strings.EqualFold(a, "e") {
		fmt.Fprintln(out, "\nVazgeçildi.")
		return nil
	}
	if err := Remove(dir, e.ID); err != nil {
		return err
	}
	fmt.Fprintf(out, "\n%s\n", th.Good("Silindi."))
	return nil
}

func rotateFlow(dir string, r *bufio.Reader, out io.Writer, th *term.Theme, entries []Entry) error {
	e, err := pick(r, out, entries, "\nAnahtarı yenilenecek kişinin sıra numarası: ")
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "\n%s\n", th.Warn(fmt.Sprintf(
		"%s için yeni anahtar üretilecek; eski kodları çalışmaz.", e.Name)))

	secret, counter, err := pairing.Pair(r, out, e.Name, func() error {
		return store.Append(dir, store.Event{TS: time.Now().UTC(), Ev: "pairing_manual", Who: e.ID})
	})
	if err != nil {
		return err
	}
	if err := Rotate(dir, e.ID, secret, counter); err != nil {
		return err
	}
	fmt.Fprintf(out, "\n%s\n", th.Good("Anahtar yenilendi."))
	return nil
}

// Summary, menu basliginda gosterilecek tek satirlik ozettir.
func Summary(dir string) string {
	entries, err := List(dir)
	if err != nil {
		return "Kişiler: okunamadı"
	}
	if len(entries) == 0 {
		return "Kişiler: yok — kapı devre dışı"
	}
	var names []string
	usable := 0
	for _, e := range entries {
		if e.Usable() {
			usable++
		}
		names = append(names, e.Name)
	}
	if usable == 0 {
		return "Kişiler: " + strings.Join(names, ", ") + " — hiçbirinin anahtarı çalışmıyor"
	}
	return fmt.Sprintf("Kişiler: %s (%d kişi kapıyı açabiliyor)", strings.Join(names, ", "), usable)
}
