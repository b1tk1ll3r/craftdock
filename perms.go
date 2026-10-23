package main

// Permission catalogue and roles. Permissions come in two scopes:
//
//   - server permissions apply to a single server and are granted either for
//     all servers, for servers the user owns, or per server via an explicit
//     assignment (user.Access);
//   - global permissions apply to the panel as a whole.
//
// Roles are named permission sets. Five roles are built in; administrators
// can edit all but "admin" and define their own.

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// Server-scoped permissions
const (
	PermView      = "server.view"      // status, console output, statistics, players
	PermControl   = "server.control"   // start, stop, restart
	PermConsole   = "server.console"   // send console commands (subject to role allow/deny lists)
	PermPlayers   = "players.manage"   // kick, ban, op, whitelist, messages
	PermCheats    = "players.cheat"    // give items, teleport, game mode, effects, heal
	PermConfigure = "server.configure" // settings, image update
	PermAdvanced  = "server.advanced"  // ports, env vars, data path, network, limits beyond hosting rules
	PermMods      = "server.mods"      // mods & plugins
	PermFiles     = "server.files"     // edit configuration files
	PermWorld     = "server.world"     // reset / import world
	PermBackup    = "server.backup"    // download backups
	PermDelete    = "server.delete"    // delete server
)

// Global permissions
const (
	PermCreate   = "servers.create" // create and clone servers (within quota)
	PermNetworks = "networks"       // Velocity networks
	PermNews     = "news"           // news & events
	PermSettings = "settings"       // public site & hosting settings
	PermAlerts   = "alerts"         // alerting
	PermUsers    = "users"          // user management, invitations
	PermRoles    = "roles"          // role editor
	PermAudit    = "audit"          // audit log
)

type PermInfo struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Group  string `json:"group"`
	Server bool   `json:"server"`
}

var permCatalog = []PermInfo{
	{PermView, "Server sehen (Status, Konsole lesen, Statistiken, Spieler)", "Server", true},
	{PermControl, "Starten, Stoppen, Neustarten", "Server", true},
	{PermConsole, "Konsolenbefehle senden", "Server", true},
	{PermConfigure, "Einstellungen ändern, Image aktualisieren", "Server", true},
	{PermAdvanced, "Erweiterte Einstellungen (Ports, Variablen, Datenpfad, Netzwerk, Limits)", "Server", true},
	{PermMods, "Mods & Plugins verwalten", "Server", true},
	{PermFiles, "Konfigurationsdateien bearbeiten", "Server", true},
	{PermWorld, "Welt zurücksetzen und importieren", "Server", true},
	{PermBackup, "Backups herunterladen", "Server", true},
	{PermDelete, "Server löschen", "Server", true},
	{PermPlayers, "Spieler verwalten (Kick, Bann, OP, Whitelist, Nachrichten)", "Spieler", true},
	{PermCheats, "Spieler-Eingriffe (Items, Teleport, Spielmodus, Effekte)", "Spieler", true},
	{PermCreate, "Server anlegen und klonen (im Rahmen der Quote)", "Panel", false},
	{PermNetworks, "Velocity-Netzwerke verwalten", "Panel", false},
	{PermNews, "Neuigkeiten & Events", "Panel", false},
	{PermSettings, "Öffentliche Seite & Hosting-Einstellungen", "Panel", false},
	{PermAlerts, "Alarmierung verwalten", "Panel", false},
	{PermUsers, "Benutzer & Einladungen verwalten", "Verwaltung", false},
	{PermRoles, "Rollen bearbeiten", "Verwaltung", false},
	{PermAudit, "Audit-Log einsehen", "Verwaltung", false},
}

var (
	permKnown  = map[string]PermInfo{}
	serverPerm = map[string]bool{}
)

func init() {
	for _, p := range permCatalog {
		permKnown[p.ID] = p
		serverPerm[p.ID] = p.Server
	}
}

func allPermIDs() []string {
	out := make([]string, len(permCatalog))
	for i, p := range permCatalog {
		out[i] = p.ID
	}
	return out
}

// legacyPerms maps permission names of CraftDock ≤ 1.3 (custom data only).
var legacyPerms = map[string][]string{
	"view": {PermView}, "control": {PermControl}, "console": {PermConsole},
	"players":   {PermPlayers, PermCheats},
	"configure": {PermConfigure, PermAdvanced, PermMods, PermFiles, PermWorld},
	"backup":    {PermBackup}, "create": {PermCreate}, "delete": {PermDelete},
	"news": {PermNews}, "settings": {PermSettings, PermNetworks}, "users": {PermUsers, PermAudit},
}

