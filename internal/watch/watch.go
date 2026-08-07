// Package watch, izleyici dongusudur: kapidaki oyunlari durdurur,
// oturum acikken sureyi tutar ve odak/AFK orneklemesi yapar.
//
// Dongu Step olarak disari aciliyor; Run yalnizca Step'i saat uzerinden
// cagiran ince bir sarmalayici. Boylece testler zamani elle surebiliyor.
package watch

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/guts/antigame/internal/config"
	"github.com/guts/antigame/internal/session"
	"github.com/guts/antigame/internal/store"
	"github.com/guts/antigame/internal/winproc"
)

const (
	heartbeatEvery  = time.Minute
	stateSaveEvery  = 30 * time.Second
	usageFlushEvery = time.Minute
	gateCooldown    = 5 * time.Second
	// hbEventEvery, gunluge yazilan nabiz araligidir. state.json'daki nabiz
	// her yazimda uzerine biner, yani gecmisi yoktur; raporun "izleyici
	// kapaliydi" araliklarini bulabilmesi icin gunluge de iz gerekiyor.
	// Gunde ~144 satir, ihmal edilebilir.
	hbEventEvery = 10 * time.Minute
	// configCheckEvery, config.json'un degisip degismedigine bakma araligidir.
	// Her turda (250 ms) stat cagirmak gereksiz; listeye eklenen bir oyunun
	// bes saniye icinde kapiya girmesi yeterince hizli.
	configCheckEvery = 5 * time.Second
)

type Options struct {
	// Dir, veri klasorudur. Acilistaki degeri; klasor sonradan
	// degisebilir, o yuzden kod icinde w.dir() kullanilir.
	Dir string
	// DirFunc, guncel veri klasorunu verir. Nil birakilirsa Dir sabit
	// kabul edilir. Kullanici klasoru tasidiginda izleyicinin yeni yere
	// gecmesi buna bagli.
	DirFunc       func() string
	Cfg           *config.Config
	List          func() ([]winproc.Proc, error)
	Path          func(int) (string, error)
	Terminate     func(int) error
	Trim          func() error
	Idle          func() (int, error)
	ForegroundPID func() (int, error)
	SpawnGate     func(appName string) error
	// SecretReady, MFA eslestirmesinin yapilip yapilmadigini soyler. Nil
	// birakilirsa yapilmis sayilir.
	SecretReady func() bool
}

// tracked, calismakta olan bir kapidaki oyunun sayaclaridir.
type tracked struct {
	exe     string
	name    string
	start   time.Time
	activeS int
	// who, oyun basladiginda oturumu acmis olan kisidir. Oyun bittiginde
	// bakilmaz: oturum o ana kadar dusmus olabilir ve sure sahipsiz
	// kalirdi.
	who string
}

type Watcher struct {
	o              Options
	grace          time.Duration
	launcherWindow time.Duration
	pinned         map[string]bool // yol sabitlemesi olan exe adlari

	st             *store.State
	running        map[int]*tracked
	started        bool
	warnedNoSecret bool

	lastSample    time.Time
	lastHeartbeat time.Time
	lastStateSave time.Time
	lastGate      time.Time
	lastHBEvent   time.Time

	lastCfgCheck time.Time
	cfgMod       time.Time
	cfgSize      int64

	// cur, izleyicinin su an kullandigi veri klasorudur. Bir turun
	// ortasinda degismemeli: yarisi eski yarisi yeni klasore yazilan bir
	// tur, oturumu iki dosyaya bolerdi.
	cur string

	usageExe    string
	usageDurS   int
	usageActive int
	usageSince  time.Time
}

func New(o Options) (*Watcher, error) {
	st, err := store.LoadState(o.Dir)
	if err != nil {
		return nil, err
	}
	w := &Watcher{
		o:       o,
		cur:     o.Dir,
		st:      st,
		running: make(map[int]*tracked),
	}
	w.applyConfig(o.Cfg)
	return w, nil
}

