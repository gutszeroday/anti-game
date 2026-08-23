//go:build windows

package ui

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

// newPairingCode, "Sohbet ekle" akisinda kullanicinin bota gonderecegi
// 6 haneli rastgele kodu uretir.
func newPairingCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}
