//go:build linux

package organizer

import "golang.org/x/sys/unix"

func removeEmptyDirectory(path string) error { return unix.Rmdir(path) }
