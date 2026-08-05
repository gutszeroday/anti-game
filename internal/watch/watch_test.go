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
	session.Open(st, t0, "")
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
	session.Open(st, t0, "")
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
	session.Open(st, t0, "")
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
	session.Open(st, t0, "")
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

func TestDoesNotBlockWhenPairingIsMissing(t *testing.T) {
	// Kurulum yapilmadan izleyici calisirsa oyun oldurulur ama kapi
	// penceresi secret olmadigi icin acilamaz: kullanici oyunlardan
	// tamamen kilitlenir ve kod girecek yer kalmaz.
	f := &fakes{procs: []winproc.Proc{{PID: 42, Exe: "VALORANT.exe"}}}
	w, dir := newWatcher(t, f)
	w.o.SecretReady = func() bool { return false }

	if err := w.Step(t0); err != nil {
		t.Fatal(err)
	}
	if len(f.killed) != 0 {
		t.Fatalf("eslestirme yokken oyun oldurüldu: %v", f.killed)
	}
	if len(f.gateCalls) != 0 {
		t.Errorf("acilamayacak kapi penceresi istendi: %v", f.gateCalls)
	}
	if !hasEvent(events(t, dir), "gate_disabled", "") {
		t.Error("kapinin devre disi oldugu gunluge yazilmadi")
	}
}

func TestPairingMissingWarningWrittenOnce(t *testing.T) {
	f := &fakes{procs: []winproc.Proc{{PID: 42, Exe: "VALORANT.exe"}}}
	w, dir := newWatcher(t, f)
	w.o.SecretReady = func() bool { return false }

	for i := range 20 {
		w.Step(t0.Add(time.Duration(i) * time.Minute))
	}
	var n int
	for _, e := range events(t, dir) {
		if e.Ev == "gate_disabled" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("uyari %d kez yazildi, 1 bekleniyordu", n)
	}
}

func TestStillTracksTimeWhenPairingIsMissing(t *testing.T) {
	f := &fakes{procs: []winproc.Proc{{PID: 42, Exe: "VALORANT.exe"}}}
	w, dir := newWatcher(t, f)
	w.o.SecretReady = func() bool { return false }

	w.Step(t0)
	f.procs = nil
	w.Step(t0.Add(30 * time.Minute))

	if !hasEvent(events(t, dir), "game_end", "VALORANT.exe") {
		t.Error("eslestirme yokken sure hic kaydedilmedi")
	}
}

