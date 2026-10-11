package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T) (string, string, []string) {
	t.Helper()
	root := t.TempDir()
	source := filepath.Join(root, "source")
	target := filepath.Join(root, "target")
	for _, p := range []string{source, target} {
		if e := os.Mkdir(p, 0755); e != nil {
			t.Fatal(e)
		}
	}
	return source, target, []string{root}
}
func video(t *testing.T, root, name string) Media {
	t.Helper()
	p := filepath.Join(root, name)
	if e := os.WriteFile(p, []byte("test media content"), 0600); e != nil {
		t.Fatal(e)
	}
	return Media{ID: name, Title: "电影：示例", Year: "2024", Kind: "movie", Source: p}
}
func TestPreviewAndMove(t *testing.T) {
	src, dst, roots := fixture(t)
	m := video(t, src, "original.MKV")
	p, e := buildPlan([]Media{m}, src, dst, roots, mediaExtensions)
	if e != nil {
		t.Fatal(e)
	}
	r := p.Rows[0]
	if r.Status != "ready" || !strings.HasSuffix(r.Target, "电影：示例 (2024).MKV") {
		t.Fatalf("bad row: %+v", r)
	}
	if _, e = moveFile(r, roots); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(m.Source); !os.IsNotExist(e) {
		t.Fatal("source still exists")
	}
	b, e := os.ReadFile(r.Target)
	if e != nil || string(b) != "test media content" {
		t.Fatal("target content differs")
	}
}
func TestNoOverwrite(t *testing.T) {
	src, dst, roots := fixture(t)
	m := video(t, src, "original.mkv")
	p, e := buildPlan([]Media{m}, src, dst, roots, mediaExtensions)
	if e != nil {
		t.Fatal(e)
	}
	r := p.Rows[0]
	os.MkdirAll(filepath.Dir(r.Target), 0755)
	os.WriteFile(r.Target, []byte("existing"), 0600)
	if _, e = moveFile(r, roots); e == nil {
		t.Fatal("should refuse existing target")
	}
	b, _ := os.ReadFile(r.Target)
	if string(b) != "existing" {
		t.Fatal("overwrote target")
	}
	if _, e = os.Stat(m.Source); e != nil {
		t.Fatal("lost source")
	}
}
func TestChangedSource(t *testing.T) {
	src, dst, roots := fixture(t)
	m := video(t, src, "a.mp4")
	p, _ := buildPlan([]Media{m}, src, dst, roots, mediaExtensions)
	os.WriteFile(m.Source, []byte("changed"), 0600)
	if _, e := moveFile(p.Rows[0], roots); e == nil {
		t.Fatal("changed file should be rejected")
	}
	if _, e := os.Stat(p.Rows[0].Target); !os.IsNotExist(e) {
		t.Fatal("created destination for changed source")
	}
}
func TestDuplicatesAndMetadata(t *testing.T) {
	src, dst, roots := fixture(t)
	a := video(t, src, "a.mkv")
	b := video(t, src, "b.mkv")
	p, _ := buildPlan([]Media{a, b}, src, dst, roots, mediaExtensions)
	for _, r := range p.Rows {
		if r.Status != "blocked" {
			t.Fatal("duplicate target accepted")
		}
	}
	a.Year = ""
	p, _ = buildPlan([]Media{a}, src, dst, roots, mediaExtensions)
	if p.Rows[0].Status != "blocked" {
		t.Fatal("missing year accepted")
	}
	a.Year = "2024"
	a.Kind = "episode"
	a.Season = -1
	a.Episode = 1
	p, _ = buildPlan([]Media{a}, src, dst, roots, mediaExtensions)
	if p.Rows[0].Status != "blocked" {
		t.Fatal("missing season accepted")
	}
	a.Season = 0
	p, _ = buildPlan([]Media{a}, src, dst, roots, mediaExtensions)
	if p.Rows[0].Status != "ready" || !strings.Contains(p.Rows[0].Target, "Season 00") {
		t.Fatal("special season rejected")
	}
}
func TestFormatsAndTraversal(t *testing.T) {
	src, dst, roots := fixture(t)
	m := video(t, src, "a.mp4")
	formats, e := parseFormats([]string{"MKV", ".rmvb"})
	if e != nil || !formats[".mkv"] || !formats[".rmvb"] {
		t.Fatal("invalid format parsing")
	}
	p, _ := buildPlan([]Media{m}, src, dst, roots, formats)
	if p.Rows[0].Status != "blocked" {
		t.Fatal("unselected extension accepted")
	}
	if _, e = parseFormats([]string{"../mkv"}); e == nil {
		t.Fatal("invalid extension accepted")
	}
	if e = allowedPath(filepath.Join(roots[0], "..", "escape"), roots); e == nil {
		t.Fatal("traversal accepted")
	}
}
func TestSymlinksRejected(t *testing.T) {
	src, dst, roots := fixture(t)
	m := video(t, src, "a.mkv")
	link := filepath.Join(src, "link.mkv")
	if e := os.Symlink(m.Source, link); e != nil {
		t.Skip("symlink requires permission")
	}
	m.Source = link
	p, _ := buildPlan([]Media{m}, src, dst, roots, mediaExtensions)
	if p.Rows[0].Status != "blocked" {
		t.Fatal("symlink accepted")
	}
}
func TestSameVolumeOnly(t *testing.T) {
	src, dst, roots := fixture(t)
	if _, e := os.Stat("/dev/shm"); e != nil {
		t.Skip("requires Linux shared memory mount")
	}
	other, e := os.MkdirTemp("/dev/shm", "organizer-test-")
	if e != nil {
		t.Skip(e)
	}
	defer os.RemoveAll(other)
	m := video(t, src, "a.mkv")
	roots = append(roots, other)
	a, _ := os.Stat(src)
	b, _ := os.Stat(other)
	if sameVolume(a, b, src, other) {
		t.Skip("same device")
	}
	p, e := buildPlan([]Media{m}, src, other, roots, mediaExtensions)
	if e != nil || p.Rows[0].Status != "blocked" || !strings.Contains(p.Rows[0].Reason, "跨存储") {
		t.Fatalf("cross device accepted: %v %+v", e, p)
	}
	p, _ = buildPlan([]Media{m}, src, dst, roots, mediaExtensions)
	r := p.Rows[0]
	r.Target = filepath.Join(other, "a.mkv")
	if _, e = moveFile(r, roots); e == nil {
		t.Fatal("execution allowed cross device")
	}
	if _, e = os.Stat(m.Source); e != nil {
		t.Fatal("lost original")
	}
}
