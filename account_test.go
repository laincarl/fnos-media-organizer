package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCurrentMediaAccount(t *testing.T) {
	admin := true
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "current-token" {
			jsonResponse(w, 401, object{})
			return
		}
		var data any
		switch r.URL.Path {
		case "/v/api/v1/user/info":
			data = object{"is_admin": admin, "username": "current-user"}
		case "/v/api/v1/mdb/list":
			data = []object{{"guid": "library"}}
		case "/v/api/v1/sys/version":
			data = object{"version": "0.9.8"}
		default:
			http.NotFound(w, r)
			return
		}
		jsonResponse(w, 200, object{"code": 0, "data": data})
	}))
	defer upstream.Close()
	fn, _ := newFnTV(upstream.URL)
	fn.Token = "current-token"
	a := &App{sessions: map[string]*Session{"1000": {expires: time.Now().Add(time.Hour), fn: fn}}, mediaBase: upstream.URL, ctx: context.Background()}
	request := func(token string) int {
		r := httptest.NewRequest("GET", "/api/state", nil)
		if token == "current-token" {
			r = r.WithContext(context.WithValue(r.Context(), gatewayIdentity{}, gatewayIdentity{"1000", "current-user"}))
		}
		if token != "" {
			r.AddCookie(&http.Cookie{Name: "Trim-MC-token", Value: token})
		}
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		return w.Code
	}
	for _, token := range []string{"", "invalid"} {
		if request(token) != 401 {
			t.Fatal("unverified cookie accepted")
		}
	}
	if request("current-token") != 200 {
		t.Fatal("current account was not reused")
	}
	admin = false
	if request("current-token") != 401 {
		t.Fatal("administrator revocation ignored")
	}
}

func TestAutomaticLibraryRoots(t *testing.T) {
	src, dst, roots := fixture(t)
	a := video(t, src, "a.mkv")
	b := video(t, dst, "b.mkv")
	b.ID = "second"
	b.Title = "第二部"
	p, e := buildLibraryPlan([]Media{a, b}, []string{src, dst}, roots, mediaExtensions)
	if e != nil || len(p.Rows) != 2 {
		t.Fatalf("plan: %v", e)
	}
	if !inside(src, p.Rows[0].Target) || !inside(dst, p.Rows[1].Target) {
		t.Fatal("file moved out of its own library root")
	}
	p, e = buildLibraryPlan([]Media{a}, []string{dst}, roots, mediaExtensions)
	if e != nil || p.Rows[0].Status != "blocked" {
		t.Fatal("outside-library file accepted")
	}
}

func TestPosterProxyPaths(t *testing.T) {
	for _, p := range []string{"https://example.com/image", "//example.com/image", "../secret", "/v/api/v1/sys/img/../secret", "a?token=secret", "a\\b"} {
		if _, e := posterPath(p); e == nil {
			t.Fatalf("unsafe image path: %s", p)
		}
	}
	if p, e := posterPath("/v/api/v1/sys/img/9d/02/poster.webp"); e != nil || p != "/v/api/v1/sys/img/9d/02/poster.webp?w=160" {
		t.Fatalf("valid poster: %s %v", p, e)
	}
}

func TestClassifiedLibraryDoesNotNestCategory(t *testing.T) {
	src, _, roots := fixture(t)
	root := filepath.Join(src, "电影")
	folder := filepath.Join(root, "哆啦A梦：大雄的地球交响乐 (2024)")
	if e := os.MkdirAll(folder, 0700); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(folder, "哆啦A梦：大雄的地球交响乐 (2024).mkv")
	if e := os.WriteFile(path, []byte("fixture"), 0600); e != nil {
		t.Fatal(e)
	}
	m := Media{ID: "already-organized", Title: "哆啦A梦：大雄的地球交响乐", Year: "2024", Kind: "movie", Source: path}
	p, e := buildLibraryPlan([]Media{m}, []string{root}, roots, mediaExtensions)
	if e != nil || p.Rows[0].Status != "unchanged" || p.Rows[0].Target != path {
		t.Fatalf("already organized: %v %+v", e, p)
	}
	for _, category := range []string{"电视剧", "综艺"} {
		tvRoot := filepath.Join(src, category)
		m.Kind = "episode"
		m.Season = 1
		m.Episode = 2
		target, e := targetPath(m, tvRoot, mediaExtensions)
		expected := filepath.Join(tvRoot, "哆啦A梦：大雄的地球交响乐 (2024)", "Season 01", "哆啦A梦：大雄的地球交响乐 - S01E02.mkv")
		if e != nil || target != expected {
			t.Fatalf("TV category duplicated: %s %v", target, e)
		}
	}
}
