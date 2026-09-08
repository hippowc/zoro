//go:build !darwin || !cgo

package hotkey

import "fmt"

// Listen is a no-op on unsupported platforms: the launcher MVP targets macOS.
func Listen() (<-chan struct{}, func(), error) {
	return nil, func() {}, fmt.Errorf("global summon hotkey is only supported on macOS")
}
