package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/guts/antigame/internal/store"
	"github.com/guts/antigame/internal/totp"
)

var secret = []byte("12345678901234567890")

func newVerifier(t *testing.T, now time.Time) Verifier {
	t.Helper()
	return Verifier{
		Dir:   t.TempDir(),
		Keys:  []Key{{ID: "p1", Secret: secret}},
		Grace: 10 * time.Minute,
		Now:   func() time.Time { return now },
	}
}

func TestValidCodeOpensSession(t *testing.T) {
	now := time.Unix(1111111109, 0).UTC()
	v := newVerifier(t, now)
	out, err := v.Attempt(totp.Code(secret, totp.Counter(now)))
	if err != nil {
		t.Fatalf("Attempt: %v", err)
	}
	if !out.OK {
		t.Fatalf("gecerli kod reddedildi: %s", out.Message)
	}
	st, _ := store.LoadState(v.Dir)
	if st.Session == nil {
		t.Fatal("oturum acilmadi")
	}
	if st.Counter("p1") != totp.Counter(now) {
		t.Errorf("sayac kaydedilmedi: %d", st.Counter("p1"))
	}
	if st.Session.OpenedBy != "p1" {
		t.Errorf("oturumu acan kisi yazilmadi: %q", st.Session.OpenedBy)
	}
}

func TestValidCodeLogsUnlockEvent(t *testing.T) {
	now := time.Unix(1111111109, 0).UTC()
	v := newVerifier(t, now)
	if _, err := v.Attempt(totp.Code(secret, totp.Counter(now))); err != nil {
		t.Fatal(err)
	}
	events, _ := store.Read(v.Dir, now.Add(-time.Hour), now.Add(time.Hour))
	for _, e := range events {
		if e.Ev == "unlock" && e.Method == "totp" {
			return
		}
	}
	t.Fatalf("unlock olayi yazilmadi: %+v", events)
}

func TestSameCodeRejectedSecondTime(t *testing.T) {
	now := time.Unix(1111111109, 0).UTC()
	v := newVerifier(t, now)
	code := totp.Code(secret, totp.Counter(now))
	if _, err := v.Attempt(code); err != nil {
		t.Fatal(err)
	}
	out, err := v.Attempt(code)
	if err != nil {
		t.Fatal(err)
	}
	if out.OK {
		t.Fatal("ayni kod ikinci kez kabul edildi")
	}
	if !strings.Contains(out.Message, "daha önce") {
		t.Errorf("tekrar kullanim mesaji beklenmiyordu: %q", out.Message)
	}
}

func TestWrongCodeIncrementsFailCount(t *testing.T) {
	now := time.Unix(1111111109, 0).UTC()
	v := newVerifier(t, now)
	out, err := v.Attempt("000000")
	if err != nil {
		t.Fatal(err)
	}
	if out.OK {
		t.Fatal("yanlis kod kabul edildi")
	}
	if out.Remaining != 4 {
		t.Errorf("kalan deneme 4 olmaliydi, %d geldi", out.Remaining)
	}
	st, _ := store.LoadState(v.Dir)
	if st.FailCount != 1 {
		t.Errorf("hata sayaci artmadi: %d", st.FailCount)
	}
}

func TestFiveFailuresLockOut(t *testing.T) {
	now := time.Unix(1111111109, 0).UTC()
	v := newVerifier(t, now)
	var out Outcome
	for range 5 {
		out, _ = v.Attempt("000000")
	}
	if out.LockedUntil.IsZero() {
		t.Fatal("bes hatadan sonra kilit uygulanmadi")
	}
	if got := out.LockedUntil.Sub(now); got != 15*time.Minute {
		t.Errorf("ilk kilit 15 dk olmaliydi, %v geldi", got)
	}
}

func TestLockedOutRejectsEvenValidCode(t *testing.T) {
	now := time.Unix(1111111109, 0).UTC()
	v := newVerifier(t, now)
	for range 5 {
		v.Attempt("000000")
	}
	out, err := v.Attempt(totp.Code(secret, totp.Counter(now)))
	if err != nil {
		t.Fatal(err)
	}
	if out.OK {
		t.Fatal("kilitliyken gecerli kod kabul edildi")
	}
	st, _ := store.LoadState(v.Dir)
	if st.Session != nil {
		t.Fatal("kilitliyken oturum acildi")
	}
}

func TestLockExpiresAndValidCodeWorksAgain(t *testing.T) {
	start := time.Unix(1111111109, 0).UTC()
	v := newVerifier(t, start)
	for range 5 {
		v.Attempt("000000")
	}
	later := start.Add(16 * time.Minute)
	v.Now = func() time.Time { return later }
	out, err := v.Attempt(totp.Code(secret, totp.Counter(later)))
	if err != nil {
		t.Fatal(err)
	}
	if !out.OK {
		t.Fatalf("kilit suresi dolduktan sonra reddedildi: %s", out.Message)
	}
}

func TestSuccessResetsFailCount(t *testing.T) {
	now := time.Unix(1111111109, 0).UTC()
	v := newVerifier(t, now)
	v.Attempt("000000")
	v.Attempt("000000")
	if _, err := v.Attempt(totp.Code(secret, totp.Counter(now))); err != nil {
		t.Fatal(err)
	}
	st, _ := store.LoadState(v.Dir)
	if st.FailCount != 0 {
		t.Errorf("basarili girisde hata sayaci sifirlanmadi: %d", st.FailCount)
	}
}

