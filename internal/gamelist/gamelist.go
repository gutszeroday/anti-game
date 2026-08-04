// Package gamelist, kapida durdurulacak oyun listesini yonetir.
package gamelist

import (
	"fmt"
	"io"
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
		if g.Path != "" {
			fmt.Fprintf(&b, "  (yol sabit: %s)", g.Path)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func Add(dir, name, exe, path string) error {
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
	c.Gated = append(c.Gated, config.Game{Name: name, Exe: exe, Path: strings.TrimSpace(path)})
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
