package main

// Alerting: rules evaluated once a minute (server down, crash loop, low TPS,
// memory, disk, proxy down) plus event rules (failed tasks, login attacks,
// administrator logins from new addresses). Notifications go to Discord,
// Slack-compatible webhooks, generic JSON webhooks, ntfy, Gotify, Telegram
// or e-mail. State-based alerts send a recovery message when they resolve.

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"net/smtp"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type AlertChannel struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"` // discord | slack | webhook | ntfy | gotify | telegram | email
	URL     string `json:"url"`
	Token   string `json:"token"`
	Target  string `json:"target"` // Telegram chat ID, e-mail recipients
	Enabled bool   `json:"enabled"`
}

type SMTPConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	From     string `json:"from"`
	Security string `json:"security"` // starttls | tls | none
}

type AlertRule struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Event     string   `json:"event"`
	Servers   []string `json:"servers"` // empty = all
	Threshold float64  `json:"threshold"`
	Minutes   int      `json:"minutes"`  // condition must hold this long
	Cooldown  int      `json:"cooldown"` // minutes between two notifications
	Channels  []string `json:"channels"`
	Resolve   bool     `json:"resolve"` // send recovery message
	Enabled   bool     `json:"enabled"`
}

type AlertConfig struct {
	Channels    []AlertChannel   `json:"channels"`
	Rules       []AlertRule      `json:"rules"`
	SMTP        SMTPConfig       `json:"smtp"`
	Maintenance map[string]int64 `json:"maintenance"` // "*" or server → unix time until
}

type AlertEvent struct {
	ID       string   `json:"id"`
	Time     int64    `json:"time"`
	Rule     string   `json:"rule"`
	RuleName string   `json:"ruleName"`
	Event    string   `json:"event"`
	Server   string   `json:"server,omitempty"`
	Level    string   `json:"level"` // critical | warning | info | ok
	Title    string   `json:"title"`
	Text     string   `json:"text"`
	Resolved bool     `json:"resolved,omitempty"`
	Muted    bool     `json:"muted,omitempty"` // maintenance
	Results  []string `json:"results"`
}

type AlertEventInfo struct {
	ID          string  `json:"id"`
	Label       string  `json:"label"`
	Description string  `json:"description"`
	Server      bool    `json:"server"`    // evaluated per server
	State       bool    `json:"state"`     // has a resolved state
	Threshold   float64 `json:"threshold"` // default, 0 = none
	Unit        string  `json:"unit,omitempty"`
	Minutes     int     `json:"minutes"`
	Level       string  `json:"level"`
}

var alertEvents = []AlertEventInfo{
	{"down", "Server ausgefallen", "Abgestürzt, hängt beim Neustart oder antwortet nicht mehr auf Pings.", true, true, 0, "", 3, "critical"},
	{"crashloop", "Absturzschleife", "Der Container wurde in 15 Minuten mehrfach neu gestartet.", true, true, 3, "Neustarts", 0, "critical"},
	{"unhealthy", "Healthcheck fehlgeschlagen", "Docker meldet den Server als „unhealthy“.", true, true, 0, "", 5, "warning"},
	{"tps", "Niedrige TPS", "Der Server laggt (Paper, Purpur, Spigot).", true, true, 15, "TPS", 5, "warning"},
	{"memory", "Hohe RAM-Auslastung", "Speicherverbrauch des Containers gemessen am Limit.", true, true, 90, "%", 5, "warning"},
	{"disk", "Wenig Speicherplatz", "Freier Platz auf dem Datenträger mit den Serverdaten.", true, true, 10, "% frei", 0, "warning"},
	{"proxy_down", "Velocity-Netzwerk ausgefallen", "Der Proxy eines Netzwerks läuft nicht.", false, true, 0, "", 2, "critical"},
	{"task_failed", "Vorgang fehlgeschlagen", "Anlegen, Update, Klonen oder Wiederherstellen ist gescheitert.", true, false, 0, "", 0, "warning"},
	{"login_failed", "Angriff auf Anmeldung", "Viele fehlgeschlagene Anmeldungen in 10 Minuten oder eine Sperre.", false, false, 10, "Fehlversuche", 0, "warning"},
	{"admin_login", "Admin-Anmeldung von neuer Adresse", "Ein Konto mit Verwaltungsrechten meldet sich von einer unbekannten IP an.", false, false, 0, "", 0, "info"},
}

func alertEventInfo(id string) (AlertEventInfo, bool) {
	for _, e := range alertEvents {
		if e.ID == id {
			return e, true
		}
	}
	return AlertEventInfo{}, false
}

var channelTypes = set("discord", "slack", "webhook", "ntfy", "gotify", "telegram", "email")

const secretMask = "••••••••"

type alertState struct {
	since    time.Time
	firing   bool
	lastSent time.Time
	detail   string
}

