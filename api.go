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
	admin  fs.FS
	public fs.FS
}

func (a *API) Routes() http.Handler {
	mux := http.NewServeMux()
	p := a.protect

	mux.HandleFunc("POST /api/login", a.login)
	mux.HandleFunc("POST /api/logout", a.logout)
	mux.HandleFunc("GET /api/session", a.session)
	mux.HandleFunc("PUT /api/me/password", p("", a.changeOwnPassword))

	mux.HandleFunc("GET /api/info", p(PermView, a.info))
	mux.HandleFunc("GET /api/versions", p(PermView, a.versions))
	mux.HandleFunc("GET /api/servers", p(PermView, a.listServers))
	mux.HandleFunc("POST /api/servers", p(PermCreate, a.createServer))
	mux.HandleFunc("GET /api/servers/{name}", p(PermView, a.getServer))
	mux.HandleFunc("PUT /api/servers/{name}", p(PermConfigure, a.updateServer))
	mux.HandleFunc("DELETE /api/servers/{name}", p(PermDelete, a.deleteServer))
	mux.HandleFunc("POST /api/servers/{name}/actions/{action}", p(PermControl, a.action))
	mux.HandleFunc("DELETE /api/servers/{name}/task", p(PermView, a.clearTask))
	mux.HandleFunc("POST /api/servers/{name}/command", p(PermConsole, a.command))
	mux.HandleFunc("GET /api/servers/{name}/players", p(PermView, a.players))
	mux.HandleFunc("GET /api/servers/{name}/playerdata", p(PermView, a.playerData))
	mux.HandleFunc("POST /api/servers/{name}/playeraction", p(PermPlayers, a.playerAction))
	mux.HandleFunc("GET /api/servers/{name}/stats", p(PermView, a.stats))
	mux.HandleFunc("GET /api/servers/{name}/logs", p(PermView, a.logs))
	mux.HandleFunc("GET /api/servers/{name}/files", p(PermConfigure, a.listFiles))
	mux.HandleFunc("GET /api/servers/{name}/file", p(PermConfigure, a.readFile))
	mux.HandleFunc("PUT /api/servers/{name}/file", p(PermConfigure, a.writeFile))
	mux.HandleFunc("GET /api/servers/{name}/backup", p(PermBackup, a.backup))
	mux.HandleFunc("GET /api/servers/{name}/mods", p(PermView, a.mods))
	mux.HandleFunc("GET /api/servers/{name}/mods/search", p(PermConfigure, a.modSearch))
	mux.HandleFunc("GET /api/servers/{name}/mods/compat", p(PermView, a.modCompat))
	mux.HandleFunc("PUT /api/servers/{name}/mods/projects", p(PermConfigure, a.setProjects))
	mux.HandleFunc("POST /api/servers/{name}/mods/files", p(PermConfigure, a.uploadMod))
	mux.HandleFunc("DELETE /api/servers/{name}/mods/files", p(PermConfigure, a.deleteMod))
	mux.HandleFunc("POST /api/servers/{name}/clone", p(PermCreate, a.cloneServer))
	mux.HandleFunc("POST /api/servers/{name}/world/reset", p(PermConfigure, a.worldReset))
	mux.HandleFunc("POST /api/servers/{name}/world/import", p(PermConfigure, a.worldImport))

	mux.HandleFunc("GET /api/settings", p(PermSettings, a.getSettings))
	mux.HandleFunc("PUT /api/settings", p(PermSettings, a.putSettings))
	mux.HandleFunc("GET /api/proxy", p(PermView, a.getProxy))
	mux.HandleFunc("PUT /api/proxy", p(PermSettings, a.putProxy))
	mux.HandleFunc("POST /api/proxy/actions/{action}", p(PermSettings, a.proxyAction))
	mux.HandleFunc("GET /api/proxy/logs", p(PermSettings, a.proxyLogs))
	mux.HandleFunc("GET /api/news", p(PermNews, a.listNews))
	mux.HandleFunc("POST /api/news", p(PermNews, a.createNews))
	mux.HandleFunc("PUT /api/news/{id}", p(PermNews, a.updateNews))
	mux.HandleFunc("DELETE /api/news/{id}", p(PermNews, a.deleteNews))
	mux.HandleFunc("GET /api/users", p(PermUsers, a.listUsers))
	mux.HandleFunc("POST /api/users", p(PermUsers, a.createUser))
	mux.HandleFunc("PUT /api/users/{username}", p(PermUsers, a.updateUser))
	mux.HandleFunc("DELETE /api/users/{username}", p(PermUsers, a.deleteUser))
	mux.HandleFunc("GET /api/audit", p(PermUsers, a.auditLog))

	mux.HandleFunc("GET /api/public/site", cors(a.publicSite))
	mux.HandleFunc("GET /api/public/servers/{name}", cors(a.publicServer))
	mux.Handle("/api/", http.NotFoundHandler())

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

func userFrom(r *http.Request) *User {
	u, _ := r.Context().Value(ctxKey{}).(*User)
	return u
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (s *statusWriter) WriteHeader(c int) { s.status = c; s.ResponseWriter.WriteHeader(c) }

// protect enforces a valid session, the permission perm ("" = any user), the
// user's server scope and, for state-changing requests, a custom header that
// cross-site forms cannot send (CSRF protection on top of SameSite).
// State-changing requests are written to the audit log.
func (a *API) protect(perm string, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := a.auth.User(r)
		if u == nil {
			writeErr(w, &APIError{http.StatusUnauthorized, "Nicht angemeldet"})
			return
		}
		mutating := r.Method != http.MethodGet && r.Method != http.MethodHead
		if mutating && r.Header.Get("X-CraftDock") != "1" {
			writeErr(w, &APIError{http.StatusForbidden, "Fehlender CSRF-Header"})
			return
		}
		if name := r.PathValue("name"); name != "" && (!nameRe.MatchString(name) || !u.CanServer(name)) {
			writeErr(w, ErrServerNotFound)
			return
		}
		if perm != "" && !u.Can(perm) {
			writeErr(w, &APIError{http.StatusForbidden, "Dafür fehlt dir die Berechtigung"})
			return
		}
		r = r.WithContext(context.WithValue(r.Context(), ctxKey{}, u))
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

func clientIP(r *http.Request) string {
	if f := r.Header.Get("X-Forwarded-For"); f != "" {
		return strings.TrimSpace(strings.Split(f, ",")[0])
	}
	host, _, _ := strings.Cut(r.RemoteAddr, ":")
	if strings.HasPrefix(r.RemoteAddr, "[") {
		host = r.RemoteAddr
	}
	return host
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

// ---- Session & users -------------------------------------------------------------

func sessionInfo(u *User) map[string]any {
	return map[string]any{"authenticated": true, "user": u.Username, "role": u.Role, "perms": u.Perms(), "servers": u.Servers, "builtin": u.Builtin}
}

func (a *API) login(w http.ResponseWriter, r *http.Request) {
	var in struct{ Username, Password string }
	if err := readJSON(r, &in); err != nil {
		writeErr(w, err)
		return
	}
	tok, u, ok := a.auth.Login(strings.TrimSpace(in.Username), in.Password)
	if !ok {
		time.Sleep(700 * time.Millisecond) // slow down brute force
		log.Printf("Fehlgeschlagener Login für %q von %s", in.Username, clientIP(r))
		a.auth.Audit(AuditEntry{Time: time.Now().Unix(), User: in.Username, Action: "LOGIN fehlgeschlagen", Status: 401, IP: clientIP(r)})
		writeErr(w, &APIError{http.StatusUnauthorized, "Benutzername oder Passwort falsch"})
		return
	}
	a.auth.Audit(AuditEntry{Time: time.Now().Unix(), User: u.Username, Action: "LOGIN", Status: 200, IP: clientIP(r)})
	a.auth.setCookie(w, r, tok, int(a.auth.ttl.Seconds()))
	writeJSON(w, 200, sessionInfo(u))
}

func (a *API) logout(w http.ResponseWriter, r *http.Request) {
	a.auth.Logout(r)
	a.auth.setCookie(w, r, "", -1)
	writeJSON(w, 200, map[string]any{"authenticated": false})
}

func (a *API) session(w http.ResponseWriter, r *http.Request) {
	u := a.auth.User(r)
	if u == nil {
		writeJSON(w, 200, map[string]any{"authenticated": false})
		return
	}
	writeJSON(w, 200, sessionInfo(u))
}

func (a *API) changeOwnPassword(w http.ResponseWriter, r *http.Request) {
	var in struct{ Old, New string }
	if err := readJSON(r, &in); err != nil {
		writeErr(w, err)
		return
	}
	if err := a.auth.SetOwnPassword(userFrom(r).Username, in.Old, in.New); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (a *API) listUsers(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"users": a.auth.List(), "roles": rolePerms})
}

func (a *API) createUser(w http.ResponseWriter, r *http.Request) {
	var in UserInput
	if err := readJSON(r, &in); err != nil {
		writeErr(w, err)
		return
	}
	if err := a.auth.Create(in); err != nil {
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
	if err := a.auth.Update(r.PathValue("username"), in); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (a *API) deleteUser(w http.ResponseWriter, r *http.Request) {
	if r.PathValue("username") == userFrom(r).Username {
		writeErr(w, badRequest("Du kannst dich nicht selbst löschen"))
		return
	}
	if err := a.auth.Delete(r.PathValue("username")); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (a *API) auditLog(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, a.auth.AuditLog(300))
}

// ---- Handlers -------------------------------------------------------------------

func (a *API) info(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxT(r, 5*time.Second)
	defer cancel()
	info, err := a.m.d.Info(ctx)
	out := map[string]any{"version": appVersion, "dockerOk": err == nil}
	if err != nil {
		out["dockerError"] = err.Error()
	} else {
		out["docker"] = info
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
		if u.CanServer(s.Name) {
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
	writeJSON(w, 200, d)
}

func (a *API) createServer(w http.ResponseWriter, r *http.Request) {
	if len(userFrom(r).Servers) > 0 {
		writeErr(w, &APIError{http.StatusForbidden, "Benutzer mit Server-Beschränkung können keine Server anlegen"})
		return
	}
	var cfg ServerConfig
	if err := readJSON(r, &cfg); err != nil {
		writeErr(w, err)
		return
	}
	ctx, cancel := ctxT(r, 15*time.Second)
	defer cancel()
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
	if err := a.m.Update(ctx, r.PathValue("name"), cfg); err != nil {
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
