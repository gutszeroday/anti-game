// Package menu, cift tiklayarak calistiranlar icin basit bir konsol
// menusudur. Argumansiz baslatilan binary kullanim metnini yazip hemen
// kapandigi icin ekranda hicbir sey gorunmuyordu.
package menu

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

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
	for {
		fmt.Fprintln(out, "\nantigame — oyun süresi takibi ve MFA kapısı")
		if header != nil {
			fmt.Fprintln(out)
			fmt.Fprint(out, header())
		}
		fmt.Fprintln(out)
		for _, it := range items {
			fmt.Fprintf(out, "  %s) %s\n", it.Key, it.Label)
		}
		fmt.Fprintln(out, "  0) Çıkış")
		fmt.Fprint(out, "\nSeçiminiz: ")

		line, err := r.ReadString('\n')
		choice := strings.TrimSpace(line)
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
			fmt.Fprintf(out, "\nGeçersiz seçim: %q\n", choice)
			continue
		}
		if err := hit.Run(); err != nil {
			fmt.Fprintf(out, "\nhata: %v\n", err)
		}
	}
}
