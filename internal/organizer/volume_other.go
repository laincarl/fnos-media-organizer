//go:build !linux

package organizer

import (
	"os"
	"path/filepath"
	"strings"
)

func sameVolume(a, b os.FileInfo, pa, pb string) bool {
	return a != nil && b != nil && strings.EqualFold(filepath.VolumeName(pa), filepath.VolumeName(pb))
}
