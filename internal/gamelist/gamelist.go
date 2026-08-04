// Package gamelist, kapida durdurulacak oyun listesini yonetir.
package gamelist

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/guts/antigame/internal/config"
)

func Format(c *config.Config) string {
	if len(c.Gated) == 0 {
		return "Liste boş — hiçbir oyun kapıda durdurulmayacak, yalnızca süre kaydedilecek.\n"
	}
	var b strings.Builder
	b.WriteString("Kapıda durdurulan oyunlar:\n")
	for i, g := range c.Gated {
		fmt.Fprintf(&b, "  %2d. %-28s %s", i+1, g.Name, g.Exe)
		if g.Launcher {
			b.WriteString("  (başlatıcı)")
		}
		if g.Path != "" {
			fmt.Fprintf(&b, "  (yol sabit: %s)", g.Path)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func Add(dir, name, exe, path string) error {
	return add(dir, name, exe, path, false)
}

func add(dir, name, exe, path string, launcher bool) error {
	exe = strings.TrimSpace(exe)
	if exe == "" {
		return fmt.Errorf("exe adı boş olamaz")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = exe
	}
	c, err := config.Load(dir)
	if err != nil {
		return err
	}
	for _, g := range c.Gated {
		if strings.EqualFold(g.Exe, exe) {
			return fmt.Errorf("%s zaten listede (%s)", exe, g.Name)
		}
	}
	c.Gated = append(c.Gated, config.Game{
		Name:     name,
		Exe:      exe,
		Path:     strings.TrimSpace(path),
		Launcher: launcher,
	})
	return config.Save(dir, c)
}

func Remove(dir, exe string) error {
	c, err := config.Load(dir)
	if err != nil {
		return err
	}
	for i, g := range c.Gated {
		if strings.EqualFold(g.Exe, exe) {
			c.Gated = append(c.Gated[:i], c.Gated[i+1:]...)
			return config.Save(dir, c)
		}
	}
	return fmt.Errorf("%s listede değil", exe)
}

// AddInteractive, menuden oyun eklemeyi yurutur. running, o anda calisan
// program adlaridir: kullanicinin exe adini bilmesi gerekmesin diye
// secenek olarak sunuluyor.
func AddInteractive(dir string, in io.Reader, out io.Writer, running []string) error {
	c, err := config.Load(dir)
	if err != nil {
		return err
	}
	r := bufio.NewReader(in)

	// Zaten listede olanlari secenek olarak gostermek kafa karistirir.
	var options []string
	for _, exe := range running {
		if _, gated := c.Match(exe, ""); !gated {
			options = append(options, exe)
		}
	}
	sort.Slice(options, func(i, j int) bool {
		return strings.ToLower(options[i]) < strings.ToLower(options[j])
	})

	if len(options) > 0 {
		fmt.Fprintln(out, "\nŞu anda çalışan programlar:")
		for i, exe := range options {
			fmt.Fprintf(out, "  %2d) %s\n", i+1, exe)
		}
		fmt.Fprint(out, "\nSıra numarası girin veya exe adını yazın: ")
	} else {
		fmt.Fprint(out, "\nEklenecek exe adı (ör. Palworld.exe): ")
	}

	choice, _ := r.ReadString('\n')
	choice = strings.TrimSpace(choice)
	if choice == "" {
		return errors.New("seçim yapılmadı")
	}

	exe := choice
	if n, err := strconv.Atoi(choice); err == nil {
		if n < 1 || n > len(options) {
			return fmt.Errorf("%d listede yok", n)
		}
		exe = options[n-1]
	}

	suggested := strings.TrimSuffix(exe, filepath.Ext(exe))
	fmt.Fprintf(out, "Görünen ad [%s]: ", suggested)
	name, _ := r.ReadString('\n')
	name = strings.TrimSpace(name)
	if name == "" {
		name = suggested
	}

	fmt.Fprint(out, "Bu bir başlatıcı mı (oyunun kendisi değil, ör. Steam)? (e/h): ")
	ans, _ := r.ReadString('\n')
	launcher := strings.EqualFold(strings.TrimSpace(ans), "e")

	if err := add(dir, name, exe, "", launcher); err != nil {
		return err
	}
	fmt.Fprintf(out, "\n%s listeye eklendi.\n", name)
	return nil
}

// RemoveInteractive, menuden oyun cikarmayi yurutur.
func RemoveInteractive(dir string, in io.Reader, out io.Writer) error {
	c, err := config.Load(dir)
	if err != nil {
		return err
	}
	if len(c.Gated) == 0 {
		return errors.New("liste zaten boş")
	}
	fmt.Fprint(out, "\n"+Format(c))
	fmt.Fprint(out, "\nÇıkarılacak oyunun sıra numarası: ")

	line, _ := bufio.NewReader(in).ReadString('\n')
	n, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		return errors.New("sıra numarası girilmedi")
	}
	if n < 1 || n > len(c.Gated) {
		return fmt.Errorf("%d listede yok", n)
	}
	name := c.Gated[n-1].Name
	c.Gated = append(c.Gated[:n-1], c.Gated[n:]...)
	if err := config.Save(dir, c); err != nil {
		return err
	}
	fmt.Fprintf(out, "\n%s listeden çıkarıldı.\n", name)
	return nil
}

// Run, `antigame list` alt komutunu isler.
func Run(dir string, args []string, out io.Writer) error {
	if len(args) == 0 {
		c, err := config.Load(dir)
		if err != nil {
			return err
		}
		fmt.Fprint(out, Format(c))
		return nil
	}
	switch args[0] {
	case "add":
		if len(args) < 3 {
			return fmt.Errorf("kullanım: antigame list add <exe> <görünen ad> [tam yol]")
		}
		path := ""
		if len(args) >= 4 {
			path = args[3]
		}
		if err := Add(dir, args[2], args[1], path); err != nil {
			return err
		}
		fmt.Fprintf(out, "%s eklendi.\n", args[1])
		return nil
	case "remove":
		if len(args) < 2 {
			return fmt.Errorf("kullanım: antigame list remove <exe>")
		}
		if err := Remove(dir, args[1]); err != nil {
			return err
		}
		fmt.Fprintf(out, "%s çıkarıldı.\n", args[1])
		return nil
	default:
		return fmt.Errorf("bilinmeyen alt komut: %s", args[0])
	}
}
