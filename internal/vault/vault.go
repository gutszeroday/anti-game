//go:build windows

// Package vault, TOTP secret'ini diskte sifreli tutar.
package vault

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/guts/antigame/internal/dpapi"
)

// ErrNoSecret, kurulum henuz yapilmadiginda doner.
var ErrNoSecret = errors.New("MFA kurulumu yapılmamış")

const fileName = "secret.bin"

func Save(dir string, secret []byte) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	blob, err := dpapi.Protect(secret)
	if err != nil {
		return fmt.Errorf("secret şifrelenemedi: %w", err)
	}
	return os.WriteFile(filepath.Join(dir, fileName), blob, 0o600)
}

func Load(dir string) ([]byte, error) {
	blob, err := os.ReadFile(filepath.Join(dir, fileName))
	if os.IsNotExist(err) {
		return nil, ErrNoSecret
	}
	if err != nil {
		return nil, err
	}
	secret, err := dpapi.Unprotect(blob)
	if err != nil {
		return nil, fmt.Errorf("secret.bin çözülemedi (Windows profili değişmiş olabilir); `antigame setup` ile yeniden kurun: %w", err)
	}
	return secret, nil
}
