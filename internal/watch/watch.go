// Package watch, izleyici dongusudur: kapidaki oyunlari durdurur,
// oturum acikken sureyi tutar ve odak/AFK orneklemesi yapar.
//
// Dongu Step olarak disari aciliyor; Run yalnizca Step'i saat uzerinden
// cagiran ince bir sarmalayici. Boylece testler zamani elle surebiliyor.
package watch

import (
	"context"
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
)

type Options struct {
	Dir           string
	Cfg           *config.Config
	List          func() ([]winproc.Proc, error)
	Path          func(int) (string, error)
	Terminate     func(int) error
	Trim          func() error
	Idle          func() (int, error)
	ForegroundPID func() (int, error)
	SpawnGate     func(appName string) error
}

// tracked, calismakta olan bir kapidaki oyunun sayaclaridir.
type tracked struct {
	exe     string
	name    string
	start   time.Time
	activeS int
}

type Watcher struct {
	o              Options
	grace          time.Duration
	launcherWindow time.Duration
	pinned         map[string]bool // yol sabitlemesi olan exe adlari

	st      *store.State
	running map[int]*tracked
	started bool

	lastSample    time.Time
	lastHeartbeat time.Time
	lastStateSave time.Time
	lastGate      time.Time
	lastHBEvent   time.Time

	usageExe    string
	usageDurS   int
	usageActive int
	usageSince  time.Time
}

func New(o Options) (*Watcher, error) {
	// Tam yol yalnizca sabitleme yapilmis exe adlari icin sorgulanir;
	// aksi halde her turda her process icin OpenProcess cagrilirdi.
	pinned := make(map[string]bool)
	for _, g := range o.Cfg.Gated {
		if g.Path != "" {
			pinned[strings.ToLower(g.Exe)] = true
		}
	}
	st, err := store.LoadState(o.Dir)
	if err != nil {
		return nil, err
	}
	// Eski config.json'da bu alan yok; sifir birakilirsa oturum hic
	// acilamazdi.
	launcherWindow := time.Duration(o.Cfg.LauncherWindowMinutes) * time.Minute
	if launcherWindow <= 0 {
		launcherWindow = 45 * time.Minute
	}
	return &Watcher{
		o:              o,
		grace:          time.Duration(o.Cfg.GraceMinutes) * time.Minute,
		launcherWindow: launcherWindow,
		pinned:         pinned,
		st:             st,
		running:        make(map[int]*tracked),
	}, nil
}

// Reload, durumu diskten yeniden okur. Kapi penceresi ayri bir process
// oldugu icin oturumu o acar; izleyicinin degisikligi gormesi gerekir.
func (w *Watcher) Reload() error {
	st, err := store.LoadState(w.o.Dir)
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
		if err := store.Append(w.o.Dir, store.Event{TS: now, Ev: "watch_start"}); err != nil {
			return err
		}
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

	active := session.Active(w.st, now, w.grace, w.launcherWindow)
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
			w.running[p.PID] = &tracked{exe: p.Exe, name: g.Name, start: now}
			if err := store.Append(w.o.Dir, store.Event{
				TS: now, Ev: "game_start", Exe: p.Exe, Name: g.Name, PID: p.PID,
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
	if err := store.Append(w.o.Dir, store.Event{
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

func (w *Watcher) reapExited(now time.Time, seen map[int]bool) error {
	for pid, tr := range w.running {
		if seen[pid] {
			continue
		}
		delete(w.running, pid)
		if err := store.Append(w.o.Dir, store.Event{
			TS:      now,
			Ev:      "game_end",
			Exe:     tr.exe,
			Name:    tr.name,
			PID:     pid,
			DurS:    int(now.Sub(tr.start).Seconds()),
			ActiveS: tr.activeS,
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
	err := store.Append(w.o.Dir, store.Event{
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
		if err := store.Append(w.o.Dir, store.Event{TS: now, Ev: "hb"}); err != nil {
			return err
		}
	}
	if now.Sub(w.lastHeartbeat) >= heartbeatEvery {
		w.st.Heartbeat = now
		w.lastHeartbeat = now
		w.lastStateSave = now
		return store.SaveState(w.o.Dir, w.st)
	}
	if now.Sub(w.lastStateSave) >= stateSaveEvery {
		w.lastStateSave = now
		return store.SaveState(w.o.Dir, w.st)
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
	if err := store.SaveState(w.o.Dir, w.st); err != nil {
		return err
	}
	return store.Append(w.o.Dir, store.Event{TS: now, Ev: "watch_stop"})
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
