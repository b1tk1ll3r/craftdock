package main

// Small JSON-file persistence for site settings, player history and a mirror
// of every server configuration (the container label stays authoritative
// while the container exists; the mirror allows restoring a server whose
// container was removed outside CraftDock).

import (
	"encoding/json"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type SiteSettings struct {
	PublicEnabled   bool   `json:"publicEnabled"`
	Title           string `json:"title"`
	Tagline         string `json:"tagline"`
	PublicHost      string `json:"publicHost"`
	DiscordURL      string `json:"discordUrl"`
	ShowPlayerNames bool   `json:"showPlayerNames"`
}

func defaultSettings() SiteSettings {
	return SiteSettings{PublicEnabled: true, Title: "Unsere Minecraft-Server", Tagline: "Spring rein – die Server laufen rund um die Uhr.", ShowPlayerNames: true}
}

func (s *SiteSettings) Validate() error {
	s.Title = strings.TrimSpace(s.Title)
	s.Tagline = strings.TrimSpace(s.Tagline)
	s.PublicHost = strings.TrimSpace(s.PublicHost)
	s.DiscordURL = strings.TrimSpace(s.DiscordURL)
	switch {
	case s.Title == "" || len(s.Title) > 80:
		return badRequest("Titel: 1–80 Zeichen")
	case len(s.Tagline) > 300:
		return badRequest("Untertitel: max. 300 Zeichen")
	case s.PublicHost != "" && !pubAddrRe.MatchString(s.PublicHost):
		return badRequest("Öffentlicher Hostname im Format play.example.com")
	}
	if s.DiscordURL != "" {
		u, err := url.Parse(s.DiscordURL)
		if err != nil || u.Scheme != "https" || u.Host == "" {
			return badRequest("Discord-Link muss eine https-URL sein")
		}
	}
	return nil
}

type HistPoint struct {
	T int64 `json:"t"` // unix seconds
	P int   `json:"p"` // players online, -1 = offline
}

const historyWindow = 24 * time.Hour

type Store struct {
	dir      string
	mu       sync.RWMutex
	settings SiteSettings
	history  map[string][]HistPoint
	stats    statsFile
	news     []NewsItem
	versions map[string]knownVer
	configs  map[string]ServerConfig
	hosting  HostingSettings
}

type knownVer struct {
	FP      string `json:"fp"`
	Version string `json:"version"`
}

// DayStat aggregates one calendar day (server local time) of samples.
type DayStat struct {
	Up    int `json:"up"`    // minutes reachable (incl. sleeping)
	Total int `json:"total"` // minutes sampled
	Peak  int `json:"peak"`
	PM    int `json:"pm"` // player-minutes
}

type PlayerStat struct {
	Minutes int   `json:"minutes"`
	First   int64 `json:"first"`
	Last    int64 `json:"last"`
}

type statsFile struct {
	Daily   map[string]map[string]*DayStat    `json:"daily"`   // server → YYYY-MM-DD → stat
	Players map[string]map[string]*PlayerStat `json:"players"` // server → player → stat
}

const (
	dailyKeep  = 90 * 24 * time.Hour
	playerKeep = 180 * 24 * time.Hour
)

func NewStore(dir string) *Store {
	s := &Store{dir: dir, settings: defaultSettings(), history: map[string][]HistPoint{}}
	s.stats = statsFile{Daily: map[string]map[string]*DayStat{}, Players: map[string]map[string]*PlayerStat{}}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("WARNUNG: Datenverzeichnis %s nicht nutzbar (%v) – Einstellungen werden nicht gespeichert", dir, err)
	}
	s.load("settings.json", &s.settings)
	s.load("history.json", &s.history)
	s.load("stats.json", &s.stats)
	s.load("news.json", &s.news)
	s.load("versions.json", &s.versions)
	s.load("servers.json", &s.configs)
	s.hosting = defaultHosting()
	s.load("hosting.json", &s.hosting)
	if s.configs == nil {
		s.configs = map[string]ServerConfig{}
	}
	if s.versions == nil {
		s.versions = map[string]knownVer{}
	}
	if s.stats.Daily == nil {
		s.stats.Daily = map[string]map[string]*DayStat{}
	}
	if s.stats.Players == nil {
		s.stats.Players = map[string]map[string]*PlayerStat{}
	}
	return s
}

func (s *Store) load(name string, v any) {
	b, err := os.ReadFile(filepath.Join(s.dir, name))
	if err != nil {
		return
	}
	if err := json.Unmarshal(b, v); err != nil {
		log.Printf("WARNUNG: %s ist beschädigt: %v", name, err)
	}
}

func (s *Store) save(name string, v any) error { return s.saveMode(name, v, 0o644) }

func (s *Store) saveMode(name string, v any, mode os.FileMode) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(filepath.Join(s.dir, name), b, mode)
}

// ---- server configuration mirror ----------------------------------------------------

