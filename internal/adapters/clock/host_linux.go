//go:build linux

package clock

import (
	"fmt"
	"syscall"
	"time"
)

// Kernel clock states and status bits (see adjtimex(2)).
const (
	timeError = 5
	staUnsync = 0x0040
)

// hostProbe performs a read-only adjtimex call (modes = 0). It needs no
// privilege and makes no network request.
func hostProbe() (bool, time.Duration, string) {
	var tx syscall.Timex
	state, err := syscall.Adjtimex(&tx)
	if err != nil {
		return false, 0, "host clock synchronisation could not be read: " + err.Error()
	}
	if state == timeError || tx.Status&staUnsync != 0 {
		return false, 0, "host clock is not synchronised"
	}
	estimated := time.Duration(tx.Esterror) * time.Microsecond
	return true, estimated, fmt.Sprintf("host clock synchronised; estimated error %s", estimated)
}
