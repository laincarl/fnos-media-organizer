package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// Only remove empty directories on the successful move's source ancestor chain.
// A directory-only syscall ensures a concurrent replacement by a file is retained.
func cleanupEmptySourceDirs(row Row, authorized []string) ([]string, error) {
	removed := []string{}
	root := filepath.Clean(row.libraryRoot)
	if row.libraryRoot == "" || !filepath.IsAbs(root) {
		return removed, nil
	}
	for dir := filepath.Dir(row.Source); inside(root, dir) && dir != root; dir = filepath.Dir(dir) {
		for _, a := range authorized {
			if dir == filepath.Clean(a) {
				return removed, nil
			}
		}
		if inside(dir, row.Target) {
			return removed, nil
		}
		if err := allowedPath(dir, authorized); err != nil {
			return removed, err
		}
		info, err := os.Lstat(dir)
		if os.IsNotExist(err) {
			return removed, nil
		}
		if err != nil {
			return removed, err
		}
		if !info.IsDir() || !sameVolume(row.info, info, row.Source, dir) {
			return removed, nil
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return removed, err
		}
		if len(entries) > 0 {
			return removed, nil
		}
		if err := removeEmptyDirectory(dir); err != nil {
			if os.IsNotExist(err) || errors.Is(err, syscall.ENOTEMPTY) || errors.Is(err, syscall.EEXIST) {
				return removed, nil
			}
			return removed, fmt.Errorf("%s: %w", dir, err)
		}
		removed = append(removed, dir)
		syncDir(filepath.Dir(dir))
	}
	return removed, nil
}
