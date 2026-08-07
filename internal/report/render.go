package report

import (
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/windows"

	"github.com/guts/antigame/internal/config"
	"github.com/guts/antigame/internal/store"
)

func hm(seconds int) string {
	h := seconds / 3600
	m := (seconds % 3600) / 60
	if h == 0 {
		return fmt.Sprintf("%d dk", m)
	}
	return fmt.Sprintf("%d sa %d dk", h, m)
}

// bars, etiketli degerlerden yatayda esit aralikli bir SVG cubuk grafik uretir.
// Harici bir grafik kutuphanesi yok: rapor tek dosya olarak calismali.
func bars(labels []string, values []int, w, h int) string {
	if len(values) == 0 {
		return `<p class="empty">Veri yok.</p>`
	}
	maxV := 1
	for _, v := range values {
		maxV = max(maxV, v)
	}
	barW := float64(w) / float64(len(values))
	var b strings.Builder
	fmt.Fprintf(&b, `<svg viewBox="0 0 %d %d" class="chart" role="img">`, w, h+28)
	for i, v := range values {
		bh := float64(h) * float64(v) / float64(maxV)
		x := float64(i) * barW
		fmt.Fprintf(&b,
			`<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="3" class="bar"><title>%s: %s</title></rect>`,
			x+2, float64(h)-bh, barW-4, bh, html.EscapeString(labels[i]), hm(v))
		fmt.Fprintf(&b, `<text x="%.1f" y="%d" class="lbl">%s</text>`,
			x+barW/2, h+18, html.EscapeString(labels[i]))
	}
	b.WriteString(`</svg>`)
	return b.String()
}

