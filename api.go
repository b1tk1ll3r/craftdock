package main

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

type API struct {
	m      *Manager
	auth   *Auth
	alerts *Alerter
	admin  fs.FS
	public fs.FS
}

func (a *API) Routes() http.Handler {
	mux := http.NewServeMux()
	p := a.protect
	me := func(h http.HandlerFunc) http.HandlerFunc { return a.guard("", true, h) }

	mux.HandleFunc("POST /api/login", a.login)
	mux.HandleFunc("POST /api/login/2fa", a.login2FA)
	mux.HandleFunc("POST /api/logout", a.logout)
	mux.HandleFunc("GET /api/session", a.session)
	mux.HandleFunc("GET /api/register", a.registerInfo)
	mux.HandleFunc("POST /api/register", a.register)
	mux.HandleFunc("PUT /api/me/password", me(a.changeOwnPassword))
	mux.HandleFunc("GET /api/me/sessions", me(a.mySessions))
	mux.HandleFunc("DELETE /api/me/sessions/{id}", me(a.revokeMySession))
	mux.HandleFunc("POST /api/me/sessions/revoke-others", me(a.revokeMyOthers))
	mux.HandleFunc("POST /api/me/2fa/setup", me(a.setup2FA))
	mux.HandleFunc("POST /api/me/2fa/confirm", me(a.confirm2FA))
	mux.HandleFunc("POST /api/me/2fa/disable", me(a.disable2FA))
	mux.HandleFunc("POST /api/me/2fa/recovery", me(a.recovery2FA))

	mux.HandleFunc("GET /api/info", p("", a.info))
	mux.HandleFunc("GET /api/versions", p("", a.versions))
	mux.HandleFunc("GET /api/servers", p("", a.listServers))
	mux.HandleFunc("POST /api/servers", p(PermCreate, a.createServer))
	mux.HandleFunc("GET /api/servers/{name}", p(PermView, a.getServer))
	mux.HandleFunc("PUT /api/servers/{name}", p(PermConfigure, a.updateServer))
	mux.HandleFunc("DELETE /api/servers/{name}", p(PermDelete, a.deleteServer))
	mux.HandleFunc("POST /api/servers/{name}/actions/{action}", p(PermControl, a.action))
	mux.HandleFunc("DELETE /api/servers/{name}/task", p(PermView, a.clearTask))
	mux.HandleFunc("POST /api/servers/{name}/command", p(PermConsole, a.command))
	mux.HandleFunc("GET /api/servers/{name}/players", p(PermView, a.players))
	mux.HandleFunc("GET /api/servers/{name}/playerdata", p(PermView, a.playerData))
	mux.HandleFunc("POST /api/servers/{name}/playeraction", p(PermView, a.playerAction)) // checked per action
	mux.HandleFunc("GET /api/servers/{name}/playerinfo", p(PermView, a.playerInfo))
	mux.HandleFunc("GET /api/servers/{name}/stats", p(PermView, a.stats))
	mux.HandleFunc("GET /api/servers/{name}/logs", p(PermView, a.logs))
	mux.HandleFunc("GET /api/servers/{name}/files", p(PermFiles, a.listFiles))
	mux.HandleFunc("GET /api/servers/{name}/file", p(PermFiles, a.readFile))
	mux.HandleFunc("PUT /api/servers/{name}/file", p(PermFiles, a.writeFile))
	mux.HandleFunc("GET /api/servers/{name}/backup", p(PermBackup, a.backup))
	mux.HandleFunc("GET /api/servers/{name}/mods", p(PermView, a.mods))
	mux.HandleFunc("GET /api/servers/{name}/mods/search", p(PermMods, a.modSearch))
	mux.HandleFunc("GET /api/servers/{name}/mods/compat", p(PermView, a.modCompat))
	mux.HandleFunc("PUT /api/servers/{name}/mods/projects", p(PermMods, a.setProjects))
	mux.HandleFunc("POST /api/servers/{name}/mods/files", p(PermMods, a.uploadMod))
	mux.HandleFunc("DELETE /api/servers/{name}/mods/files", p(PermMods, a.deleteMod))
	mux.HandleFunc("POST /api/servers/{name}/clone", p(PermCreate, a.cloneServer))
	mux.HandleFunc("POST /api/servers/{name}/world/reset", p(PermWorld, a.worldReset))
	mux.HandleFunc("POST /api/servers/{name}/world/import", p(PermWorld, a.worldImport))
	mux.HandleFunc("PUT /api/servers/{name}/maintenance", p(PermControl, a.setMaintenance))

	mux.HandleFunc("GET /api/settings", p(PermSettings, a.getSettings))
	mux.HandleFunc("PUT /api/settings", p(PermSettings, a.putSettings))
	mux.HandleFunc("GET /api/hosting", p(PermSettings, a.getHosting))
	mux.HandleFunc("PUT /api/hosting", p(PermSettings, a.putHosting))
	mux.HandleFunc("GET /api/proxies", p("", a.getProxies))
	mux.HandleFunc("POST /api/proxies", p(PermNetworks, a.createProxy))
	mux.HandleFunc("PUT /api/proxies/{id}", p(PermNetworks, a.putProxy))
	mux.HandleFunc("POST /api/proxies/{id}/actions/{action}", p(PermNetworks, a.proxyAction))
	mux.HandleFunc("GET /api/proxies/{id}/logs", p(PermNetworks, a.proxyLogs))
	mux.HandleFunc("GET /api/news", p(PermNews, a.listNews))
	mux.HandleFunc("POST /api/news", p(PermNews, a.createNews))
	mux.HandleFunc("PUT /api/news/{id}", p(PermNews, a.updateNews))
	mux.HandleFunc("DELETE /api/news/{id}", p(PermNews, a.deleteNews))

	mux.HandleFunc("GET /api/users", p(PermUsers, a.listUsers))
	mux.HandleFunc("POST /api/users", p(PermUsers, a.createUser))
	mux.HandleFunc("PUT /api/users/{username}", p(PermUsers, a.updateUser))
	mux.HandleFunc("DELETE /api/users/{username}", p(PermUsers, a.deleteUser))
	mux.HandleFunc("POST /api/users/{username}/reset-2fa", p(PermUsers, a.resetUser2FA))
	mux.HandleFunc("DELETE /api/users/{username}/sessions", p(PermUsers, a.revokeUserSessions))
	mux.HandleFunc("GET /api/invites", p(PermUsers, a.listInvites))
	mux.HandleFunc("POST /api/invites", p(PermUsers, a.createInvite))
	mux.HandleFunc("DELETE /api/invites/{id}", p(PermUsers, a.deleteInvite))
	mux.HandleFunc("GET /api/roles", p("", a.listRoles))
	mux.HandleFunc("POST /api/roles", p(PermRoles, a.createRole))
	mux.HandleFunc("PUT /api/roles/{id}", p(PermRoles, a.updateRole))
	mux.HandleFunc("DELETE /api/roles/{id}", p(PermRoles, a.deleteRole))
	mux.HandleFunc("GET /api/audit", p(PermAudit, a.auditLog))

	mux.HandleFunc("GET /api/alerts", p(PermAlerts, a.getAlerts))
	mux.HandleFunc("PUT /api/alerts", p(PermAlerts, a.putAlerts))
	mux.HandleFunc("POST /api/alerts/test", p(PermAlerts, a.testAlert))
	mux.HandleFunc("PUT /api/alerts/maintenance", p(PermAlerts, a.putGlobalMaintenance))

	mux.HandleFunc("GET /api/public/site", cors(a.publicSite))
	mux.HandleFunc("GET /api/public/servers/{name}", cors(a.publicServer))
	mux.Handle("/api/", http.NotFoundHandler())

	mux.HandleFunc("GET /healthz", a.healthz)
	mux.HandleFunc("GET /metrics", a.metrics)

	// Admin UI under /admin/, public pages at / and /s/<name>.
	mux.Handle("GET /admin/", http.StripPrefix("/admin/", http.FileServer(http.FS(a.admin))))
	mux.Handle("GET /admin", http.RedirectHandler("/admin/", http.StatusFound))
	mux.Handle("GET /p/", http.StripPrefix("/p/", http.FileServer(http.FS(a.public))))
	mux.HandleFunc("GET /{$}", a.publicPage)
	mux.HandleFunc("GET /s/{name}", a.publicPage)
	mux.HandleFunc("GET /widget", a.widgetPage)
	mux.HandleFunc("GET /widget/{name}", a.widgetPage)
	mux.HandleFunc("GET /badge/{file}", a.badge)
	mux.HandleFunc("GET /map/{name}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/map/"+r.PathValue("name")+"/", http.StatusFound)
	})
	mux.HandleFunc("/map/{name}/{path...}", a.mapProxy)
	return securityHeaders(mux)
}

