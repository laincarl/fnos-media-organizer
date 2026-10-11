//go:build !linux && !windows

package main

import "errors"

func removeEmptyDirectory(path string) error {
	return errors.New("此系统不支持安全清理空目录")
}