func TestLauncherLeftOpenEventuallyRelocksGate(t *testing.T) {
	// Riot Client tepside acik unutulunca oturum sonsuza kadar
	// tazeleniyordu: sabah alinan kodla gun boyu girip cikilabiliyordu.
	f := &fakes{procs: []winproc.Proc{{PID: 7, Exe: "RiotClient.exe"}}}
	w, dir := newWatcher(t, f)
	w.o.Cfg.Gated = []config.Game{
		{Name: "Riot Client", Exe: "RiotClient.exe", Launcher: true},
		{Name: "Valorant", Exe: "VALORANT.exe"},
	}
	w.launcherWindow = 45 * time.Minute

	st, _ := store.LoadState(dir)
	session.Open(st, t0, "")
	store.SaveState(dir, st)
	w.Reload()

	// Once gercek oyun oynaniyor.
	f.procs = []winproc.Proc{{PID: 7, Exe: "RiotClient.exe"}, {PID: 42, Exe: "VALORANT.exe"}}
	if err := w.Step(t0); err != nil {
		t.Fatal(err)
	}

	// Oyun kapaniyor, yalnizca istemci acik kaliyor.
	f.procs = []winproc.Proc{{PID: 7, Exe: "RiotClient.exe"}}
	for i := 1; i <= 40; i++ {
		if err := w.Step(t0.Add(time.Duration(i) * time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	if len(f.killed) != 0 {
		t.Fatalf("baslatici penceresi dolmadan istemci oldurüldu: %v", f.killed)
	}

	// Pencere dolduktan sonra oturum dusmeli ve istemci kapida durmali.
	if err := w.Step(t0.Add(50 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	if len(f.killed) == 0 {
		t.Error("baslatici penceresi dolduktan sonra oturum dusmedi")
	}
}

// Riot istemcisi tek process degil. Servis oldurulup Electron arayuz ayakta
// kalirsa arayuz servisi yeniden dogurur ve kapi delinir; bu yuzden oturum
// dustugunde ailenin tamami gitmeli. Test varsayilan liste uzerinden gider:
// aileden biri Default()'tan dusurulurse burada yakalanir.
func TestWholeRiotFamilyIsTerminatedWhenSessionExpires(t *testing.T) {
	family := []winproc.Proc{
		{PID: 1, Exe: "RiotClientServices.exe"},
		{PID: 2, Exe: "Riot Client.exe"},
		{PID: 3, Exe: "Riot Client.exe"},
		{PID: 4, Exe: "RiotClientCrashHandler.exe"},
		{PID: 5, Exe: "LeagueClient.exe"},
		{PID: 6, Exe: "LeagueClientUx.exe"},
		{PID: 7, Exe: "League of Legends.exe"},
	}
	f := &fakes{procs: append([]winproc.Proc(nil), family...)}
	w, _ := newWatcher(t, f)
	w.o.Cfg.Gated = config.Default().Gated

	// Oturum yok: her tur kapida duran her sey sonlandirilmali.
	if err := w.Step(t0); err != nil {
		t.Fatal(err)
	}

	killed := map[int]bool{}
	for _, pid := range f.killed {
		killed[pid] = true
	}
	for _, p := range family {
		if !killed[p.PID] {
			t.Errorf("%s (PID %d) sonlandirilmadi: ayakta kalan surec kapiyi deler",
				p.Exe, p.PID)
		}
	}
}

// Baslatici penceresi dolunca aile birlikte gitmeli: kullanici oyundan
// cikip istemciyi acik biraktiginda sure dolunca Riot Client da kapanir.
func TestFamilyClosesAfterLauncherWindowExpires(t *testing.T) {
	f := &fakes{}
	w, dir := newWatcher(t, f)
	w.o.Cfg.Gated = config.Default().Gated
	w.launcherWindow = 10 * time.Minute

	st, _ := store.LoadState(dir)
	session.Open(st, t0, "")
	store.SaveState(dir, st)
	w.Reload()

	// Mac oynaniyor: oyun ve istemci birlikte calisiyor.
	f.procs = []winproc.Proc{
		{PID: 1, Exe: "RiotClientServices.exe"},
		{PID: 5, Exe: "LeagueClient.exe"},
		{PID: 7, Exe: "League of Legends.exe"},
	}
	if err := w.Step(t0); err != nil {
		t.Fatal(err)
	}

	// Mac bitti, yalnizca istemci acik kaldi.
	f.procs = []winproc.Proc{
		{PID: 1, Exe: "RiotClientServices.exe"},
		{PID: 5, Exe: "LeagueClient.exe"},
	}
	for i := 1; i <= 9; i++ {
		if err := w.Step(t0.Add(time.Duration(i) * time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	if len(f.killed) != 0 {
		t.Fatalf("pencere dolmadan istemci oldurüldu: %v", f.killed)
	}

	if err := w.Step(t0.Add(11 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	if len(f.killed) != 2 {
		t.Errorf("pencere dolunca istemci ailesi kapanmadi, oldurulen: %v", f.killed)
	}
}

func TestLauncherKeepsSessionAliveBetweenMatches(t *testing.T) {
	// Mac arasinda yalnizca istemci calisir; oturum burada dusmemeli,
	// yoksa yeni mac yuklenirken oyun oldurulur.
	f := &fakes{procs: []winproc.Proc{{PID: 7, Exe: "RiotClient.exe"}}}
	w, dir := newWatcher(t, f)
	w.o.Cfg.Gated = []config.Game{
		{Name: "Riot Client", Exe: "RiotClient.exe", Launcher: true},
		{Name: "Valorant", Exe: "VALORANT.exe"},
	}
	w.launcherWindow = 45 * time.Minute

	st, _ := store.LoadState(dir)
	session.Open(st, t0, "")
	store.SaveState(dir, st)
	w.Reload()

	f.procs = []winproc.Proc{{PID: 7, Exe: "RiotClient.exe"}, {PID: 42, Exe: "VALORANT.exe"}}
	w.Step(t0)

	f.procs = []winproc.Proc{{PID: 7, Exe: "RiotClient.exe"}}
	for i := 1; i <= 25; i++ {
		w.Step(t0.Add(time.Duration(i) * time.Minute))
	}

	// 25 dakika sonra yeni mac basliyor.
	f.procs = []winproc.Proc{{PID: 7, Exe: "RiotClient.exe"}, {PID: 43, Exe: "VALORANT.exe"}}
	if err := w.Step(t0.Add(26 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	if len(f.killed) != 0 {
		t.Errorf("mac arasindan sonra baslayan oyun oldurüldu: %v", f.killed)
	}
}

// Menu veya tepsi listeye oyun eklediginde config.json degisir ama izleyici
// ayri bir process'tir. Ayari yalnizca acilista okursa, eklenen oyun ancak
// izleyici yeniden baslatildiginda kapida durur; kullanici bunu bilmeden
// korumasiz oynamis olur.
func TestPicksUpGameAddedToConfigWhileRunning(t *testing.T) {
	f := &fakes{}
	w, dir := newWatcher(t, f)

	if err := w.Step(t0); err != nil {
		t.Fatal(err)
	}

	cfg := config.Default()
	cfg.Gated = []config.Game{
		{Name: "Valorant", Exe: "VALORANT.exe"},
		{Name: "Palworld", Exe: "Palworld.exe"},
	}
	if err := config.Save(dir, cfg); err != nil {
		t.Fatal(err)
	}

	f.procs = []winproc.Proc{{PID: 99, Exe: "Palworld.exe"}}
	if err := w.Step(t0.Add(configCheckEvery)); err != nil {
		t.Fatal(err)
	}
	if len(f.killed) != 1 || f.killed[0] != 99 {
		t.Fatalf("sonradan eklenen oyun durdurulmadi: %v", f.killed)
	}
}

// Ayar dosyasi yoksa (testler ve ilk calistirma) bellekteki yapilandirma
// varsayilanla ezilmemeli.
func TestKeepsConfigWhenFileMissing(t *testing.T) {
	f := &fakes{procs: []winproc.Proc{{PID: 7, Exe: "LeagueClient.exe"}}}
	w, _ := newWatcher(t, f)

	if err := w.Step(t0); err != nil {
		t.Fatal(err)
	}
	// newWatcher yalnizca VALORANT.exe'yi kapiya koyar; varsayilan liste
	// yuklenmis olsaydi LeagueClient de durdurulurdu.
	if len(f.killed) != 0 {
		t.Fatalf("liste varsayilanla ezildi: %v", f.killed)
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
