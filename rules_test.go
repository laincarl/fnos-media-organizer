package main

import (
	"path/filepath"
	"testing"
)

func TestCustomRulesAndRecognizedExtensions(t *testing.T) {
	src, _, roots := fixture(t)
	m := video(t, src, "recognized.custom")
	r := defaultRules()
	r.MovieDirectory = "{title}"
	r.MovieFilename = "{title} - {year}"
	p, e := buildLibraryPlan([]Media{m}, []string{src}, roots, nil, r)
	expected := filepath.Join(src, m.Title, m.Title+" - "+m.Year+".custom")
	if e != nil || p.Rows[0].Status != "ready" || p.Rows[0].Target != expected {
		t.Fatalf("custom extension/rule: %v %+v", e, p)
	}
	m.Year = ""
	r.MovieFilename = "{title}"
	if _, e = ruleTarget(m, src, r); e != nil {
		t.Fatal("unused year must not block", e)
	}
	r.MovieFilename = "{title} ({year})"
	if _, e = ruleTarget(m, src, r); e == nil {
		t.Fatal("required year missing")
	}
}
func TestRulesRejectTraversalAndUnknownTokens(t *testing.T) {
	for _, bad := range []string{"../{title}", "/{title}", "{title}/../../escape", "{title}/{unknown}", "{title}\\escape", "{title}//a", "{title}/{year}/.."} {
		r := defaultRules()
		r.MovieDirectory = bad
		if validateRules(r) == nil {
			t.Fatal("unsafe rule accepted", bad)
		}
	}
	r := defaultRules()
	r.MovieFilename = "{title}/{year}"
	if validateRules(r) == nil {
		t.Fatal("filename separator accepted")
	}
}

func TestEpisodeCategoryAtLibraryRoot(t *testing.T) {
	m := Media{Kind: "episode", Title: "花儿与少年", Year: "2023", Season: 5, Episode: 1, Source: "original.mp4"}
	for _, name := range []string{"剧集", "电视剧", "综艺", "影视"} {
		root := filepath.Join(t.TempDir(), name)
		wantRoot := root
		if name == "影视" {
			wantRoot = filepath.Join(root, "电视剧")
		}
		want := filepath.Join(wantRoot, "花儿与少年 (2023)", "Season 05", "花儿与少年 - S05E01.mp4")
		for _, candidate := range []string{root, root + string(filepath.Separator)} {
			got, err := ruleTarget(m, candidate, defaultRules())
			if err != nil || got != want {
				t.Fatalf("root %q: got %q, want %q, error %v", candidate, got, want, err)
			}
		}
	}
}

func TestEpisodeTitleNaming(t *testing.T) {
	root := t.TempDir()
	m := Media{Kind: "episode", Title: "剧名", Year: "2024", Season: 1, Episode: 2, EpisodeTitle: "重逢：新的开始", Source: "original.mkv"}
	got, err := ruleTarget(m, root, defaultRules())
	want := filepath.Join(root, "电视剧", "剧名 (2024)", "Season 01", "剧名 - S01E02 - 重逢：新的开始.mkv")
	if err != nil || got != want {
		t.Fatalf("got %q, want %q, err %v", got, want, err)
	}
	m.EpisodeTitle = ""
	got, err = ruleTarget(m, root, defaultRules())
	if err != nil || filepath.Base(got) != "剧名 - S01E02.mkv" {
		t.Fatalf("missing episode title: %q %v", got, err)
	}
	r := defaultRules()
	r.EpisodeFilename = "{title} - {episode_title} - S{season}E{episode}"
	if _, err = ruleTarget(m, root, r); err == nil {
		t.Fatal("custom rule requiring missing episode title must skip")
	}
}