// applyConfig, yapilandirmayi ve ondan turetilen alanlari birlikte kurar.
// Turetilenler ayri kalirsa yeniden yuklemede biri guncellenip digeri eski
// degeriyle kalabilir.
func (w *Watcher) applyConfig(cfg *config.Config) {
	// Tam yol yalnizca sabitleme yapilmis exe adlari icin sorgulanir;
	// aksi halde her turda her process icin OpenProcess cagrilirdi.
	pinned := make(map[string]bool)
	for _, g := range cfg.Gated {
		if g.Path != "" {
			pinned[strings.ToLower(g.Exe)] = true
		}
	}
	// Eski config.json'da bu alan yok; sifir birakilirsa oturum hic
	// acilamazdi.
	launcherWindow := time.Duration(cfg.LauncherWindowMinutes) * time.Minute
	if launcherWindow <= 0 {
		launcherWindow = 45 * time.Minute
	}
	w.o.Cfg = cfg
	w.pinned = pinned
	w.grace = time.Duration(cfg.GraceMinutes) * time.Minute
	w.launcherWindow = launcherWindow
}

// dir, izleyicinin su an kullandigi veri klasorunu verir.
func (w *Watcher) dir() string { return w.cur }

// reloadDir, veri klasoru degistiyse izleyiciyi yeni yere tasir.
//
// Klasoru arayuz degistirebiliyor ve arayuz ayri bir process. Izleyici
// klasoru yalnizca acilista okursa: kapi yeni klasore oturum acar,
// izleyici eskiye bakmaya devam eder, oturumu hic gormez ve oyunu
// oldurmeyi surdurur.
//
// Klasorun yok olmasi da buraya dusuyor. Once store.Append hata verip
// Run'i dondururdu, yani koruma tamamen dururdu; artik ayar yeniden
// okunup yeni yere geciliyor.
func (w *Watcher) reloadDir(now time.Time) error {
	if w.o.DirFunc == nil {
		return nil
	}
	next := w.o.DirFunc()
	if next == "" || next == w.cur {
		return nil
	}

	// Hedefin gercekten var olan bir dizin oldugu once dogrulaniyor.
	// LoadState ve config.Load eksik dosyayi hata saymiyor (ilk
	// calistirmada kurulum gerekmesin diye); tek baslarina gecerli
	// olmayan bir yola gecmeyi engellemiyorlar.
	if fi, err := os.Stat(next); err != nil || !fi.IsDir() {
		return nil
	}

	// Yeni klasordeki durum ve liste okunuyor. Okunamiyorsa gecis
	// yapilmiyor: yarim kurulmus bir hedefe gecmek, acik oturumu
	// gorunmez kilardi.
	st, err := store.LoadState(next)
	if err != nil {
		return nil
	}
	cfg, err := config.Load(next)
	if err != nil {
		return nil
	}

	old := w.cur
	w.cur = next
	w.st = st
	w.applyConfig(cfg)
	// Bir sonraki reloadConfig dosyayi taze saysin diye izler sifirlaniyor.
	w.cfgMod, w.cfgSize = time.Time{}, 0

	return store.Append(next, store.Event{
		TS: now, Ev: "data_dir_changed", From: old, To: next,
	})
}

// reloadConfig, config.json degistiyse listeyi yeniden okur.
//
// Listeyi menu ve tepsi degistirir, ama onlar ayri process. Izleyici ayari
// yalnizca acilista okursa sonradan eklenen oyun ancak izleyici yeniden
// baslatilinca kapida durur; kullanici listede gordugu oyunu korumasiz
// oynar. Degisiklik boyut+mtime ile anlasiliyor, icerik her turda okunmuyor.
//
// PollMS burada etkisini gostermez: dongu saati Run icinde bir kez kurulur.
func (w *Watcher) reloadConfig(now time.Time) error {
	if !w.lastCfgCheck.IsZero() && now.Sub(w.lastCfgCheck) < configCheckEvery {
		return nil
	}
	w.lastCfgCheck = now

	fi, err := os.Stat(config.FilePath(w.dir()))
	if err != nil {
		// Dosya yoksa veya okunamiyorsa bellektekini korumak tek dogru
		// davranis: varsayilana donmek kullanicinin listesini sessizce
		// degistirirdi.
		return nil
	}
	if fi.ModTime().Equal(w.cfgMod) && fi.Size() == w.cfgSize {
		return nil
	}
	cfg, err := config.Load(w.dir())
	if err != nil {
		// Yarim yazilmis dosya bir sonraki turda tekrar denenir.
		return nil
	}
	first := w.cfgMod.IsZero()
	w.cfgMod, w.cfgSize = fi.ModTime(), fi.Size()
	w.applyConfig(cfg)
	if first {
		// Acilistaki ilk okuma degisiklik degil.
		return nil
	}
	return store.Append(w.dir(), store.Event{TS: now, Ev: "config_reload"})
}

