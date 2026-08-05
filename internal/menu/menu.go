// Package menu, cift tiklayarak calistiranlar icin basit bir konsol
// menusudur. Argumansiz baslatilan binary kullanim metnini yazip hemen
// kapandigi icin ekranda hicbir sey gorunmuyordu.
package menu

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/guts/antigame/internal/term"
)

// clean, okunan satiri secimle karsilastirilabilir hale getirir.
//
// Bosluk disinda BOM da atiliyor: girdi borudan geldiginde (Windows'ta
// yonlendirilmis metin sik sik BOM'la baslar) ilk secim her zaman
// "gecersiz secim" olarak reddediliyordu.
func clean(line string) string {
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "\ufeff"))
}

type Item struct {
	Key   string
	Label string
	Run   func() error
}

// Run, secim yapilana kadar menuyu cizer. Eylem hatalari ekrana yazilir
// ama donguyu bitirmez: pencere kapaninca kullanici hatayi okuyamaz.
//
// header her cizimde yeniden cagrilir; menuyu acik birakan kullanici
// durumun degistigini gorebilsin diye.
func Run(in io.Reader, out io.Writer, header func() string, items []Item) error {
	r := bufio.NewReader(in)
	th := term.New(out)
	for {
		// Her cizimde ekran temizleniyor: menu alt alta birikince
		// durum satirlari ekranin disina kayiyordu.
		fmt.Fprint(out, th.Clear())
		fmt.Fprint(out, th.Banner("antigame — oyun süresi takibi ve MFA kapısı"))
		if header != nil {
			fmt.Fprintln(out)
			fmt.Fprint(out, header())
		}
		fmt.Fprintln(out)
		for _, it := range items {
			fmt.Fprintf(out, "  %s %s\n", th.Key(it.Key+")"), it.Label)
		}
		fmt.Fprintf(out, "  %s Çıkış\n", th.Key("0)"))
		fmt.Fprint(out, "\nSeçiminiz: ")

		line, err := r.ReadString('\n')
		choice := clean(line)
		// Girdi tukendiginde (borudan calistirma, kapanan konsol) temiz cik.
		if err != nil && choice == "" {
			return nil
		}
		if choice == "0" {
			return nil
		}

		var hit *Item
		for i := range items {
			if strings.EqualFold(items[i].Key, choice) {
				hit = &items[i]
				break
			}
		}
		if hit == nil {
			fmt.Fprintf(out, "\n%s %q\n", th.Bad("Geçersiz seçim:"), choice)
			continue
		}
		if err := hit.Run(); err != nil {
			fmt.Fprintf(out, "\n%s %v\n", th.Bad("hata:"), err)
		}
	}
}
