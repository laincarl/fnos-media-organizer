package organizer

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSettingsCheckAndExecute(t *testing.T) {
	src, _, roots := fixture(t)
	m := video(t, src, "fixture.mkv")
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 200, object{"code": 0, "data": object{"is_admin": true, "username": "fixture"}})
	}))
	defer upstream.Close()
	fn, _ := newFnTV(upstream.URL)
	fn.Token = "fixture"
	s := &Session{fn: fn, expires: time.Now().Add(time.Hour), state: State{Items: []Media{m}, Directories: []string{src}}}
	a := &App{sessions: map[string]*Session{"1000": s}, mediaBase: upstream.URL, roots: roots, settingsPath: filepath.Join(roots[0], "settings.json"), journal: Journal{filepath.Join(roots[0], "operations.jsonl")}, ctx: context.Background()}
	request := func(path string, v any) (int, []byte) {
		t.Helper()
		b, _ := json.Marshal(v)
		r := httptest.NewRequest("POST", path, bytes.NewReader(b))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-Organizer-Request", "1")
		r.Header.Set("Origin", "http://nas")
		r = authorizedTestRequest(r)
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		return w.Code, w.Body.Bytes()
	}
	settings := Settings{Rules: defaultRules()}
	if status, b := request("/api/settings", settings); status != 200 {
		t.Fatalf("save: %d %s", status, b)
	}
	saved := loadSettings(a.settingsPath)
	if saved.Rules != defaultRules() {
		t.Fatal("settings did not persist")
	}
	payload := object{"ids": []string{m.ID}}
	if status, b := request("/api/check", payload); status != 200 {
		t.Fatalf("check: %d %s", status, b)
	}
	if s.state.Inspection == nil || s.state.Plan != nil {
		t.Fatal("inspection should not create executable plan")
	}
	if _, e := os.Stat(m.Source); e != nil {
		t.Fatal("inspection changed source")
	}
	status, b := request("/api/preview", payload)
	if status != 200 {
		t.Fatalf("preview: %d %s", status, b)
	}
	var plan Plan
	json.Unmarshal(b, &plan)
	a.roots = nil
	status, b = request("/api/preview", payload)
	if status != 403 || !bytes.Contains(b, []byte("permission_issues")) {
		t.Fatalf("missing grant preview: %d %s", status, b)
	}
	status, b = request("/api/execute", object{"plan_id": plan.ID, "indices": []int{0}})
	if status != 403 || s.state.Busy || !bytes.Contains(b, []byte("permission_issues")) {
		t.Fatalf("missing grant execute: %d %s", status, b)
	}
	if _, err := os.Stat(m.Source); err != nil {
		t.Fatal("permission rejection changed source", err)
	}
	a.roots = roots
	status, b = request("/api/execute", object{"plan_id": plan.ID, "indices": []int{0}})
	if status != 202 {
		t.Fatalf("execute: %d %s", status, b)
	}
	a.jobs.Wait()
	if len(s.state.Results) != 1 || s.state.Results[0].Status != "done" {
		t.Fatal("execution failed")
	}
	if s.state.Task == nil || s.state.Task.Active || s.state.Task.Total != 1 || s.state.Task.Completed != 1 || s.state.Task.Succeeded != 1 || s.state.Task.Failed != 0 || s.state.Task.Current != "" {
		t.Fatalf("incorrect completed task progress: %+v", s.state.Task)
	}
	if s.state.Task.CurrentID != "" || len(s.state.Task.FileIDs) != 1 || s.state.Task.FileIDs[0] != m.ID {
		t.Fatalf("incorrect task file tracking: %+v", s.state.Task)
	}
	if s.state.Items[0].Source != plan.Rows[0].Target {
		t.Fatal("library source not updated after moving")
	}
	if s.state.Plan != nil || s.state.Inspection == nil || s.state.Inspection.Rows[0].Status != "unchanged" {
		t.Fatal("stale plan or inspection retained")
	}
	log, e := os.ReadFile(a.journal.Path)
	if e != nil || bytes.Count(log, []byte("\n")) != 2 {
		t.Fatal("missing durable operation journal")
	}
	if status, _ = request("/api/execute", object{"plan_id": plan.ID, "indices": []int{0}}); status == 202 {
		t.Fatal("plan replay accepted")
	}
	// A failed move still counts as processed, but must not count as successful.
	failedMedia := video(t, src, "failure.mkv")
	failedMedia.Title = "失败测试"
	s.state.Items = []Media{failedMedia}
	status, b = request("/api/preview", object{"ids": []string{failedMedia.ID}})
	if status != 200 {
		t.Fatalf("failure preview: %d %s", status, b)
	}
	json.Unmarshal(b, &plan)
	if err := os.Remove(failedMedia.Source); err != nil {
		t.Fatal(err)
	}
	if status, b = request("/api/execute", object{"plan_id": plan.ID, "indices": []int{0}}); status != 202 {
		t.Fatalf("failure execute: %d %s", status, b)
	}
	a.jobs.Wait()
	if s.state.Task.Active || s.state.Task.Completed != 1 || s.state.Task.Succeeded != 0 || s.state.Task.Failed != 1 {
		t.Fatalf("incorrect failure progress: %+v", s.state.Task)
	}
	// Shutdown before the next file starts must retain zero completed files.
	stoppedMedia := video(t, src, "stopped.mkv")
	stoppedMedia.Title = "停止测试"
	s.state.Items = []Media{stoppedMedia}
	status, b = request("/api/preview", object{"ids": []string{stoppedMedia.ID}})
	if status != 200 {
		t.Fatalf("stopped preview: %d %s", status, b)
	}
	json.Unmarshal(b, &plan)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	a.ctx = ctx
	if status, b = request("/api/execute", object{"plan_id": plan.ID, "indices": []int{0}}); status != 202 {
		t.Fatalf("stopped execute: %d %s", status, b)
	}
	a.jobs.Wait()
	if s.state.Task.Active || s.state.Task.Completed != 0 || s.state.Task.Total != 1 {
		t.Fatalf("stopped task reported completion: %+v", s.state.Task)
	}
	if _, err := os.Stat(stoppedMedia.Source); err != nil {
		t.Fatal("stopped task moved source")
	}
}