// Reload, durumu diskten yeniden okur. Kapi penceresi ayri bir process
// oldugu icin oturumu o acar; izleyicinin degisikligi gormesi gerekir.
func (w *Watcher) Reload() error {
	st, err := store.LoadState(w.dir())
	if err != nil {
		return err
	}
	w.st = st
	return nil
}

func (w *Watcher) Step(now time.Time) error {
	// watch_start, New icinde degil ilk turda yazilir: boylece olayin zaman
	// damgasi cagiranin saatinden gelir ve testler gercek zamana bagli olmaz.
	if !w.started {
		w.started = true
		if err := store.Append(w.dir(), store.Event{TS: now, Ev: "watch_start"}); err != nil {
			return err
		}
	}

	// Veri klasoru degismis olabilir; her seyden once ona bakilir,
	// yoksa bu turun yazilari eski klasore giderdi.
	if err := w.reloadDir(now); err != nil {
		return err
	}

	// Liste process disinda degismis olabilir; taramadan once tazelenir.
	if err := w.reloadConfig(now); err != nil {
		return err
	}

	procs, err := w.o.List()
	if err != nil {
		return err
	}

	// Kapi baska bir process'te oturum acmis olabilir; oturum kapaliyken
	// her turda diski okumak yerine yalnizca gerektiginde okuruz.
	if !session.Active(w.st, now, w.grace, w.launcherWindow) {
		if err := w.Reload(); err != nil {
			return err
		}
	}

	// Eslestirme yoksa kapi penceresi acilamaz. Oyunu yine de oldurmek
	// kullaniciyi kod girecek yeri olmadan tamamen kilitlerdi; bu yuzden
	// kapi devre disi kaliyor, sure tutulmaya devam ediyor ve durum bir
	// kez gunluge yaziliyor.
	gating := w.o.SecretReady == nil || w.o.SecretReady()
	if !gating && !w.warnedNoSecret {
		w.warnedNoSecret = true
		if err := store.Append(w.dir(), store.Event{TS: now, Ev: "gate_disabled"}); err != nil {
			return err
		}
	}

	active := !gating || session.Active(w.st, now, w.grace, w.launcherWindow)
	seen := make(map[int]bool, len(w.running))
	byPID := make(map[int]string, len(procs))

	for _, p := range procs {
		byPID[p.PID] = p.Exe

		path := ""
		if w.pinned[strings.ToLower(p.Exe)] {
			path, _ = w.o.Path(p.PID)
		}
		g, ok := w.o.Cfg.Match(p.Exe, path)
		if !ok {
			continue
		}

		if !active {
			if err := w.block(now, p, g.Name); err != nil {
				return err
			}
			continue
		}

		seen[p.PID] = true
		if _, known := w.running[p.PID]; !known {
			who := w.sessionOwner()
			w.running[p.PID] = &tracked{exe: p.Exe, name: g.Name, start: now, who: who}
			if err := store.Append(w.dir(), store.Event{
				TS: now, Ev: "game_start", Exe: p.Exe, Name: g.Name, PID: p.PID, Who: who,
			}); err != nil {
				return err
			}
		}
		// Baslatici oturumu tazeler ama baslatici penceresini uzatmaz.
		session.Touch(w.st, now, !g.Launcher)
	}

	if err := w.reapExited(now, seen); err != nil {
		return err
	}
	if err := w.sample(now, byPID); err != nil {
		return err
	}
	if err := w.persist(now); err != nil {
		return err
	}
	if w.o.Trim != nil {
		_ = w.o.Trim()
	}
	return nil
}

func (w *Watcher) block(now time.Time, p winproc.Proc, name string) error {
	if err := w.o.Terminate(p.PID); err != nil {
		// Process kendiliginden kapanmis olabilir; olayi yine de yazmayiz.
		return nil
	}
	if err := store.Append(w.dir(), store.Event{
		TS: now, Ev: "blocked", Exe: p.Exe, Name: name, PID: p.PID,
	}); err != nil {
		return err
	}
	// Oyunu ust uste baslatma denemeleri tek pencere uretmeli.
	if w.o.SpawnGate != nil && now.Sub(w.lastGate) >= gateCooldown {
		w.lastGate = now
		_ = w.o.SpawnGate(name)
	}
	return nil
}

// sessionOwner, acik oturumu acmis kisinin ID'sini dondurur. Oturum yoksa
// (kapi kurulmadan tutulan sureler) bos doner.
func (w *Watcher) sessionOwner() string {
	if w.st == nil || w.st.Session == nil {
		return ""
	}
	return w.st.Session.OpenedBy
}