func (a *API) publicPage(w http.ResponseWriter, r *http.Request) {
	if !a.m.store.Settings().PublicEnabled {
		http.Redirect(w, r, "/admin/", http.StatusFound)
		return
	}
	b, err := fs.ReadFile(a.public, "index.html")
	if err != nil {
		http.Error(w, "missing page", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(b)
}

const cspBase = "default-src 'self'; img-src 'self' data: https://cdn.modrinth.com https://mc-heads.net; style-src 'self'; script-src 'self'; connect-src 'self'"

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "same-origin")
		if isHTTPS(r) {
			h.Set("Strict-Transport-Security", "max-age=31536000")
		}
		switch {
		case strings.HasPrefix(r.URL.Path, "/map/"):
			// BlueMap's own web app: framed by our dashboards only.
			h.Set("X-Frame-Options", "SAMEORIGIN")
			h.Set("Content-Security-Policy", "frame-ancestors 'self'")
		case strings.HasPrefix(r.URL.Path, "/widget"), strings.HasPrefix(r.URL.Path, "/badge/"):
			// Embeddable anywhere.
			h.Set("Content-Security-Policy", cspBase+"; frame-ancestors *")
			h.Set("Referrer-Policy", "no-referrer")
		default:
			h.Set("X-Frame-Options", "DENY")
			h.Set("Content-Security-Policy", cspBase+"; frame-src 'self'; frame-ancestors 'none'")
		}
		next.ServeHTTP(w, r)
	})
}

