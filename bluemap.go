package main

// Files CraftDock writes into a server before it starts (BlueMap, Velocity
// forwarding) and the reverse proxy that serves BlueMap under /map/<name>/.

import (
	"context"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// prepareScript runs as root (exec or helper container). Everything it creates
// must belong to the server user, otherwise the server (uid 1000 by default)
// cannot write into e.g. /data/plugins afterwards.
const prepareScript = `BM="$1"; PV="$2"; SECRET="$3"; FP="$4"; ONL="$5"; OWN="${6:-1000:1000}"
mkd() {
  d="$1"; mkdir -p "$d"
  while [ "$d" != "/data" ] && [ "$d" != "/" ] && [ "$d" != "." ]; do chown "$OWN" "$d" 2>/dev/null; d=$(dirname "$d"); done
  chown "$OWN" /data 2>/dev/null
}
if [ -n "$BM" ]; then
  f="/data/$BM"; mkd "$(dirname "$f")"
  if [ -f "$f" ]; then
    sed -i 's/^\( *\)accept-download: *false/\1accept-download: true/' "$f"
  else
    printf '%s\n' '# Angelegt von CraftDock: Download der Minecraft-Client-Ressourcen akzeptiert.' \
      'accept-download: true' 'data: "bluemap"' 'render-thread-count: 1' 'scan-for-mod-resources: true' 'metrics: false' > "$f"
  fi
  chown -R "$OWN" "$(dirname "$f")" 2>/dev/null
fi
if [ -n "$PV" ]; then
  f=/data/config/paper-global.yml; mkd /data/config
  EN=false; [ "$PV" = on ] && EN=true
  if [ ! -f "$f" ]; then
    if [ "$PV" = on ]; then
      printf 'proxies:\n  velocity:\n    enabled: true\n    online-mode: %s\n    secret: %s\n' "$ONL" "'$SECRET'" > "$f"
    fi
  else
    awk -v en="$EN" -v s="$SECRET" -v onl="$ONL" '
      /^proxies:/ {inp=1; print; next}
      inp && /^[^ #]/ {inp=0; inv=0}
      inp && /^  velocity:/ {inv=1; print; next}
      inv && /^  [^ ]/ {inv=0}
      inv && /^    enabled:/ {print "    enabled: " en; next}
      inv && /^    online-mode:/ {print "    online-mode: " onl; next}
      inv && /^    secret:/ { if (en == "true") print "    secret: '"'"'" s "'"'"'"; else print; next }
      {print}' "$f" > "$f.tmp" && cat "$f.tmp" > "$f" && rm -f "$f.tmp"
  fi
  [ -f "$f" ] && chown "$OWN" "$f" 2>/dev/null
fi
if [ -n "$FP" ]; then
  f=/data/config/FabricProxy-Lite.toml; mkd /data/config
  [ -f "$f" ] || printf 'hackOnlineMode = true\nhackEarlySend = false\nhackMessageChain = true\ndisconnectMessage = "Bitte verbinde dich ueber das Netzwerk."\nsecret = ""\n' > "$f"
  sed -i "s/^secret = .*/secret = \"$SECRET\"/" "$f"
  chown "$OWN" "$f" 2>/dev/null
fi
true`

func (m *Manager) runPrepare(ctx context.Context, id string, running bool, cfg ServerConfig, bm, pv, fp string) error {
	state := "exited"
	if running {
		state = "running"
	}
	pc := m.proxyConfig()
	online := "true"
	if pc != nil && !pc.OnlineMode {
		online = "false"
	}
	c := &ContainerSummary{ID: id, Image: cfg.Image(), State: state, Labels: map[string]string{labelConfig: mustJSON(cfg)}}
	_, err := m.runInServer(ctx, c, prepareScript, bm, pv, proxySecretValue(), fp, online, cfg.owner())
	return err
}

// prepareFiles writes the files the server needs before (re)starting.
func (m *Manager) prepareFiles(ctx context.Context, id string, running bool, cfg ServerConfig) error {
	bm, pv, fp := "", "", ""
	if cfg.Map {
		bm = cfg.blueMapConf()
	}
	if cfg.ProxyMember && cfg.isPaperFamily() {
		pv = "on"
	}
	if cfg.ProxyMember && cfg.isFabricFamily() {
		fp = "on"
	}
	if bm == "" && pv == "" && fp == "" {
		return nil
	}
	return m.runPrepare(ctx, id, running, cfg, bm, pv, fp)
}

// paperVelocity switches Velocity forwarding in paper-global.yml on or off.
func (m *Manager) paperVelocity(ctx context.Context, id string, running bool, cfg ServerConfig, on bool) error {
	mode := "off"
	if on {
		mode = "on"
	}
	return m.runPrepare(ctx, id, running, cfg, "", mode, "")
}

// ---- BlueMap reverse proxy -------------------------------------------------------------

var (
	mapTransport = &http.Transport{
		DialContext:           (&net.Dialer{Timeout: 3 * time.Second}).DialContext,
		ResponseHeaderTimeout: 20 * time.Second,
		MaxIdleConnsPerHost:   8,
	}
	mapTargets sync.Map // server name → "host:port"
)

func (m *Manager) mapTarget(ctx context.Context, c *ContainerSummary, cfg ServerConfig) string {
	if v, ok := mapTargets.Load(cfg.Name); ok {
		return v.(string)
	}
	var cands []string
	if ins, err := m.d.Inspect(ctx, c.ID); err == nil {
		for _, n := range ins.NetworkSettings.Networks {
			if n.IPAddress != "" {
				cands = append(cands, net.JoinHostPort(n.IPAddress, "8100"))
			}
		}
	}
	if cfg.MapPort > 0 {
		p := strconv.Itoa(cfg.MapPort)
		if h := os.Getenv("CRAFTDOCK_PING_HOST"); h != "" {
			cands = append(cands, net.JoinHostPort(h, p))
		}
		cands = append(cands, net.JoinHostPort("host.docker.internal", p), net.JoinHostPort("127.0.0.1", p))
	}
	for _, a := range cands {
		conn, err := net.DialTimeout("tcp", a, 600*time.Millisecond)
		if err == nil {
			conn.Close()
			mapTargets.Store(cfg.Name, a)
			return a
		}
	}
	return ""
}

func mapUnavailable(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = w.Write([]byte(`<!doctype html><meta charset="utf-8"><body style="margin:0;height:100vh;display:grid;place-items:center;background:#070a12;color:#a1acbd;font:15px system-ui,sans-serif;text-align:center">` +
		`<div><div style="font-size:40px">🗺️</div><p>` + msg + `</p></div></body>`))
}

func (a *API) mapProxy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name := r.PathValue("name")
	if !nameRe.MatchString(name) {
		http.NotFound(w, r)
		return
	}
	ctx, cancel := ctxT(r, 10*time.Second)
	defer cancel()
	c, err := a.m.find(ctx, name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	cfg := configFromLabels(c.Labels)
	allowed := false
	if s := a.m.store.Settings(); s.PublicEnabled && !cfg.PublicHidden {
		allowed = true
	} else if u := a.auth.User(r); u != nil && u.Can(PermView) && u.CanServer(name) {
		allowed = true
	}
	if !cfg.Map || !allowed {
		http.NotFound(w, r)
		return
	}
	if c.State != "running" {
		mapUnavailable(w, "Der Server ist offline – die Karte ist verfügbar, sobald er läuft.")
		return
	}
	if cfg.IdleMode == "pause" && a.m.cachedSleeping(name) {
		mapUnavailable(w, "Der Server schläft gerade – die Karte ist wieder da, sobald jemand online ist.")
		return
	}
	target := a.m.mapTarget(ctx, c, cfg)
	if target == "" {
		mapUnavailable(w, "Die Karte ist noch nicht erreichbar. BlueMap startet mit dem Server und rendert die Welt beim ersten Mal im Hintergrund.")
		return
	}
	prefix := "/map/" + name
	rest := strings.TrimPrefix(r.URL.EscapedPath(), prefix)
	if rest == "" {
		rest = "/"
	}
	rp := &httputil.ReverseProxy{
		Transport: mapTransport,
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(&url.URL{Scheme: "http", Host: target})
			p, _ := url.PathUnescape(rest)
			pr.Out.URL.Path, pr.Out.URL.RawPath = p, rest
			pr.Out.Host = target
			pr.Out.Header.Del("Cookie") // never forward the admin session
			pr.Out.Header.Del("Authorization")
		},
		ModifyResponse: func(resp *http.Response) error {
			resp.Header.Del("Set-Cookie")
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, _ error) {
			mapTargets.Delete(name)
			mapUnavailable(w, "Die Karte antwortet gerade nicht. Versuch es gleich noch einmal.")
		},
	}
	rp.ServeHTTP(w, r)
}

// owner is the uid:gid the server runs as (itzg UID/GID, default 1000:1000).
func (c *ServerConfig) owner() string {
	uid, gid := c.ExtraEnv["UID"], c.ExtraEnv["GID"]
	if uid == "" || !numRe.MatchString(uid) {
		uid = "1000"
	}
	if gid == "" || !numRe.MatchString(gid) {
		gid = "1000"
	}
	return uid + ":" + gid
}

var numRe = regexp.MustCompile(`^[0-9]{1,10}$`)
