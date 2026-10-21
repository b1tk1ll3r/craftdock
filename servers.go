package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	labelManaged = "craftdock.managed"
	labelName    = "craftdock.name"
	labelConfig  = "craftdock.config"
	imageRepo    = "itzg/minecraft-server"
	stopTimeout  = 60 // seconds the server gets to save the world
)

// APIError carries an HTTP status for user-facing errors.
type APIError struct {
	Status int
	Msg    string
}

func (e *APIError) Error() string { return e.Msg }

func badRequest(format string, a ...any) error {
	return &APIError{http.StatusBadRequest, fmt.Sprintf(format, a...)}
}
func conflict(format string, a ...any) error {
	return &APIError{http.StatusConflict, fmt.Sprintf(format, a...)}
}

var ErrServerNotFound = &APIError{http.StatusNotFound, "Server nicht gefunden"}

// ---- Configuration -------------------------------------------------------------

type PortMapping struct {
	Host      int    `json:"host"`
	Container int    `json:"container"`
	Protocol  string `json:"protocol"`
}

type ServerConfig struct {
	Name             string            `json:"name"`
	Type             string            `json:"type"`
	Version          string            `json:"version"`
	JavaTag          string            `json:"javaTag"`
	Motd             string            `json:"motd"`
	Port             int               `json:"port"`
	OnlineMode       bool              `json:"onlineMode"`
	Gamemode         string            `json:"gamemode"`
	Difficulty       string            `json:"difficulty"`
	MaxPlayers       int               `json:"maxPlayers"`
	PVP              bool              `json:"pvp"`
	Hardcore         bool              `json:"hardcore"`
	ViewDistance     int               `json:"viewDistance"`
	Seed             string            `json:"seed"`
	LevelType        string            `json:"levelType"`
	Ops              string            `json:"ops"`
	Whitelist        string            `json:"whitelist"`
	EnforceWhitelist bool              `json:"enforceWhitelist"`
	Memory           string            `json:"memory"`
	MemoryLimitMB    int               `json:"memoryLimitMB"`
	CPUs             float64           `json:"cpus"`
	RestartPolicy    string            `json:"restartPolicy"`
	ModrinthProjects string            `json:"modrinthProjects"`
	Modpack          string            `json:"modpack"`
	CFApiKey         string            `json:"cfApiKey,omitempty"`
	PublicHidden     bool              `json:"publicHidden"`
	DisplayName      string            `json:"displayName"`
	Description      string            `json:"description"`
	PublicAddress    string            `json:"publicAddress"`
	IdleMode         string            `json:"idleMode"`    // "" | pause | stop
	IdleMinutes      int               `json:"idleMinutes"` // minutes without players
	Map              bool              `json:"map"`         // BlueMap live map
	MapPort          int               `json:"mapPort"`     // optional host port for BlueMap
	ProxyMember      bool              `json:"proxyMember"` // part of the Velocity network
	Icon             string            `json:"icon"`
	DataPath         string            `json:"dataPath"`
	ExtraEnv         map[string]string `json:"extraEnv"`
	Ports            []PortMapping     `json:"ports"`
	RconPassword     string            `json:"rconPassword,omitempty"`
	CreatedAt        time.Time         `json:"createdAt"`
}

var (
	nameRe    = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,30}$`)
	versionRe = regexp.MustCompile(`^[0-9A-Za-z._+-]{1,40}$`)
	tagRe     = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,40}$`)
	memRe     = regexp.MustCompile(`^([0-9]{1,6})([MG])$`)
	envKeyRe  = regexp.MustCompile(`^[A-Z_][A-Z0-9_]{0,63}$`)
	playerRe  = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,40}$`)

	serverTypes  = set("VANILLA", "PAPER", "PURPUR", "FOLIA", "SPIGOT", "FABRIC", "QUILT", "FORGE", "NEOFORGE", "MODRINTH", "AUTO_CURSEFORGE")
	modpackRe    = regexp.MustCompile(`^[A-Za-z0-9._:/?=&%~+#-]{1,300}$`)
	pubAddrRe    = regexp.MustCompile(`^[A-Za-z0-9.-]{1,253}(:[0-9]{1,5})?$`)
	difficulties = set("peaceful", "easy", "normal", "hard")
	gamemodes    = set("survival", "creative", "adventure", "spectator")
	levelTypes   = set("", "minecraft:normal", "minecraft:flat", "minecraft:large_biomes", "minecraft:amplified")
	restarts     = set("no", "unless-stopped", "always", "on-failure")
)

func set(v ...string) map[string]bool {
	m := map[string]bool{}
	for _, s := range v {
		m[s] = true
	}
	return m
}

