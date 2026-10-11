//go:build windows

package organizer

import "syscall"

func removeEmptyDirectory(path string) error {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	return syscall.RemoveDirectory(p)
}
