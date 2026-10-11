package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

type unixTestConn struct{ net.Conn }

func (unixTestConn) LocalAddr() net.Addr { return &net.UnixAddr{Name: "test.sock", Net: "unix"} }
func gatewayRequest(r *http.Request, uid, name string) *http.Request {
	r = r.WithContext(context.WithValue(r.Context(), gatewayContextKey{}, unixTestConn{}))
	r.Header.Set("X-Trim-Userid", uid)
	r.Header.Set("X-Trim-Username", name)
	r.Header.Set("X-Trim-Isadmin", "true")
	return r
}
func authorizedTestRequest(r *http.Request) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), gatewayIdentity{}, gatewayIdentity{"1000", "fixture"}))
}

func TestGatewayTrustBoundary(t *testing.T) {
	a := &App{sessions: map[string]*Session{}}
	h := gatewayHandler(a)
	r := httptest.NewRequest("GET", gatewayPrefix+"/", nil)
	r.Header.Set("X-Trim-Userid", "1000")
	r.Header.Set("X-Trim-Isadmin", "true")
	r.Header.Set("X-Trim-Username", "admin")
	r.AddCookie(&http.Cookie{Name: "Trim-MC-token", Value: "ignored"})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("TCP identity spoof accepted")
	}
	r = gatewayRequest(r, "1000", "admin")
	r.Header.Set("X-Trim-Isadmin", "false")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("non-admin accepted")
	}
	r.Header.Set("X-Trim-Isadmin", "true")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), gatewayPrefix+"/app.js") {
		t.Fatal("gateway assets not served")
	}
	r = httptest.NewRequest("GET", gatewayPrefix+"/api/state", nil)
	r = gatewayRequest(r, "1000", "admin")
	r.AddCookie(&http.Cookie{Name: "Trim-MC-token", Value: "ignored"})
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatal("shared cookie accepted")
	}
}

func TestGatewayOAuth(t *testing.T) {
	exchanges := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var data any
		switch r.URL.Path {
		case "/v/api/v1/sys/config":
			data = object{"nas_oauth": object{"app_id": "fntv-client"}}
		case "/v/api/v1/auth":
			exchanges++
			var body object
			json.NewDecoder(r.Body).Decode(&body)
			if str(body, "source") != "Trim-NAS" || str(body, "code") != "one-time-code" {
				t.Error("invalid exchange")
			}
			data = object{"token": "server-only-token"}
		case "/v/api/v1/user/info":
			data = object{"is_admin": true, "username": "admin"}
		case "/v/api/v1/mdb/list":
			data = []object{{"guid": "library"}}
		case "/v/api/v1/sys/version":
			data = object{"version": "0.9.8"}
		default:
			t.Error("unexpected endpoint", r.URL.Path)
		}
		jsonResponse(w, 200, object{"code": 0, "data": data})
	}))
	defer upstream.Close()
	a := &App{sessions: map[string]*Session{}, mediaBase: upstream.URL}
	h := gatewayHandler(a)
	r := httptest.NewRequest("POST", "http://nas"+gatewayPrefix+"/api/auth/start", strings.NewReader(`{"origin":"http://nas"}`))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Organizer-Request", "1")
	r.Header.Set("Origin", "http://nas")
	r.Host = "gateway.internal"
	r = gatewayRequest(r, "1000", "admin")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var response object
	json.Unmarshal(w.Body.Bytes(), &response)
	u, _ := url.Parse(str(response, "url"))
	nonce := u.Query().Get("state")
	if u.Query().Get("redirect_uri") != "http://nas"+gatewayPrefix+"/oauth/callback" || nonce == "" {
		t.Fatal("bad callback")
	}
	callback := func(uid string) int {
		r := gatewayRequest(httptest.NewRequest("GET", gatewayPrefix+"/oauth/callback?state="+nonce+"&code=one-time-code", nil), uid, "admin")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	// A mismatching identity cannot redeem another user's pending grant.
	saved := a.oauth[nonce]
	if callback("1001") != 400 || exchanges != 0 {
		t.Fatal("cross-user exchange")
	}
	a.oauth[nonce] = saved
	if callback("1000") != 303 || exchanges != 1 || a.sessions["1000"].fn.Token != "server-only-token" {
		t.Fatal("exchange failed")
	}
	if callback("1000") != 400 || exchanges != 1 {
		t.Fatal("grant replay")
	}
	a.oauth[nonce] = oauthPending{saved.User, time.Now().Add(-time.Minute)}
	if callback("1000") != 400 || exchanges != 1 {
		t.Fatal("expired grant")
	}
	if a.sessions["1001"] != nil {
		t.Fatal("session leaked")
	}
}

func TestGatewayRejectsCrossOriginWrites(t *testing.T) {
	a := &App{sessions: map[string]*Session{}}
	for _, headers := range []map[string]string{
		{"Content-Type": "application/json"},
		{"Content-Type": "application/json", "X-Organizer-Request": "1", "Sec-Fetch-Site": "cross-site"},
	} {
		r := gatewayRequest(httptest.NewRequest("POST", gatewayPrefix+"/api/auth/start", strings.NewReader(`{"origin":"https://evil.example"}`)), "1000", "admin")
		for k, v := range headers {
			r.Header.Set(k, v)
		}
		w := httptest.NewRecorder()
		gatewayHandler(a).ServeHTTP(w, r)
		if w.Code != 403 || w.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Fatal("cross-origin write accepted")
		}
	}
	r := gatewayRequest(httptest.NewRequest("OPTIONS", gatewayPrefix+"/api/auth/start", nil), "1000", "admin")
	r.Header.Set("Origin", "https://evil.example")
	r.Header.Set("Access-Control-Request-Headers", "x-organizer-request,content-type")
	w := httptest.NewRecorder()
	gatewayHandler(a).ServeHTTP(w, r)
	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("CORS preflight granted")
	}
}