type Role struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Perms        []string `json:"perms"`
	Require2FA   bool     `json:"require2fa"`
	ConsoleAllow []string `json:"consoleAllow"` // if set: only these commands
	ConsoleDeny  []string `json:"consoleDeny"`  // never these commands
	Builtin      bool     `json:"builtin"`
	Locked       bool     `json:"locked"` // admin: not editable
}

func (r *Role) Has(perm string) bool {
	for _, p := range r.Perms {
		if p == perm {
			return true
		}
	}
	return false
}

// AllowsCommand applies the console allow/deny lists to the first word of cmd.
func (r *Role) AllowsCommand(cmd string) bool {
	word := strings.ToLower(strings.TrimPrefix(strings.Fields(cmd + " ")[0], "/"))
	if i := strings.LastIndexByte(word, ':'); i >= 0 { // minecraft:op → op
		word = word[i+1:]
	}
	for _, d := range r.ConsoleDeny {
		if d == word {
			return false
		}
	}
	if len(r.ConsoleAllow) == 0 {
		return true
	}
	for _, a := range r.ConsoleAllow {
		if a == word {
			return true
		}
	}
	return false
}

func serverPermsAll() []string {
	var out []string
	for _, p := range permCatalog {
		if p.Server {
			out = append(out, p.ID)
		}
	}
	return out
}

func builtinRoles() []*Role {
	allServer := serverPermsAll()
	without := func(list []string, drop ...string) []string {
		var out []string
	next:
		for _, p := range list {
			for _, d := range drop {
				if p == d {
					continue next
				}
			}
			out = append(out, p)
		}
		return out
	}
	return []*Role{
		{ID: "admin", Name: "Administrator", Description: "Darf alles – auch Benutzer, Rollen und Einstellungen.", Perms: allPermIDs(), Builtin: true, Locked: true},
		{ID: "operator", Name: "Operator", Description: "Betreibt alle Server, legt neue an, schreibt Neuigkeiten.",
			Perms: append(without(allServer, PermDelete), PermCreate, PermNews), Builtin: true},
		{ID: "moderator", Name: "Moderator", Description: "Kümmert sich um Spieler und den laufenden Betrieb.",
			Perms: []string{PermView, PermControl, PermConsole, PermPlayers, PermCheats, PermNews}, ConsoleDeny: []string{"op", "deop", "stop", "reload"}, Builtin: true},
		{ID: "customer", Name: "Kunde", Description: "Verwaltet die eigenen Server im Rahmen seiner Quote – ohne Zugriff auf Host-Einstellungen.",
			Perms: append(without(allServer, PermAdvanced), PermCreate), Builtin: true},
		{ID: "viewer", Name: "Betrachter", Description: "Sieht Status und Konsole, ändert nichts.", Perms: []string{PermView}, Builtin: true},
	}
}

var roleIDRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{1,31}$`)
var consoleWordRe = regexp.MustCompile(`^[a-z0-9_:.-]{1,40}$`)

// Roles is the persistent role registry.
type Roles struct {
	path  string
	mu    sync.RWMutex
	roles map[string]*Role
	order []string
}

func NewRoles(dataDir string) *Roles {
	rs := &Roles{roles: map[string]*Role{}}
	for _, r := range builtinRoles() {
		rs.roles[r.ID] = r
	}
	if dataDir == "" {
		return rs
	}
	rs.path = filepath.Join(dataDir, "roles.json")
	b, err := os.ReadFile(rs.path)
	if err != nil {
		return rs
	}
	var list []*Role
	if err := json.Unmarshal(b, &list); err != nil {
		log.Printf("WARNUNG: roles.json ist beschädigt: %v", err)
		return rs
	}
	for _, r := range list {
		if r.ID == "admin" {
			continue // always the built-in definition
		}
		r.Perms = cleanPerms(r.Perms)
		if b := rs.roles[r.ID]; b != nil {
			r.Builtin = true
		}
		rs.roles[r.ID] = r
	}
	return rs
}

func cleanPerms(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, p := range in {
		list := []string{p}
		if l, ok := legacyPerms[p]; ok {
			list = l
		}
		for _, q := range list {
			if _, ok := permKnown[q]; ok && !seen[q] {
				seen[q] = true
				out = append(out, q)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return permIndex(out[i]) < permIndex(out[j]) })
	return out
}

func permIndex(id string) int {
	for i, p := range permCatalog {
		if p.ID == id {
			return i
		}
	}
	return len(permCatalog)
}

func (rs *Roles) Get(id string) *Role {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	if r := rs.roles[id]; r != nil {
		cp := *r
		return &cp
	}
	return nil
}

func (rs *Roles) List() []Role {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	order := map[string]int{"admin": 0, "operator": 1, "moderator": 2, "customer": 3, "viewer": 4}
	out := make([]Role, 0, len(rs.roles))
	for _, r := range rs.roles {
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool {
		oi, iok := order[out[i].ID]
		oj, jok := order[out[j].ID]
		switch {
		case iok && jok:
			return oi < oj
		case iok != jok:
			return iok
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func (rs *Roles) saveLocked() error {
	if rs.path == "" {
		return nil
	}
	defaults := map[string]string{}
	for _, b := range builtinRoles() {
		defaults[b.ID] = mustJSON(b)
	}
	list := make([]*Role, 0, len(rs.roles))
	for _, r := range rs.roles {
		// Unchanged built-in roles are not stored, so they pick up new
		// permissions of future versions automatically.
		if r.ID != "admin" && defaults[r.ID] != mustJSON(r) {
			list = append(list, r)
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	return writeFileAtomic(rs.path, mustJSONIndent(list), 0o600)
}

func normalizeWords(in []string) ([]string, error) {
	var out []string
	seen := map[string]bool{}
	for _, w := range in {
		w = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(w), "/"))
		if w == "" || seen[w] {
			continue
		}
		if !consoleWordRe.MatchString(w) {
			return nil, badRequest("Ungültiger Konsolenbefehl %q", w)
		}
		seen[w] = true
		out = append(out, w)
	}
	return out, nil
}

// Save creates or updates a role. grantable limits which permissions the
// editor may hand out (nobody can create a role more powerful than himself).
func (rs *Roles) Save(in Role, create bool, grantable func(string) bool) (*Role, error) {
	in.ID = strings.ToLower(strings.TrimSpace(in.ID))
	in.Name = strings.TrimSpace(in.Name)
	in.Description = strings.TrimSpace(in.Description)
	if !roleIDRe.MatchString(in.ID) {
		return nil, badRequest("Rollen-ID: 2–32 Zeichen, a–z, 0–9, _ -")
	}
	if in.Name == "" || len(in.Name) > 40 || len(in.Description) > 300 {
		return nil, badRequest("Name: 1–40 Zeichen, Beschreibung max. 300 Zeichen")
	}
	var err error
	if in.ConsoleAllow, err = normalizeWords(in.ConsoleAllow); err != nil {
		return nil, err
	}
	if in.ConsoleDeny, err = normalizeWords(in.ConsoleDeny); err != nil {
		return nil, err
	}
	in.Perms = cleanPerms(in.Perms)
	rs.mu.Lock()
	defer rs.mu.Unlock()
	old := rs.roles[in.ID]
	switch {
	case create && old != nil:
		return nil, conflict("Rolle „%s“ existiert bereits", in.ID)
	case !create && old == nil:
		return nil, &APIError{http.StatusNotFound, "Rolle nicht gefunden"}
	case old != nil && old.Locked:
		return nil, badRequest("Die Administrator-Rolle kann nicht verändert werden")
	}
	// Only permissions the editor holds may be added; existing ones may stay.
	for _, p := range in.Perms {
		if !grantable(p) && (old == nil || !old.Has(p)) {
			return nil, &APIError{http.StatusForbidden, "Du kannst nur Berechtigungen vergeben, die du selbst hast (" + permKnown[p].Label + ")"}
		}
	}
	in.Builtin = old != nil && old.Builtin
	in.Locked = false
	r := in
	rs.roles[in.ID] = &r
	if err := rs.saveLocked(); err != nil {
		return nil, err
	}
	cp := r
	return &cp, nil
}

func (rs *Roles) Delete(id string, inUse func(string) bool) error {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	r := rs.roles[id]
	switch {
	case r == nil:
		return &APIError{http.StatusNotFound, "Rolle nicht gefunden"}
	case r.Builtin:
		return badRequest("Eingebaute Rollen können nicht gelöscht werden")
	case inUse(id):
		return conflict("Die Rolle ist noch Benutzern zugewiesen")
	}
	delete(rs.roles, id)
	return rs.saveLocked()
}

// ---- small file helpers ----------------------------------------------------------

func writeFileAtomic(path string, b []byte, mode os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, mode); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func mustJSONIndent(v any) []byte {
	b, _ := json.MarshalIndent(v, "", "  ")
	return b
}
