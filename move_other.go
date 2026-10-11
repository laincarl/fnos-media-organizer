//go:build !linux

package main

import "os"

func moveNoReplace(source, target string) error {
	if err := os.Link(source, target); err != nil {
		return err
	}
	return os.Remove(source)
}
func directoryWritable(path string) error { _, err := os.Stat(path); return err }