func (c *ServerConfig) Normalize() {
	c.Name = strings.TrimSpace(strings.ToLower(c.Name))
	c.Type = strings.ToUpper(strings.TrimSpace(c.Type))
	c.Version = strings.TrimSpace(c.Version)
	c.JavaTag = strings.TrimSpace(c.JavaTag)
	c.Memory = strings.ToUpper(strings.TrimSpace(c.Memory))
	if c.Type == "" {
		c.Type = "VANILLA"
	}
	if c.Version == "" {
		c.Version = "LATEST"
	}
	if strings.EqualFold(c.Version, "latest") || strings.EqualFold(c.Version, "snapshot") {
		c.Version = strings.ToUpper(c.Version)
	}
	if c.JavaTag == "" {
		c.JavaTag = "latest"
	}
	if c.Port == 0 {
		c.Port = 25565
	}
	if c.Gamemode == "" {
		c.Gamemode = "survival"
	}
	if c.Difficulty == "" {
		c.Difficulty = "normal"
	}
	if c.MaxPlayers == 0 {
		c.MaxPlayers = 20
	}
	if c.ViewDistance == 0 {
		c.ViewDistance = 10
	}
	if c.Memory == "" {
		c.Memory = "2G"
	}
	if c.RestartPolicy == "" {
		c.RestartPolicy = "unless-stopped"
	}
	c.Ops = normalizeList(c.Ops)
	c.Whitelist = normalizeList(c.Whitelist)
	c.ModrinthProjects = normalizeList(c.ModrinthProjects)
	c.DataPath = strings.TrimSpace(c.DataPath)
	c.Modpack = strings.TrimSpace(c.Modpack)
	c.CFApiKey = strings.TrimSpace(c.CFApiKey)
	c.DisplayName = strings.TrimSpace(c.DisplayName)
	c.Description = strings.TrimSpace(c.Description)
	c.PublicAddress = strings.TrimSpace(c.PublicAddress)
	if c.IdleMode != "" && c.IdleMinutes == 0 {
		c.IdleMinutes = 10
	}
	// Auto-stop exits the container; a restart policy would bring it right back.
	if c.IdleMode == "stop" && (c.RestartPolicy == "always" || c.RestartPolicy == "unless-stopped") {
		c.RestartPolicy = "on-failure"
	}
	for i := range c.Ports {
		c.Ports[i].Protocol = strings.ToLower(strings.TrimSpace(c.Ports[i].Protocol))
		if c.Ports[i].Protocol == "" {
			c.Ports[i].Protocol = "tcp"
		}
	}
}

func normalizeList(s string) string {
	f := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == '\n' || r == ' ' || r == ';' || r == '\r' })
	return strings.Join(f, ",")
}

func (c *ServerConfig) heapMB() int {
	m := memRe.FindStringSubmatch(c.Memory)
	if m == nil {
		return 0
	}
	n, _ := strconv.Atoi(m[1])
	if m[2] == "G" {
		n *= 1024
	}
	return n
}

func (c *ServerConfig) Validate() error {
	switch {
	case !nameRe.MatchString(c.Name):
		return badRequest("Name: nur Kleinbuchstaben, Ziffern und Bindestriche (max. 31 Zeichen)")
	case !serverTypes[c.Type]:
		return badRequest("Unbekannter Servertyp %q", c.Type)
	case !versionRe.MatchString(c.Version):
		return badRequest("Ungültige Version %q", c.Version)
	case !tagRe.MatchString(c.JavaTag):
		return badRequest("Ungültiger Image-Tag %q", c.JavaTag)
	case c.Port < 1 || c.Port > 65535:
		return badRequest("Port muss zwischen 1 und 65535 liegen")
	case !gamemodes[c.Gamemode]:
		return badRequest("Ungültiger Spielmodus %q", c.Gamemode)
	case !difficulties[c.Difficulty]:
		return badRequest("Ungültiger Schwierigkeitsgrad %q", c.Difficulty)
	case c.MaxPlayers < 1 || c.MaxPlayers > 1000:
		return badRequest("Max. Spieler muss zwischen 1 und 1000 liegen")
	case c.ViewDistance < 2 || c.ViewDistance > 32:
		return badRequest("Sichtweite muss zwischen 2 und 32 liegen")
	case !levelTypes[c.LevelType]:
		return badRequest("Ungültiger Welttyp %q", c.LevelType)
	case c.heapMB() < 512:
		return badRequest("Arbeitsspeicher im Format 2G oder 1536M angeben (mind. 512M)")
	case c.MemoryLimitMB != 0 && c.MemoryLimitMB < c.heapMB()+256:
		return badRequest("Container-Limit muss mindestens Java-Speicher + 256 MB betragen (%d MB)", c.heapMB()+256)
	case c.CPUs < 0 || c.CPUs > 256:
		return badRequest("Ungültiges CPU-Limit")
	case !restarts[c.RestartPolicy]:
		return badRequest("Ungültige Neustart-Richtlinie %q", c.RestartPolicy)
	case len(c.Motd) > 200 || strings.ContainsAny(c.Motd, "\n\r"):
		return badRequest("MOTD: eine Zeile, max. 200 Zeichen")
	case len(c.Seed) > 64:
		return badRequest("Seed ist zu lang")
	case c.DataPath != "" && (!path.IsAbs(c.DataPath) || strings.ContainsAny(c.DataPath, ":\n")):
		return badRequest("Datenpfad muss ein absoluter Pfad auf dem Docker-Host sein")
	case c.Icon != "" && !strings.HasPrefix(c.Icon, "https://") && !strings.HasPrefix(c.Icon, "http://"):
		return badRequest("Server-Icon muss eine http(s)-URL sein")
	case c.isModpack() && c.Modpack == "":
		return badRequest("Für einen Modpack-Server bitte Slug, ID oder URL des Modpacks angeben")
	case c.Modpack != "" && !modpackRe.MatchString(c.Modpack):
		return badRequest("Ungültige Modpack-Angabe")
	case strings.ContainsAny(c.CFApiKey, "\n\r"):
		return badRequest("Ungültiger CurseForge-API-Key")
	case len(c.DisplayName) > 48:
		return badRequest("Anzeigename: max. 48 Zeichen")
	case len(c.Description) > 1500:
		return badRequest("Beschreibung: max. 1500 Zeichen")
	case c.PublicAddress != "" && !pubAddrRe.MatchString(c.PublicAddress):
		return badRequest("Öffentliche Adresse im Format play.example.com oder play.example.com:25566")
	case c.IdleMode != "" && c.IdleMode != "pause" && c.IdleMode != "stop":
		return badRequest("Ungültiger Leerlauf-Modus %q", c.IdleMode)
	case c.IdleMode != "" && (c.IdleMinutes < 1 || c.IdleMinutes > 1440):
		return badRequest("Leerlauf-Zeit: 1 bis 1440 Minuten")
	case c.Map && (c.Type == "VANILLA" || c.Type == "AUTO_CURSEFORGE"):
		return badRequest("Die Live-Karte (BlueMap) braucht Paper, Purpur, Spigot, Fabric, Quilt, Forge, NeoForge oder ein Modrinth-Pack")
	case c.MapPort != 0 && (c.MapPort < 1 || c.MapPort > 65535):
		return badRequest("Ungültiger Karten-Port")
	case c.ProxyMember && !c.proxyCapable():
		return badRequest("Im Velocity-Netzwerk können nur Paper, Purpur, Folia, Fabric und Quilt laufen")
	}
	for _, list := range []string{c.Ops, c.Whitelist} {
		for _, p := range strings.Split(list, ",") {
			if p != "" && !playerRe.MatchString(p) {
				return badRequest("Ungültiger Spielername oder UUID %q", p)
			}
		}
	}
	for k := range c.ExtraEnv {
		if !envKeyRe.MatchString(k) {
			return badRequest("Ungültiger Variablenname %q (A-Z, 0-9, _)", k)
		}
	}
	if len(c.Ports) > 20 {
		return badRequest("Maximal 20 zusätzliche Ports")
	}
	for _, p := range c.Ports {
		if p.Host < 1 || p.Host > 65535 || p.Container < 1 || p.Container > 65535 || (p.Protocol != "tcp" && p.Protocol != "udp") {
			return badRequest("Ungültige Portfreigabe %d:%d/%s", p.Host, p.Container, p.Protocol)
		}
		if p.Container == 25565 && p.Protocol == "tcp" {
			return badRequest("Container-Port 25565/tcp ist bereits der Spielport")
		}
	}
	return nil
}

