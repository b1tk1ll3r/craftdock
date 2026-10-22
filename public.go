package main

// Public, unauthenticated pages: a landing page with all public servers and a
// live dashboard per server. Only non-sensitive data is exposed here.

import (
	"context"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

type publicMod struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	IconURL     string `json:"iconUrl,omitempty"`
	URL         string `json:"url,omitempty"`
	Downloads   int    `json:"downloads,omitempty"`
}

type publicServer struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"displayName"`
	Description string   `json:"description,omitempty"`
	Status      string   `json:"status"` // online | starting | offline
	Type        string   `json:"type"`
	Loader      string   `json:"loader,omitempty"`
	Version     string   `json:"version"`
	MOTD        string   `json:"motd"`
	Favicon     string   `json:"favicon,omitempty"`
	Online      int      `json:"online"`
	Max         int      `json:"max"`
	Names       []string `json:"names"`
	LatencyMs   int      `json:"latencyMs"`
	TPS         *float64 `json:"tps,omitempty"`
	StartedAt   string   `json:"startedAt,omitempty"`
	Gamemode    string   `json:"gamemode"`
	Difficulty  string   `json:"difficulty"`
	PVP         bool     `json:"pvp"`
	Hardcore    bool     `json:"hardcore"`
	Whitelist   bool     `json:"whitelist"`
	OnlineMode  bool     `json:"onlineMode"`
	ModCount    int      `json:"modCount"`
	Modpack     string   `json:"modpack,omitempty"`
	Uptime30    *float64 `json:"uptime30,omitempty"`
	Map         bool     `json:"map"`
	Network     bool     `json:"network"`
	NetworkID   string   `json:"networkId,omitempty"`
	NetworkName string   `json:"networkName,omitempty"`

	port          int
	publicAddress string
	Address       string `json:"address"`
}

type publicDetail struct {
	publicServer
	Uptime     UptimeView   `json:"uptime"`
	Top        []PlayerRank `json:"top"`
	Unique     int          `json:"uniquePlayers"`
	Events     []NewsItem   `json:"events"`
	History    []HistPoint  `json:"history"`
	ModKind    string       `json:"modKind"`
	Pack       *PackInfo    `json:"pack,omitempty"`
	Mods       []publicMod  `json:"mods"`
	ExtraFiles []string     `json:"extraFiles"`
}

func (m *Manager) toPublic(ctx context.Context, c *ContainerSummary, cfg ServerConfig) publicServer {
	lv := m.Live(ctx, c, 10*time.Second)
	ps := publicServer{
		Name: cfg.Name, DisplayName: cfg.DisplayName, Description: cfg.Description, Type: cfg.Type,
		Loader: cfg.Loader(), Version: cfg.Version, MOTD: cfg.Motd, Max: cfg.MaxPlayers, Names: []string{},
		LatencyMs: -1, Gamemode: cfg.Gamemode, Difficulty: cfg.Difficulty, PVP: cfg.PVP, Hardcore: cfg.Hardcore,
		Whitelist: cfg.Whitelist != "" || cfg.EnforceWhitelist, OnlineMode: cfg.OnlineMode,
		port: cfg.Port, publicAddress: cfg.PublicAddress, Modpack: cfg.Modpack,
		Map: cfg.Map, Network: cfg.Network != "", NetworkID: cfg.Network,
	}
	if pc := m.proxyConfig(cfg.Network); pc != nil {
		ps.NetworkName = pc.displayName()
		if pc.Hidden {
			ps.Network, ps.NetworkID, ps.NetworkName = false, "", ""
		}
	}
	ps.Uptime30 = m.store.Uptime(cfg.Name).D30
	if ps.DisplayName == "" {
		ps.DisplayName = cfg.Name
	}
	if cfg.ModrinthProjects != "" {
		ps.ModCount = len(strings.Split(cfg.ModrinthProjects, ","))
	}
	switch {
	case lv.Sleeping:
		ps.Status = "sleeping"
	case lv.Reachable:
		ps.Status = "online"
	case lv.Running:
		ps.Status = "starting"
	default:
		ps.Status = "offline"
	}
	if lv.Reachable {
		if lv.MOTD != "" {
			ps.MOTD = lv.MOTD
		}
		ps.Favicon, ps.Online, ps.Max, ps.LatencyMs, ps.TPS = lv.Favicon, lv.Online, lv.Max, lv.LatencyMs, lv.TPS
		ps.Names = lv.Names
	}
	if lv.Running {
		ps.StartedAt = lv.StartedAt
	}
	// Version shown to players: the version that actually runs (live or last
	// seen), never a guess derived from "LATEST".
	ps.Version = ""
	if lv.Reachable {
		ps.Version = gameVersion(lv.Version)
	}
	if ps.Version == "" {
		ps.Version = m.knownVersion(cfg)
	}
	if ps.Version == "" {
		ps.Version = map[string]string{"SNAPSHOT": "Snapshot"}[cfg.Version]
	}
	if ps.Version == "" {
		ps.Version = "Neueste"
	}
	return ps
}

func (m *Manager) publicServers(ctx context.Context) ([]publicServer, error) {
	m.publicMu.Lock()
	defer m.publicMu.Unlock()
	if m.publicList != nil && time.Since(m.publicAt) < 4*time.Second {
		return m.publicList, nil
	}
	cs, err := m.d.ListContainers(ctx, labelManaged+"=true")
	if err != nil {
		return nil, err
	}
	out := []publicServer{}
	for i := range cs {
		cfg := configFromLabels(cs[i].Labels)
		if cfg.PublicHidden {
			continue
		}
		out = append(out, m.toPublic(ctx, &cs[i], cfg))
	}
	// Online first, then by player count, then name.
	rank := map[string]int{"online": 0, "sleeping": 1, "starting": 2, "offline": 3}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0; j-- {
			a, b := out[j-1], out[j]
			less := rank[b.Status] < rank[a.Status] || (rank[b.Status] == rank[a.Status] && (b.Online > a.Online || (b.Online == a.Online && b.Name < a.Name)))
			if !less {
				break
			}
			out[j-1], out[j] = b, a
		}
	}
	m.publicList, m.publicAt = out, time.Now()
	return out, nil
}

func publicHost(s SiteSettings, r *http.Request) string {
	host := s.PublicHost
	if host == "" {
		host = r.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
	}
	return host
}

func hostPort(host string, port int) string {
	if port == 25565 {
		return host
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}

// networkAddress is the public address of a Velocity network.
func networkAddress(pc *ProxyConfig, s SiteSettings, r *http.Request) string {
	if pc.PublicAddress != "" {
		return pc.PublicAddress
	}
	return hostPort(publicHost(s, r), pc.Port)
}

func (m *Manager) addressFor(ps *publicServer, s SiteSettings, r *http.Request) string {
	if pc := m.proxyConfig(ps.NetworkID); ps.NetworkID != "" && pc != nil {
		hosts := make([]string, 0, len(pc.ForcedHosts))
		for h, srv := range pc.ForcedHosts {
			if srv == ps.Name {
				hosts = append(hosts, h)
			}
		}
		if len(hosts) > 0 {
			sort.Strings(hosts)
			return hosts[0]
		}
		return networkAddress(pc, s, r)
	}
	if ps.publicAddress != "" {
		return ps.publicAddress
	}
	return hostPort(publicHost(s, r), ps.port)
}

func (a *API) publicGuard(w http.ResponseWriter) (SiteSettings, bool) {
	s := a.m.store.Settings()
	if !s.PublicEnabled {
		writeErr(w, &APIError{http.StatusNotFound, "Die öffentliche Seite ist deaktiviert"})
		return s, false
	}
	return s, true
}

func (a *API) publicSite(w http.ResponseWriter, r *http.Request) {
	s, ok := a.publicGuard(w)
	if !ok {
		return
	}
	ctx, cancel := ctxT(r, 20*time.Second)
	defer cancel()
	list, err := a.m.publicServers(ctx)
	if err != nil {
		writeErr(w, &APIError{http.StatusServiceUnavailable, "Serverstatus derzeit nicht verfügbar"})
		return
	}
	out := make([]publicServer, len(list))
	for i := range list {
		out[i] = list[i]
		out[i].Address = a.m.addressFor(&out[i], s, r)
		if !s.ShowPlayerNames {
			out[i].Names = []string{}
		}
	}
	news, events := a.m.publicNews("")
	resp := map[string]any{
		"title": s.Title, "tagline": s.Tagline, "discordUrl": s.DiscordURL, "servers": out,
		"news": news, "events": events,
	}
	resp["networks"] = a.m.publicNetworks(ctx, s, r, out)
	w.Header().Set("Cache-Control", "public, max-age=5")
	writeJSON(w, 200, resp)
}

func (a *API) publicServer(w http.ResponseWriter, r *http.Request) {
	s, ok := a.publicGuard(w)
	if !ok {
		return
	}
	name := r.PathValue("name")
	if !nameRe.MatchString(name) {
		writeErr(w, ErrServerNotFound)
		return
	}
	ctx, cancel := ctxT(r, 20*time.Second)
	defer cancel()
	c, err := a.m.find(ctx, name)
	if err != nil {
		writeErr(w, ErrServerNotFound)
		return
	}
	cfg := configFromLabels(c.Labels)
	if cfg.PublicHidden {
		writeErr(w, ErrServerNotFound)
		return
	}
	d := publicDetail{publicServer: a.m.toPublic(ctx, c, cfg), History: a.m.store.History(name), Mods: []publicMod{}, ExtraFiles: []string{}}
	d.Address = a.m.addressFor(&d.publicServer, s, r)
	d.Uptime = a.m.store.Uptime(name)
	ranks := a.m.store.Players(name)
	d.Unique = len(ranks)
	d.Top = []PlayerRank{}
	if s.ShowPlayerNames {
		if len(ranks) > 10 {
			ranks = ranks[:10]
		}
		d.Top = ranks
	}
	_, d.Events = a.m.publicNews(name)
	if !s.ShowPlayerNames {
		d.Names = []string{}
	}
	d.ModKind = cfg.ModDir()
	d.Pack = a.m.packInfo(ctx, cfg)
	var slugs []string
	if cfg.ModrinthProjects != "" {
		for _, p := range a.m.ModrinthProjects(ctx, strings.Split(cfg.ModrinthProjects, ",")) {
			d.Mods = append(d.Mods, publicMod{Title: p.Title, Description: p.Description, IconURL: p.IconURL, URL: p.URL, Downloads: p.Downloads})
			slugs = append(slugs, strings.ToLower(p.Slug))
		}
	}
	// Jars not covered by a Modrinth entry (manual uploads, modpack contents).
	for _, f := range a.m.cachedModFiles(ctx, c, cfg.ModDir()) {
		lower := strings.ToLower(f.Name)
		if strings.HasSuffix(lower, ".disabled") {
			continue
		}
		covered := false
		for _, sl := range slugs {
			if sl != "" && strings.Contains(strings.ReplaceAll(lower, "_", "-"), sl) {
				covered = true
				break
			}
		}
		if !covered {
			d.ExtraFiles = append(d.ExtraFiles, prettyJar(f.Name))
		}
	}
	w.Header().Set("Cache-Control", "public, max-age=5")
	writeJSON(w, 200, d)
}

// prettyJar turns "sodium-fabric-0.5.8+mc1.20.4.jar" into "sodium-fabric 0.5.8+mc1.20.4".
func prettyJar(n string) string {
	n = strings.TrimSuffix(n, ".jar")
	for i := 1; i < len(n); i++ {
		if (n[i-1] == '-' || n[i-1] == '_') && n[i] >= '0' && n[i] <= '9' {
			return n[:i-1] + " " + n[i:]
		}
	}
	return n
}

// ---- settings (admin) ---------------------------------------------------------------

func (a *API) getSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, a.m.store.Settings())
}

func (a *API) putSettings(w http.ResponseWriter, r *http.Request) {
	var s SiteSettings
	if err := readJSON(r, &s); err != nil {
		writeErr(w, err)
		return
	}
	if err := a.m.store.SetSettings(s); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, a.m.store.Settings())
}

// publicNews returns published news (pinned first, newest first, max 8) and
// upcoming or running events. With server != "" only that server's items
// (plus general ones for events) are returned.
func (m *Manager) publicNews(server string) ([]NewsItem, []NewsItem) {
	now := time.Now().Unix()
	news, events := []NewsItem{}, []NewsItem{}
	for _, n := range m.store.News() {
		if server != "" && n.Server != "" && n.Server != server {
			continue
		}
		switch n.Kind {
		case "news":
			news = append(news, n)
		case "event":
			end := n.End
			if end == 0 {
				end = n.Start + 3*3600
			}
			if end >= now {
				events = append(events, n)
			}
		}
	}
	sort.SliceStable(news, func(i, j int) bool { return news[i].Pinned && !news[j].Pinned })
	if len(news) > 8 {
		news = news[:8]
	}
	sort.Slice(events, func(i, j int) bool { return events[i].Start < events[j].Start })
	return news, events
}

// ---- news (admin) -------------------------------------------------------------------

func (a *API) listNews(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, a.m.store.News())
}

func (a *API) createNews(w http.ResponseWriter, r *http.Request) {
	var n NewsItem
	if err := readJSON(r, &n); err != nil {
		writeErr(w, err)
		return
	}
	n.ID = ""
	out, err := a.m.store.SaveNews(n, userFrom(r).Username)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 201, out)
}

func (a *API) updateNews(w http.ResponseWriter, r *http.Request) {
	var n NewsItem
	if err := readJSON(r, &n); err != nil {
		writeErr(w, err)
		return
	}
	n.ID = r.PathValue("id")
	out, err := a.m.store.SaveNews(n, "")
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, out)
}

func (a *API) deleteNews(w http.ResponseWriter, r *http.Request) {
	if err := a.m.store.DeleteNews(r.PathValue("id")); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

type publicNetwork struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Address string   `json:"address"`
	Lobby   string   `json:"lobby,omitempty"`
	Online  bool     `json:"online"`
	Players int      `json:"players"`
	Servers []string `json:"servers"` // public member servers
}

func (m *Manager) publicNetworks(ctx context.Context, s SiteSettings, r *http.Request, servers []publicServer) []publicNetwork {
	out := []publicNetwork{}
	cs, err := m.listProxies(ctx)
	if err != nil {
		return out
	}
	byID := map[string]*ContainerSummary{}
	for i := range cs {
		byID[proxyIDOf(cs[i])] = &cs[i]
	}
	for _, pc := range m.proxyConfigs() {
		if pc.Hidden {
			continue
		}
		pn := publicNetwork{ID: pc.ID, Name: pc.displayName(), Address: networkAddress(&pc, s, r), Lobby: pc.Lobby, Servers: []string{}}
		for _, ps := range servers {
			if ps.NetworkID == pc.ID {
				pn.Servers = append(pn.Servers, ps.Name)
			}
		}
		if c := byID[pc.ID]; c != nil && c.State == "running" {
			if st := m.cachedProxyPing(ctx, pc.ID, c); st != nil {
				pn.Online, pn.Players = true, st.Online
			}
		}
		out = append(out, pn)
	}
	return out
}
