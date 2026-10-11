package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const gatewayPrefix = "/app/media-organizer"

type gatewayIdentity struct{ ID, Name string }
type gatewayContextKey struct{}
type oauthPending struct {
	User    gatewayIdentity
	Expires time.Time
}

// Identity headers are accepted only on the Unix listener used by fnOS.
func gatewayHandler(a *App) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := r.Context().Value(gatewayContextKey{}).(net.Conn)
		if conn == nil || conn.LocalAddr().Network() != "unix" {
			http.Error(w, "请从 fnOS 应用入口访问", 403)
			return
		}
		uid := r.Header.Get("X-Trim-Userid")
		name := r.Header.Get("X-Trim-Username")
		n, e := strconv.ParseUint(uid, 10, 32)
		if e != nil || n == 0 || name == "" || r.Header.Get("X-Trim-Isadmin") != "true" {
			http.Error(w, "需要 NAS 管理员身份", 403)
			return
		}
		if r.URL.Path == gatewayPrefix {
			http.Redirect(w, r, gatewayPrefix+"/", http.StatusTemporaryRedirect)
			return
		}
		if !strings.HasPrefix(r.URL.Path, gatewayPrefix+"/") {
			http.NotFound(w, r)
			return
		}
		r = r.Clone(context.WithValue(r.Context(), gatewayIdentity{}, gatewayIdentity{uid, name}))
		r.URL.Path = strings.TrimPrefix(r.URL.Path, gatewayPrefix)
		a.ServeHTTP(w, r)
	})
}

func identity(r *http.Request) (gatewayIdentity, error) {
	u, ok := r.Context().Value(gatewayIdentity{}).(gatewayIdentity)
	if !ok {
		return u, errors.New("请通过 fnOS 统一网关打开应用")
	}
	return u, nil
}

func (a *App) oauthStart(w http.ResponseWriter, r *http.Request) {
	u, e := identity(r)
	if e != nil {
		errorResponse(w, e)
		return
	}
	var input struct {
		Origin string `json:"origin"`
	}
	if e = decode(w, r, &input); e != nil {
		errorResponse(w, e)
		return
	}
	origin, e := url.Parse(input.Origin)
	if e != nil || input.Origin != r.Header.Get("Origin") || origin.Host == "" || (origin.Scheme != "http" && origin.Scheme != "https") || origin.Path != "" || origin.User != nil || origin.RawQuery != "" || origin.Fragment != "" {
		errorResponse(w, errors.New("访问域名不匹配"))
		return
	}
	fn, e := newFnTV(a.mediaBase)
	if e != nil {
		errorResponse(w, e)
		return
	}
	config, e := fn.Call(r.Context(), "GET", "/v/api/v1/sys/config", nil)
	if e != nil {
		errorResponse(w, e)
		return
	}
	client := str(obj(obj(config)["nas_oauth"]), "app_id")
	if client == "" {
		errorResponse(w, errors.New("影视服务未提供 NAS OAuth 配置"))
		return
	}
	nonce := randomID()
	a.mu.Lock()
	if a.oauth == nil {
		a.oauth = map[string]oauthPending{}
	}
	for key, v := range a.oauth {
		if time.Now().After(v.Expires) {
			delete(a.oauth, key)
		}
	}
	a.oauth[nonce] = oauthPending{u, time.Now().Add(5 * time.Minute)}
	a.mu.Unlock()
	q := url.Values{"client_id": {client}, "redirect_uri": {input.Origin + gatewayPrefix + "/oauth/callback"}, "state": {nonce}, "app_name": {"飞牛影视整理器"}}
	jsonResponse(w, 200, object{"url": input.Origin + "/signin?" + q.Encode()})
}

func (a *App) oauthCallback(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Referrer-Policy", "no-referrer")
	u, e := identity(r)
	if e != nil {
		http.Error(w, e.Error(), 403)
		return
	}
	nonce, code := r.URL.Query().Get("state"), r.URL.Query().Get("code")
	a.mu.Lock()
	pending, ok := a.oauth[nonce]
	delete(a.oauth, nonce)
	a.mu.Unlock()
	if !ok || time.Now().After(pending.Expires) || pending.User != u || code == "" {
		http.Error(w, "授权已过期或账号不匹配，请返回应用重试", 400)
		return
	}
	fn, e := newFnTV(a.mediaBase)
	if e == nil {
		var result any
		result, e = fn.Call(r.Context(), "POST", "/v/api/v1/auth", object{"source": "Trim-NAS", "code": code})
		fn.Token = str(obj(result), "token")
	}
	if e != nil || fn.Token == "" {
		http.Error(w, "无法换取影视凭据，请返回应用重新授权", 400)
		return
	}
	s, e := a.connectedSession(r.Context(), fn, u.Name)
	if e != nil {
		http.Error(w, e.Error(), 403)
		return
	}
	a.mu.Lock()
	a.sessions[u.ID] = s
	a.mu.Unlock()
	http.Redirect(w, r, gatewayPrefix+"/", http.StatusSeeOther)
}

func (a *App) connectedSession(ctx context.Context, fn *FnTV, username string) (*Session, error) {
	user, e := fn.Call(ctx, "GET", "/v/api/v1/user/info", nil)
	if e != nil {
		return nil, e
	}
	u := obj(user)
	admin, _ := u["is_admin"].(bool)
	if !admin && num(u, "is_admin") != 1 {
		return nil, errors.New("需要飞牛影视管理员权限")
	}
	name := str(u, "username")
	if name == "" {
		name = str(u, "name")
	}
	if name != username {
		return nil, errors.New("影视账号与当前 NAS 账号不匹配")
	}
	libs, e := fn.Libraries(ctx)
	if e != nil {
		return nil, e
	}
	v, e := fn.Call(ctx, "GET", "/v/api/v1/sys/version", nil)
	if e != nil {
		return nil, e
	}
	return &Session{expires: time.Now().Add(8 * time.Hour), fn: fn, state: State{Libraries: libs, Version: obj(v), Account: username, Progress: "已通过 NAS 账号连接影视"}}, nil
}
