//go:build windows

package dpapi

import (
	"bytes"
	"testing"
)

func TestProtectUnprotectRoundTrip(t *testing.T) {
	plain := []byte("gizli-totp-secret-20-byte")
	blob, err := Protect(plain)
	if err != nil {
		t.Fatalf("Protect: %v", err)
	}
	if bytes.Contains(blob, plain) {
		t.Fatal("sifreli ciktida duz metin gorunuyor")
	}
	got, err := Unprotect(blob)
	if err != nil {
		t.Fatalf("Unprotect: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Errorf("gidis-donus bozuk: %q", got)
	}
}

func TestUnprotectRejectsCorruptBlob(t *testing.T) {
	blob, err := Protect([]byte("veri"))
	if err != nil {
		t.Fatal(err)
	}
	blob[len(blob)/2] ^= 0xff
	if _, err := Unprotect(blob); err == nil {
		t.Fatal("bozuk blob hata vermeden cozuldu")
	}
}