func cors(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		h(w, r)
	}
}

type ctxKey struct{}
type roleKey struct{}
type sessKey struct{}

func userFrom(r *http.Request) *User {
	u, _ := r.Context().Value(ctxKey{}).(*User)
	return u
}

// roleFrom is the role the user acts with on the server of the request.
func roleFrom(r *http.Request) *Role {
	if ro, ok := r.Context().Value(roleKey{}).(*Role); ok {
		return ro
	}
	return &Role{ID: "none"}
}

func sessionHashFrom(r *http.Request) string {
	s, _ := r.Context().Value(sessKey{}).(string)
	return s
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (s *statusWriter) WriteHeader(c int) { s.status = c; s.ResponseWriter.WriteHeader(c) }

// Flush keeps log streams working through the wrapper.
func (s *statusWriter) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (a *API) protect(perm string, h http.HandlerFunc) http.HandlerFunc {
	return a.guard(perm, false, h)
}

// guard enforces a valid session, a completed 2FA setup where required, the
// permission perm ("" = any user) – for server routes in the role the user
// holds on that server – and, for state-changing requests, a custom header
// that cross-site forms cannot send (CSRF protection on top of SameSite).
// State-changing requests are written to the audit log.
func (a *API) guard(perm string, allowSetup bool, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, sess := a.auth.userSession(r)
		if u == nil {
			writeErr(w, &APIError{http.StatusUnauthorized, "Nicht angemeldet"})
			return
		}
		mutating := r.Method != http.MethodGet && r.Method != http.MethodHead
		if mutating && r.Header.Get("X-CraftDock") != "1" {
			writeErr(w, &APIError{http.StatusForbidden, "Fehlender CSRF-Header"})
			return
		}
		if !allowSetup && a.auth.Needs2FASetup(u) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "Bitte richte zuerst die Zwei-Faktor-Anmeldung ein", "code": "2fa_setup"})
			return
		}
		ctx := context.WithValue(r.Context(), ctxKey{}, u)
		ctx = context.WithValue(ctx, sessKey{}, sess.Hash)
		if name := r.PathValue("name"); name != "" {
			if !nameRe.MatchString(name) {
				writeErr(w, ErrServerNotFound)
				return
			}
			role := a.auth.ServerRole(u, name)
			if role == nil {
				writeErr(w, ErrServerNotFound)
				return
			}
			if perm != "" && serverPerm[perm] && !role.Has(perm) {
				writeErr(w, &APIError{http.StatusForbidden, "Dafür fehlt dir die Berechtigung"})
				return
			}
			ctx = context.WithValue(ctx, roleKey{}, role)
		} else if perm != "" && serverPerm[perm] {
			writeErr(w, &APIError{http.StatusForbidden, "Dafür fehlt dir die Berechtigung"})
			return
		}
		if perm != "" && !serverPerm[perm] && !a.auth.Can(u, perm) {
			writeErr(w, &APIError{http.StatusForbidden, "Dafür fehlt dir die Berechtigung"})
			return
		}
		r = r.WithContext(ctx)
		if !mutating {
			h(w, r)
			return
		}
		sw := &statusWriter{ResponseWriter: w, status: 200}
		h(sw, r)
		target := r.PathValue("name")
		if target == "" {
			target = r.PathValue("username") + r.PathValue("id")
		}
		a.auth.Audit(AuditEntry{Time: time.Now().Unix(), User: u.Username, Action: r.Method + " " + r.URL.Path, Target: target, Status: sw.status, IP: clientIP(r)})
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	var ae *APIError
	var de *DockerError
	switch {
	case errors.As(err, &ae):
		status = ae.Status
	case errors.As(err, &de):
		status = de.Status
		if status < 400 || status > 599 {
			status = http.StatusBadGateway
		}
	case strings.Contains(err.Error(), "Docker nicht erreichbar"):
		status = http.StatusBadGateway
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func readJSON(r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return badRequest("Ungültige Anfrage: %v", err)
	}
	return nil
}