func (c *ServerConfig) Image() string { return imageRepo + ":" + c.JavaTag }

func (c *ServerConfig) isModpack() bool { return c.Type == "MODRINTH" || c.Type == "AUTO_CURSEFORGE" }

func (c *ServerConfig) isPaperFamily() bool {
	return c.Type == "PAPER" || c.Type == "PURPUR" || c.Type == "FOLIA"
}

func (c *ServerConfig) isFabricFamily() bool { return c.Type == "FABRIC" || c.Type == "QUILT" }

// proxyCapable: backends that support Velocity modern forwarding.
func (c *ServerConfig) proxyCapable() bool { return c.isPaperFamily() || c.isFabricFamily() }

// levelName is the world folder (itzg LEVEL, default "world").
func (c *ServerConfig) levelName() string {
	if v := c.ExtraEnv["LEVEL"]; v != "" {
		return v
	}
	return "world"
}

// blueMapConf is the path (relative to /data) of BlueMap's core.conf.
func (c *ServerConfig) blueMapConf() string {
	if c.ModDir() == "plugins" {
		return "plugins/BlueMap/core.conf"
	}
	return "config/bluemap/core.conf"
}

// effectiveProjects includes projects CraftDock adds itself (BlueMap, FabricProxy-Lite).
func (c *ServerConfig) effectiveProjects() string {
	list := c.ModrinthProjects
	add := func(slug string) {
		for _, p := range strings.Split(list, ",") {
			if refSlug(p) == slug {
				return
			}
		}
		if list != "" {
			list += ","
		}
		list += slug
	}
	if c.Map {
		add("bluemap")
	}
	if c.ProxyMember && c.isFabricFamily() {
		add("fabricproxy-lite")
	}
	return list
}

// Loader returns the mod/plugin ecosystem of the server type ("" if unknown).
func (c *ServerConfig) Loader() string {
	switch c.Type {
	case "FABRIC", "QUILT", "FORGE", "NEOFORGE", "PAPER", "PURPUR", "FOLIA", "SPIGOT":
		return strings.ToLower(c.Type)
	}
	return ""
}

// ModDir is where jars of this server type live (relative to /data).
func (c *ServerConfig) ModDir() string {
	switch c.Type {
	case "PAPER", "PURPUR", "FOLIA", "SPIGOT":
		return "plugins"
	case "VANILLA":
		return ""
	}
	return "mods"
}

func (c *ServerConfig) cfKey() string {
	if c.CFApiKey != "" {
		return c.CFApiKey
	}
	return os.Getenv("CRAFTDOCK_CF_API_KEY")
}

func (c *ServerConfig) hostPorts() []string {
	out := []string{}
	if !c.ProxyMember {
		out = append(out, fmt.Sprintf("%d/tcp", c.Port))
	}
	if c.Map && c.MapPort > 0 {
		out = append(out, fmt.Sprintf("%d/tcp", c.MapPort))
	}
	for _, p := range c.Ports {
		out = append(out, fmt.Sprintf("%d/%s", p.Host, p.Protocol))
	}
	return out
}

func boolStr(b bool) string { return strconv.FormatBool(b) }

