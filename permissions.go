package main

import (
	"os"
	"path/filepath"
)

type MissingDirectory struct {
	Path  string `json:"path"`
	Files int    `json:"files"`
}

// Match the planner's most-specific library root; suggest only directories
// containing recognized files, without broadening to a volume or home folder.
func missingDirectories(items []Media, dirs, authorized []string) []MissingDirectory {
	counts := map[string]int{}
	for _, m := range items {
		root := ""
		for _, d := range dirs {
			d = filepath.Clean(d)
			if filepath.IsAbs(d) && inside(d, m.Source) && len(d) > len(root) {
				root = d
			}
		}
		if root == "" {
			continue
		}
		covered := false
		for _, a := range authorized {
			if filepath.IsAbs(a) && inside(filepath.Clean(a), root) {
				covered = true
				break
			}
		}
		if !covered {
			counts[root]++
		}
	}
	result := []MissingDirectory{}
	for _, d := range dirs {
		d = filepath.Clean(d)
		if counts[d] > 0 {
			result = append(result, MissingDirectory{d, counts[d]})
			delete(counts, d)
		}
	}
	return result
}

// Read-only preflight; execution still handles permissions changing afterwards.
type PermissionIssue struct {
	Path   string `json:"path"`
	Files  int    `json:"files"`
	Reason string `json:"reason"`
}

func permissionIssues(rows []Row, dirs, authorized []string) []PermissionIssue {
	issues := []PermissionIssue{}
	for _, row := range rows {
		if row.Status != "ready" {
			continue
		}
		reason := ""
		covered := func(path string) bool {
			for _, a := range authorized {
				if filepath.IsAbs(a) && inside(filepath.Clean(a), path) {
					return true
				}
			}
			return false
		}
		if !covered(row.Source) || (row.Target != "" && !covered(row.Target)) {
			reason = "应用尚未获得此目录的访问授权"
		} else if row.Status != "unchanged" {
			if err := directoryWritable(filepath.Dir(row.Source)); os.IsPermission(err) {
				reason = "应用无法修改源文件所在目录"
			}
			target := filepath.Dir(row.Source)
			if row.Target != "" {
				target = filepath.Dir(row.Target)
			}
			for {
				_, err := os.Stat(target)
				if err == nil {
					break
				}
				if os.IsPermission(err) {
					reason = "应用无法访问目标目录"
					break
				}
				if !os.IsNotExist(err) || filepath.Dir(target) == target {
					break
				}
				target = filepath.Dir(target)
			}
			if err := directoryWritable(target); os.IsPermission(err) {
				reason = "应用无法写入目标目录"
			}
		}
		if reason == "" {
			continue
		}
		path := filepath.Dir(row.Source)
		matched := ""
		for _, d := range dirs {
			d = filepath.Clean(d)
			if inside(d, row.Source) && len(d) > len(matched) {
				matched = d
			}
		}
		if matched != "" {
			path = matched
		}
		found := false
		for i := range issues {
			if issues[i].Path == path {
				issues[i].Files++
				found = true
				break
			}
		}
		if !found {
			issues = append(issues, PermissionIssue{path, 1, reason})
		}
	}
	return issues
}
