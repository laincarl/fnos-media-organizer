//go:build linux

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
)

func TestAtomicMoveNoReplace(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	os.WriteFile(source, []byte("original"), 0600)
	os.WriteFile(target, []byte("existing"), 0600)
	if err := moveNoReplace(source, target); err == nil {
		t.Fatal("replaced existing target")
	}
	a, _ := os.ReadFile(source)
	b, _ := os.ReadFile(target)
	if string(a) != "original" || string(b) != "existing" {
		t.Fatal("changed source or target")
	}
}
func TestMoveOtherOwner(t *testing.T) {
	if dir := os.Getenv("ORGANIZER_MOVE_TEST_DIR"); dir != "" {
		if err := directoryWritable(dir); err != nil {
			t.Fatal(err)
		}
		if err := moveNoReplace(filepath.Join(dir, "source"), filepath.Join(dir, "target")); err != nil {
			t.Fatal(err)
		}
		if err := directoryWritable(filepath.Join(dir, "private")); !os.IsPermission(err) {
			t.Fatal("permission not detected", err)
		}
		return
	}
	if os.Geteuid() != 0 {
		t.Skip("requires root for different file owner")
	}
	dir, err := os.MkdirTemp("", "organizer-owner-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	os.Chmod(dir, 0777)
	os.WriteFile(filepath.Join(dir, "source"), []byte("original"), 0600)
	os.Mkdir(filepath.Join(dir, "private"), 0700)
	exe, _ := os.Executable()
	cmd := exec.Command(exe, "-test.run=^TestMoveOtherOwner$")
	cmd.Env = append(os.Environ(), "ORGANIZER_MOVE_TEST_DIR="+dir)
	cmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: 65534, Gid: 65534}}
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("other owner: %v %s", err, output)
	}
	if _, err := os.Stat(filepath.Join(dir, "source")); !os.IsNotExist(err) {
		t.Fatal("source retained")
	}
	b, _ := os.ReadFile(filepath.Join(dir, "target"))
	if string(b) != "original" {
		t.Fatal("content changed")
	}
}