func ctxT(r *http.Request, d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), d)
}

func ok(w http.ResponseWriter) { writeJSON(w, 200, map[string]bool{"ok": true}) }

// ---- Session, login, registration ------------------------------------------------

func (a *API) sessionInfo(u *User) map[string]any {
	role := a.auth.roleOf(u)
	var sp []string
	for _, p := range role.Perms {
		if serverPerm[p] {
			sp = append(sp, p)
		}
	}
	h := a.m.store.Hosting()
	return map[string]any{
		"authenticated": true, "user": u.Username, "email": u.Email, "role": u.Role, "roleName": role.Name,
		"perms": a.auth.GlobalPerms(u), "serverPerms": sp, "allServers": u.AllServers || u.Role == "admin",
		"twofa": u.TOTP != "", "needs2fa": a.auth.Needs2FASetup(u), "recoveryLeft": len(u.Recovery),
		"quota": u.Quota, "usage": a.m.Usage(u.Username, ""),
		"hosting": map[string]any{"portMin": h.PortMin, "portMax": h.PortMax, "maxServerMB": h.MaxServerMB, "customerPublic": h.CustomerPublic},
	}
}

func (a *API) login(w http.ResponseWriter, r *http.Request) {
	var in struct{ Username, Password string }
	if err := readJSON(r, &in); err != nil {
		writeErr(w, err)
		return
	}
	ip := clientIP(r)
	res, err := a.auth.Login(in.Username, in.Password, ip, r.UserAgent())
	if err != nil {
		var ae *APIError
		if errors.As(err, &ae) && ae.Status == http.StatusUnauthorized {
			time.Sleep(500 * time.Millisecond)
			log.Printf("Fehlgeschlagener Login für %q von %s", in.Username, ip)
			a.auth.Audit(AuditEntry{Time: time.Now().Unix(), User: in.Username, Action: "LOGIN fehlgeschlagen", Status: 401, IP: ip})
		}
		writeErr(w, err)
		return
	}
	if res.Ticket != "" {
		writeJSON(w, 200, map[string]any{"authenticated": false, "twofa": true, "ticket": res.Ticket})
		return
	}
	a.loggedIn(w, r, res)
}

func (a *API) login2FA(w http.ResponseWriter, r *http.Request) {
	var in struct{ Ticket, Code string }
	if err := readJSON(r, &in); err != nil {
		writeErr(w, err)
		return
	}
	res, err := a.auth.Login2FA(in.Ticket, in.Code, clientIP(r), r.UserAgent())
	if err != nil {
		time.Sleep(300 * time.Millisecond)
		writeErr(w, err)
		return
	}
	a.loggedIn(w, r, res)
}

func (a *API) loggedIn(w http.ResponseWriter, r *http.Request, res *LoginResult) {
	a.auth.Audit(AuditEntry{Time: time.Now().Unix(), User: res.User.Username, Action: "LOGIN", Status: 200, IP: clientIP(r)})
	a.auth.setCookie(w, r, res.Token, int(a.auth.ttl.Seconds()))
	writeJSON(w, 200, a.sessionInfo(res.User))
}

func (a *API) logout(w http.ResponseWriter, r *http.Request) {
	a.auth.Logout(r)
	a.auth.setCookie(w, r, "", -1)
	writeJSON(w, 200, map[string]any{"authenticated": false})
}

func (a *API) session(w http.ResponseWriter, r *http.Request) {
	u := a.auth.User(r)
	if u == nil {
		writeJSON(w, 200, map[string]any{"authenticated": false, "registration": a.m.store.Hosting().Registration})
		return
	}
	writeJSON(w, 200, a.sessionInfo(u))
}

func (a *API) registerInfo(w http.ResponseWriter, r *http.Request) {
	inv := a.auth.InviteFor(r.URL.Query().Get("invite"))
	out := map[string]any{"open": a.openInvite() != nil, "invite": inv != nil}
	if inv != nil {
		if ro := a.auth.roles.Get(inv.Role); ro != nil {
			out["roleName"] = ro.Name
		}
		out["note"] = inv.Note
		out["expires"] = inv.Expires
	}
	writeJSON(w, 200, out)
}

func (a *API) register(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-CraftDock") != "1" {
		writeErr(w, &APIError{http.StatusForbidden, "Fehlender CSRF-Header"})
		return
	}
	var in struct{ Username, Password, Email, Invite string }
	if err := readJSON(r, &in); err != nil {
		writeErr(w, err)
		return
	}
	res, err := a.auth.Register(in.Username, in.Password, in.Email, in.Invite, a.openInvite(), clientIP(r), r.UserAgent())
	if err != nil {
		writeErr(w, err)
		return
	}
	a.auth.Audit(AuditEntry{Time: time.Now().Unix(), User: res.User.Username, Action: "REGISTER", Status: 200, IP: clientIP(r)})
	a.loggedIn(w, r, res)
}