func (c *ServerConfig) Env() []string {
	env := map[string]string{
		"EULA":          "TRUE",
		"TYPE":          c.Type,
		"VERSION":       c.Version,
		"MEMORY":        c.Memory,
		"MODE":          c.Gamemode,
		"DIFFICULTY":    c.Difficulty,
		"MAX_PLAYERS":   strconv.Itoa(c.MaxPlayers),
		"ONLINE_MODE":   boolStr(c.OnlineMode),
		"PVP":           boolStr(c.PVP),
		"HARDCORE":      boolStr(c.Hardcore),
		"VIEW_DISTANCE": strconv.Itoa(c.ViewDistance),
		"ENABLE_RCON":   "true",
		"RCON_PASSWORD": c.RconPassword,
	}
	switch c.Type {
	case "MODRINTH":
		env["MODRINTH_MODPACK"] = c.Modpack
	case "AUTO_CURSEFORGE":
		if strings.Contains(c.Modpack, "curseforge.com/") {
			env["CF_PAGE_URL"] = c.Modpack
		} else {
			env["CF_SLUG"] = c.Modpack
		}
		if k := c.cfKey(); k != "" {
			env["CF_API_KEY"] = k
		}
	}
	if c.isModpack() && c.Version == "LATEST" {
		delete(env, "VERSION") // the pack decides
	}
	opt := map[string]string{
		"MOTD": c.Motd, "SEED": c.Seed, "LEVEL_TYPE": c.LevelType, "OPS": c.Ops,
		"ICON": c.Icon, "MODRINTH_PROJECTS": c.effectiveProjects(), "TZ": os.Getenv("TZ"),
	}
	secs := strconv.Itoa(c.IdleMinutes * 60)
	switch c.IdleMode {
	case "pause":
		env["ENABLE_AUTOPAUSE"] = "TRUE"
		env["AUTOPAUSE_TIMEOUT_EST"] = secs
		env["AUTOPAUSE_TIMEOUT_INIT"] = secs
		env["MAX_TICK_TIME"] = "-1" // the watchdog would kill a resumed server
	case "stop":
		env["ENABLE_AUTOSTOP"] = "TRUE"
		env["AUTOSTOP_TIMEOUT_EST"] = secs
		env["AUTOSTOP_TIMEOUT_INIT"] = secs
	}
	if c.ProxyMember {
		env["ONLINE_MODE"] = "false" // authentication happens at the proxy
		if c.isFabricFamily() && proxySecretValue() != "" {
			env["FABRIC_PROXY_SECRET"] = proxySecretValue()
		}
	}
	for k, v := range opt {
		if v != "" {
			env[k] = v
		}
	}
	if c.Whitelist != "" || c.EnforceWhitelist {
		env["ENABLE_WHITELIST"] = "true"
		env["ENFORCE_WHITELIST"] = boolStr(c.EnforceWhitelist)
		if c.Whitelist != "" {
			env["WHITELIST"] = c.Whitelist
		}
	}
	for k, v := range c.ExtraEnv {
		env[k] = v
	}
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, k+"="+env[k])
	}
	return out
}

func containerName(name string) string { return "mc-" + name }
func volumeName(name string) string    { return "mc-" + name + "-data" }

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ---- Manager ---------------------------------------------------------------------

type Task struct {
	Action  string        `json:"action"`
	Message string        `json:"message"`
	Error   string        `json:"error,omitempty"`
	Running bool          `json:"running"`
	Updated time.Time     `json:"updated"`
	pending *ServerConfig // config of a server that does not exist yet
}

type ServerView struct {
	Name        string       `json:"name"`
	Config      ServerConfig `json:"config"`
	ContainerID string       `json:"containerId,omitempty"`
	Image       string       `json:"image,omitempty"`
	State       string       `json:"state"`
	Health      string       `json:"health,omitempty"`
	StatusText  string       `json:"statusText,omitempty"`
	Phase       string       `json:"phase"`
	Task        *Task        `json:"task,omitempty"`
	CFKeySet    bool         `json:"cfKeySet,omitempty"`
	Sleeping    bool         `json:"sleeping,omitempty"`
}

type ServerDetail struct {
	ServerView
	StartedAt string `json:"startedAt,omitempty"`
	Created   string `json:"created,omitempty"`
	Volume    string `json:"volume"`
	DataPath  string `json:"dataPath,omitempty"`
}

type Manager struct {
	d       *Docker
	network string
	store   *Store
	mu      sync.Mutex
	tasks   map[string]*Task

	liveMu      sync.Mutex
	live        map[string]*liveEntry
	sampleEvery time.Duration
	sleeping    map[string]bool

	proxyMu  sync.Mutex
	proxyCfg *ProxyConfig

	modMu      sync.Mutex
	modrinth   map[string]modrinthCacheEntry
	modFiles   map[string]modFilesCacheEntry
	publicMu   sync.Mutex
	publicList []publicServer
	publicAt   time.Time

	verMu    sync.Mutex
	versions *versionList
	verTime  time.Time
}

func NewManager(d *Docker, network string, store *Store) *Manager {
	return &Manager{d: d, network: network, store: store, tasks: map[string]*Task{},
		modrinth: map[string]modrinthCacheEntry{}, modFiles: map[string]modFilesCacheEntry{}}
}

