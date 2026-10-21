package main

// Velocity proxy network: one address for all member servers, modern player
// info forwarding with a shared secret. The proxy is a managed container
// (itzg/mc-proxy) named mc-velocity; its configuration lives in a label.

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
	"sync/atomic"
	"time"
)

const (
	proxyContainer = "mc-velocity"
	proxyVolume    = "mc-velocity-data"
	proxyImage     = "itzg/mc-proxy:latest"
	labelProxy     = "craftdock.proxy"
	labelProxyCfg  = "craftdock.proxyconfig"
	proxyTaskKey   = "~proxy"
)

type ProxyConfig struct {
	Port          int               `json:"port"`
	Motd          string            `json:"motd"`
	MaxPlayers    int               `json:"maxPlayers"`
	OnlineMode    bool              `json:"onlineMode"`
	Lobby         string            `json:"lobby"`
	ForcedHosts   map[string]string `json:"forcedHosts"` // hostname → server
	Memory        string            `json:"memory"`
	PublicAddress string            `json:"publicAddress"`
	Secret        string            `json:"secret,omitempty"`
}

var proxySecret atomic.Value // string

func proxySecretValue() string {
	s, _ := proxySecret.Load().(string)
	return s
}

func (m *Manager) proxyConfig() *ProxyConfig {
	m.proxyMu.Lock()
	defer m.proxyMu.Unlock()
	if m.proxyCfg == nil {
		return nil
	}
	cp := *m.proxyCfg
	return &cp
}

func (m *Manager) setProxyConfig(c *ProxyConfig) {
	m.proxyMu.Lock()
	m.proxyCfg = c
	m.proxyMu.Unlock()
	if c != nil {
		proxySecret.Store(c.Secret)
	} else {
		proxySecret.Store("")
	}
}

func (m *Manager) findProxy(ctx context.Context) (*ContainerSummary, error) {
	cs, err := m.d.ListContainers(ctx, labelProxy+"=true")
	if err != nil {
		return nil, err
	}
	if len(cs) == 0 {
		return nil, nil
	}
	return &cs[0], nil
}

func (m *Manager) loadProxy(ctx context.Context) {
	c, err := m.findProxy(ctx)
	if err != nil || c == nil {
		return
	}
	var pc ProxyConfig
	if err := json.Unmarshal([]byte(c.Labels[labelProxyCfg]), &pc); err == nil {
		m.setProxyConfig(&pc)
	}
}

func (m *Manager) checkProxyMember(cfg ServerConfig) error {
	if !cfg.ProxyMember {
		return nil
	}
	if m.network == "" {
		return badRequest("Für das Velocity-Netzwerk muss CRAFTDOCK_NETWORK gesetzt sein")
	}
	if m.proxyConfig() == nil {
		return badRequest("Richte zuerst unter „Netzwerk“ den Velocity-Proxy ein")
	}
	return nil
}

var memArgRe = regexp.MustCompile(`^[0-9]{2,6}[MG]$`)

func (p *ProxyConfig) normalize() error {
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
	Member      bool   `json:"member"`
	Capable     bool   `json:"capable"`
}

type ProxyView struct {
	Available bool          `json:"available"`
	Network   string        `json:"network"`
	Exists    bool          `json:"exists"`
	Config    *ProxyConfig  `json:"config,omitempty"`
	State     string        `json:"state,omitempty"`
	Status    string        `json:"status,omitempty"`
	Task      *Task         `json:"task,omitempty"`
	Online    int           `json:"online"`
	Reachable bool          `json:"reachable"`
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
		if cfg.ProxyMember {
			cfgs = append(cfgs, cfg)
		}
		out = append(out, proxyMember{Name: s.Name, DisplayName: cfg.DisplayName, Type: cfg.Type, Phase: s.Phase, Member: cfg.ProxyMember, Capable: cfg.proxyCapable()})
	}
	return cfgs, out, nil
}