func (a *API) changeOwnPassword(w http.ResponseWriter, r *http.Request) {
	var in struct{ Old, New string }
	if err := readJSON(r, &in); err != nil {
		writeErr(w, err)
		return
	}
	if err := a.auth.SetOwnPassword(userFrom(r).Username, in.Old, in.New, sessionHashFrom(r)); err != nil {
		writeErr(w, err)
		return
	}
	ok(w)
}

func (a *API) mySessions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, a.auth.Sessions(userFrom(r).Username, sessionHashFrom(r)))
}

func (a *API) revokeMySession(w http.ResponseWriter, r *http.Request) {
	if err := a.auth.RevokeSession(userFrom(r).Username, r.PathValue("id")); err != nil {
		writeErr(w, err)
		return
	}
	ok(w)
}

func (a *API) revokeMyOthers(w http.ResponseWriter, r *http.Request) {
	n := a.auth.RevokeAll(userFrom(r).Username, sessionHashFrom(r))
	writeJSON(w, 200, map[string]int{"revoked": n})
}

func (a *API) setup2FA(w http.ResponseWriter, r *http.Request) {
	secret, uri, err := a.auth.Begin2FA(userFrom(r).Username)
	if err != nil {
		writeErr(w, err)
		return
	}
	q, err := qrEncode(uri)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"secret": secret, "uri": uri, "qr": q.SVG()})
}

func (a *API) confirm2FA(w http.ResponseWriter, r *http.Request) {
	var in struct{ Code string }
	if err := readJSON(r, &in); err != nil {
		writeErr(w, err)
		return
	}
	codes, err := a.auth.Confirm2FA(userFrom(r).Username, in.Code, sessionHashFrom(r))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"recovery": codes})
}

func (a *API) disable2FA(w http.ResponseWriter, r *http.Request) {
	var in struct{ Password, Code string }
	if err := readJSON(r, &in); err != nil {
		writeErr(w, err)
		return
	}
	if err := a.auth.Disable2FA(userFrom(r).Username, in.Password, in.Code); err != nil {
		writeErr(w, err)
		return
	}
	ok(w)
}

func (a *API) recovery2FA(w http.ResponseWriter, r *http.Request) {
	var in struct{ Password string }
	if err := readJSON(r, &in); err != nil {
		writeErr(w, err)
		return
	}
	codes, err := a.auth.NewRecoveryCodes(userFrom(r).Username, in.Password)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"recovery": codes})
}

// ---- Users, roles, invitations -------------------------------------------------------

func (a *API) listUsers(w http.ResponseWriter, r *http.Request) {
	users := a.auth.List()
	usage := map[string]Usage{}
	for _, u := range users {
		usage[u.Username] = a.m.Usage(u.Username, "")
	}
	writeJSON(w, 200, map[string]any{"users": users, "usage": usage, "roles": a.auth.roles.List(), "perms": permCatalog})
}

func (a *API) createUser(w http.ResponseWriter, r *http.Request) {
	var in UserInput
	if err := readJSON(r, &in); err != nil {
		writeErr(w, err)
		return
	}
	if err := a.auth.Create(in, userFrom(r)); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 201, map[string]bool{"ok": true})
}

func (a *API) updateUser(w http.ResponseWriter, r *http.Request) {
	var in UserInput
	if err := readJSON(r, &in); err != nil {
		writeErr(w, err)
		return
	}
	if err := a.auth.Update(r.PathValue("username"), in, userFrom(r)); err != nil {
		writeErr(w, err)
		return
	}
	ok(w)
}

func (a *API) deleteUser(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("username")
	if name == userFrom(r).Username {
		writeErr(w, badRequest("Du kannst dich nicht selbst löschen"))
		return
	}
	for _, c := range a.m.store.Configs() {
		if c.Owner == name {
			writeErr(w, conflict("„%s“ besitzt noch Server (z. B. %s) – bitte zuerst löschen oder übertragen", name, c.Name))
			return
		}
	}
	if err := a.auth.Delete(name, userFrom(r)); err != nil {
		writeErr(w, err)
		return
	}
	ok(w)
}

func (a *API) resetUser2FA(w http.ResponseWriter, r *http.Request) {
	if err := a.auth.Reset2FA(r.PathValue("username"), userFrom(r)); err != nil {
		writeErr(w, err)
		return
	}
	ok(w)
}

