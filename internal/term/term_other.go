//go:build !windows

package term

import "os"

// prepare, Windows disinda ek bir hazirlik gerektirmez: terminal zaten
// ANSI ve UTF-8 konusur.
func prepare(*os.File) (color, unicode bool) { return true, true }
