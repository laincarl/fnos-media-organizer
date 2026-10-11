package main

import (
	"os"
	"path/filepath"
	"testing"
)

func cleanupFixture(t *testing.T) (Row, string) {
	t.Helper()
	root := t.TempDir()
	old := filepath.Join(root, "release", "season")
	if err := os.MkdirAll(old, 0755); err != nil {
		t.Fatal(err)
	}
	m := video(t, old, "original.mkv")
	p, err := buildPlan([]Media{m}, root, root, []string{root}, mediaExtensions)
	if err != nil {
		t.Fatal(err)
	}
	r := p.Rows[0]
	if r.Status != "ready" {
		t.Fatalf("unexpected plan: %+v", r)
	}
	if _, err := moveFile(r, []string{root}); err != nil {
		t.Fatal(err)
	}
	return r, root
}

func TestCleanupEmptyAncestors(t *testing.T) {
	r, root := cleanupFixture(t)
	neighbor := filepath.Join(root, "unrelated")
	if err := os.Mkdir(neighbor, 0755); err != nil {
		t.Fatal(err)
	}
	removed, err := cleanupEmptySourceDirs(r, []string{root})
	if err != nil || len(removed) != 2 {
		t.Fatalf("removed=%v err=%v", removed, err)
	}
	for _, p := range []string{root, neighbor, r.Target} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("must preserve %s: %v", p, err)
		}
	}
}

func TestCleanupPreservesOtherFiles(t *testing.T) {
	for _, name := range []string{"subtitle.srt", "poster.jpg", ".hidden"} {
		t.Run(name, func(t *testing.T) {
			r, root := cleanupFixture(t)
			p := filepath.Join(filepath.Dir(r.Source), name)
			if err := os.WriteFile(p, []byte("keep"), 0600); err != nil {
				t.Fatal(err)
			}
			removed, err := cleanupEmptySourceDirs(r, []string{root})
			if err != nil || len(removed) != 0 {
				t.Fatalf("removed=%v err=%v", removed, err)
			}
			if b, err := os.ReadFile(p); err != nil || string(b) != "keep" {
				t.Fatal("lost sidecar")
			}
		})
	}
}

func TestCleanupStopsAtLibraryAndGrantRoots(t *testing.T) {
	for _, boundary := range []string{"library", "grant"} {
		t.Run(boundary, func(t *testing.T) {
			r, root := cleanupFixture(t)
			parent := filepath.Dir(filepath.Dir(r.Source))
			grants := []string{root}
			if boundary == "library" {
				r.libraryRoot = parent
			} else {
				grants = append(grants, parent)
			}
			removed, err := cleanupEmptySourceDirs(r, grants)
			if err != nil || len(removed) != 1 {
				t.Fatalf("removed=%v err=%v", removed, err)
			}
			if _, err := os.Stat(parent); err != nil {
				t.Fatal("deleted boundary")
			}
		})
	}
}

func TestCleanupRequiresGrant(t *testing.T) {
	r, root := cleanupFixture(t)
	removed, err := cleanupEmptySourceDirs(r, []string{filepath.Join(root, "other")})
	if err == nil || len(removed) != 0 {
		t.Fatalf("removed=%v err=%v", removed, err)
	}
	if _, err := os.Stat(filepath.Dir(r.Source)); err != nil {
		t.Fatal("deleted unauthorized directory")
	}
}

func TestDirectoryRemovalNeverDeletesFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "replacement")
	if err := os.WriteFile(p, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := removeEmptyDirectory(p); err == nil {
		t.Fatal("accepted regular file")
	}
	if b, err := os.ReadFile(p); err != nil || string(b) != "keep" {
		t.Fatal("lost replacement file")
	}
}
