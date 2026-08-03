//go:build windows

package gate

import (
	"testing"

	"github.com/guts/antigame/internal/auth"
)

func TestSingleInstanceBlocksSecondGate(t *testing.T) {
	release, ok := AcquireSingleInstance()
	if !ok {
		t.Fatal("ilk kapi kilidi alinamadi")
	}
	defer release()

	if _, ok := AcquireSingleInstance(); ok {
		t.Fatal("ikinci kapi penceresi acilabildi")
	}
}

func TestSingleInstanceReleasableAndReacquirable(t *testing.T) {
	release, ok := AcquireSingleInstance()
	if !ok {
		t.Fatal("kilit alinamadi")
	}
	release()

	release2, ok := AcquireSingleInstance()
	if !ok {
		t.Fatal("birakildiktan sonra kilit tekrar alinamadi")
	}
	release2()
}

func TestParamsVerifyIsInvokedWithTrimmedCode(t *testing.T) {
	var got string
	p := Params{
		AppName: "Valorant",
		Verify: func(code string) (auth.Outcome, error) {
			got = code
			return auth.Outcome{OK: true}, nil
		},
	}
	out, err := p.check("  123456 \r\n")
	if err != nil {
		t.Fatal(err)
	}
	if !out.OK {
		t.Fatal("dogrulama sonucu tasinmadi")
	}
	if got != "123456" {
		t.Errorf("kod kirpilmadan gecti: %q", got)
	}
}

func TestCheckWithoutVerifierReturnsError(t *testing.T) {
	p := Params{AppName: "Valorant"}
	if _, err := p.check("123456"); err == nil {
		t.Fatal("Verify tanimsizken hata beklendi")
	}
}