type Alerter struct {
	m     *Manager
	path  string
	start time.Time

	mu       sync.Mutex
	cfg      AlertConfig
	state    map[string]*alertState // rule|server
	history  []AlertEvent
	restarts map[string][]restartSample
	disk     map[string]diskSample
	fails    []time.Time
	client   *http.Client
}

type restartSample struct {
	t time.Time
	n int
}

type diskSample struct {
	t        time.Time
	freePct  float64
	freeText string
	err      bool
}

func NewAlerter(m *Manager, dataDir string) *Alerter {
	al := &Alerter{m: m, start: time.Now(), state: map[string]*alertState{}, restarts: map[string][]restartSample{},
		disk: map[string]diskSample{}, client: &http.Client{Timeout: 10 * time.Second}}
	al.cfg.Maintenance = map[string]int64{}
	if dataDir != "" {
		al.path = filepath.Join(dataDir, "alerts.json")
		if b, err := os.ReadFile(al.path); err == nil {
			if err := json.Unmarshal(b, &al.cfg); err != nil {
				log.Printf("WARNUNG: alerts.json ist beschädigt: %v", err)
			}
		}
		if b, err := os.ReadFile(filepath.Join(dataDir, "alert-history.json")); err == nil {
			_ = json.Unmarshal(b, &al.history)
		}
	}
	if al.cfg.Maintenance == nil {
		al.cfg.Maintenance = map[string]int64{}
	}
	if al.cfg.SMTP.Security == "" {
		al.cfg.SMTP.Security = "starttls"
	}
	return al
}

// ---- configuration ----------------------------------------------------------------

var alertIDRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,40}$`)

func (al *Alerter) Config(masked bool) AlertConfig {
	al.mu.Lock()
	defer al.mu.Unlock()
	c := al.cfg
	c.Channels = append([]AlertChannel{}, al.cfg.Channels...)
	c.Rules = append([]AlertRule{}, al.cfg.Rules...)
	c.Maintenance = map[string]int64{}
	now := time.Now().Unix()
	for k, v := range al.cfg.Maintenance {
		if v > now {
			c.Maintenance[k] = v
		}
	}
	if masked {
		for i := range c.Channels {
			if c.Channels[i].Token != "" {
				c.Channels[i].Token = secretMask
			}
			if c.Channels[i].Type == "discord" || c.Channels[i].Type == "slack" || c.Channels[i].Type == "telegram" {
				c.Channels[i].URL = maskURL(c.Channels[i].URL)
			}
		}
		if c.SMTP.Password != "" {
			c.SMTP.Password = secretMask
		}
	}
	return c
}

// maskURL hides the secret path of webhook URLs (Discord/Slack embed the token).
func maskURL(s string) string {
	u, err := url.Parse(s)
	if err != nil || s == "" {
		return s
	}
	if len(u.Path) > 12 {
		return u.Scheme + "://" + u.Host + u.Path[:12] + secretMask
	}
	return s
}

func (al *Alerter) SetConfig(in AlertConfig) error {
	al.mu.Lock()
	old := al.cfg
	al.mu.Unlock()
	oldCh := map[string]AlertChannel{}
	for _, c := range old.Channels {
		oldCh[c.ID] = c
	}
	ids := map[string]bool{}
	for i := range in.Channels {
		c := &in.Channels[i]
		c.ID = strings.TrimSpace(c.ID)
		if c.ID == "" {
			c.ID = randomHex(4)
		}
		c.Name, c.URL, c.Target = strings.TrimSpace(c.Name), strings.TrimSpace(c.URL), strings.TrimSpace(c.Target)
		prev := oldCh[c.ID]
		if c.Token == secretMask {
			c.Token = prev.Token
		}
		if strings.Contains(c.URL, secretMask) {
			c.URL = prev.URL
		}
		switch {
		case !alertIDRe.MatchString(c.ID) || ids[c.ID]:
			return badRequest("Ungültige oder doppelte Kanal-ID %q", c.ID)
		case !channelTypes[c.Type]:
			return badRequest("Unbekannter Kanaltyp %q", c.Type)
		case c.Name == "" || len(c.Name) > 60:
			return badRequest("Kanalname: 1–60 Zeichen")
		case c.Type == "email" && c.Target == "":
			return badRequest("E-Mail-Kanal „%s“: Empfänger fehlt", c.Name)
		case c.Type == "telegram" && (c.Token == "" || c.Target == ""):
			return badRequest("Telegram-Kanal „%s“: Bot-Token und Chat-ID angeben", c.Name)
		case c.Type != "email" && c.Type != "telegram":
			u, err := url.Parse(c.URL)
			if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
				return badRequest("Kanal „%s“: gültige http(s)-URL angeben", c.Name)
			}
		}
		ids[c.ID] = true
	}
	rids := map[string]bool{}
	for i := range in.Rules {
		r := &in.Rules[i]
		if r.ID == "" {
			r.ID = randomHex(4)
		}
		r.Name = strings.TrimSpace(r.Name)
		info, ok := alertEventInfo(r.Event)
		switch {
		case !alertIDRe.MatchString(r.ID) || rids[r.ID]:
			return badRequest("Ungültige oder doppelte Regel-ID %q", r.ID)
		case !ok:
			return badRequest("Unbekanntes Ereignis %q", r.Event)
		case len(r.Name) > 80:
			return badRequest("Regelname: max. 80 Zeichen")
		case r.Minutes < 0 || r.Minutes > 1440 || r.Cooldown < 0 || r.Cooldown > 10080:
			return badRequest("Dauer 0–1440 Minuten, Pause 0–10080 Minuten")
		case r.Threshold < 0 || r.Threshold > 100000:
			return badRequest("Ungültiger Schwellwert")
		}
		if r.Name == "" {
			r.Name = info.Label
		}
		if info.Threshold > 0 && r.Threshold == 0 {
			r.Threshold = info.Threshold
		}
		for _, s := range r.Servers {
			if !nameRe.MatchString(s) {
				return badRequest("Ungültiger Servername %q", s)
			}
		}
		for _, c := range r.Channels {
			if !ids[c] {
				return badRequest("Regel „%s“ verweist auf einen unbekannten Kanal", r.Name)
			}
		}
		rids[r.ID] = true
	}
	in.SMTP.Host = strings.TrimSpace(in.SMTP.Host)
	in.SMTP.From = strings.TrimSpace(in.SMTP.From)
	if in.SMTP.Password == secretMask {
		in.SMTP.Password = old.SMTP.Password
	}
	if in.SMTP.Security == "" {
		in.SMTP.Security = "starttls"
	}
	if !set("starttls", "tls", "none")[in.SMTP.Security] || in.SMTP.Port < 0 || in.SMTP.Port > 65535 || strings.ContainsAny(in.SMTP.From, "\r\n") {
		return badRequest("Ungültige SMTP-Einstellungen")
	}
	al.mu.Lock()
	defer al.mu.Unlock()
	in.Maintenance = al.cfg.Maintenance
	al.cfg = in
	// Drop state of removed rules.
	for k := range al.state {
		if !rids[strings.SplitN(k, "|", 2)[0]] {
			delete(al.state, k)
		}
	}
	return al.saveLocked()
}

func (al *Alerter) saveLocked() error {
	if al.path == "" {
		return nil
	}
	return writeFileAtomic(al.path, mustJSONIndent(al.cfg), 0o600)
}

func (al *Alerter) SetMaintenance(key string, minutes int) {
	al.mu.Lock()
	defer al.mu.Unlock()
	if minutes <= 0 {
		delete(al.cfg.Maintenance, key)
	} else {
		al.cfg.Maintenance[key] = time.Now().Add(time.Duration(minutes) * time.Minute).Unix()
	}
	_ = al.saveLocked()
}

func (al *Alerter) MaintenanceUntil(server string) int64 {
	al.mu.Lock()
	defer al.mu.Unlock()
	return al.maintenanceLocked(server)
}

func (al *Alerter) maintenanceLocked(server string) int64 {
	now := time.Now().Unix()
	until := int64(0)
	for _, k := range []string{"*", server} {
		if v := al.cfg.Maintenance[k]; v > now && v > until {
			until = v
		}
	}
	return until
}

func (al *Alerter) History(limit int) []AlertEvent {
	al.mu.Lock()
	defer al.mu.Unlock()
	out := []AlertEvent{}
	for i := len(al.history) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, al.history[i])
	}
	return out
}

type FiringAlert struct {
	Rule   string `json:"rule"`
	Name   string `json:"name"`
	Event  string `json:"event"`
	Server string `json:"server,omitempty"`
	Since  int64  `json:"since"`
	Detail string `json:"detail"`
}

func (al *Alerter) Firing() []FiringAlert {
	al.mu.Lock()
	defer al.mu.Unlock()
	names := map[string]AlertRule{}
	for _, r := range al.cfg.Rules {
		names[r.ID] = r
	}
	out := []FiringAlert{}
	for k, s := range al.state {
		if !s.firing {
			continue
		}
		parts := strings.SplitN(k, "|", 2)
		r := names[parts[0]]
		out = append(out, FiringAlert{Rule: r.ID, Name: r.Name, Event: r.Event, Server: parts[1], Since: s.since.Unix(), Detail: s.detail})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Since < out[j].Since })
	return out
}

// ---- evaluation ---------------------------------------------------------------------

func (al *Alerter) Run(ctx context.Context) {
	select {
	case <-ctx.Done():
		return
	case <-time.After(30 * time.Second):
	}
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		ectx, cancel := context.WithTimeout(ctx, 50*time.Second)
		al.evaluate(ectx)
		cancel()
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (r *AlertRule) matches(server string) bool {
	if len(r.Servers) == 0 {
		return true
	}
	for _, s := range r.Servers {
		if s == server {
			return true
		}
	}
	return false
}

type serverObs struct {
	name    string
	cfg     ServerConfig
	c       ContainerSummary
	ins     *ContainerInspect
	live    *Live
	stats   *ContainerStats
	statErr bool
}

func parseDockerTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, s)
	return t
}

func (al *Alerter) evaluate(ctx context.Context) {
	al.mu.Lock()
	var rules []AlertRule
	for _, r := range al.cfg.Rules {
		if info, ok := alertEventInfo(r.Event); ok && r.Enabled && info.State {
			rules = append(rules, r)
		}
	}
	al.mu.Unlock()
	if len(rules) == 0 {
		return
	}
	cs, err := al.m.d.ListContainers(ctx, labelManaged+"=true")
	if err != nil {
		return
	}
	now := time.Now()
	for i := range cs {
		o := &serverObs{c: cs[i], cfg: configFromLabels(cs[i].Labels)}
		o.name = o.cfg.Name
		var wanted []AlertRule
		for _, r := range rules {
			if r.Event != "proxy_down" && r.matches(o.name) {
				wanted = append(wanted, r)
			}
		}
		if len(wanted) == 0 {
			continue
		}
		if ins, err := al.m.d.Inspect(ctx, o.c.ID); err == nil {
			o.ins = &ins
			al.trackRestarts(o.name, ins.RestartCount, now)
		} else {
			continue
		}
		for _, r := range wanted {
			cond, detail := al.check(ctx, r, o, now)
			al.step(r, o.name, cond, detail)
		}
	}
	// Velocity networks
	for _, r := range rules {
		if r.Event != "proxy_down" {
			continue
		}
		pcs, err := al.m.listProxies(ctx)
		if err != nil {
			continue
		}
		running := map[string]bool{}
		for _, c := range pcs {
			running[proxyIDOf(c)] = c.State == "running"
		}
		for _, pc := range al.m.proxyConfigs() {
			key := "netz:" + pc.ID
			intent := al.m.intentSince(proxyTaskKey(pc.ID))
			down := !running[pc.ID] && now.Sub(intent) > 10*time.Minute
			al.step(r, key, down, fmt.Sprintf("Netzwerk „%s“ (Port %d) läuft nicht", pc.displayName(), pc.Port))
		}
	}
}

func (al *Alerter) trackRestarts(name string, n int, now time.Time) {
	al.mu.Lock()
	defer al.mu.Unlock()
	list := al.restarts[name]
	var keep []restartSample
	for _, s := range list {
		if now.Sub(s.t) <= 16*time.Minute {
			keep = append(keep, s)
		}
	}
	al.restarts[name] = append(keep, restartSample{now, n})
}

// check evaluates one state rule for one server.
func (al *Alerter) check(ctx context.Context, r AlertRule, o *serverObs, now time.Time) (bool, string) {
	ins := o.ins
	health := ""
	if ins.State.Health != nil {
		health = ins.State.Health.Status
	}
	running := ins.State.Running
	live := func() *Live {
		if o.live == nil {
			lv := al.m.Live(ctx, &o.c, 90*time.Second)
			o.live = &lv
		}
		return o.live
	}
	switch r.Event {
	case "down":
		switch {
		case ins.State.Status == "restarting":
			return true, "Der Server hängt in einer Neustart-Schleife"
		case !running:
			fin := parseDockerTime(ins.State.FinishedAt)
			if ins.State.ExitCode != 0 && fin.After(al.start) && al.m.intentSince(o.name).Before(parseDockerTime(ins.State.StartedAt)) {
				return true, fmt.Sprintf("Der Server ist abgestürzt (Exit-Code %d) und läuft nicht mehr", ins.State.ExitCode)
			}
			return false, ""
		}
		lv := live()
		if lv.Sleeping || lv.Reachable {
			return false, ""
		}
		started := parseDockerTime(ins.State.StartedAt)
		if health == "starting" && now.Sub(started) < 20*time.Minute {
			return false, "" // still booting (large modpacks take a while)
		}
		return true, "Der Server läuft, antwortet aber nicht auf Server-List-Pings"
	case "crashloop":
		al.mu.Lock()
		list := al.restarts[o.name]
		al.mu.Unlock()
		if len(list) < 2 {
			return false, ""
		}
		first := list[0]
		for _, s := range list {
			if now.Sub(s.t) <= 15*time.Minute {
				first = s
				break
			}
		}
		d := list[len(list)-1].n - first.n
		return float64(d) >= r.Threshold, fmt.Sprintf("%d Neustarts in den letzten 15 Minuten", d)
	case "unhealthy":
		return running && health == "unhealthy", "Docker-Healthcheck meldet „unhealthy“"
	case "tps":
		if !running {
			return false, ""
		}
		lv := live()
		if lv.TPS == nil {
			return false, ""
		}
		return *lv.TPS < r.Threshold, fmt.Sprintf("TPS %.1f (Schwelle %.0f)", *lv.TPS, r.Threshold)
	case "memory":
		if !running {
			return false, ""
		}
		st, err := al.m.d.Stats(ctx, o.c.ID)
		if err != nil || st.MemLimit == 0 {
			return false, ""
		}
		pct := float64(st.MemUsage) / float64(st.MemLimit) * 100
		return pct >= r.Threshold, fmt.Sprintf("RAM %.0f %% (%s von %s)", pct, fmtBytes(st.MemUsage), fmtBytes(st.MemLimit))
	case "disk":
		if !running {
			return false, ""
		}
		ds := al.diskFree(ctx, o)
		if ds.err {
			return false, ""
		}
		return ds.freePct < r.Threshold, fmt.Sprintf("Nur noch %.1f %% frei (%s)", ds.freePct, ds.freeText)
	}
	return false, ""
}

func fmtBytes(b uint64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(b)/(1<<30))
	default:
		return fmt.Sprintf("%d MB", b>>20)
	}
}

// diskFree runs df inside the container at most every 5 minutes.
func (al *Alerter) diskFree(ctx context.Context, o *serverObs) diskSample {
	al.mu.Lock()
	s, ok := al.disk[o.name]
	al.mu.Unlock()
	if ok && time.Since(s.t) < 5*time.Minute {
		return s
	}
	s = diskSample{t: time.Now(), err: true}
	out, code, err := al.m.d.Exec(ctx, o.c.ID, []string{"df", "-Pk", "/data"})
	if err == nil && code == 0 {
		lines := strings.Split(strings.TrimSpace(out), "\n")
		f := strings.Fields(lines[len(lines)-1])
		if len(f) >= 4 {
			total, e1 := strconv.ParseFloat(f[1], 64)
			avail, e2 := strconv.ParseFloat(f[3], 64)
			if e1 == nil && e2 == nil && total > 0 {
				s.freePct, s.err = avail/total*100, false
				s.freeText = fmtBytes(uint64(avail*1024)) + " von " + fmtBytes(uint64(total*1024))
			}
		}
	}
	al.mu.Lock()
	al.disk[o.name] = s
	al.mu.Unlock()
	return s
}

// step advances the state machine of a rule for one subject.
func (al *Alerter) step(r AlertRule, subject string, cond bool, detail string) {
	info, _ := alertEventInfo(r.Event)
	key := r.ID + "|" + subject
	now := time.Now()
	al.mu.Lock()
	st := al.state[key]
	if st == nil {
		st = &alertState{}
		al.state[key] = st
	}
	var send *AlertEvent
	if cond {
		if st.since.IsZero() {
			st.since = now
		}
		st.detail = detail
		hold := time.Duration(r.Minutes) * time.Minute
		cool := time.Duration(r.Cooldown) * time.Minute
		if !st.firing && now.Sub(st.since) >= hold && now.Sub(st.lastSent) >= cool {
			st.firing, st.lastSent = true, now
			send = &AlertEvent{Level: info.Level, Title: alertTitle(info, subject, false), Text: detail}
		}
	} else {
		if st.firing {
			if r.Resolve {
				send = &AlertEvent{Level: "ok", Resolved: true, Title: alertTitle(info, subject, true),
					Text: fmt.Sprintf("Problem bestand seit %s (%s).", st.since.Format("02.01. 15:04"), humanDur(now.Sub(st.since)))}
			}
		}
		st.since, st.firing, st.detail = time.Time{}, false, ""
	}
	al.mu.Unlock()
	if send != nil {
		send.Rule, send.RuleName, send.Event, send.Server = r.ID, r.Name, r.Event, strings.TrimPrefix(subject, "netz:")
		if strings.HasPrefix(subject, "netz:") {
			send.Server = ""
		}
		al.dispatch(r, subject, *send)
	}
}

func humanDur(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "unter einer Minute"
	case d < time.Hour:
		return fmt.Sprintf("%d Min.", int(d.Minutes()))
	}
	return fmt.Sprintf("%d Std. %d Min.", int(d.Hours()), int(d.Minutes())%60)
}

func alertTitle(info AlertEventInfo, subject string, resolved bool) string {
	what := "Server „" + subject + "“"
	if strings.HasPrefix(subject, "netz:") {
		what = "Netzwerk „" + strings.TrimPrefix(subject, "netz:") + "“"
	}
	if resolved {
		return "Behoben: " + info.Label + " – " + what
	}
	return info.Label + " – " + what
}

// Emit reports a one-off event (task failed, login attack, admin login).
func (al *Alerter) Emit(event, server, title, text string) {
	info, ok := alertEventInfo(event)
	if !ok {
		return
	}
	al.mu.Lock()
	var rules []AlertRule
	for _, r := range al.cfg.Rules {
		if r.Enabled && r.Event == event && (server == "" || r.matches(server)) {
			rules = append(rules, r)
		}
	}
	al.mu.Unlock()
	for _, r := range rules {
		key := r.ID + "|" + server
		al.mu.Lock()
		st := al.state[key]
		if st == nil {
			st = &alertState{}
			al.state[key] = st
		}
		cool := time.Duration(r.Cooldown) * time.Minute
		if !st.lastSent.IsZero() && time.Since(st.lastSent) < cool {
			al.mu.Unlock()
			continue
		}
		st.lastSent = time.Now()
		al.mu.Unlock()
		al.dispatch(r, server, AlertEvent{Rule: r.ID, RuleName: r.Name, Event: event, Server: server, Level: info.Level, Title: title, Text: text})
	}
}

// AuthEvent hooks the authentication into the alerting.
func (al *Alerter) AuthEvent(e AuthEvent) {
	switch e.Kind {
	case "login_failed":
		al.mu.Lock()
		now := time.Now()
		var keep []time.Time
		for _, t := range al.fails {
			if now.Sub(t) < 10*time.Minute {
				keep = append(keep, t)
			}
		}
		al.fails = append(keep, now)
		n := len(al.fails)
		threshold := 0.0
		for _, r := range al.cfg.Rules {
			if r.Enabled && r.Event == "login_failed" {
				threshold = r.Threshold
			}
		}
		al.mu.Unlock()
		if threshold > 0 && float64(n) >= threshold {
			al.Emit("login_failed", "", "Viele fehlgeschlagene Anmeldungen", fmt.Sprintf("%d Fehlversuche in 10 Minuten, zuletzt für „%s“ von %s.", n, e.User, e.IP))
		}
	case "locked":
		al.Emit("login_failed", "", "Anmeldung gesperrt", fmt.Sprintf("Nach wiederholten Fehlversuchen für „%s“ wurde %s vorübergehend gesperrt.", e.User, e.IP))
	case "login":
		if e.Privileged && e.NewIP {
			al.Emit("admin_login", "", "Admin-Anmeldung von neuer Adresse", fmt.Sprintf("„%s“ hat sich von %s angemeldet – diese Adresse war bisher unbekannt.", e.User, e.IP))
		}
	}
}

// ---- delivery -----------------------------------------------------------------------

func (al *Alerter) dispatch(r AlertRule, subject string, ev AlertEvent) {
	ev.ID, ev.Time = randomHex(6), time.Now().Unix()
	al.mu.Lock()
	muted := al.maintenanceLocked(ev.Server) > 0
	var chans []AlertChannel
	for _, id := range r.Channels {
		for _, c := range al.cfg.Channels {
			if c.ID == id && c.Enabled {
				chans = append(chans, c)
			}
		}
	}
	smtpCfg := al.cfg.SMTP
	al.mu.Unlock()
	ev.Muted = muted
	if muted {
		ev.Results = []string{"Wartungsmodus – nicht versendet"}
		al.record(ev)
		return
	}
	log.Printf("Alarm: %s – %s", ev.Title, ev.Text)
	if len(chans) == 0 {
		ev.Results = []string{"Kein aktiver Kanal"}
		al.record(ev)
		return
	}
	go func() {
		var wg sync.WaitGroup
		res := make([]string, len(chans))
		for i, c := range chans {
			wg.Add(1)
			go func(i int, c AlertChannel) {
				defer wg.Done()
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				if err := al.send(ctx, c, smtpCfg, ev); err != nil {
					res[i] = c.Name + ": " + err.Error()
					log.Printf("Alarm-Kanal %s: %v", c.Name, err)
				} else {
					res[i] = c.Name + ": zugestellt"
				}
			}(i, c)
		}
		wg.Wait()
		ev.Results = res
		al.record(ev)
	}()
}

func (al *Alerter) record(ev AlertEvent) {
	al.mu.Lock()
	defer al.mu.Unlock()
	al.history = append(al.history, ev)
	if len(al.history) > 300 {
		al.history = al.history[len(al.history)-300:]
	}
	if al.path != "" {
		_ = writeFileAtomic(filepath.Join(filepath.Dir(al.path), "alert-history.json"), mustJSONIndent(al.history), 0o600)
	}
}

func (al *Alerter) panelLink(ev AlertEvent) string {
	base := al.m.store.Hosting().PanelURL
	if base == "" {
		return ""
	}
	if ev.Server != "" {
		return base + "/admin/#/server/" + ev.Server
	}
	return base + "/admin/#/alerts"
}

var levelColor = map[string]int{"critical": 0xE5484D, "warning": 0xF5A524, "info": 0x3B82F6, "ok": 0x30A46C}
var levelEmoji = map[string]string{"critical": "🔴", "warning": "🟠", "info": "🔵", "ok": "✅"}

func (al *Alerter) send(ctx context.Context, c AlertChannel, sm SMTPConfig, ev AlertEvent) error {
	link := al.panelLink(ev)
	plain := ev.Text
	if link != "" {
		plain += "\n" + link
	}
	switch c.Type {
	case "discord":
		embed := map[string]any{"title": levelEmoji[ev.Level] + " " + ev.Title, "description": ev.Text, "color": levelColor[ev.Level],
			"timestamp": time.Unix(ev.Time, 0).UTC().Format(time.RFC3339), "footer": map[string]string{"text": "CraftDock"}}
		if link != "" {
			embed["url"] = link
		}
		return al.postJSON(ctx, c.URL, map[string]any{"username": "CraftDock", "embeds": []any{embed}}, nil)
	case "slack":
		return al.postJSON(ctx, c.URL, map[string]string{"text": levelEmoji[ev.Level] + " *" + ev.Title + "*\n" + plain}, nil)
	case "webhook":
		h := map[string]string{}
		if c.Token != "" {
			h["Authorization"] = "Bearer " + c.Token
		}
		return al.postJSON(ctx, c.URL, map[string]any{"event": ev.Event, "level": ev.Level, "title": ev.Title, "text": ev.Text,
			"server": ev.Server, "rule": ev.RuleName, "resolved": ev.Resolved, "time": ev.Time, "link": link}, h)
	case "ntfy":
		prio := map[string]string{"critical": "5", "warning": "4", "info": "3", "ok": "3"}[ev.Level]
		tags := map[string]string{"critical": "rotating_light", "warning": "warning", "info": "information_source", "ok": "white_check_mark"}[ev.Level]
		h := map[string]string{"Title": headerSafe(ev.Title), "Priority": prio, "Tags": tags}
		if link != "" {
			h["Click"] = link
		}
		if c.Token != "" {
			h["Authorization"] = "Bearer " + c.Token
		}
		return al.post(ctx, c.URL, "text/plain; charset=utf-8", []byte(ev.Text), h)
	case "gotify":
		prio := map[string]int{"critical": 8, "warning": 5, "info": 3, "ok": 3}[ev.Level]
		return al.postJSON(ctx, strings.TrimRight(c.URL, "/")+"/message", map[string]any{"title": ev.Title, "message": plain, "priority": prio},
			map[string]string{"X-Gotify-Key": c.Token})
	case "telegram":
		base := "https://api.telegram.org"
		if c.URL != "" {
			base = strings.TrimRight(c.URL, "/")
		}
		return al.postJSON(ctx, base+"/bot"+c.Token+"/sendMessage", map[string]any{"chat_id": c.Target,
			"text": levelEmoji[ev.Level] + " " + ev.Title + "\n\n" + plain, "disable_web_page_preview": true}, nil)
	case "email":
		return sendMail(sm, c.Target, "[CraftDock] "+ev.Title, ev.Title+"\n\n"+plain+"\n\n– CraftDock, "+time.Unix(ev.Time, 0).Format("02.01.2006 15:04:05"))
	}
	return fmt.Errorf("unbekannter Kanaltyp")
}

func headerSafe(s string) string {
	s = strings.NewReplacer("\r", " ", "\n", " ").Replace(s)
	return mime.QEncoding.Encode("utf-8", s)
}

func (al *Alerter) postJSON(ctx context.Context, u string, v any, h map[string]string) error {
	b, _ := json.Marshal(v)
	return al.post(ctx, u, "application/json", b, h)
}

func (al *Alerter) post(ctx context.Context, u, ctype string, body []byte, h map[string]string) error {
	req, err := http.NewRequestWithContext(ctx, "POST", u, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("User-Agent", "CraftDock/"+appVersion)
	for k, v := range h {
		req.Header.Set(k, v)
	}
	resp, err := al.client.Do(req)
	if err != nil {
		return redactErr(err, u)
	}
	defer resp.Body.Close()
	msg, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
	if resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	return nil
}

// redactErr keeps tokens in URLs (Telegram, Discord) out of error messages.
func redactErr(err error, u string) error {
	msg := err.Error()
	if p, e := url.Parse(u); e == nil {
		msg = strings.ReplaceAll(msg, u, p.Scheme+"://"+p.Host+"/…")
	}
	return fmt.Errorf("%s", msg)
}

func sendMail(sm SMTPConfig, to, subject, body string) error {
	if sm.Host == "" || sm.From == "" {
		return fmt.Errorf("SMTP-Server ist nicht eingerichtet")
	}
	port := sm.Port
	if port == 0 {
		port = map[string]int{"tls": 465, "starttls": 587, "none": 25}[sm.Security]
	}
	addr := net.JoinHostPort(sm.Host, strconv.Itoa(port))
	var rcpts []string
	for _, r := range strings.Split(to, ",") {
		if r = strings.TrimSpace(r); r != "" {
			if !emailRe.MatchString(r) {
				return fmt.Errorf("ungültiger Empfänger %q", r)
			}
			rcpts = append(rcpts, r)
		}
	}
	tlsCfg := &tls.Config{ServerName: sm.Host, MinVersion: tls.VersionTLS12}
	var conn net.Conn
	var err error
	d := &net.Dialer{Timeout: 10 * time.Second}
	if sm.Security == "tls" {
		conn, err = tls.DialWithDialer(d, "tcp", addr, tlsCfg)
	} else {
		conn, err = d.Dial("tcp", addr)
	}
	if err != nil {
		return err
	}
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	c, err := smtp.NewClient(conn, sm.Host)
	if err != nil {
		conn.Close()
		return err
	}
	defer c.Close()
	if sm.Security == "starttls" {
		if err := c.StartTLS(tlsCfg); err != nil {
			return fmt.Errorf("STARTTLS: %w", err)
		}
	}
	if sm.Username != "" {
		if err := c.Auth(smtp.PlainAuth("", sm.Username, sm.Password, sm.Host)); err != nil {
			return fmt.Errorf("Anmeldung am SMTP-Server: %w", err)
		}
	}
	if err := c.Mail(sm.From); err != nil {
		return err
	}
	for _, r := range rcpts {
		if err := c.Rcpt(r); err != nil {
			return err
		}
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	msg := "From: " + sm.From + "\r\nTo: " + strings.Join(rcpts, ", ") + "\r\nSubject: " + headerSafe(subject) +
		"\r\nDate: " + time.Now().Format(time.RFC1123Z) + "\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n" +
		strings.ReplaceAll(body, "\n", "\r\n") + "\r\n"
	if _, err := w.Write([]byte(msg)); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

// ---- HTTP -----------------------------------------------------------------------------

func (a *API) getAlerts(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"config": a.alerts.Config(true), "events": alertEvents, "history": a.alerts.History(150), "firing": a.alerts.Firing()})
}

func (a *API) putAlerts(w http.ResponseWriter, r *http.Request) {
	var in AlertConfig
	if err := readJSON(r, &in); err != nil {
		writeErr(w, err)
		return
	}
	if err := a.alerts.SetConfig(in); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, a.alerts.Config(true))
}

func (a *API) testAlert(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Channel string `json:"channel"`
	}
	if err := readJSON(r, &in); err != nil {
		writeErr(w, err)
		return
	}
	cfg := a.alerts.Config(false)
	for _, c := range cfg.Channels {
		if c.ID == in.Channel {
			ctx, cancel := ctxT(r, 20*time.Second)
			defer cancel()
			ev := AlertEvent{Time: time.Now().Unix(), Level: "info", Title: "Testnachricht von CraftDock",
				Text: "Wenn du das liest, funktioniert der Kanal „" + c.Name + "“. Ausgelöst von " + userFrom(r).Username + "."}
			if err := a.alerts.send(ctx, c, cfg.SMTP, ev); err != nil {
				writeErr(w, &APIError{http.StatusBadGateway, "Zustellung fehlgeschlagen: " + err.Error()})
				return
			}
			ok(w)
			return
		}
	}
	writeErr(w, &APIError{http.StatusNotFound, "Kanal nicht gefunden – erst speichern, dann testen"})
}

func (a *API) putGlobalMaintenance(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Minutes int `json:"minutes"`
	}
	if err := readJSON(r, &in); err != nil {
		writeErr(w, err)
		return
	}
	if in.Minutes < 0 || in.Minutes > 7*24*60 {
		writeErr(w, badRequest("Wartung: 0 bis 10080 Minuten"))
		return
	}
	a.alerts.SetMaintenance("*", in.Minutes)
	writeJSON(w, 200, map[string]int64{"until": a.alerts.MaintenanceUntil("")})
}

func (a *API) setMaintenance(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Minutes int `json:"minutes"`
	}
	if err := readJSON(r, &in); err != nil {
		writeErr(w, err)
		return
	}
	if in.Minutes < 0 || in.Minutes > 7*24*60 {
		writeErr(w, badRequest("Wartung: 0 bis 10080 Minuten"))
		return
	}
	a.alerts.SetMaintenance(r.PathValue("name"), in.Minutes)
	writeJSON(w, 200, map[string]int64{"until": a.alerts.MaintenanceUntil(r.PathValue("name"))})
}