func (m *Manager) startTask(name, action, msg string, pending *ServerConfig, fn func(ctx context.Context, progress func(string)) error) error {
	m.mu.Lock()
	if t, ok := m.tasks[name]; ok && t.Running {
		m.mu.Unlock()
		return conflict("Für „%s“ läuft bereits ein Vorgang", name)
	}
	t := &Task{Action: action, Message: msg, Running: true, Updated: time.Now(), pending: pending}
	m.tasks[name] = t
	m.mu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		err := fn(ctx, func(s string) {
			m.mu.Lock()
			t.Message, t.Updated = s, time.Now()
			m.mu.Unlock()
		})
		m.mu.Lock()
		defer m.mu.Unlock()
		t.Running, t.Updated = false, time.Now()
		if err != nil {
			t.Error = err.Error()
			log.Printf("[%s] %s fehlgeschlagen: %v", name, action, err)
		} else {
			log.Printf("[%s] %s abgeschlossen", name, action)
			if m.tasks[name] == t {
				delete(m.tasks, name)
			}
		}
	}()
	return nil
}

func (m *Manager) task(name string) *Task {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tasks[name]
	if !ok {
		return nil
	}
	if !t.Running && time.Since(t.Updated) > 15*time.Minute {
		delete(m.tasks, name)
		return nil
	}
	cp := *t
	return &cp
}

func (m *Manager) ClearTask(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.tasks[name]; ok && !t.Running {
		delete(m.tasks, name)
	}
}

func configFromLabels(labels map[string]string) ServerConfig {
	var c ServerConfig
	_ = json.Unmarshal([]byte(labels[labelConfig]), &c)
	if c.Name == "" {
		c.Name = labels[labelName]
	}
	return c
}

func publicConfig(c ServerConfig) ServerConfig {
	c.RconPassword = ""
	c.CFApiKey = ""
	return c
}

func (m *Manager) view(c ContainerSummary) ServerView {
	cfg := configFromLabels(c.Labels)
	v := ServerView{
		Name: cfg.Name, Config: publicConfig(cfg), ContainerID: c.ID, Image: c.Image,
		State: c.State, StatusText: c.Status, Task: m.task(cfg.Name), CFKeySet: cfg.cfKey() != "",
	}
	switch {
	case strings.Contains(c.Status, "(health: starting)"):
		v.Health = "starting"
	case strings.Contains(c.Status, "(unhealthy)"):
		v.Health = "unhealthy"
	case strings.Contains(c.Status, "(healthy)"):
		v.Health = "healthy"
	}
	if cfg.IdleMode == "pause" && c.State == "running" {
		v.Sleeping = m.cachedSleeping(cfg.Name)
	}
	v.Phase = phase(v)
	return v
}

func phase(v ServerView) string {
	if v.Task != nil && v.Task.Running {
		return "busy"
	}
	switch v.State {
	case "running":
		if v.Sleeping {
			return "sleeping"
		}
		switch v.Health {
		case "starting":
			return "starting"
		case "unhealthy":
			return "unhealthy"
		}
		return "online"
	case "restarting":
		return "restarting"
	case "paused":
		return "paused"
	case "":
		return "error"
	}
	return "stopped"
}

