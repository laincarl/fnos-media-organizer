//go:build linux

package main

import "golang.org/x/sys/unix"

func moveNoReplace(source, target string) error {
	return unix.Renameat2(unix.AT_FDCWD, source, unix.AT_FDCWD, target, unix.RENAME_NOREPLACE)
}
func directoryWritable(path string) error {
	return unix.Faccessat(unix.AT_FDCWD, path, unix.W_OK|unix.X_OK, unix.AT_EACCESS)
}
