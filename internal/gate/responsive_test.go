//go:build windows

package gate

import (
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/guts/antigame/internal/auth"
	"github.com/guts/antigame/internal/config"
)

var (
	procFindWindow         = user32.NewProc("FindWindowW")
	procSendMessageTimeout = user32.NewProc("SendMessageTimeoutW")
	procPostMessage        = user32.NewProc("PostMessageW")
)

const (
	wmNull            = 0x0000
	wmClose           = 0x0010
	smtoAbortIfHung   = 0x0002
	responseTimeoutMS = 3000
)

// TestGateWindowStaysResponsive, kullanicinin gordugu "Yanit Vermiyor"
// durumunu yakalar: mesaj dongusu pencereyi olusturan thread'de donmezse
// pencere kuyrugu hic bosalmaz ve SendMessageTimeout zaman asimina ugrar.
func TestGateWindowStaysResponsive(t *testing.T) {
	done := make(chan error, 1)
	go func() {
		done <- Show(Params{
			AppName: "Test Oyun",
			People:  []config.Person{{ID: "p1", Name: "arkadas"}},
			Verify: func(string) (auth.Outcome, error) {
				return auth.Outcome{Message: "test"}, nil
			},
		})
	}()

	hwnd := waitForWindow(t)

	// Pencereyi mesajla durtup cevap veriyor mu diye bakiyoruz. Donmus bir
	// pencerede bu cagri zaman asimina ugrar ve 0 doner.
	var result uintptr
	for range 5 {
		r, _, _ := procSendMessageTimeout.Call(hwnd, wmNull, 0, 0,
			smtoAbortIfHung, responseTimeoutMS, uintptr(unsafe.Pointer(&result)))
		if r == 0 {
			procPostMessage.Call(hwnd, wmClose, 0, 0)
			t.Fatal("kapı penceresi mesajlara cevap vermiyor (donmus)")
		}
		time.Sleep(50 * time.Millisecond)
	}

	procPostMessage.Call(hwnd, wmClose, 0, 0)
	select {
	case err := <-done:
		// Kod girilmeden kapatmak beklenen sonuc; burada olculen sey
		// pencerenin kapatma mesajina cevap verip vermedigi.
		if err != nil && err.Error() != "kod girilmeden kapatıldı" {
			t.Fatalf("Show: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("pencere kapatma mesajina cevap vermedi")
	}
}

func waitForWindow(t *testing.T) uintptr {
	t.Helper()
	class, _ := windows.UTF16PtrFromString("AntigameGate")
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		h, _, _ := procFindWindow.Call(uintptr(unsafe.Pointer(class)), 0)
		if h != 0 {
			return h
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("kapı penceresi acilmadi")
	return 0
}
