//go:build windows

// Package task, izleyiciyi oturum acilisinda baslatan zamanlanmis gorevi
// yonetir. Run anahtari yerine Gorev Zamanlayici secildi cunku "cokerse
// yeniden baslat" davranisini bedavaya veriyor.
package task

import (
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"unicode/utf16"
)

// Name, Gorev Zamanlayici'daki gorev adidir.
const Name = "antigame-watch"

func esc(s string) string {
	var b bytes.Buffer
	xml.EscapeText(&b, []byte(s))
	return b.String()
}

// XML, gorev tanimini uretir. schtasks komut satiri "cokerse yeniden
// baslat" ayarini desteklemedigi icin gorev XML uzerinden kuruluyor.
func XML(exePath, userID string) string {
	return `<?xml version="1.0" encoding="UTF-16"?>
<Task version="1.2" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
  <RegistrationInfo>
    <Description>anti-game izleyici</Description>
  </RegistrationInfo>
  <Triggers>
    <LogonTrigger>
      <Enabled>true</Enabled>
      <UserId>` + esc(userID) + `</UserId>
    </LogonTrigger>
  </Triggers>
  <Principals>
    <Principal id="Author">
      <UserId>` + esc(userID) + `</UserId>
      <LogonType>InteractiveToken</LogonType>
      <RunLevel>LeastPrivilege</RunLevel>
    </Principal>
  </Principals>
  <Settings>
    <MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>
    <DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>
    <StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>
    <AllowHardTerminate>true</AllowHardTerminate>
    <StartWhenAvailable>true</StartWhenAvailable>
    <RunOnlyIfNetworkAvailable>false</RunOnlyIfNetworkAvailable>
    <IdleSettings>
      <StopOnIdleEnd>false</StopOnIdleEnd>
      <RestartOnIdle>false</RestartOnIdle>
    </IdleSettings>
    <AllowStartOnDemand>true</AllowStartOnDemand>
    <Enabled>true</Enabled>
    <Hidden>false</Hidden>
    <RunOnlyIfIdle>false</RunOnlyIfIdle>
    <WakeToRun>false</WakeToRun>
    <ExecutionTimeLimit>PT0S</ExecutionTimeLimit>
    <Priority>7</Priority>
    <RestartOnFailure>
      <Interval>PT1M</Interval>
      <Count>3</Count>
    </RestartOnFailure>
  </Settings>
  <Actions Context="Author">
    <Exec>
      <Command>` + esc(exePath) + `</Command>
      <Arguments>watch --background</Arguments>
    </Exec>
  </Actions>
</Task>`
}

// utf16LE, metni BOM'lu UTF-16 little endian olarak kodlar.
// schtasks /XML yalnizca Unicode dosya kabul eder.
func utf16LE(s string) []byte {
	var buf bytes.Buffer
	buf.Write([]byte{0xff, 0xfe})
	for _, r := range utf16.Encode([]rune(s)) {
		binary.Write(&buf, binary.LittleEndian, r)
	}
	return buf.Bytes()
}

func currentUser() string {
	domain := os.Getenv("USERDOMAIN")
	user := os.Getenv("USERNAME")
	if domain == "" {
		return user
	}
	return domain + `\` + user
}

func Install(exePath string) error {
	f := filepath.Join(os.TempDir(), "antigame-task.xml")
	if err := os.WriteFile(f, utf16LE(XML(exePath, currentUser())), 0o600); err != nil {
		return err
	}
	defer os.Remove(f)

	out, err := exec.Command("schtasks", "/Create", "/TN", Name, "/XML", f, "/F").CombinedOutput()
	if err != nil {
		return fmt.Errorf("zamanlanmış görev kurulamadı: %w\n%s", err, out)
	}
	return nil
}

func Remove() error {
	out, err := exec.Command("schtasks", "/Delete", "/TN", Name, "/F").CombinedOutput()
	if err != nil {
		return fmt.Errorf("zamanlanmış görev kaldırılamadı: %w\n%s", err, out)
	}
	return nil
}

// Installed, gorev kayitli mi soyler. schtasks gorev yoksa sifir disi
// donerek hata verir; bu bir arizadan cok "yok" cevabidir.
func Installed() (bool, error) {
	if err := exec.Command("schtasks", "/Query", "/TN", Name).Run(); err != nil {
		return false, nil
	}
	return true, nil
}
