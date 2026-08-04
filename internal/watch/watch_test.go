package watch

import (
	"testing"
	"time"

	"github.com/guts/antigame/internal/config"
	"github.com/guts/antigame/internal/session"
	"github.com/guts/antigame/internal/store"
	"github.com/guts/antigame/internal/winproc"
)

var t0 = time.Date(2026, 8, 3, 18, 0, 0, 0, time.UTC)

type fakes struct {
	procs      []winproc.Proc
	killed     []int
	gateCalls  []string
	idle       int
	foreground int
}

func newWatcher(t *testing.T, f *fakes) (*Watcher, string) {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Default()
	cfg.Gated = []config.Game{{Name: "Valorant", Exe: "VALORANT.exe"}}
	cfg.PollMS = 250
	cfg.FocusSampleS = 5
	cfg.IdleThresholdS = 300
	cfg.GraceMinutes = 10

	w, err := New(Options{
		Dir:  dir,
		Cfg:  cfg,
		List: func() ([]winproc.Proc, error) { return f.procs, nil },
		Path: func(int) (string, error) { return "", nil },
		Terminate: func(pid int) error {
			f.killed = append(f.killed, pid)
			// Sonlandirilan process bir sonraki turda listede olmaz.
			var keep []winproc.Proc
			for _, p := range f.procs {
				if p.PID != pid {
					keep = append(keep, p)
				}
			}
			f.procs = keep
			return nil
		},
		Trim:          func() error { return nil },
		Idle:          func() (int, error) { return f.idle, nil },
		ForegroundPID: func() (int, error) { return f.foreground, nil },
		SpawnGate: func(app string) error {
			f.gateCalls = append(f.gateCalls, app)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return w, dir
}

func events(t *testing.T, dir string) []store.Event {
	t.Helper()
	ev, err := store.Read(dir, t0.Add(-time.Hour), t0.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("store.Read: %v", err)
	}
	return ev
}

func hasEvent(ev []store.Event, kind, exe string) bool {
	for _, e := range ev {
		if e.Ev == kind && (exe == "" || e.Exe == exe) {
			return true
		}
	}
	return false
}

func TestBlocksGatedGameWhenNoSession(t *testing.T) {
	f := &fakes{procs: []winproc.Proc{{PID: 42, Exe: "VALORANT.exe"}}}
	w, dir := newWatcher(t, f)

	if err := w.Step(t0); err != nil {
		t.Fatalf("Step: %v", err)
	}
	if len(f.killed) != 1 || f.killed[0] != 42 {
		t.Fatalf("oyun sonlandirilmadi: %v", f.killed)
	}
	if !hasEvent(events(t, dir), "blocked", "VALORANT.exe") {
		t.Error("blocked olayi yazilmadi")
	}
	if len(f.gateCalls) != 1 || f.gateCalls[0] != "Valorant" {
		t.Errorf("kapi penceresi acilmadi: %v", f.gateCalls)
	}
}

func TestDoesNotSpawnGateRepeatedlyWithinCooldown(t *testing.T) {
	f := &fakes{procs: []winproc.Proc{{PID: 42, Exe: "VALORANT.exe"}}}
	w, _ := newWatcher(t, f)

	for i := range 8 {
		f.procs = []winproc.Proc{{PID: 42 + i, Exe: "VALORANT.exe"}}
		if err := w.Step(t0.Add(time.Duration(i) * 250 * time.Millisecond)); err != nil {
			t.Fatal(err)
		}
	}
	if len(f.gateCalls) != 1 {
		t.Errorf("kapi %d kez acildi, 1 bekleniyordu", len(f.gateCalls))
	}
}

func TestDoesNotBlockWhenSessionActive(t *testing.T) {
	f := &fakes{procs: []winproc.Proc{{PID: 42, Exe: "VALORANT.exe"}}}
	w, dir := newWatcher(t, f)

	st, _ := store.LoadState(dir)
	session.Open(st, t0)
	if err := store.SaveState(dir, st); err != nil {
		t.Fatal(err)
	}
	w.Reload()

	if err := w.Step(t0); err != nil {
		t.Fatal(err)
	}
	if len(f.killed) != 0 {
		t.Fatalf("oturum acikken oyun sonlandirildi: %v", f.killed)
	}
	if !hasEvent(events(t, dir), "game_start", "VALORANT.exe") {
		t.Error("game_start olayi yazilmadi")
	}
}

func TestLogsGameEndWithDurationWhenProcessExits(t *testing.T) {
	f := &fakes{procs: []winproc.Proc{{PID: 42, Exe: "VALORANT.exe"}}}
	w, dir := newWatcher(t, f)

	st, _ := store.LoadState(dir)
	session.Open(st, t0)
	store.SaveState(dir, st)
	w.Reload()

	if err := w.Step(t0); err != nil {
		t.Fatal(err)
	}
	f.procs = nil
	if err := w.Step(t0.Add(90 * time.Minute)); err != nil {
		t.Fatal(err)
	}

	for _, e := range events(t, dir) {
		if e.Ev == "game_end" {
			if e.DurS != 5400 {
				t.Errorf("sure 5400 sn olmaliydi, %d geldi", e.DurS)
			}
			return
		}
	}
	t.Fatal("game_end olayi yazilmadi")
}

func TestSessionStaysAliveWhileGameRuns(t *testing.T) {
	f := &fakes{procs: []winproc.Proc{{PID: 42, Exe: "VALORANT.exe"}}}
	w, dir := newWatcher(t, f)

	st, _ := store.LoadState(dir)
	session.Open(st, t0)
	store.SaveState(dir, st)
	w.Reload()

	// Odemesiz sureden cok daha uzun sure oyna.
	for i := range 61 {
		if err := w.Step(t0.Add(time.Duration(i) * time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	if len(f.killed) != 0 {
		t.Fatalf("calisan oyun sonlandirildi: %v", f.killed)
	}
}

func TestUsageRecordedForNonGatedForegroundApp(t *testing.T) {
	f := &fakes{
		procs:      []winproc.Proc{{PID: 7, Exe: "chrome.exe"}},
		foreground: 7,
	}
	w, dir := newWatcher(t, f)

	for i := range 15 {
		if err := w.Step(t0.Add(time.Duration(i) * 5 * time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Shutdown(t0.Add(75 * time.Second)); err != nil {
		t.Fatal(err)
	}

	var total int
	for _, e := range events(t, dir) {
		if e.Ev == "usage" && e.Exe == "chrome.exe" {
			total += e.DurS
		}
	}
	if total < 60 {
		t.Errorf("kullanim suresi eksik kaydedildi: %d sn", total)
	}
}

func TestUsageNotRecordedForGatedGame(t *testing.T) {
	f := &fakes{
		procs:      []winproc.Proc{{PID: 42, Exe: "VALORANT.exe"}},
		foreground: 42,
	}
	w, dir := newWatcher(t, f)

	st, _ := store.LoadState(dir)
	session.Open(st, t0)
	store.SaveState(dir, st)
	w.Reload()

	for i := range 21 {
		w.Step(t0.Add(time.Duration(i) * 5 * time.Second))
	}
	w.Shutdown(t0.Add(105 * time.Second))

	if hasEvent(events(t, dir), "usage", "VALORANT.exe") {
		t.Error("kapidaki oyun ayrica usage olarak da kaydedildi (cift sayim)")
	}
}

func TestIdleTimeNotCountedAsActive(t *testing.T) {
	f := &fakes{
		procs:      []winproc.Proc{{PID: 7, Exe: "chrome.exe"}},
		foreground: 7,
		idle:       600, // 10 dakikadir girdi yok
	}
	w, dir := newWatcher(t, f)

	for i := range 15 {
		w.Step(t0.Add(time.Duration(i) * 5 * time.Second))
	}
	w.Shutdown(t0.Add(75 * time.Second))

	for _, e := range events(t, dir) {
		if e.Ev == "usage" && e.ActiveS != 0 {
			t.Errorf("AFK sure aktif sayildi: %+v", e)
		}
	}
}

func TestHeartbeatPersisted(t *testing.T) {
	f := &fakes{}
	w, dir := newWatcher(t, f)

	if err := w.Step(t0); err != nil {
		t.Fatal(err)
	}
	if err := w.Step(t0.Add(2 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	st, _ := store.LoadState(dir)
	if st.Heartbeat.IsZero() {
		t.Fatal("nabiz yazilmadi")
	}
	if st.Heartbeat.Before(t0.Add(time.Minute)) {
		t.Errorf("nabiz guncellenmedi: %v", st.Heartbeat)
	}
}

func TestWatchStartEventWrittenOnceOnFirstStep(t *testing.T) {
	f := &fakes{}
	w, dir := newWatcher(t, f)

	if hasEvent(events(t, dir), "watch_start", "") {
		t.Fatal("watch_start New icinde yazildi; ilk turda yazilmali")
	}
	for i := range 3 {
		if err := w.Step(t0.Add(time.Duration(i) * time.Second)); err != nil {
			t.Fatal(err)
		}
	}

	var starts int
	for _, e := range events(t, dir) {
		if e.Ev == "watch_start" {
			starts++
		}
	}
	if starts != 1 {
		t.Errorf("watch_start %d kez yazildi, 1 bekleniyordu", starts)
	}
}

func TestHeartbeatEventWrittenEveryTenMinutes(t *testing.T) {
	f := &fakes{}
	w, dir := newWatcher(t, f)

	for i := range 26 {
		if err := w.Step(t0.Add(time.Duration(i) * time.Minute)); err != nil {
			t.Fatal(err)
		}
	}

	var beats int
	for _, e := range events(t, dir) {
		if e.Ev == "hb" {
			beats++
		}
	}
	// 0, 10 ve 20. dakikalarda uc nabiz beklenir.
	if beats < 2 {
		t.Errorf("nabiz olayi yeterince yazilmadi: %d", beats)
	}
}