func (m *Manager) List(ctx context.Context) ([]ServerView, error) {
	cs, err := m.d.ListContainers(ctx, labelManaged+"=true")
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	out := make([]ServerView, 0, len(cs))
	for _, c := range cs {
		v := m.view(c)
		seen[v.Name] = true
		out = append(out, v)
	}
	// Servers that exist only as a running/failed task (e.g. image still pulling).
	m.mu.Lock()
	var pending []*ServerConfig
	for name, t := range m.tasks {
		if !seen[name] && t.pending != nil {
			pending = append(pending, t.pending)
		}
	}
	m.mu.Unlock()
	for _, cfg := range pending {
		v := ServerView{Name: cfg.Name, Config: publicConfig(*cfg), Task: m.task(cfg.Name)}
		v.Phase = phase(v)
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (m *Manager) find(ctx context.Context, name string) (*ContainerSummary, error) {
	cs, err := m.d.ListContainers(ctx, labelManaged+"=true", labelName+"="+name)
	if err != nil {
		return nil, err
	}
	if len(cs) == 0 {
		return nil, ErrServerNotFound
	}
	return &cs[0], nil
}

func (m *Manager) Get(ctx context.Context, name string) (ServerDetail, error) {
	c, err := m.find(ctx, name)
	if err != nil {
		if errors.Is(err, ErrServerNotFound) {
			if t := m.task(name); t != nil {
				m.mu.Lock()
				p := m.tasks[name].pending
				m.mu.Unlock()
				if p != nil {
					v := ServerView{Name: name, Config: publicConfig(*p), Task: t}
					v.Phase = phase(v)
					return ServerDetail{ServerView: v, Volume: volumeName(name), DataPath: p.DataPath}, nil
				}
			}
		}
		return ServerDetail{}, err
	}
	d := ServerDetail{ServerView: m.view(*c), Volume: volumeName(name)}
	d.DataPath = d.Config.DataPath
	if ins, err := m.d.Inspect(ctx, c.ID); err == nil {
		d.Created = ins.Created
		if ins.State.Running {
			d.StartedAt = ins.State.StartedAt
		}
	}
	return d, nil
}

func (m *Manager) checkPorts(ctx context.Context, cfg ServerConfig) error {
	mine := map[string]bool{}
	for _, p := range cfg.hostPorts() {
		if mine[p] {
			return badRequest("Host-Port %s ist doppelt vergeben", p)
		}
		mine[p] = true
	}
	if pc := m.proxyConfig(); pc != nil && mine[strconv.Itoa(pc.Port)+"/tcp"] {
		return conflict("Host-Port %d wird bereits vom Velocity-Proxy verwendet", pc.Port)
	}
	cs, err := m.d.ListContainers(ctx, labelManaged+"=true")
	if err != nil {
		return err
	}
	for _, c := range cs {
		other := configFromLabels(c.Labels)
		if other.Name == cfg.Name {
			continue
		}
		for _, p := range other.hostPorts() {
			if mine[p] {
				return conflict("Host-Port %s wird bereits von Server „%s“ verwendet", p, other.Name)
			}
		}
	}
	return nil
}

func (m *Manager) spec(cfg ServerConfig) map[string]any {
	cfgJSON, _ := json.Marshal(cfg)
	exposed := map[string]struct{}{"25565/tcp": {}}
	bindings := map[string][]map[string]string{}
	if !cfg.ProxyMember { // network members are only reachable through the proxy
		bindings["25565/tcp"] = []map[string]string{{"HostPort": strconv.Itoa(cfg.Port)}}
	}
	if cfg.Map {
		exposed["8100/tcp"] = struct{}{}
		if cfg.MapPort > 0 {
			bindings["8100/tcp"] = []map[string]string{{"HostPort": strconv.Itoa(cfg.MapPort)}}
		}
	}
	for _, p := range cfg.Ports {
		key := fmt.Sprintf("%d/%s", p.Container, p.Protocol)
		exposed[key] = struct{}{}
		bindings[key] = append(bindings[key], map[string]string{"HostPort": strconv.Itoa(p.Host)})
	}
	bind := volumeName(cfg.Name) + ":/data"
	if cfg.DataPath != "" {
		bind = cfg.DataPath + ":/data"
	}
	host := map[string]any{
		"PortBindings":  bindings,
		"Binds":         []string{bind},
		"RestartPolicy": map[string]any{"Name": cfg.RestartPolicy},
	}
	if cfg.MemoryLimitMB > 0 {
		host["Memory"] = int64(cfg.MemoryLimitMB) << 20
	}
	if cfg.CPUs > 0 {
		host["NanoCpus"] = int64(cfg.CPUs * 1e9)
	}
	if m.network != "" {
		host["NetworkMode"] = m.network
	}
	return map[string]any{
		"Image":        cfg.Image(),
		"Env":          cfg.Env(),
		"Tty":          true,
		"OpenStdin":    true,
		"ExposedPorts": exposed,
		"Labels": map[string]string{
			labelManaged: "true",
			labelName:    cfg.Name,
			labelConfig:  string(cfgJSON),
		},
		"HostConfig": host,
	}
}

func (m *Manager) ensureImage(ctx context.Context, ref string, p func(string)) error {
	p("Image " + ref + " wird geladen …")
	err := m.d.PullImage(ctx, ref, p)
	if err == nil {
		return nil
	}
	if m.d.ImageExists(ctx, ref) {
		log.Printf("Pull von %s fehlgeschlagen (%v), verwende lokales Image", ref, err)
		return nil
	}
	return fmt.Errorf("Image %s konnte nicht geladen werden: %w", ref, err)
}

func (m *Manager) Create(ctx context.Context, cfg ServerConfig) error {
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return err
	}
	if _, err := m.find(ctx, cfg.Name); err == nil {
		return conflict("Ein Server namens „%s“ existiert bereits", cfg.Name)
	} else if !errors.Is(err, ErrServerNotFound) {
		return err
	}
	if _, err := m.d.Inspect(ctx, containerName(cfg.Name)); err == nil {
		return conflict("Es existiert bereits ein fremder Container „%s“", containerName(cfg.Name))
	}
	if err := m.checkPorts(ctx, cfg); err != nil {
		return err
	}
	if err := m.checkProxyMember(cfg); err != nil {
		return err
	}
	cfg.RconPassword = randomHex(16)
	cfg.CreatedAt = time.Now().UTC()
	pending := cfg
	return m.startTask(cfg.Name, "create", "Wird vorbereitet …", &pending, func(ctx context.Context, p func(string)) error {
		if err := m.ensureImage(ctx, cfg.Image(), p); err != nil {
			return err
		}
		p("Container wird erstellt …")
		id, err := m.d.CreateContainer(ctx, containerName(cfg.Name), m.spec(cfg))
		if err != nil {
			return err
		}
		if cfg.ProxyMember {
			defer m.refreshProxyAsync()
		}
		return m.startContainer(ctx, id, false, cfg, p)
	})
}

// startContainer writes generated config files (BlueMap, Velocity forwarding)
// and starts the container.
func (m *Manager) startContainer(ctx context.Context, id string, running bool, cfg ServerConfig, p func(string)) error {
	if err := m.prepareFiles(ctx, id, running, cfg); err != nil {
		return fmt.Errorf("Konfigurationsdateien: %w", err)
	}
	p("Server wird gestartet …")
	return m.d.Start(ctx, id)
}

func (m *Manager) Update(ctx context.Context, name string, cfg ServerConfig) error {
	return m.recreate(ctx, name, &cfg, false)
}

// recreate replaces the container with a new one (new config and/or fresh image).
// World data lives in the volume and is untouched. cfg == nil keeps the current config.
func (m *Manager) recreate(ctx context.Context, name string, newCfg *ServerConfig, forcePull bool) error {
	c, err := m.find(ctx, name)
	if err != nil {
		return err
	}
	old := configFromLabels(c.Labels)
	cfg := old
	if newCfg != nil {
		cfg = *newCfg
	}
	cfg.Name = name
	cfg.RconPassword = old.RconPassword
	cfg.CreatedAt = old.CreatedAt
	cfg.DataPath = old.DataPath // changing it would orphan the world
	if cfg.CFApiKey == "" {
		cfg.CFApiKey = old.CFApiKey // the UI never sees the stored key
	}
	if cfg.RconPassword == "" {
		cfg.RconPassword = randomHex(16)
	}
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return err
	}
	if err := m.checkPorts(ctx, cfg); err != nil {
		return err
	}
	if err := m.checkProxyMember(cfg); err != nil {
		return err
	}
	id := c.ID
	action, msg := "update", "Konfiguration wird angewendet …"
	if forcePull {
		action, msg = "update-image", "Image wird aktualisiert …"
	}
	return m.startTask(name, action, msg, nil, func(ctx context.Context, p func(string)) error {
		if forcePull || cfg.Image() != old.Image() || !m.d.ImageExists(ctx, cfg.Image()) {
			if err := m.ensureImage(ctx, cfg.Image(), p); err != nil {
				return err
			}
		}
		_, err := m.replaceContainer(ctx, id, old, cfg, true, p)
		if old.ProxyMember != cfg.ProxyMember || (cfg.ProxyMember && old.Type != cfg.Type) {
			m.refreshProxyAsync()
		}
		return err
	})
}

