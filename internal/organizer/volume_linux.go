package organizer

import (
	"os"
	"syscall"
)

func sameVolume(a, b os.FileInfo, _, _ string) bool {
	x, ok := a.Sys().(*syscall.Stat_t)
	y, ok2 := b.Sys().(*syscall.Stat_t)
	return ok && ok2 && x.Dev == y.Dev
}
