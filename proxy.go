package main

// Velocity proxy networks: each network bundles its member servers behind one
// address, with modern player info forwarding and its own secret. Every
// network is a managed container (itzg/mc-proxy) whose configuration lives in
// a label. A server can belong to at most one network, because a backend only
// accepts a single forwarding secret.

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	proxyImage    = "itzg/mc-proxy:latest"
	labelProxy    = "craftdock.proxy"
	labelProxyID  = "craftdock.proxyid"
	labelProxyCfg = "craftdock.proxyconfig"
	// legacyNetworkID is used for the single proxy of CraftDock ≤ 1.2 (and for
	// servers that only have proxyMember=true in their stored configuration).
	legacyNetworkID = "main"
)

type ProxyConfig struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Port          int               `json:"port"`
	Motd          string            `json:"motd"`
	MaxPlayers    int               `json:"maxPlayers"`
	OnlineMode    bool              `json:"onlineMode"`
	Lobby         string            `json:"lobby"`
	ForcedHosts   map[string]string `json:"forcedHosts"` // hostname → server
	Memory        string            `json:"memory"`
	PublicAddress string            `json:"publicAddress"`
	Hidden        bool              `json:"hidden"` // not shown on the public site
	Secret        string            `json:"secret,omitempty"`
}

func (p *ProxyConfig) displayName() string {
	if p.Name != "" {
		return p.Name
	}
	return p.ID
}

func proxyContainerName(id string) string {
	if id == legacyNetworkID {
		return "mc-velocity" // keeps the container and data of CraftDock ≤ 1.2
	}
	return "mc-velocity-" + id
}

func proxyVolumeName(id string) string { return proxyContainerName(id) + "-data" }
func proxyTaskKey(id string) string    { return "~proxy-" + id }

// proxySecrets holds network ID → forwarding secret. ServerConfig.Env needs
// it without access to the Manager.
var proxySecrets sync.Map

func proxySecretFor(id string) string {
	if v, ok := proxySecrets.Load(id); ok {
		return v.(string)
	}
	return ""
}

func (m *Manager) proxyConfig(id string) *ProxyConfig {
	m.proxyMu.Lock()
	defer m.proxyMu.Unlock()
	pc, ok := m.proxies[id]
	if !ok {
		return nil
	}
	cp := *pc
	return &cp
}

