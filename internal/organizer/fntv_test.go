package organizer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFnTVScan(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var data any
		if !strings.Contains(r.Header.Get("Authx"), "sign=") {
			t.Error("missing signature")
		}
		if r.URL.Path != "/v/api/v2/user/loginByPassword" && r.Header.Get("Authorization") != "fixture-token" {
			t.Error("missing auth token")
		}
		switch r.URL.Path {
		case "/v/api/v2/user/loginByPassword":
			var body object
			json.NewDecoder(r.Body).Decode(&body)
			h := sha256.Sum256([]byte("fixture-password"))
			if str(body, "password") != hex.EncodeToString(h[:]) {
				t.Error("password not hashed")
			}
			data = object{"token": "fixture-token"}
		case "/v/api/v1/sys/version":
			data = object{"version": "0.9.8", "mediasrvVersion": "0.8.42"}
		case "/v/api/v1/mdb/list":
			data = []object{{"guid": "library", "name": "电影"}}
		case "/v/api/v1/item/list":
			data = object{"total": 2, "list": []object{{"guid": "movie", "title": "电影", "type": "Movie", "trim_id": "t1", "release_date": "2024-01-01"}, {"guid": "tv", "title": "电视剧", "type": "TV", "trim_id": "t2", "release_date": "2020-01-01"}}}
		case "/v/api/v1/season/list/tv":
			if r.Method != "GET" {
				t.Error("season list must use GET")
			}
			data = []object{{"guid": "season1", "season_number": 1}, {"guid": "season2", "season_number": 2}}
		case "/v/api/v1/episode/list/season1":
			data = []object{{"title": "重逢", "episode_number": 2}}
		case "/v/api/v1/episode/list/season2":
			data = []object{{"title": "新的开始", "season_number": 2, "episode_number": 2}}
		case "/v/api/v1/item/movie":
			data = object{"guid": "movie", "title": "已校正片名", "release_date": "2024-01-01"}
		case "/v/api/v1/item/tv":
			data = object{"guid": "tv", "title": "电视剧", "release_date": "2020-01-01"}
		case "/v/api/v1/media/itemfile/movie":
			data = []object{{"media_guid": "m1", "file_path": "/vol1/media/movie.mkv"}}
		case "/v/api/v1/media/itemfile/tv":
			data = []object{{"media_guid": "m2", "file_path": "/vol1/media/episode.mp4", "season_number": 1, "episode_number": 2}}
		default:
			t.Errorf("unexpected endpoint: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		jsonResponse(w, 200, object{"code": 0, "data": data})
	}))
	defer server.Close()
	f, e := newFnTV(server.URL + "/v")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = f.Login(context.Background(), "fixture-user", "fixture-password"); e != nil {
		t.Fatal(e)
	}
	libs, e := f.Libraries(context.Background())
	if e != nil || len(libs) != 1 {
		t.Fatal("libraries missing")
	}
	items, _, e := f.Scan(context.Background(), "library", func(string) {})
	if e != nil || len(items) != 2 {
		t.Fatalf("scan: %v %+v", e, items)
	}
	if items[0].Title != "已校正片名" || items[1].Kind != "episode" || items[1].Episode != 2 || items[1].EpisodeTitle != "重逢" {
		t.Fatal("metadata mapping incorrect")
	}
}
func TestAuthAndOrigin(t *testing.T) {
	a := &App{sessions: map[string]*Session{}}
	r := httptest.NewRequest("GET", "/api/state", nil)
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatal("unauthenticated access")
	}
	r = httptest.NewRequest("POST", "/api/login", strings.NewReader(`{"key":"anything"}`))
	r.Header.Set("Origin", "https://other.example")
	r.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("foreign origin accepted")
	}
}