// SaveConfig mirrors a server configuration (contains secrets → 0600).
func (s *Store) SaveConfig(c ServerConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if old, ok := s.configs[c.Name]; ok && mustJSON(old) == mustJSON(c) {
		return
	}
	s.configs[c.Name] = c
	if err := s.saveMode("servers.json", s.configs, 0o600); err != nil {
		log.Printf("WARNUNG: servers.json: %v", err)
	}
}

func (s *Store) DeleteConfig(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.configs[name]; !ok {
		return
	}
	delete(s.configs, name)
	_ = s.saveMode("servers.json", s.configs, 0o600)
}

func (s *Store) Config(name string) (ServerConfig, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.configs[name]
	return c, ok
}

func (s *Store) Configs() []ServerConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ServerConfig, 0, len(s.configs))
	for _, c := range s.configs {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Owner of a server according to the mirror ("" = nobody / unknown).
func (s *Store) Owner(name string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.configs[name].Owner
}

func (s *Store) Hosting() HostingSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.hosting
}

func (s *Store) SetHosting(h HostingSettings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hosting = h
	return s.save("hosting.json", h)
}

func (s *Store) Settings() SiteSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settings
}

func (s *Store) SetSettings(v SiteSettings) error {
	if err := v.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	s.settings = v
	s.mu.Unlock()
	return s.save("settings.json", v)
}

// AddSample records one sampler tick: history point, daily stat and playtime.
func (s *Store) AddSample(name string, p HistPoint, names []string, interval time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	mins := int(interval / time.Minute)
	if mins < 1 {
		mins = 1
	}
	day := time.Unix(p.T, 0).Format("2006-01-02")
	dm := s.stats.Daily[name]
	if dm == nil {
		dm = map[string]*DayStat{}
		s.stats.Daily[name] = dm
	}
	ds := dm[day]
	if ds == nil {
		ds = &DayStat{}
		dm[day] = ds
	}
	ds.Total += mins
	if p.P >= 0 {
		ds.Up += mins
		ds.PM += p.P * mins
		if p.P > ds.Peak {
			ds.Peak = p.P
		}
	}
	if len(names) > 0 {
		pm := s.stats.Players[name]
		if pm == nil {
			pm = map[string]*PlayerStat{}
			s.stats.Players[name] = pm
		}
		for _, n := range names {
			st := pm[n]
			if st == nil {
				st = &PlayerStat{First: p.T}
				pm[n] = st
			}
			st.Minutes += mins
			st.Last = p.T
		}
	}
	s.addPointLocked(name, p)
}

func (s *Store) addPointLocked(name string, p HistPoint) {
	h := append(s.history[name], p)
	cut := time.Now().Add(-historyWindow).Unix()
	i := 0
	for i < len(h) && h[i].T < cut {
		i++
	}
	s.history[name] = h[i:]
}

// SetKnownVersion remembers the Minecraft version a server reported for its
// current setup (fingerprint), so it can be shown while the server is offline.
func (s *Store) SetKnownVersion(name, fp, version string) {
	s.mu.Lock()
	if cur, ok := s.versions[name]; ok && cur.FP == fp && cur.Version == version {
		s.mu.Unlock()
		return
	}
	s.versions[name] = knownVer{fp, version}
	cp := make(map[string]knownVer, len(s.versions))
	for k, v := range s.versions {
		cp[k] = v
	}
	s.mu.Unlock()
	if err := s.save("versions.json", cp); err != nil {
		log.Printf("Versionen konnten nicht gespeichert werden: %v", err)
	}
}

// KnownVersion returns the remembered version if the setup is unchanged.
func (s *Store) KnownVersion(name, fp string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if v, ok := s.versions[name]; ok && v.FP == fp {
		return v.Version
	}
	return ""
}

func (s *Store) History(name string) []HistPoint {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]HistPoint{}, s.history[name]...)
}

// SaveHistory persists history, dropping servers that no longer exist.
func (s *Store) SaveHistory(existing map[string]bool) {
	s.mu.Lock()
	if existing != nil {
		for n := range s.history {
			if !existing[n] {
				delete(s.history, n)
			}
		}
	}
	if existing != nil {
		for n := range s.stats.Daily {
			if !existing[n] {
				delete(s.stats.Daily, n)
				delete(s.stats.Players, n)
			}
		}
	}
	cutDay := time.Now().Add(-dailyKeep).Format("2006-01-02")
	for _, dm := range s.stats.Daily {
		for d := range dm {
			if d < cutDay {
				delete(dm, d)
			}
		}
	}
	cutP := time.Now().Add(-playerKeep).Unix()
	for _, pm := range s.stats.Players {
		for n, st := range pm {
			if st.Last < cutP {
				delete(pm, n)
			}
		}
	}
	cp := make(map[string][]HistPoint, len(s.history))
	for k, v := range s.history {
		cp[k] = v
	}
	stats, _ := json.Marshal(s.stats)
	s.mu.Unlock()
	if err := s.save("history.json", cp); err != nil {
		log.Printf("Verlauf konnte nicht gespeichert werden: %v", err)
	}
	if err := s.save("stats.json", json.RawMessage(stats)); err != nil {
		log.Printf("Statistik konnte nicht gespeichert werden: %v", err)
	}
}

