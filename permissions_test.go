package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMissingDirectoryGuidance(t *testing.T) {
	base := t.TempDir()
	movies := filepath.Join(base, "影视")
	download := filepath.Join(base, "下载", "剧集")
	dirs := []string{movies, filepath.Join(base, "下载"), download, download, filepath.Join(base, "空库")}
	items := []Media{{Source: filepath.Join(movies, "a.mkv")}, {Source: filepath.Join(download, "b.mkv")}, {Source: filepath.Join(download, "c.mkv")}, {Source: filepath.Join(base, "不在库内", "d.mkv")}}
	got := missingDirectories(items, dirs, []string{movies})
	if len(got) != 1 || got[0].Path != download || got[0].Files != 2 {
		t.Fatalf("unexpected guidance: %+v", got)
	}
	if len(missingDirectories(items, dirs, []string{base})) != 0 {
		t.Fatal("authorized directories still prompted")
	}
	if len(missingDirectories(nil, dirs, nil)) != 0 {
		t.Fatal("empty libraries prompted")
	}
}

func TestPermissionPreflightDoesNotMoveFiles(t *testing.T) {
	src, _, roots := fixture(t)
	m := video(t, src, "permission.mkv")
	plan, _ := buildLibraryPlan([]Media{m}, []string{src}, roots, nil)
	if got := permissionIssues(plan.Rows, []string{src}, nil); len(got) != 1 || got[0].Path != src {
		t.Fatalf("missing grant: %+v", got)
	}
	if got := permissionIssues(plan.Rows, []string{src}, roots); len(got) != 0 {
		t.Fatalf("writable rejected: %+v", got)
	}
	if _, err := os.Stat(m.Source); err != nil {
		t.Fatal("changed source", err)
	}
	if _, err := os.Stat(plan.Rows[0].Target); !os.IsNotExist(err) {
		t.Fatal("created target")
	}
}

func TestMetadataInspectionDoesNotRequireGrant(t *testing.T) {
	src, _, roots := fixture(t)
	m := video(t, src, "a.mkv")
	plan, err := buildLibraryPlan([]Media{m}, []string{src}, nil, nil)
	if err != nil || len(plan.Rows) != 1 || plan.Rows[0].Status != "ready" || plan.Rows[0].Reason != "" || plan.Rows[0].Target == "" {
		t.Fatalf("no grant blocked metadata inspection: %+v %v", plan, err)
	}
	if issues := permissionIssues(plan.Rows, []string{src}, nil); len(issues) != 1 {
		t.Fatal("execution guidance lost")
	}
	m.Source = plan.Rows[0].Target
	plan, err = buildLibraryPlan([]Media{m}, []string{src}, nil, nil)
	if err != nil || plan.Rows[0].Status != "unchanged" {
		t.Fatalf("compliant path not recognized: %+v %v", plan, err)
	}
	if issues := permissionIssues(plan.Rows, []string{src}, nil); len(issues) != 0 {
		t.Fatal("unchanged file should not require move permissions")
	}
	m.Year = ""
	plan, err = buildLibraryPlan([]Media{m}, []string{src}, nil, nil)
	if err != nil || plan.Rows[0].Status != "blocked" {
		t.Fatal("invalid metadata should skip")
	}
	m = video(t, src, "b.mkv")
	plan, _ = buildLibraryPlan([]Media{m, m}, []string{src}, nil, nil)
	if plan.Rows[0].Status != "blocked" {
		t.Fatal("metadata duplicates must skip")
	}
	_ = roots
}
