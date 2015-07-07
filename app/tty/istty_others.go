// +build !bsd,!darwin,!linux,!freebsd,!openbsd,!cgo

package tty

import (
	"os"
)

// Return true if f refers to a tty
func IsTTY(f *os.File) bool {
	return false
}