func TestRecoveryCodeWorksOnceOnly(t *testing.T) {
	now := time.Unix(1111111109, 0).UTC()
	v := newVerifier(t, now)

	code, salt, hash, err := NewRecoveryCode()
	if err != nil {
		t.Fatal(err)
	}
	st, _ := store.LoadState(v.Dir)
	st.RecoverySalt, st.RecoveryHash = salt, hash
	if err := store.SaveState(v.Dir, st); err != nil {
		t.Fatal(err)
	}

	out, err := v.Attempt(code)
	if err != nil {
		t.Fatal(err)
	}
	if !out.OK {
		t.Fatalf("kurtarma kodu reddedildi: %s", out.Message)
	}

	// Ikinci kullanim reddedilmeli.
	st, _ = store.LoadState(v.Dir)
	st.Session = nil
	store.SaveState(v.Dir, st)
	out, _ = v.Attempt(code)
	if out.OK {
		t.Fatal("kurtarma kodu ikinci kez kabul edildi")
	}
}

func TestHashRecoveryIsSaltDependent(t *testing.T) {
	if HashRecovery("tuz1", "kod") == HashRecovery("tuz2", "kod") {
		t.Fatal("farkli tuzlar ayni ozeti uretti")
	}
}

var secondSecret = []byte("09876543210987654321")

func newTwoPersonVerifier(t *testing.T, now time.Time) Verifier {
	t.Helper()
	return Verifier{
		Dir: t.TempDir(),
		Keys: []Key{
			{ID: "p1", Secret: secret},
			{ID: "p2", Secret: secondSecret},
		},
		Grace: 10 * time.Minute,
		Now:   func() time.Time { return now },
	}
}

func TestSecondPersonCodeIsAccepted(t *testing.T) {
	now := time.Unix(1111111109, 0).UTC()
	v := newTwoPersonVerifier(t, now)

	out, err := v.Attempt(totp.Code(secondSecret, totp.Counter(now)))
	if err != nil {
		t.Fatal(err)
	}
	if !out.OK {
		t.Fatalf("ikinci kisinin kodu reddedildi: %s", out.Message)
	}
	if out.Who != "p2" {
		t.Errorf("kapiyi acan kisi p2 olmaliydi, %q geldi", out.Who)
	}
	st, _ := store.LoadState(v.Dir)
	if st.Session.OpenedBy != "p2" {
		t.Errorf("oturum p2 adina acilmadi: %q", st.Session.OpenedBy)
	}
}

// Sayaclar ortak tutulursa bir kisinin kullandigi kod, digerinin ayni
// zaman diliminde urettigi kodu "kullanilmis" sayip reddeder.
func TestUsedCodeOfOnePersonDoesNotBlockAnother(t *testing.T) {
	now := time.Unix(1111111109, 0).UTC()
	v := newTwoPersonVerifier(t, now)

	if out, err := v.Attempt(totp.Code(secret, totp.Counter(now))); err != nil || !out.OK {
		t.Fatalf("ilk kisi giremedi: %v %+v", err, out)
	}
	st, _ := store.LoadState(v.Dir)
	st.Session = nil
	if err := store.SaveState(v.Dir, st); err != nil {
		t.Fatal(err)
	}

	out, err := v.Attempt(totp.Code(secondSecret, totp.Counter(now)))
	if err != nil {
		t.Fatal(err)
	}
	if !out.OK {
		t.Fatalf("ikinci kisi ayni dilimde reddedildi: %s", out.Message)
	}
}

func TestReplayOfOnePersonStillReportsReplay(t *testing.T) {
	now := time.Unix(1111111109, 0).UTC()
	v := newTwoPersonVerifier(t, now)
	code := totp.Code(secret, totp.Counter(now))
	if _, err := v.Attempt(code); err != nil {
		t.Fatal(err)
	}

	out, err := v.Attempt(code)
	if err != nil {
		t.Fatal(err)
	}
	if out.OK {
		t.Fatal("kullanilmis kod kabul edildi")
	}
	if !strings.Contains(out.Message, "daha önce") {
		t.Errorf("tekrar kullanim mesaji beklendi, %q geldi", out.Message)
	}
}

func TestUnlockEventCarriesPerson(t *testing.T) {
	now := time.Unix(1111111109, 0).UTC()
	v := newTwoPersonVerifier(t, now)
	if _, err := v.Attempt(totp.Code(secondSecret, totp.Counter(now))); err != nil {
		t.Fatal(err)
	}
	events, _ := store.Read(v.Dir, now.Add(-time.Hour), now.Add(time.Hour))
	for _, e := range events {
		if e.Ev == "unlock" && e.Who == "p2" {
			return
		}
	}
	t.Fatalf("unlock olayi kisiyi tasimadi: %+v", events)
}

func TestRecoveryUnlockHasNoPerson(t *testing.T) {
	now := time.Unix(1111111109, 0).UTC()
	v := newTwoPersonVerifier(t, now)

	code, salt, hash, err := NewRecoveryCode()
	if err != nil {
		t.Fatal(err)
	}
	st, _ := store.LoadState(v.Dir)
	st.RecoverySalt, st.RecoveryHash = salt, hash
	if err := store.SaveState(v.Dir, st); err != nil {
		t.Fatal(err)
	}

	out, err := v.Attempt(code)
	if err != nil {
		t.Fatal(err)
	}
	if !out.OK {
		t.Fatalf("kurtarma kodu reddedildi: %s", out.Message)
	}
	if out.Who != "" {
		t.Errorf("kurtarma kodu bir kisiye yazildi: %q", out.Who)
	}
}