func (a *API) revokeUserSessions(w http.ResponseWriter, r *http.Request) {
	target := a.auth.Get(r.PathValue("username"))
	if target == nil {
		writeErr(w, ErrUserNotFound)
		return
	}
	if !a.auth.roleWithin(userFrom(r), target.Role) {
		writeErr(w, &APIError{http.StatusForbidden, "Dieser Benutzer hat mehr Rechte als du"})
		return
	}
	keep := ""
	if target.Username == userFrom(r).Username {
		keep = sessionHashFrom(r)
	}
	writeJSON(w, 200, map[string]int{"revoked": a.auth.RevokeAll(target.Username, keep)})
}

func (a *API) listInvites(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, a.auth.Invites())
}

func (a *API) createInvite(w http.ResponseWriter, r *http.Request) {
	var in InviteInput
	if err := readJSON(r, &in); err != nil {
		writeErr(w, err)
		return
	}
	tok, inv, err := a.auth.CreateInvite(in, userFrom(r))
	if err != nil {
		writeErr(w, err)
		return
	}
	link := "/admin/#register=" + tok
	if base := a.m.store.Hosting().PanelURL; base != "" {
		link = base + link
	}
	inv.Hash = ""
	writeJSON(w, 201, map[string]any{"invite": inv, "token": tok, "link": link})
}

func (a *API) deleteInvite(w http.ResponseWriter, r *http.Request) {
	if err := a.auth.DeleteInvite(r.PathValue("id")); err != nil {
		writeErr(w, err)
		return
	}
	ok(w)
}

func (a *API) listRoles(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"roles": a.auth.roles.List(), "perms": permCatalog})
}

func (a *API) saveRole(w http.ResponseWriter, r *http.Request, create bool) {
	var in Role
	if err := readJSON(r, &in); err != nil {
		writeErr(w, err)
		return
	}
	if !create {
		in.ID = r.PathValue("id")
	}
	ro, err := a.auth.roles.Save(in, create, a.auth.grantableBy(userFrom(r)))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, ro)
}

func (a *API) createRole(w http.ResponseWriter, r *http.Request) { a.saveRole(w, r, true) }
func (a *API) updateRole(w http.ResponseWriter, r *http.Request) { a.saveRole(w, r, false) }

func (a *API) deleteRole(w http.ResponseWriter, r *http.Request) {
	if err := a.auth.roles.Delete(r.PathValue("id"), a.auth.RoleInUse); err != nil {
		writeErr(w, err)
		return
	}
	ok(w)
}

func (a *API) auditLog(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, a.auth.AuditLog(500, r.URL.Query().Get("user")))
}

// ---- Servers --------------------------------------------------------------------

func (a *API) info(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxT(r, 5*time.Second)
	defer cancel()
	info, err := a.m.d.Info(ctx)
	out := map[string]any{"version": appVersion, "dockerOk": err == nil}
	if a.auth.Can(userFrom(r), PermSettings) { // host details only for administrators
		if err != nil {
			out["dockerError"] = err.Error()
		} else {
			out["docker"] = info
		}
	}
	writeJSON(w, 200, out)
}

func (a *API) versions(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxT(r, 12*time.Second)
	defer cancel()
	v, err := a.m.Versions(ctx)
	if err != nil {
		writeJSON(w, 200, versionList{Releases: []string{}})
		return
	}
	writeJSON(w, 200, v)
}

func (a *API) listServers(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxT(r, 10*time.Second)
	defer cancel()
	list, err := a.m.List(ctx)
	if err != nil {
		writeErr(w, err)
		return
	}
	u := userFrom(r)
	out := list[:0]
	for _, s := range list {
		if perms := a.auth.ServerPerms(u, s.Name); len(perms) > 0 {
			s.Perms = perms
			s.Maintenance = a.alerts.MaintenanceUntil(s.Name)
			out = append(out, s)
		}
	}
	writeJSON(w, 200, out)
}

func (a *API) getServer(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxT(r, 10*time.Second)
	defer cancel()
	d, err := a.m.Get(ctx, r.PathValue("name"))
	if err != nil {
		writeErr(w, err)
		return
	}
	d.Perms = a.auth.ServerPerms(userFrom(r), d.Name)
	d.Maintenance = a.alerts.MaintenanceUntil(d.Name)
	writeJSON(w, 200, d)
}

func (a *API) createServer(w http.ResponseWriter, r *http.Request) {
	var cfg ServerConfig
	if err := readJSON(r, &cfg); err != nil {
		writeErr(w, err)
		return
	}
	ctx, cancel := ctxT(r, 15*time.Second)
	defer cancel()
	if err := a.applyPolicy(ctx, a.policyFor(userFrom(r), ""), &cfg, nil); err != nil {
		writeErr(w, err)
		return
	}
	if err := a.m.Create(ctx, cfg); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"name": strings.ToLower(strings.TrimSpace(cfg.Name))})
}

