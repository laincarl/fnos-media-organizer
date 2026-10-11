package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestScanAutomaticallyInspects(t *testing.T) {
	src, _, roots := fixture(t)
	m := video(t, src, "recognized.xyz")
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var data any
		switch r.URL.Path {
		case "/v/api/v1/user/info":
			data = object{"is_admin": true, "username": "fixture"}
		case "/v/api/v1/mdb/library":
			data = object{"dir_list": []string{src}}
		case "/v/api/v1/item/list":
			data = object{"total": 1, "list": []object{{"guid": "movie", "title": m.Title, "type": "Movie", "trim_id": "recognized", "release_date": "2024-01-01"}}}
		case "/v/api/v1/item/movie":
			data = object{"title": m.Title, "release_date": "2024-01-01"}
		case "/v/api/v1/media/itemfile/movie":
			data = []object{{"media_guid": "file", "file_path": m.Source}}
		default:
			t.Errorf("unexpected endpoint %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		jsonResponse(w, 200, object{"code": 0, "data": data})
	}))
	defer upstream.Close()
	fn, _ := newFnTV(upstream.URL)
	fn.Token = "fixture"
	s := &Session{expires: time.Now().Add(time.Hour), fn: fn, state: State{Libraries: []object{{"guid": "library"}}}}
	a := &App{sessions: map[string]*Session{"1000": s}, mediaBase: upstream.URL, roots: roots, ctx: context.Background(), settings: Settings{Rules: defaultRules()}}
	r := httptest.NewRequest("POST", "/api/scan", bytes.NewBufferString(`{"library":"library"}`))
	r = authorizedTestRequest(r)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Organizer-Request", "1")
	r.Header.Set("Origin", "http://nas")
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	a.jobs.Wait()
	if s.state.Busy || s.state.Inspection == nil || len(s.state.Inspection.Rows) != 1 || s.state.Inspection.Rows[0].Status != "ready" {
		t.Fatalf("automatic inspection failed: %+v", s.state)
	}
	if filepath.Ext(s.state.Inspection.Rows[0].Target) != ".xyz" {
		t.Fatal("recognized extension lost")
	}
}