func (m *Manager) proxyConfigs() []ProxyConfig {
	m.proxyMu.Lock()
	defer m.proxyMu.Unlock()
	out := make([]ProxyConfig, 0, len(m.proxies))
	for _, pc := range m.proxies {
		out = append(out, *pc)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (m *Manager) setProxyConfig(id string, c *ProxyConfig) {
	m.proxyMu.Lock()
	if m.proxies == nil {
		m.proxies = map[string]*ProxyConfig{}
	}
	if c == nil {
		delete(m.proxies, id)
	} else {
		cp := *c
		m.proxies[id] = &cp
	}
	m.proxyMu.Unlock()
	if c != nil {
		proxySecrets.Store(id, c.Secret)
	} else {
		proxySecrets.Delete(id)
	}
}

func proxyIDOf(c ContainerSummary) string {
	if id := c.Labels[labelProxyID]; id != "" {
		return id
	}
	var pc ProxyConfig
	if json.Unmarshal([]byte(c.Labels[labelProxyCfg]), &pc) == nil && pc.ID != "" {
		return pc.ID
	}
	return legacyNetworkID
}

func (m *Manager) listProxies(ctx context.Context) ([]ContainerSummary, error) {
	return m.d.ListContainers(ctx, labelProxy+"=true")
}

func (m *Manager) findProxy(ctx context.Context, id string) (*ContainerSummary, error) {
	cs, err := m.listProxies(ctx)
	if err != nil {
		return nil, err
	}
	for i := range cs {
		if proxyIDOf(cs[i]) == id {
			return &cs[i], nil
		}
	}
	return nil, nil
}

// loadProxies reads all network configurations from their container labels.
func (m *Manager) loadProxies(ctx context.Context) {
	cs, err := m.listProxies(ctx)
	if err != nil {
		return
	}
	for _, c := range cs {
		var pc ProxyConfig
		if err := json.Unmarshal([]byte(c.Labels[labelProxyCfg]), &pc); err != nil {
			continue
		}
		if pc.ID == "" {
			pc.ID = proxyIDOf(c)
		}
		m.setProxyConfig(pc.ID, &pc)
	}
}

func (m *Manager) checkProxyMember(cfg ServerConfig) error {
	if cfg.Network == "" {
		return nil
	}
	if m.network == "" {
		return badRequest("Für Velocity-Netzwerke muss CRAFTDOCK_NETWORK gesetzt sein")
	}
	if m.proxyConfig(cfg.Network) == nil {
		return badRequest("Das Netzwerk „%s“ existiert nicht – lege es zuerst unter „Netzwerke“ an", cfg.Network)
	}
	return nil
}

var memArgRe = regexp.MustCompile(`^[0-9]{2,6}[MG]$`)

func (p *ProxyConfig) normalize() error {
	p.ID = strings.ToLower(strings.TrimSpace(p.ID))
	p.Name = strings.TrimSpace(p.Name)
	p.Motd = strings.TrimSpace(p.Motd)
	p.Lobby = strings.TrimSpace(p.Lobby)
	p.PublicAddress = strings.TrimSpace(p.PublicAddress)
	p.Memory = strings.ToUpper(strings.TrimSpace(p.Memory))
	if p.Port == 0 {
		p.Port = 25565
	}
	if p.MaxPlayers == 0 {
		p.MaxPlayers = 100
	}
	if p.Memory == "" {
		p.Memory = "512M"
	}
	if p.Motd == "" {
		p.Motd = "Ein Minecraft-Netzwerk"
	}
	switch {
	case !nameRe.MatchString(p.ID):
		return badRequest("Kennung: nur Kleinbuchstaben, Ziffern und Bindestriche (max. 31 Zeichen)")
	case len(p.Name) > 48:
		return badRequest("Name: max. 48 Zeichen")
	case p.Port < 1 || p.Port > 65535:
		return badRequest("Ungültiger Port")
	case len(p.Motd) > 200 || strings.ContainsAny(p.Motd, "\n\r\"\\"):
		return badRequest("MOTD: eine Zeile, max. 200 Zeichen, ohne \" und \\")
	case p.MaxPlayers < 1 || p.MaxPlayers > 100000:
		return badRequest("Ungültige Spielerzahl")
	case !memArgRe.MatchString(p.Memory):
		return badRequest("Speicher im Format 512M oder 1G")
	case p.Lobby != "" && !nameRe.MatchString(p.Lobby):
		return badRequest("Ungültige Lobby")
	case p.PublicAddress != "" && !pubAddrRe.MatchString(p.PublicAddress):
		return badRequest("Öffentliche Adresse im Format play.example.com")
	}
	for h, s := range p.ForcedHosts {
		if !pubAddrRe.MatchString(h) || strings.Contains(h, ":") || !nameRe.MatchString(s) {
			return badRequest("Ungültiger Forced Host %q → %q", h, s)
		}
	}
	return nil
}

type proxyMember struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Type        string `json:"type"`
	Phase       string `json:"phase"`
	Network     string `json:"network"`
	Capable     bool   `json:"capable"`
}

type ProxyView struct {
	Config    ProxyConfig `json:"config"`
	Exists    bool        `json:"exists"`
	State     string      `json:"state,omitempty"`
	Status    string      `json:"status,omitempty"`
	Task      *Task       `json:"task,omitempty"`
	Online    int         `json:"online"`
	Reachable bool        `json:"reachable"`
	Members   []string    `json:"members"`
}

type ProxiesView struct {
	Available bool          `json:"available"`
	Network   string        `json:"network"`
	Networks  []ProxyView   `json:"networks"`
	Servers   []proxyMember `json:"servers"`
}

func (m *Manager) members(ctx context.Context) ([]ServerConfig, []proxyMember, error) {
	list, err := m.List(ctx)
	if err != nil {
		return nil, nil, err
	}
	var cfgs []ServerConfig
	out := []proxyMember{}
	for _, s := range list {
		cfg := s.Config
		if cfg.Network != "" && s.ContainerID != "" {
			cfgs = append(cfgs, cfg)
		}
		out = append(out, proxyMember{Name: s.Name, DisplayName: cfg.DisplayName, Type: cfg.Type, Phase: s.Phase, Network: cfg.Network, Capable: cfg.proxyCapable()})
	}
	return cfgs, out, nil
}

func membersOf(all []ServerConfig, id string) []ServerConfig {
	var out []ServerConfig
	for _, c := range all {
		if c.Network == id {
			out = append(out, c)
		}
	}
	return out
}

func (m *Manager) ProxiesStatus(ctx context.Context) (ProxiesView, error) {
	v := ProxiesView{Available: m.network != "", Network: m.network, Networks: []ProxyView{}, Servers: []proxyMember{}}
	all, mem, err := m.members(ctx)
	if err != nil {
		return v, err
	}
	v.Servers = mem
	cs, err := m.listProxies(ctx)
	if err != nil {
		return v, err
	}
	byID := map[string]*ContainerSummary{}
	for i := range cs {
		byID[proxyIDOf(cs[i])] = &cs[i]
	}
	for _, pc := range m.proxyConfigs() {
		pc.Secret = ""
		pv := ProxyView{Config: pc, Task: m.task(proxyTaskKey(pc.ID)), Members: []string{}}
		if pv.Config.ForcedHosts == nil {
			pv.Config.ForcedHosts = map[string]string{}
		}
		for _, c := range membersOf(all, pc.ID) {
			pv.Members = append(pv.Members, c.Name)
		}
		if c := byID[pc.ID]; c != nil {
			pv.Exists, pv.State, pv.Status = true, c.State, c.Status
			if c.State == "running" {
				if st := m.cachedProxyPing(ctx, pc.ID, c); st != nil {
					pv.Reachable, pv.Online = true, st.Online
				}
			}
		}
		v.Networks = append(v.Networks, pv)
	}
	return v, nil
}

func (m *Manager) pingProxy(ctx context.Context, id string, c *ContainerSummary) *SLPStatus {
	var cands []string
	if ins, err := m.d.Inspect(ctx, c.ID); err == nil {
		for _, n := range ins.NetworkSettings.Networks {
			if n.IPAddress != "" {
				cands = append(cands, net.JoinHostPort(n.IPAddress, "25577"))
			}
		}
	}
	if pc := m.proxyConfig(id); pc != nil {
		p := strconv.Itoa(pc.Port)
		if h := envOr("CRAFTDOCK_PING_HOST", ""); h != "" {
			cands = append(cands, net.JoinHostPort(h, p))
		}
		cands = append(cands, net.JoinHostPort("host.docker.internal", p), net.JoinHostPort("127.0.0.1", p))
	}
	for _, a := range cands {
		pctx, cancel := context.WithTimeout(ctx, 1200*time.Millisecond)
		st, err := slpPing(pctx, a)
		cancel()
		if err == nil {
			return st
		}
	}
	return nil
}

type proxyPing struct {
	at time.Time
	st *SLPStatus
}

var proxyPingCache sync.Map // id → proxyPing

func (m *Manager) cachedProxyPing(ctx context.Context, id string, c *ContainerSummary) *SLPStatus {
	if v, ok := proxyPingCache.Load(id); ok && time.Since(v.(proxyPing).at) < 10*time.Second {
		return v.(proxyPing).st
	}
	st := m.pingProxy(ctx, id, c)
	proxyPingCache.Store(id, proxyPing{time.Now(), st})
	return st
}

func tomlStr(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

// velocityToml renders the proxy configuration for the given members.
func velocityToml(pc ProxyConfig, members []ServerConfig) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Von CraftDock erzeugt – Änderungen bitte in der CraftDock-Oberfläche vornehmen.\n")
	fmt.Fprintf(&b, "config-version = \"2.7\"\nbind = \"0.0.0.0:25577\"\nmotd = %s\nshow-max-players = %d\n", tomlStr(pc.Motd), pc.MaxPlayers)
	fmt.Fprintf(&b, "online-mode = %t\nforce-key-authentication = true\nprevent-client-proxy-connections = false\n", pc.OnlineMode)
	b.WriteString("player-info-forwarding-mode = \"modern\"\nforwarding-secret-file = \"forwarding.secret\"\nannounce-forge = false\nkick-existing-players = false\nping-passthrough = \"DISABLED\"\nenable-player-address-logging = true\n\n[servers]\n")
	names := make([]string, 0, len(members))
	for _, c := range members {
		names = append(names, c.Name)
	}
	sort.Strings(names)
	for _, n := range names {
		fmt.Fprintf(&b, "%s = %s\n", tomlStr(n), tomlStr(containerName(n)+":25565"))
	}
	try := []string{}
	if pc.Lobby != "" {
		for _, n := range names {
			if n == pc.Lobby {
				try = append(try, n)
			}
		}
	}
	for _, n := range names {
		if n != pc.Lobby {
			try = append(try, n)
		}
	}
	quoted := make([]string, len(try))
	for i, t := range try {
		quoted[i] = tomlStr(t)
	}
	fmt.Fprintf(&b, "try = [%s]\n\n[forced-hosts]\n", strings.Join(quoted, ", "))
	hosts := make([]string, 0, len(pc.ForcedHosts))
	for h := range pc.ForcedHosts {
		hosts = append(hosts, h)
	}
	sort.Strings(hosts)
	isMember := map[string]bool{}
	for _, n := range names {
		isMember[n] = true
	}
	for _, h := range hosts {
		if s := pc.ForcedHosts[h]; isMember[s] {
			fmt.Fprintf(&b, "%s = [%s]\n", tomlStr(h), tomlStr(s))
		}
	}
	b.WriteString(`
[advanced]
compression-threshold = 256
compression-level = -1
login-ratelimit = 3000
connection-timeout = 5000
read-timeout = 30000
haproxy-protocol = false
tcp-fast-open = false
bungee-plugin-message-channel = true
show-ping-requests = false
failover-on-unexpected-server-disconnect = true
announce-proxy-commands = true
log-command-executions = false
log-player-connections = true

[query]
enabled = false
port = 25577
map = "Velocity"
show-plugins = false
`)
	return b.String()
}

func (m *Manager) proxySpec(pc ProxyConfig) map[string]any {
	return map[string]any{
		"Image":        proxyImage,
		"Env":          []string{"TYPE=VELOCITY", "MEMORY=" + pc.Memory, "TZ=" + envOr("TZ", "UTC")},
		"Tty":          true,
		"OpenStdin":    true,
		"ExposedPorts": map[string]struct{}{"25577/tcp": {}},
		"Labels":       map[string]string{labelProxy: "true", labelProxyID: pc.ID, labelProxyCfg: mustJSON(pc)},
		"HostConfig": map[string]any{
			"PortBindings":  map[string][]map[string]string{"25577/tcp": {{"HostPort": strconv.Itoa(pc.Port)}}},
			"Binds":         []string{proxyVolumeName(pc.ID) + ":/server"},
			"RestartPolicy": map[string]any{"Name": "unless-stopped"},
			"NetworkMode":   m.network,
		},
	}
}

func (m *Manager) writeProxyFiles(ctx context.Context, cid string, pc ProxyConfig) error {
	all, _, err := m.members(ctx)
	if err != nil {
		return err
	}
	toml := velocityToml(pc, membersOf(all, pc.ID))
	if err := m.d.PutFile(ctx, cid, "/server", "velocity.toml", strings.NewReader(toml), int64(len(toml)), 1000, 1000); err != nil {
		return err
	}
	return m.d.PutFile(ctx, cid, "/server", "forwarding.secret", strings.NewReader(pc.Secret), int64(len(pc.Secret)), 1000, 1000)
}

// proxyPortConflict reports who else already uses host port p.
func (m *Manager) proxyPortConflict(ctx context.Context, p int, selfID string) error {
	cs, err := m.d.ListContainers(ctx, labelManaged+"=true")
	if err != nil {
		return err
	}
	key := strconv.Itoa(p) + "/tcp"
	for _, c := range cs {
		other := configFromLabels(c.Labels)
		for _, hp := range other.hostPorts() {
			if hp == key {
				return conflict("Port %d wird bereits von Server „%s“ verwendet", p, other.Name)
			}
		}
	}
	for _, pc := range m.proxyConfigs() {
		if pc.ID != selfID && pc.Port == p {
			return conflict("Port %d wird bereits von Netzwerk „%s“ verwendet", p, pc.displayName())
		}
	}
	return nil
}

// SaveProxy creates (create=true) or updates a network and (re)starts its proxy.
func (m *Manager) SaveProxy(ctx context.Context, pc ProxyConfig, create bool) error {
	if m.network == "" {
		return badRequest("Für Velocity-Netzwerke muss CRAFTDOCK_NETWORK gesetzt sein (siehe docker-compose.yml)")
	}
	if err := pc.normalize(); err != nil {
		return err
	}
	old := m.proxyConfig(pc.ID)
	switch {
	case create && old != nil:
		return conflict("Ein Netzwerk mit der Kennung „%s“ existiert bereits", pc.ID)
	case !create && old == nil:
		return &APIError{http.StatusNotFound, "Netzwerk nicht gefunden"}
	}
	if old != nil && old.Secret != "" {
		pc.Secret = old.Secret
	} else {
		pc.Secret = randomHex(24)
	}
	if err := m.proxyPortConflict(ctx, pc.Port, pc.ID); err != nil {
		return err
	}
	existing, err := m.findProxy(ctx, pc.ID)
	if err != nil {
		return err
	}
	if existing == nil {
		if _, err := m.d.Inspect(ctx, proxyContainerName(pc.ID)); err == nil {
			return conflict("Es existiert bereits ein fremder Container „%s“", proxyContainerName(pc.ID))
		}
	}
	// Register right away so servers can join while the image is still pulling.
	m.setProxyConfig(pc.ID, &pc)
	return m.startTask(proxyTaskKey(pc.ID), "proxy", "Proxy wird eingerichtet …", nil, func(ctx context.Context, p func(string)) error {
		if err := m.ensureImage(ctx, proxyImage, p); err != nil {
			if old == nil {
				m.setProxyConfig(pc.ID, nil)
			}
			return err
		}
		if existing != nil {
			p("Proxy wird neu erstellt …")
			_ = m.d.Stop(ctx, existing.ID, 20)
			if err := m.d.Remove(ctx, existing.ID); err != nil && !IsNotFound(err) {
				return err
			}
		}
		id, err := m.d.CreateContainer(ctx, proxyContainerName(pc.ID), m.proxySpec(pc))
		if err != nil {
			if old == nil {
				m.setProxyConfig(pc.ID, nil)
			}
			return err
		}
		if err := m.writeProxyFiles(ctx, id, pc); err != nil {
			return err
		}
		p("Proxy wird gestartet …")
		return m.d.Start(ctx, id)
	})
}

func (m *Manager) ProxyAction(ctx context.Context, id, action string) error {
	pc := m.proxyConfig(id)
	if pc == nil {
		return &APIError{http.StatusNotFound, "Netzwerk nicht gefunden"}
	}
	c, err := m.findProxy(ctx, id)
	if err != nil {
		return err
	}
	if action == "delete" {
		all, _, err := m.members(ctx)
		if err != nil {
			return err
		}
		if mem := membersOf(all, id); len(mem) > 0 {
			names := make([]string, len(mem))
			for i, c := range mem {
				names[i] = c.Name
			}
			return conflict("Im Netzwerk sind noch Server (%s). Nimm sie zuerst in deren Konfiguration heraus.", strings.Join(names, ", "))
		}
		return m.startTask(proxyTaskKey(id), action, "Netzwerk wird entfernt …", nil, func(ctx context.Context, _ func(string)) error {
			if c != nil {
				_ = m.d.Stop(ctx, c.ID, 20)
				if err := m.d.Remove(ctx, c.ID); err != nil && !IsNotFound(err) {
					return err
				}
			}
			_ = m.d.RemoveVolume(ctx, proxyVolumeName(id))
			m.setProxyConfig(id, nil)
			return nil
		})
	}
	if c == nil {
		return badRequest("Der Proxy-Container fehlt – speichere die Einstellungen, um ihn neu anzulegen")
	}
	cid := c.ID
	switch action {
	case "start", "restart":
		return m.startTask(proxyTaskKey(id), action, "Proxy wird gestartet …", nil, func(ctx context.Context, _ func(string)) error {
			if err := m.writeProxyFiles(ctx, cid, *pc); err != nil {
				return err
			}
			if action == "restart" {
				return m.d.Restart(ctx, cid, 20)
			}
			return m.d.Start(ctx, cid)
		})
	case "stop":
		return m.startTask(proxyTaskKey(id), action, "Proxy wird gestoppt …", nil, func(ctx context.Context, _ func(string)) error {
			return m.d.Stop(ctx, cid, 20)
		})
	}
	return badRequest("Unbekannte Aktion %q", action)
}

// refreshProxyAsync rewrites velocity.toml of the given networks after
// membership changes and restarts running proxies so they pick up the list.
func (m *Manager) refreshProxyAsync(ids ...string) {
	seen := map[string]bool{}
	for _, id := range ids {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		go func(id string) {
			time.Sleep(500 * time.Millisecond)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			c, err := m.findProxy(ctx, id)
			pc := m.proxyConfig(id)
			if err != nil || c == nil || pc == nil {
				return
			}
			if err := m.writeProxyFiles(ctx, c.ID, *pc); err != nil {
				log.Printf("Proxy-Konfiguration %s: %v", id, err)
				return
			}
			if c.State == "running" {
				if err := m.d.Restart(ctx, c.ID, 20); err != nil {
					log.Printf("Proxy-Neustart %s: %v", id, err)
				}
			}
		}(id)
	}
}

// ---- HTTP -------------------------------------------------------------------------------

func (a *API) getProxies(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxT(r, 20*time.Second)
	defer cancel()
	v, err := a.m.ProxiesStatus(ctx)
	if err != nil {
		writeErr(w, err)
		return
	}
	// Users restricted to some servers only see networks those servers are in.
	if u := userFrom(r); len(u.Servers) > 0 {
		keep := v.Networks[:0]
		for _, n := range v.Networks {
			for _, s := range n.Members {
				if u.CanServer(s) {
					keep = append(keep, n)
					break
				}
			}
		}
		v.Networks = keep
		srv := v.Servers[:0]
		for _, s := range v.Servers {
			if u.CanServer(s.Name) {
				srv = append(srv, s)
			}
		}
		v.Servers = srv
	}
	writeJSON(w, 200, v)
}

func (a *API) createProxy(w http.ResponseWriter, r *http.Request) {
	var pc ProxyConfig
	if err := readJSON(r, &pc); err != nil {
		writeErr(w, err)
		return
	}
	pc.Secret = ""
	ctx, cancel := ctxT(r, 15*time.Second)
	defer cancel()
	if err := a.m.SaveProxy(ctx, pc, true); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"id": strings.ToLower(strings.TrimSpace(pc.ID))})
}

func (a *API) putProxy(w http.ResponseWriter, r *http.Request) {
	var pc ProxyConfig
	if err := readJSON(r, &pc); err != nil {
		writeErr(w, err)
		return
	}
	pc.ID, pc.Secret = r.PathValue("id"), ""
	ctx, cancel := ctxT(r, 15*time.Second)
	defer cancel()
	if err := a.m.SaveProxy(ctx, pc, false); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]bool{"ok": true})
}

func (a *API) proxyAction(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxT(r, 15*time.Second)
	defer cancel()
	if err := a.m.ProxyAction(ctx, r.PathValue("id"), r.PathValue("action")); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]bool{"ok": true})
}

func (a *API) proxyLogs(w http.ResponseWriter, r *http.Request) {
	c, err := a.m.findProxy(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	if c == nil {
		writeErr(w, &APIError{http.StatusNotFound, "Kein Proxy-Container für dieses Netzwerk"})
		return
	}
	a.streamLogs(w, r, c.ID)
}
