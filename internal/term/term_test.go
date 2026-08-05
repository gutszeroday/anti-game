package term

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestBufferOutputHasNoEscapes(t *testing.T) {
	th := New(&bytes.Buffer{})
	out := th.Title("başlık") + th.Good("açık") + th.Banner("antigame") + th.Clear()
	if strings.Contains(out, "\x1b") {
		t.Fatalf("terminal olmayan cikisa kacis dizisi yazildi: %q", out)
	}
}

func TestNoColorEnvDisablesColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if New(os.Stdout).Color() {
		t.Fatal("NO_COLOR tanimliyken renk acik kaldi")
	}
}

func TestPlainKeepsTextIntact(t *testing.T) {
	th := Plain()
	if got := th.Warn("anahtarı yok"); got != "anahtarı yok" {
		t.Errorf("metin degisti: %q", got)
	}
}

// Renk kapaliyken ekran temizleme kacis dizisi basmamali; menu ciktisini
// boruya yonlendiren kullanici cop gormemeli.
func TestClearWithoutColorIsBlankLine(t *testing.T) {
	if got := Plain().Clear(); got != "\n" {
		t.Errorf("beklenen bos satir, %q geldi", got)
	}
}

func TestRuleFallsBackToAscii(t *testing.T) {
	if got := Plain().Rule(4); got != "----" {
		t.Errorf("ascii cizgi beklendi, %q geldi", got)
	}
}