func (w *Watcher) reapExited(now time.Time, seen map[int]bool) error {
	for pid, tr := range w.running {
		if seen[pid] {
			continue
		}
		delete(w.running, pid)
		if err := store.Append(w.dir(), store.Event{
			TS:      now,
			Ev:      "game_end",
			Exe:     tr.exe,
			Name:    tr.name,
			PID:     pid,
			DurS:    int(now.Sub(tr.start).Seconds()),
			ActiveS: tr.activeS,
			Who:     tr.who,
		}); err != nil {
			return err
		}
	}
	return nil
}

// sample, odak ve AFK orneklemesini yapar.
func (w *Watcher) sample(now time.Time, byPID map[int]string) error {
	every := time.Duration(w.o.Cfg.FocusSampleS) * time.Second
	if !w.lastSample.IsZero() && now.Sub(w.lastSample) < every {
		return nil
	}
	w.lastSample = now
	step := w.o.Cfg.FocusSampleS

	idle, err := w.o.Idle()
	if err != nil {
		return err
	}
	isActive := idle < w.o.Cfg.IdleThresholdS

	for _, tr := range w.running {
		if isActive {
			tr.activeS += step
		}
	}

	pid, err := w.o.ForegroundPID()
	if err != nil {
		return err
	}
	exe := byPID[pid]
	// Kapidaki oyunlar zaten game_start/game_end ile olculuyor; burada
	// tekrar sayilmamalari gerekir.
	if exe == "" {
		return nil
	}
	if _, gated := w.o.Cfg.Match(exe, ""); gated {
		return nil
	}

	if w.usageExe != "" && !strings.EqualFold(exe, w.usageExe) {
		if err := w.flushUsage(now); err != nil {
			return err
		}
	}
	if w.usageExe == "" {
		w.usageExe, w.usageSince = exe, now
	}
	w.usageDurS += step
	if isActive {
		w.usageActive += step
	}
	if now.Sub(w.usageSince) >= usageFlushEvery {
		return w.flushUsage(now)
	}
	return nil
}

func (w *Watcher) flushUsage(now time.Time) error {
	if w.usageExe == "" || w.usageDurS == 0 {
		w.usageExe, w.usageDurS, w.usageActive = "", 0, 0
		return nil
	}
	err := store.Append(w.dir(), store.Event{
		TS: now, Ev: "usage", Exe: w.usageExe,
		DurS: w.usageDurS, ActiveS: w.usageActive,
	})
	w.usageExe, w.usageDurS, w.usageActive, w.usageSince = "", 0, 0, now
	return err
}

// persist, durumu belirli araliklarla diske yazar. Her turda yazmak
// gereksiz disk trafigi uretir; odemesiz sure hesabinda 30 saniyelik
// gecikme onemsizdir.
func (w *Watcher) persist(now time.Time) error {
	if now.Sub(w.lastHBEvent) >= hbEventEvery {
		w.lastHBEvent = now
		if err := store.Append(w.dir(), store.Event{TS: now, Ev: "hb"}); err != nil {
			return err
		}
	}
	if now.Sub(w.lastHeartbeat) >= heartbeatEvery {
		w.st.Heartbeat = now
		w.lastHeartbeat = now
		w.lastStateSave = now
		return store.SaveState(w.dir(), w.st)
	}
	if now.Sub(w.lastStateSave) >= stateSaveEvery {
		w.lastStateSave = now
		return store.SaveState(w.dir(), w.st)
	}
	return nil
}

// Shutdown, bekleyen sayaclari yazar ve kapanisi kaydeder.
func (w *Watcher) Shutdown(now time.Time) error {
	if err := w.flushUsage(now); err != nil {
		return err
	}
	if err := w.reapExited(now, map[int]bool{}); err != nil {
		return err
	}
	w.st.Heartbeat = now
	if err := store.SaveState(w.dir(), w.st); err != nil {
		return err
	}
	return store.Append(w.dir(), store.Event{TS: now, Ev: "watch_stop"})
}

func (w *Watcher) Run(ctx context.Context) error {
	t := time.NewTicker(time.Duration(w.o.Cfg.PollMS) * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return w.Shutdown(time.Now().UTC())
		case <-t.C:
			if err := w.Step(time.Now().UTC()); err != nil {
				return err
			}
		}
	}
}
