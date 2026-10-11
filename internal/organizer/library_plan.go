package organizer

import (
	"errors"
	"os"
	"path/filepath"
	"time"
)

// Prefer the most specific configured library directory. Do not infer roots from files.
func buildLibraryPlan(items []Media, dirs, authorized []string, formats map[string]bool, rules ...Rules) (*Plan, error) {
	if len(dirs) == 0 {
		return nil, errors.New("请先读取影视库目录")
	}
	groups := map[string][]Media{}
	p := &Plan{ID: randomID(), Created: time.Now(), Rows: []Row{}}
	for _, m := range items {
		root := ""
		for _, d := range dirs {
			d = filepath.Clean(d)
			if filepath.IsAbs(d) && inside(d, m.Source) && len(d) > len(root) {
				root = d
			}
		}
		if root == "" {
			p.Rows = append(p.Rows, Row{Media: m, Status: "blocked", Reason: "文件不在所选影视库目录内"})
		} else {
			groups[root] = append(groups[root], m)
		}
	}
	for root, group := range groups {
		plan, e := buildPlan(group, root, root, authorized, formats, rules...)
		if e != nil {
			for _, m := range group {
				row := Row{Media: m, Status: "blocked", Reason: e.Error()}
				if errors.Is(e, errDirectoryGrant) || os.IsPermission(e) {
					row = metadataRuleRow(m, root, formats, rules...)
				}
				p.Rows = append(p.Rows, row)
			}
			continue
		}
		p.Rows = append(p.Rows, plan.Rows...)
	}
	// Stable display order, independent of map iteration.
	ordered := []Row{}
	for _, m := range items {
		for _, r := range p.Rows {
			if r.ID == m.ID && r.Source == m.Source {
				ordered = append(ordered, r)
				break
			}
		}
	}
	p.Rows = ordered
	// Metadata-only rows must apply the same duplicate checks as disk-backed rows.
	targets, sources := map[string][]int{}, map[string][]int{}
	for i, row := range p.Rows {
		if row.Target != "" {
			targets[row.Target] = append(targets[row.Target], i)
		}
		sources[row.Source] = append(sources[row.Source], i)
	}
	for _, indices := range targets {
		if len(indices) > 1 {
			for _, i := range indices {
				p.Rows[i].Status = "blocked"
				p.Rows[i].Reason = "多份视频指向同一目标，请先选择保留的版本"
			}
		}
	}
	for _, indices := range sources {
		if len(indices) > 1 {
			for _, i := range indices {
				p.Rows[i].Status = "blocked"
				p.Rows[i].Reason = "同一源文件出现在多个条目中"
			}
		}
	}
	return p, nil
}

// The media API already provides paths and recognition data. Comparing naming
// rules does not grant file access or verify filesystem conflicts.
func metadataRuleRow(m Media, root string, formats map[string]bool, rules ...Rules) Row {
	m.Source = filepath.Clean(m.Source)
	row := Row{Media: m, Status: "ready", libraryRoot: root}
	target, err := targetPath(m, root, formats, rules...)
	if err != nil {
		row.Status = "blocked"
		row.Reason = err.Error()
		return row
	}
	row.Target = target
	if target == m.Source {
		row.Status = "unchanged"
	}
	return row
}
