//go:build !linux

package clock

import "time"

// hostProbe cannot verify synchronisation on this platform.
func hostProbe() (bool, time.Duration, string) {
	return false, 0, "host clock synchronisation cannot be verified on this platform"
}