func (m *Manager) ProxyStatus(ctx context.Context) (ProxyView, error) {
	v := ProxyView{Available: m.network != "", Network: m.network, Servers: []proxyMember{}}
	_, mem, err := m.members(ctx)
	if err != nil {
		return v, err
	}
	v.Servers = mem
	v.Task = m.task(proxyTaskKey)
	c, err := m.findProxy(ctx)
	if err != nil {
		return v, err
	}
	if c == nil {
		return v, nil
	}
	v.Exists, v.State, v.Status = true, c.State, c.Status
	if pc := m.proxyConfig(); pc != nil {
		pc.Secret = ""
		v.Config = pc
	}
	if c.State == "running" {
		if st := m.pingProxy(ctx, c); st != nil {
			v.Reachable, v.Online = true, st.Online
		}
	}
	return v, nil
}

func (m *Manager) pingProxy(ctx context.Context, c *ContainerSummary) *SLPStatus {
	var cands []string
	if ins, err := m.d.Inspect(ctx, c.ID); err == nil {
		for _, n := range ins.NetworkSettings.Networks {
			if n.IPAddress != "" {
				cands = append(cands, net.JoinHostPort(n.IPAddress, "25577"))
			}
		}
	}
	if pc := m.proxyConfig(); pc != nil {
		cands = append(cands, net.JoinHostPort("host.docker.internal", strconv.Itoa(pc.Port)), net.JoinHostPort("127.0.0.1", strconv.Itoa(pc.Port)))
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
	stored := pc
	return map[string]any{
		"Image":        proxyImage,
		"Env":          []string{"TYPE=VELOCITY", "MEMORY=" + pc.Memory, "TZ=" + envOr("TZ", "UTC")},
		"Tty":          true,
		"OpenStdin":    true,
		"ExposedPorts": map[string]struct{}{"25577/tcp": {}},
		"Labels":       map[string]string{labelProxy: "true", labelProxyCfg: mustJSON(stored)},
		"HostConfig": map[string]any{
			"PortBindings":  map[string][]map[string]string{"25577/tcp": {{"HostPort": strconv.Itoa(pc.Port)}}},
			"Binds":         []string{proxyVolume + ":/server"},
			"RestartPolicy": map[string]any{"Name": "unless-stopped"},
			"NetworkMode":   m.network,
		},
	}
}

func (m *Manager) writeProxyFiles(ctx context.Context, id string, pc ProxyConfig) error {
	members, _, err := m.members(ctx)
	if err != nil {
		return err
	}
	toml := velocityToml(pc, members)
	if err := m.d.PutFile(ctx, id, "/server", "velocity.toml", strings.NewReader(toml), int64(len(toml)), 1000, 1000); err != nil {
		return err
	}
	return m.d.PutFile(ctx, id, "/server", "forwarding.secret", strings.NewReader(pc.Secret), int64(len(pc.Secret)), 1000, 1000)
}

// SaveProxy creates or recreates the proxy container with pc and starts it.
func (m *Manager) SaveProxy(ctx context.Context, pc ProxyConfig) error {
	if m.network == "" {
		return badRequest("Für das Velocity-Netzwerk muss CRAFTDOCK_NETWORK gesetzt sein (siehe docker-compose.yml)")
	}
	if err := pc.normalize(); err != nil {
		return err
	}
	existing, err := m.findProxy(ctx)
	if err != nil {
		return err
	}
	if old := m.proxyConfig(); old != nil && old.Secret != "" {
		pc.Secret = old.Secret
	} else {
		pc.Secret = randomHex(24)
	}
	// Port clash with a published server port?
	cs, err := m.d.ListContainers(ctx, labelManaged+"=true")
	if err != nil {
		return err
	}
	for _, c := range cs {
		other := configFromLabels(c.Labels)
		for _, hp := range other.hostPorts() {
			if hp == strconv.Itoa(pc.Port)+"/tcp" {
				return conflict("Port %d wird bereits von Server „%s“ verwendet", pc.Port, other.Name)
			}
		}
	}
	return m.startTask(proxyTaskKey, "proxy", "Proxy wird eingerichtet …", nil, func(ctx context.Context, p func(string)) error {
		if err := m.ensureImage(ctx, proxyImage, p); err != nil {
			return err
		}
		if existing != nil {
			p("Proxy wird neu erstellt …")
			_ = m.d.Stop(ctx, existing.ID, 20)
			if err := m.d.Remove(ctx, existing.ID); err != nil && !IsNotFound(err) {
				return err
			}
		}
		id, err := m.d.CreateContainer(ctx, proxyContainer, m.proxySpec(pc))
		if err != nil {
			return err
		}
		m.setProxyConfig(&pc)
		if err := m.writeProxyFiles(ctx, id, pc); err != nil {
			return err
		}
		p("Proxy wird gestartet …")
		return m.d.Start(ctx, id)
	})
}

func (m *Manager) ProxyAction(ctx context.Context, action string) error {
	c, err := m.findProxy(ctx)
	if err != nil {
		return err
	}
	if c == nil {
		return badRequest("Es ist noch kein Proxy eingerichtet")
	}
	pc := m.proxyConfig()
	id := c.ID
	switch action {
	case "start", "restart":
		return m.startTask(proxyTaskKey, action, "Proxy wird gestartet …", nil, func(ctx context.Context, _ func(string)) error {
			if pc != nil {
				if err := m.writeProxyFiles(ctx, id, *pc); err != nil {
					return err
				}
			}
			if action == "restart" {
				return m.d.Restart(ctx, id, 20)
			}
			return m.d.Start(ctx, id)
		})
	case "stop":
		return m.startTask(proxyTaskKey, action, "Proxy wird gestoppt …", nil, func(ctx context.Context, _ func(string)) error {
			return m.d.Stop(ctx, id, 20)
		})
	case "delete":
		return m.startTask(proxyTaskKey, action, "Proxy wird entfernt …", nil, func(ctx context.Context, _ func(string)) error {
			_ = m.d.Stop(ctx, id, 20)
			if err := m.d.Remove(ctx, id); err != nil && !IsNotFound(err) {
				return err
			}
			m.setProxyConfig(nil)
			return nil
		})
	}
	return badRequest("Unbekannte Aktion %q", action)
}

// refreshProxyAsync rewrites velocity.toml after membership changes and
// restarts a running proxy so it picks up the new server list.
func (m *Manager) refreshProxyAsync() {
	go func() {
		time.Sleep(500 * time.Millisecond)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		c, err := m.findProxy(ctx)
		pc := m.proxyConfig()
		if err != nil || c == nil || pc == nil {
			return
		}
		if err := m.writeProxyFiles(ctx, c.ID, *pc); err != nil {
			log.Printf("Proxy-Konfiguration: %v", err)
			return
		}
		if c.State == "running" {
			if err := m.d.Restart(ctx, c.ID, 20); err != nil {
				log.Printf("Proxy-Neustart: %v", err)
			}
		}
	}()
}

// ---- HTTP -------------------------------------------------------------------------------

func (a *API) getProxy(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxT(r, 15*time.Second)
	defer cancel()
	v, err := a.m.ProxyStatus(ctx)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, v)
}

func (a *API) putProxy(w http.ResponseWriter, r *http.Request) {
	var pc ProxyConfig
	if err := readJSON(r, &pc); err != nil {
		writeErr(w, err)
		return
	}
	pc.Secret = ""
	ctx, cancel := ctxT(r, 15*time.Second)
	defer cancel()
	if err := a.m.SaveProxy(ctx, pc); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]bool{"ok": true})
}

func (a *API) proxyAction(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxT(r, 15*time.Second)
	defer cancel()
	if err := a.m.ProxyAction(ctx, r.PathValue("action")); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]bool{"ok": true})
}

func (a *API) proxyLogs(w http.ResponseWriter, r *http.Request) {
	c, err := a.m.findProxy(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	if c == nil {
		writeErr(w, &APIError{http.StatusNotFound, "Kein Proxy eingerichtet"})
		return
	}
	a.streamLogs(w, r, c.ID)
}