func (a *API) updateServer(w http.ResponseWriter, r *http.Request) {
	var cfg ServerConfig
	if err := readJSON(r, &cfg); err != nil {
		writeErr(w, err)
		return
	}
	ctx, cancel := ctxT(r, 15*time.Second)
	defer cancel()
	name := r.PathValue("name")
	old, err := a.m.Config(ctx, name)
	if err != nil {
		writeErr(w, err)
		return
	}
	if err := a.applyPolicy(ctx, a.policyFor(userFrom(r), name), &cfg, &old); err != nil {
		writeErr(w, err)
		return
	}
	if err := a.m.Update(ctx, name, cfg); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]bool{"ok": true})
}

func (a *API) deleteServer(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxT(r, 15*time.Second)
	defer cancel()
	if err := a.m.Delete(ctx, r.PathValue("name"), r.URL.Query().Get("data") == "1"); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]bool{"ok": true})
}

func (a *API) action(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxT(r, 15*time.Second)
	defer cancel()
	if err := a.m.Action(ctx, r.PathValue("name"), r.PathValue("action")); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]bool{"ok": true})
}

func (a *API) clearTask(w http.ResponseWriter, r *http.Request) {
	a.m.ClearTask(r.PathValue("name"))
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (a *API) command(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Command string `json:"command"`
	}
	if err := readJSON(r, &in); err != nil {
		writeErr(w, err)
		return
	}
	if !roleFrom(r).AllowsCommand(in.Command) {
		writeErr(w, &APIError{http.StatusForbidden, "Dieser Befehl ist für deine Rolle gesperrt"})
		return
	}
	ctx, cancel := ctxT(r, 30*time.Second)
	defer cancel()
	out, err := a.m.Command(ctx, r.PathValue("name"), in.Command)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"output": out})
}

func (a *API) players(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxT(r, 10*time.Second)
	defer cancel()
	pl, err := a.m.Players(ctx, r.PathValue("name"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, pl)
}

func (a *API) stats(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxT(r, 10*time.Second)
	defer cancel()
	s, err := a.m.Stats(ctx, r.PathValue("name"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, s)
}

// logs streams container output as Server-Sent Events. The last 300 lines are
// replayed on every (re)connect; an "end" event signals the container stopped.
func (a *API) logs(w http.ResponseWriter, r *http.Request) {
	c, err := a.m.find(r.Context(), r.PathValue("name"))
	if err != nil {
		writeErr(w, err)
		return
	}
	a.streamLogs(w, r, c.ID)
}

func (a *API) streamLogs(w http.ResponseWriter, r *http.Request, id string) {
	ins, err := a.m.d.Inspect(r.Context(), id)
	if err != nil {
		writeErr(w, err)
		return
	}
	rc, err := a.m.d.Logs(r.Context(), id, 300, true, ins.Config.Tty)
	if err != nil {
		writeErr(w, err)
		return
	}
	defer rc.Close()

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, errors.New("Streaming nicht unterstützt"))
		return
	}
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(200)
	fmt.Fprint(w, "retry: 3000\n\n")
	flusher.Flush()

	lines := make(chan string, 256)
	go func() {
		defer close(lines)
		sc := bufio.NewScanner(rc)
		sc.Buffer(make([]byte, 64<<10), 1<<20)
		for sc.Scan() {
			select {
			case lines <- strings.TrimRight(sc.Text(), "\r"):
			case <-r.Context().Done():
				return
			}
		}
	}()

	ping := time.NewTicker(20 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ping.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		case line, ok := <-lines:
			if !ok {
				fmt.Fprint(w, "event: end\ndata: stopped\n\n")
				flusher.Flush()
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", line)
			// Batch bursts (e.g. replayed tail) into a single flush.
			for drained := false; !drained; {
				select {
				case l, ok := <-lines:
					if !ok {
						drained = true
						fmt.Fprint(w, "event: end\ndata: stopped\n\n")
						flusher.Flush()
						return
					}
					fmt.Fprintf(w, "data: %s\n\n", l)
				default:
					drained = true
				}
			}
			flusher.Flush()
		}
	}
}

func (a *API) listFiles(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxT(r, 15*time.Second)
	defer cancel()
	files, err := a.m.ListFiles(ctx, r.PathValue("name"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"files": files, "editable": editableFiles})
}

func (a *API) readFile(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxT(r, 15*time.Second)
	defer cancel()
	content, err := a.m.ReadFile(ctx, r.PathValue("name"), r.URL.Query().Get("path"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"content": content})
}