// replaceContainer stops (if needed) and recreates the container with cfg,
// rolling back to old on failure. With restart=true a previously running
// server is started again. Returns the new container ID.
func (m *Manager) replaceContainer(ctx context.Context, id string, old, cfg ServerConfig, restart bool, p func(string)) (string, error) {
	ins, err := m.d.Inspect(ctx, id)
	if err != nil {
		return "", err
	}
	if ins.State.Running {
		p("Server wird gestoppt (Welt wird gespeichert) …")
		if err := m.d.Stop(ctx, id, stopTimeout); err != nil {
			return "", err
		}
	}
	p("Container wird neu erstellt …")
	if err := m.d.Remove(ctx, id); err != nil {
		return "", err
	}
	newID, err := m.d.CreateContainer(ctx, containerName(cfg.Name), m.spec(cfg))
	if err != nil {
		// Roll back so the server does not disappear.
		if oldID, rbErr := m.d.CreateContainer(ctx, containerName(cfg.Name), m.spec(old)); rbErr == nil && ins.State.Running && restart {
			_ = m.startContainer(ctx, oldID, false, old, p)
		}
		return "", fmt.Errorf("neue Konfiguration abgelehnt, alte wiederhergestellt: %w", err)
	}
	if old.ProxyMember && !cfg.ProxyMember && cfg.isPaperFamily() {
		_ = m.paperVelocity(ctx, newID, false, cfg, false)
	}
	if ins.State.Running && restart {
		return newID, m.startContainer(ctx, newID, false, cfg, p)
	}
	return newID, nil
}

func (m *Manager) Delete(ctx context.Context, name string, removeData bool) error {
	c, err := m.find(ctx, name)
	if err != nil {
		return err
	}
	cfg := configFromLabels(c.Labels)
	id, running := c.ID, c.State == "running"
	return m.startTask(name, "delete", "Server wird gelöscht …", nil, func(ctx context.Context, p func(string)) error {
		if running {
			p("Server wird gestoppt …")
			_ = m.d.Stop(ctx, id, stopTimeout)
		}
		if err := m.d.Remove(ctx, id); err != nil && !IsNotFound(err) {
			return err
		}
		if cfg.ProxyMember {
			m.refreshProxyAsync()
		}
		if removeData && cfg.DataPath == "" {
			p("Weltdaten werden gelöscht …")
			if err := m.d.RemoveVolume(ctx, volumeName(name)); err != nil && !IsNotFound(err) {
				return err
			}
		}
		return nil
	})
}

func (m *Manager) Action(ctx context.Context, name, action string) error {
	c, err := m.find(ctx, name)
	if err != nil {
		return err
	}
	id := c.ID
	cfg := configFromLabels(c.Labels)
	switch action {
	case "start":
		return m.startTask(name, action, "Server wird gestartet …", nil, func(ctx context.Context, p func(string)) error {
			return m.startContainer(ctx, id, false, cfg, p)
		})
	case "stop":
		return m.startTask(name, action, "Server wird gestoppt (Welt wird gespeichert) …", nil, func(ctx context.Context, _ func(string)) error {
			return m.d.Stop(ctx, id, stopTimeout)
		})
	case "restart":
		running := c.State == "running"
		return m.startTask(name, action, "Server wird neu gestartet …", nil, func(ctx context.Context, _ func(string)) error {
			if err := m.prepareFiles(ctx, id, running, cfg); err != nil {
				return err
			}
			return m.d.Restart(ctx, id, stopTimeout)
		})
	case "update-image":
		return m.recreate(ctx, name, nil, true)
	}
	return badRequest("Unbekannte Aktion %q", action)
}