func Render(s Summary) []byte {
	var b strings.Builder

	b.WriteString(`<!doctype html><html lang="tr"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>anti-game — haftalık rapor</title><style>
:root{color-scheme:light dark}
body{font-family:Segoe UI,system-ui,sans-serif;margin:0;padding:2rem;
background:#0f1115;color:#e8e8ea;line-height:1.5}
main{max-width:60rem;margin:0 auto}
h1{font-size:1.6rem;margin:0 0 .25rem}
.sub{color:#9aa0aa;margin:0 0 2rem}
section{margin:2.5rem 0}
h2{font-size:1.05rem;text-transform:uppercase;letter-spacing:.06em;
color:#9aa0aa;border-bottom:1px solid #262a33;padding-bottom:.5rem}
.total{font-size:2.6rem;font-weight:600}
.delta{font-size:1rem;color:#9aa0aa;margin-left:.5rem}
.up{color:#ff6b6b}.down{color:#51cf66}
table{width:100%;border-collapse:collapse}
td,th{text-align:left;padding:.5rem 0;border-bottom:1px solid #1c2029}
th{color:#9aa0aa;font-weight:500}
td.num{text-align:right;font-variant-numeric:tabular-nums}
.chart{width:100%;height:auto}
.bar{fill:#5b8def}
.lbl{fill:#9aa0aa;font-size:11px;text-anchor:middle}
.empty{color:#6b7280}
.warn{background:#2a1f1f;border-left:3px solid #ff6b6b;padding:.75rem 1rem;margin:.5rem 0}
code{background:#1c2029;padding:.15rem .4rem;border-radius:4px}
</style></head><body><main>`)

	fmt.Fprintf(&b, `<h1>Haftalık oyun raporu</h1><p class="sub">%s – %s</p>`,
		s.From.Format("2 Ocak 2006"), s.To.AddDate(0, 0, -1).Format("2 Ocak 2006"))

	b.WriteString(`<section><h2>Bu hafta</h2>`)
	fmt.Fprintf(&b, `<div class="total">%s`, hm(s.TotalS))
	if s.PrevTotalS > 0 {
		diff := s.TotalS - s.PrevTotalS
		cls, sign := "down", "−"
		if diff > 0 {
			cls, sign = "up", "+"
		}
		if diff < 0 {
			diff = -diff
		}
		fmt.Fprintf(&b, `<span class="delta %s">%s%s geçen haftaya göre</span>`, cls, sign, hm(diff))
	}
	b.WriteString(`</div></section>`)

	b.WriteString(`<section><h2>Oyun bazında</h2>`)
	if len(s.Games) == 0 {
		b.WriteString(`<p class="empty">Bu hafta kayıtlı oyun süresi yok.</p>`)
	} else {
		b.WriteString(`<table><tr><th>Oyun</th><th class="num">Toplam</th><th class="num">Aktif</th></tr>`)
		for _, g := range s.Games {
			fmt.Fprintf(&b, `<tr><td>%s</td><td class="num">%s</td><td class="num">%s</td></tr>`,
				html.EscapeString(g.Name), hm(g.DurS), hm(g.ActiveS))
		}
		b.WriteString(`</table>`)
	}
	b.WriteString(`</section>`)

	b.WriteString(`<section><h2>Kim ile ne kadar</h2>`)
	if len(s.People) == 0 {
		b.WriteString(`<p class="empty">Bu hafta kapı açılmadı.</p>`)
	} else {
		b.WriteString(`<table><tr><th>Kişi</th><th class="num">Süre</th><th class="num">Kapıyı açma</th></tr>`)
		for _, p := range s.People {
			fmt.Fprintf(&b, `<tr><td>%s</td><td class="num">%s</td><td class="num">%d</td></tr>`,
				html.EscapeString(p.Name), hm(p.DurS), p.Unlocks)
		}
		b.WriteString(`</table>`)
	}
	b.WriteString(`</section>`)

	dayLabels := make([]string, 0, len(s.Days))
	dayValues := make([]int, 0, len(s.Days))
	for _, d := range s.Days {
		dayLabels = append(dayLabels, d.Day.Format("02.01"))
		dayValues = append(dayValues, d.DurS)
	}
	fmt.Fprintf(&b, `<section><h2>Günlük dağılım</h2>%s</section>`, bars(dayLabels, dayValues, 720, 180))

	hourLabels := make([]string, 24)
	hourValues := make([]int, 24)
	for i := range 24 {
		hourLabels[i] = fmt.Sprintf("%02d", i)
		hourValues[i] = s.Hours[i]
	}
	fmt.Fprintf(&b, `<section><h2>Gün içi saat dağılımı</h2>%s</section>`, bars(hourLabels, hourValues, 720, 180))

	weekLabels := make([]string, 0, len(s.Weeks))
	weekValues := make([]int, 0, len(s.Weeks))
	for _, w := range s.Weeks {
		weekLabels = append(weekLabels, w.Start.Format("02.01"))
		weekValues = append(weekValues, w.DurS)
	}
	fmt.Fprintf(&b, `<section><h2>Son 4 hafta</h2>%s</section>`, bars(weekLabels, weekValues, 720, 160))

	b.WriteString(`<section><h2>İzleyicinin kapalı olduğu aralıklar</h2>`)
	if len(s.Gaps) == 0 {
		b.WriteString(`<p class="empty">Bu hafta boşluk yok — izleyici kesintisiz çalıştı.</p>`)
	} else {
		for _, g := range s.Gaps {
			fmt.Fprintf(&b, `<div class="warn">%s – %s (%s)</div>`,
				g.From.Local().Format("02.01 15:04"), g.To.Local().Format("02.01 15:04"),
				hm(int(g.To.Sub(g.From).Seconds())))
		}
	}
	b.WriteString(`</section>`)

	b.WriteString(`<section><h2>Listeye eklemek ister misiniz?</h2>`)
	if len(s.Suggestions) == 0 {
		b.WriteString(`<p class="empty">Listede olmayıp dikkat çeken bir uygulama yok.</p>`)
	} else {
		b.WriteString(`<table><tr><th>Uygulama</th><th class="num">Bu hafta</th><th>Eklemek için</th></tr>`)
		for _, sg := range s.Suggestions {
			fmt.Fprintf(&b,
				`<tr><td>%s</td><td class="num">%s</td><td><code>antigame list add %s "%s"</code></td></tr>`,
				html.EscapeString(sg.Exe), hm(sg.DurS),
				html.EscapeString(sg.Exe), html.EscapeString(strings.TrimSuffix(sg.Exe, ".exe")))
		}
		b.WriteString(`</table>`)
	}
	b.WriteString(`</section></main></body></html>`)

	return []byte(b.String())
}

// Run, raporu uretip tarayicida acar ve dosya yolunu dondurur.
func Run(dir string) (string, error) {
	cfg, err := config.Load(dir)
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	ev, err := store.Read(dir, now.AddDate(0, 0, -35), now)
	if err != nil {
		return "", err
	}
	out := filepath.Join(os.TempDir(), "antigame-report.html")
	if err := os.WriteFile(out, Render(Aggregate(ev, cfg, now, time.Local)), 0o600); err != nil {
		return "", err
	}
	if err := open(out).Start(); err != nil {
		return out, nil // tarayici acilamadi ama dosya hazir
	}
	return out, nil
}

// open, dosyayi varsayilan uygulamada acan komutu kurar.
//
// CREATE_NO_WINDOW sart: arayuz -H=windowsgui ile derlendigi icin
// process'in konsolu yok ve cmd bir konsol uygulamasi. Bayrak
// verilmezse raporu acmak ekranda bir an siyah pencere parlatiyor.
func open(path string) *exec.Cmd {
	c := exec.Command("cmd", "/c", "start", "", path)
	c.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW,
	}
	return c
}
