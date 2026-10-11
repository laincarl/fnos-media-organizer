package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type Media struct {
	ID           string `json:"id"`
	Poster       string `json:"poster,omitempty"`
	Title        string `json:"title"`
	Year         string `json:"year"`
	Kind         string `json:"kind"`
	Season       int    `json:"season"`
	Episode      int    `json:"episode"`
	EpisodeTitle string `json:"episode_title,omitempty"`
	Source       string `json:"source"`
}
type Row struct {
	Media
	Target             string `json:"target"`
	Status             string `json:"status"`
	Reason             string `json:"reason"`
	IssueCode          string `json:"issue_code,omitempty"`
	info               os.FileInfo
	libraryRoot        string
	CleanedDirectories []string `json:"cleaned_directories,omitempty"`
	CleanupWarning     string   `json:"cleanup_warning,omitempty"`
}
type Plan struct {
	ID      string    `json:"id"`
	Created time.Time `json:"created"`
	Rows    []Row     `json:"rows"`
}

var invalidName = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f]`)
var yearPattern = regexp.MustCompile(`^(18|19|20|21)\d{2}$`)
var mediaExtensions = map[string]bool{".mkv": true, ".mp4": true, ".avi": true, ".mov": true, ".m4v": true, ".wmv": true, ".ts": true, ".m2ts": true, ".mpg": true, ".mpeg": true, ".webm": true, ".flv": true, ".iso": true}
var formatPattern = regexp.MustCompile(`^\.[a-z0-9]{1,8}$`)

func parseFormats(values []string) (map[string]bool, error) {
	out := map[string]bool{}
	for _, v := range values {
		v = strings.ToLower(strings.TrimSpace(v))
		if !strings.HasPrefix(v, ".") {
			v = "." + v
		}
		if !formatPattern.MatchString(v) {
			return nil, errors.New("扩展名格式无效")
		}
		out[v] = true
	}
	if len(out) == 0 {
		return nil, errors.New("请至少选择一种文件格式")
	}
	return out, nil
}

func safeName(s string) (string, error) {
	s = strings.Trim(strings.TrimSpace(invalidName.ReplaceAllString(s, "_")), ". ")
	if s == "" || len([]byte(s)) > 160 {
		return "", errors.New("名称为空或过长")
	}
	return s, nil
}
func inside(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// Reject symlinks in every existing component, including parent directories.
func noSymlinks(p string) error {
	p = filepath.Clean(p)
	for {
		info, err := os.Lstat(p)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return errors.New("路径包含软链接，已跳过")
		}
		parent := filepath.Dir(p)
		if parent == p {
			break
		}
		p = parent
	}
	return nil
}

var errDirectoryGrant = errors.New("路径不在应用获授权的目录内；请在系统应用设置中授权目录并重启应用")

func allowedPath(p string, roots []string) error {
	if !filepath.IsAbs(p) {
		return errors.New("需要 NAS 上的绝对路径")
	}
	p = filepath.Clean(p)
	for _, root := range roots {
		if filepath.IsAbs(root) && inside(filepath.Clean(root), p) {
			return noSymlinks(p)
		}
	}
	return errDirectoryGrant
}
func targetPath(m Media, root string, formats map[string]bool, rules ...Rules) (string, error) {
	if formats != nil && !formats[strings.ToLower(filepath.Ext(m.Source))] {
		return "", errors.New("文件格式未选中")
	}
	r := defaultRules()
	if len(rules) > 0 {
		r = rules[0]
	}
	return ruleTarget(m, root, r)
}
func buildPlan(items []Media, sourceRoot, targetRoot string, roots []string, formats map[string]bool, rules ...Rules) (*Plan, error) {
	if err := allowedPath(sourceRoot, roots); err != nil {
		return nil, err
	}
	if err := allowedPath(targetRoot, roots); err != nil {
		return nil, err
	}
	srcInfo, err := os.Stat(sourceRoot)
	if os.IsPermission(err) {
		return nil, err
	}
	if err != nil || !srcInfo.IsDir() {
		return nil, errors.New("源目录不存在或不是目录")
	}
	targetInfo, err := os.Stat(targetRoot)
	if os.IsPermission(err) {
		return nil, err
	}
	if err != nil || !targetInfo.IsDir() {
		return nil, errors.New("目标根目录必须已存在且为目录")
	}
	plan := &Plan{ID: randomID(), Created: time.Now(), Rows: []Row{}}
	targets := map[string][]int{}
	sources := map[string][]int{}
	for _, m := range items {
		m.Source = filepath.Clean(m.Source)
		row := Row{Media: m, Status: "ready", libraryRoot: sourceRoot}
		var e error
		switch {
		case !filepath.IsAbs(m.Source) || !inside(sourceRoot, m.Source):
			e = errors.New("不在所选源目录内")
		default:
			e = allowedPath(m.Source, roots)
			if e == nil {
				row.info, e = os.Stat(m.Source)
			}
			if e == nil && !row.info.Mode().IsRegular() {
				e = errors.New("不是普通文件")
			}
			if e == nil && !sameVolume(row.info, targetInfo, m.Source, targetRoot) {
				e = errors.New("跨存储空间，已跳过；仅支持同一存储空间内整理")
			}
			if e == nil {
				row.Target, e = targetPath(m, targetRoot, formats, rules...)
			}
			if e == nil {
				e = allowedPath(row.Target, roots)
			}
			if e == nil && row.Target == m.Source {
				row.Status = "unchanged"
				row.Reason = "已符合命名规则"
			}
			if e == nil && row.Status == "ready" {
				_, statErr := os.Lstat(row.Target)
				if statErr == nil {
					e = errors.New("目标文件已存在，不覆盖")
				}
				if statErr != nil && !os.IsNotExist(statErr) {
					e = statErr
				}
			}
		}
		if e != nil {
			row.Status = "blocked"
			row.Reason = e.Error()
			if errors.Is(e, errDirectoryGrant) || os.IsPermission(e) {
				row = metadataRuleRow(m, targetRoot, formats, rules...)
			}
		}
		i := len(plan.Rows)
		plan.Rows = append(plan.Rows, row)
		if row.Target != "" {
			targets[row.Target] = append(targets[row.Target], i)
		}
		sources[m.Source] = append(sources[m.Source], i)
	}
	for _, indices := range targets {
		if len(indices) > 1 {
			for _, i := range indices {
				plan.Rows[i].Status = "blocked"
				plan.Rows[i].Reason = "多份视频指向同一目标，请先选择保留的版本"
			}
		}
	}
	for _, indices := range sources {
		if len(indices) > 1 {
			for _, i := range indices {
				plan.Rows[i].Status = "blocked"
				plan.Rows[i].Reason = "同一源文件出现在多个条目中"
			}
		}
	}
	return plan, nil
}
func fingerprintUnchanged(p string, original os.FileInfo) error {
	info, err := os.Stat(p)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || original == nil || !os.SameFile(info, original) || info.Size() != original.Size() || !info.ModTime().Equal(original.ModTime()) {
		return errors.New("预览后文件发生变化，请重新预览")
	}
	return nil
}
func syncDir(p string) {
	f, e := os.Open(p)
	if e == nil {
		_ = f.Sync()
		_ = f.Close()
	}
}

// Same-filesystem move without replacing an existing target.
func moveFile(row Row, roots []string) (string, error) {
	if err := allowedPath(row.Source, roots); err != nil {
		return "", err
	}
	if err := allowedPath(row.Target, roots); err != nil {
		return "", err
	}
	if err := fingerprintUnchanged(row.Source, row.info); err != nil {
		return "", err
	}
	parent := filepath.Dir(row.Target)
	for {
		if _, e := os.Stat(parent); e == nil {
			break
		}
		next := filepath.Dir(parent)
		if next == parent {
			return "", errors.New("目标父目录不可访问")
		}
		parent = next
	}
	parentInfo, err := os.Stat(parent)
	if err != nil {
		return "", err
	}
	if !sameVolume(row.info, parentInfo, row.Source, parent) {
		return "", errors.New("跨存储空间，拒绝移动")
	}
	if err := os.MkdirAll(filepath.Dir(row.Target), 0755); err != nil {
		return "", err
	}
	if err := allowedPath(row.Target, roots); err != nil {
		return "", err
	}
	if _, err := os.Lstat(row.Target); err == nil {
		return "", errors.New("目标已经存在，已跳过")
	} else if !os.IsNotExist(err) {
		return "", err
	}
	mode := "same-volume"
	if err := fingerprintUnchanged(row.Source, row.info); err != nil {
		return "", err
	}
	if err := moveNoReplace(row.Source, row.Target); err != nil {
		return "", fmt.Errorf("移动失败，原文件保留: %w", err)
	}
	syncDir(filepath.Dir(row.Target))
	syncDir(filepath.Dir(row.Source))
	return mode, nil
}

type Journal struct{ Path string }

func (j Journal) Write(value any) error {
	f, err := os.OpenFile(j.Path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	if err = json.NewEncoder(f).Encode(value); err != nil {
		return err
	}
	return f.Sync()
}