// ---- Console / RCON via rcon-cli inside the container ------------------------------------

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;?]*[A-Za-z]|§[0-9a-fk-orA-FK-OR]`)

func stripFormatting(s string) string { return ansiRe.ReplaceAllString(s, "") }

func (m *Manager) runningContainer(ctx context.Context, name string) (string, error) {
	c, err := m.find(ctx, name)
	if err != nil {
		return "", err
	}
	if c.State != "running" {
		return "", conflict("Server „%s“ läuft nicht", name)
	}
	return c.ID, nil
}

func (m *Manager) rcon(ctx context.Context, id, cmd string) (string, error) {
	out, code, err := m.d.Exec(ctx, id, []string{"rcon-cli", cmd})
	out = strings.TrimSpace(stripFormatting(out))
	if err != nil {
		return out, err
	}
	if code != 0 {
		if out == "" {
			out = fmt.Sprintf("rcon-cli beendet mit Code %d", code)
		}
		return "", &APIError{http.StatusBadGateway, "RCON: " + out + " (startet der Server noch?)"}
	}
	return out, nil
}

func (m *Manager) Command(ctx context.Context, name, cmd string) (string, error) {
	cmd = strings.TrimPrefix(strings.TrimSpace(cmd), "/")
	if cmd == "" || len(cmd) > 1000 {
		return "", badRequest("Befehl fehlt oder ist zu lang")
	}
	id, err := m.runningContainer(ctx, name)
	if err != nil {
		return "", err
	}
	return m.rcon(ctx, id, cmd)
}

type PlayerList struct {
	Online int      `json:"online"`
	Max    int      `json:"max"`
	Names  []string `json:"names"`
}

var listRe = regexp.MustCompile(`(?i)There are (\d+) (?:of a max(?: of)?|/) ?(\d+) players online:?(.*)`)

func (m *Manager) Players(ctx context.Context, name string) (PlayerList, error) {
	id, err := m.runningContainer(ctx, name)
	if err != nil {
		return PlayerList{}, err
	}
	out, err := m.rcon(ctx, id, "list")
	if err != nil {
		return PlayerList{}, err
	}
	return parsePlayers(out), nil
}

func (m *Manager) Stats(ctx context.Context, name string) (ContainerStats, error) {
	id, err := m.runningContainer(ctx, name)
	if err != nil {
		return ContainerStats{}, err
	}
	return m.d.Stats(ctx, id)
}

// ---- Editable files -------------------------------------------------------------------------

var editableFiles = []string{
	"server.properties", "ops.json", "whitelist.json", "banned-players.json", "banned-ips.json",
	"bukkit.yml", "spigot.yml", "purpur.yml", "config/paper-global.yml", "config/paper-world-defaults.yml",
}

func isEditable(p string) bool {
	for _, f := range editableFiles {
		if f == p {
			return true
		}
	}
	return false
}

func (m *Manager) ListFiles(ctx context.Context, name string) ([]string, error) {
	c, err := m.find(ctx, name)
	if err != nil {
		return nil, err
	}
	out := []string{}
	for _, f := range editableFiles {
		if ok, err := m.d.StatPath(ctx, c.ID, "/data/"+f); err != nil {
			return nil, err
		} else if ok {
			out = append(out, f)
		}
	}
	return out, nil
}

func (m *Manager) ReadFile(ctx context.Context, name, file string) (string, error) {
	if !isEditable(file) {
		return "", badRequest("Datei %q ist nicht editierbar", file)
	}
	c, err := m.find(ctx, name)
	if err != nil {
		return "", err
	}
	b, err := m.d.ReadFile(ctx, c.ID, "/data/"+file, 2<<20)
	if IsNotFound(err) {
		return "", &APIError{http.StatusNotFound, "Datei existiert (noch) nicht"}
	}
	return string(b), err
}

func (m *Manager) WriteFile(ctx context.Context, name, file, content string) error {
	if !isEditable(file) {
		return badRequest("Datei %q ist nicht editierbar", file)
	}
	if strings.HasSuffix(file, ".json") && !json.Valid([]byte(content)) {
		return badRequest("Ungültiges JSON – Datei wurde nicht gespeichert")
	}
	c, err := m.find(ctx, name)
	if err != nil {
		return err
	}
	full := "/data/" + file
	return m.d.WriteFile(ctx, c.ID, path.Dir(full), path.Base(full), []byte(content), 1000, 1000)
}

// ---- Minecraft versions (Mojang manifest) --------------------------------------------------------

type versionList struct {
	Latest   string   `json:"latest"`
	Snapshot string   `json:"snapshot"`
	Releases []string `json:"releases"`
}

func (m *Manager) Versions(ctx context.Context) (*versionList, error) {
	m.verMu.Lock()
	defer m.verMu.Unlock()
	if m.versions != nil && time.Since(m.verTime) < time.Hour {
		return m.versions, nil
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://piston-meta.mojang.com/mc/game/version_manifest_v2.json", nil)
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		if m.versions != nil {
			return m.versions, nil
		}
		return nil, err
	}
	defer resp.Body.Close()
	var mf struct {
		Latest struct {
			Release  string `json:"release"`
			Snapshot string `json:"snapshot"`
		} `json:"latest"`
		Versions []struct {
			ID   string `json:"id"`
			Type string `json:"type"`
		} `json:"versions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&mf); err != nil {
		return nil, err
	}
	vl := &versionList{Latest: mf.Latest.Release, Snapshot: mf.Latest.Snapshot, Releases: []string{}}
	for _, v := range mf.Versions {
		if v.Type == "release" {
			vl.Releases = append(vl.Releases, v.ID)
		}
	}
	m.versions, m.verTime = vl, time.Now()
	return vl, nil
}