// ---- uptime & player statistics -------------------------------------------------------

type DayView struct {
	Date   string   `json:"date"`
	Uptime *float64 `json:"uptime"` // nil = no data
	Peak   int      `json:"peak"`
}

type UptimeView struct {
	H24  *float64  `json:"h24"`
	D7   *float64  `json:"d7"`
	D30  *float64  `json:"d30"`
	Days []DayView `json:"days"` // oldest first, 30 entries
}

func pct(up, total int) *float64 {
	if total == 0 {
		return nil
	}
	v := float64(up) / float64(total) * 100
	return &v
}

func (s *Store) Uptime(name string) UptimeView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out UptimeView
	up, total := 0, 0
	for _, p := range s.history[name] {
		total++
		if p.P >= 0 {
			up++
		}
	}
	out.H24 = pct(up, total)
	dm := s.stats.Daily[name]
	now := time.Now()
	sum := func(days int) *float64 {
		u, t := 0, 0
		for i := 0; i < days; i++ {
			if d := dm[now.AddDate(0, 0, -i).Format("2006-01-02")]; d != nil {
				u += d.Up
				t += d.Total
			}
		}
		return pct(u, t)
	}
	out.D7, out.D30 = sum(7), sum(30)
	for i := 29; i >= 0; i-- {
		day := now.AddDate(0, 0, -i).Format("2006-01-02")
		dv := DayView{Date: day}
		if d := dm[day]; d != nil {
			dv.Uptime, dv.Peak = pct(d.Up, d.Total), d.Peak
		}
		out.Days = append(out.Days, dv)
	}
	return out
}

type PlayerRank struct {
	Name    string `json:"name"`
	Minutes int    `json:"minutes"`
	Last    int64  `json:"last"`
	First   int64  `json:"first"`
}

func (s *Store) Players(name string) []PlayerRank {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []PlayerRank{}
	for n, st := range s.stats.Players[name] {
		out = append(out, PlayerRank{Name: n, Minutes: st.Minutes, Last: st.Last, First: st.First})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Minutes != out[j].Minutes {
			return out[i].Minutes > out[j].Minutes
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// ---- news & events -----------------------------------------------------------------------

type NewsItem struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"` // news | event
	Title     string `json:"title"`
	Body      string `json:"body"`
	Server    string `json:"server,omitempty"`
	Pinned    bool   `json:"pinned"`
	Published int64  `json:"published"`
	Start     int64  `json:"start,omitempty"` // events
	End       int64  `json:"end,omitempty"`
	Author    string `json:"author,omitempty"`
}

func (n *NewsItem) validate() error {
	n.Title = strings.TrimSpace(n.Title)
	n.Body = strings.TrimSpace(n.Body)
	switch {
	case n.Kind != "news" && n.Kind != "event":
		return badRequest("Art muss news oder event sein")
	case n.Title == "" || len(n.Title) > 120:
		return badRequest("Titel: 1–120 Zeichen")
	case len(n.Body) > 5000:
		return badRequest("Text: max. 5000 Zeichen")
	case n.Server != "" && !nameRe.MatchString(n.Server):
		return badRequest("Ungültiger Server")
	case n.Kind == "event" && n.Start == 0:
		return badRequest("Ein Event braucht einen Beginn")
	case n.End != 0 && n.End < n.Start:
		return badRequest("Das Ende liegt vor dem Beginn")
	}
	if n.Kind == "news" {
		n.Start, n.End = 0, 0
	}
	return nil
}

func (s *Store) News() []NewsItem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := append([]NewsItem{}, s.news...)
	sort.Slice(out, func(i, j int) bool { return out[i].Published > out[j].Published })
	return out
}

func (s *Store) SaveNews(n NewsItem, author string) (NewsItem, error) {
	if err := n.validate(); err != nil {
		return n, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if n.ID == "" {
		n.ID = randomHex(6)
		n.Published = time.Now().Unix()
		n.Author = author
		s.news = append(s.news, n)
	} else {
		found := false
		for i := range s.news {
			if s.news[i].ID == n.ID {
				n.Published, n.Author = s.news[i].Published, s.news[i].Author
				s.news[i] = n
				found = true
			}
		}
		if !found {
			return n, &APIError{404, "Beitrag nicht gefunden"}
		}
	}
	return n, s.save("news.json", s.news)
}

func (s *Store) DeleteNews(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.news {
		if s.news[i].ID == id {
			s.news = append(s.news[:i], s.news[i+1:]...)
			return s.save("news.json", s.news)
		}
	}
	return &APIError{404, "Beitrag nicht gefunden"}
}