func (a *API) writeFile(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Content string `json:"content"`
	}
	if err := readJSON(r, &in); err != nil {
		writeErr(w, err)
		return
	}
	ctx, cancel := ctxT(r, 15*time.Second)
	defer cancel()
	if err := a.m.WriteFile(ctx, r.PathValue("name"), r.URL.Query().Get("path"), in.Content); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

// backup streams /data as .tar.gz. On a running server, autosave is paused
// and the world flushed first so the snapshot is consistent.
func (a *API) backup(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	c, err := a.m.find(r.Context(), name)
	if err != nil {
		writeErr(w, err)
		return
	}
	if c.State == "running" {
		ctx, cancel := ctxT(r, 60*time.Second)
		if _, err := a.m.rcon(ctx, c.ID, "save-off"); err == nil {
			_, _ = a.m.rcon(ctx, c.ID, "save-all flush")
			defer func() {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				_, _ = a.m.rcon(ctx, c.ID, "save-on")
			}()
		}
		cancel()
	}
	rc, err := a.m.d.Archive(r.Context(), c.ID, "/data")
	if err != nil {
		writeErr(w, err)
		return
	}
	defer rc.Close()
	fn := fmt.Sprintf("%s-%s.tar.gz", name, time.Now().Format("2006-01-02_15-04"))
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+fn+`"`)
	gz, _ := gzip.NewWriterLevel(w, gzip.BestSpeed)
	if _, err := io.Copy(gz, rc); err != nil {
		log.Printf("[%s] Backup abgebrochen: %v", name, err)
		return
	}
	_ = gz.Close()
	log.Printf("[%s] Backup heruntergeladen (%s)", name, fn)
}

// ---- Mods ---------------------------------------------------------------------------

func (a *API) mods(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxT(r, 60*time.Second)
	defer cancel()
	info, err := a.m.Mods(ctx, r.PathValue("name"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, info)
}

func (a *API) modSearch(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxT(r, 20*time.Second)
	defer cancel()
	q := r.URL.Query()
	offset, _ := strconv.Atoi(q.Get("offset"))
	res, err := a.m.ModrinthSearch(ctx, q.Get("q"), q.Get("loader"), q.Get("version"), offset)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, res)
}

func (a *API) setProjects(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Projects []string `json:"projects"`
	}
	if err := readJSON(r, &in); err != nil {
		writeErr(w, err)
		return
	}
	ctx, cancel := ctxT(r, 15*time.Second)
	defer cancel()
	if err := a.m.SetModrinthProjects(ctx, r.PathValue("name"), in.Projects); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]bool{"ok": true})
}

const maxUpload = 512 << 20

func (a *API) uploadMod(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUpload+1<<20)
	mr, err := r.MultipartReader()
	if err != nil {
		writeErr(w, badRequest("Erwarte multipart/form-data"))
		return
	}
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			writeErr(w, badRequest("Keine Datei übermittelt"))
			return
		}
		if err != nil {
			writeErr(w, badRequest("Upload fehlgeschlagen: %v", err))
			return
		}
		if part.FormName() != "file" {
			continue
		}
		name := path.Base(strings.ReplaceAll(part.FileName(), "\\", "/"))
		// Spool to a temp file: the tar stream needs the size up front.
		tmp, err := os.CreateTemp("", "craftdock-upload-*")
		if err != nil {
			writeErr(w, err)
			return
		}
		defer os.Remove(tmp.Name())
		defer tmp.Close()
		size, err := io.Copy(tmp, io.LimitReader(part, maxUpload+1))
		if err != nil {
			writeErr(w, badRequest("Upload abgebrochen: %v", err))
			return
		}
		if size > maxUpload {
			writeErr(w, badRequest("Datei ist größer als 512 MB"))
			return
		}
		if _, err := tmp.Seek(0, 0); err != nil {
			writeErr(w, err)
			return
		}
		ctx, cancel := ctxT(r, 5*time.Minute)
		defer cancel()
		if err := a.m.UploadModFile(ctx, r.PathValue("name"), name, tmp, size); err != nil {
			writeErr(w, err)
			return
		}
		log.Printf("[%s] Mod hochgeladen: %s (%d Bytes)", r.PathValue("name"), name, size)
		writeJSON(w, 200, map[string]any{"ok": true, "name": name, "size": size})
		return
	}
}

func (a *API) deleteMod(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxT(r, 60*time.Second)
	defer cancel()
	if err := a.m.DeleteModFile(ctx, r.PathValue("name"), r.URL.Query().Get("file")); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (a *API) modCompat(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxT(r, 60*time.Second)
	defer cancel()
	res, err := a.m.CompatCheck(ctx, r.PathValue("name"), r.URL.Query().Get("type"), r.URL.Query().Get("version"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, res)
}
