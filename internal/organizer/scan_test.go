package organizer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
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

func TestLargeSeriesScanAndLightweightPolling(t *testing.T) {
	const series, episodes = 12, 300
	dir := t.TempDir()
	var inFlight, peak atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var data any
		path := r.URL.Path
		id := path[strings.LastIndexByte(path, '/')+1:]
		switch {
		case path == "/v/api/v1/user/info":
			data = object{"is_admin": true, "username": "fixture"}
		case path == "/v/api/v1/mdb/library":
			data = object{"dir_list": []string{dir}}
		case path == "/v/api/v1/item/list":
			var titles []object
			for i := range series {
				titles = append(titles, object{"guid": fmt.Sprint(i), "title": fmt.Sprintf("剧集%d", i), "type": "TV", "trim_id": "recognized"})
			}
			data = object{"total": series, "list": titles}
		case strings.HasPrefix(path, "/v/api/v1/item/"):
			active := inFlight.Add(1)
			defer inFlight.Add(-1)
			for current := peak.Load(); active > current; current = peak.Load() {
				if peak.CompareAndSwap(current, active) {
					break
				}
			}
			time.Sleep(20 * time.Millisecond)
			data = object{"release_date": "2024-01-01"}
		case strings.HasPrefix(path, "/v/api/v1/media/itemfile/"):
			files := make([]object, episodes)
			for i := range files {
				key := fmt.Sprintf("%s-%d", id, i+1)
				files[i] = object{"media_guid": key, "file_path": filepath.Join(dir, key+".mkv"), "season_number": 1, "episode_number": i + 1}
			}
			data = files
		case strings.HasPrefix(path, "/v/api/v1/season/list/"):
			data = []object{{"guid": id, "season_number": 1}}
		case strings.HasPrefix(path, "/v/api/v1/episode/list/"):
			data = []object{{"episode_number": 1, "title": "首集"}}
		default:
			http.NotFound(w, r)
			return
		}
		jsonResponse(w, 200, object{"code": 0, "data": data})
	}))
	defer upstream.Close()
	fn, _ := newFnTV(upstream.URL)
	s := &Session{expires: time.Now().Add(time.Hour), fn: fn, state: State{Libraries: []object{{"guid": "library"}}}}
	a := &App{sessions: map[string]*Session{"1000": s}, ctx: context.Background(), settings: Settings{Rules: defaultRules()}}
	r := authorizedTestRequest(httptest.NewRequest("POST", "/api/scan", strings.NewReader(`{"library":"library"}`)))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Organizer-Request", "1")
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != http.StatusAccepted {
		t.Fatalf("scan status: %d %s", w.Code, w.Body)
	}
	a.jobs.Wait()
	if peak.Load() < 2 || peak.Load() > 4 {
		t.Fatalf("expected bounded parallel item reads, peak = %d", peak.Load())
	}
	poll := func(revision string) (State, int) {
		t.Helper()
		w := httptest.NewRecorder()
		a.ServeHTTP(w, authorizedTestRequest(httptest.NewRequest("GET", "/api/state?files_revision="+revision, nil)))
		var result struct{ State State }
		if err := json.Unmarshal(w.Body.Bytes(), &result); w.Code != 200 || err != nil {
			t.Fatalf("poll failed: %d %v", w.Code, err)
		}
		return result.State, w.Body.Len()
	}
	full, fullSize := poll("older-revision")
	if full.Busy || full.Error != "" || len(full.Items) != series*episodes || full.Inspection == nil || len(full.Inspection.Rows) != len(full.Items) || full.FilesRevision == "" {
		t.Fatalf("scan incomplete: %d files, error %q", len(full.Items), full.Error)
	}
	for i, m := range full.Items {
		want := fmt.Sprintf("%d-%d", i/episodes, i%episodes+1)
		if m.ID != want || full.Inspection.Rows[i].ID != want {
			t.Fatalf("file order at %d: %q, want %q", i, m.ID, want)
		}
	}
	if full.Items[0].EpisodeTitle != "首集" {
		t.Fatal("episode titles lost during scan")
	}
	unchanged, pollSize := poll(full.FilesRevision)
	if unchanged.Items != nil || unchanged.Inspection != nil || unchanged.FilesRevision != full.FilesRevision || pollSize >= fullSize/10 {
		t.Fatalf("unchanged library retransmitted: %d bytes vs %d", pollSize, fullSize)
	}
	t.Logf("full library response: %d bytes, unchanged poll: %d bytes", fullSize, pollSize)
}
