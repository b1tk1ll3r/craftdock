package main

// Users, roles and sessions. The account from CRAFTDOCK_USER/CRAFTDOCK_PASSWORD
// is a built-in administrator; further users live in users.json.

import (
	"crypto/pbkdf2"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const sessionCookie = "craftdock_session"

// Permissions
const (
	PermView      = "view"      // see servers, status, logs
	PermControl   = "control"   // start, stop, restart
	PermConsole   = "console"   // send console commands
	PermPlayers   = "players"   // kick, ban, op, whitelist
	PermConfigure = "configure" // config, mods, files, world, image update
	PermBackup    = "backup"    // download backups
	PermCreate    = "create"    // create, clone
	PermDelete    = "delete"    // delete servers
	PermNews      = "news"      // news & events
	PermSettings  = "settings"  // public site, network/proxy
	PermUsers     = "users"     // user management, audit log
)

var allPerms = []string{PermView, PermControl, PermConsole, PermPlayers, PermConfigure, PermBackup, PermCreate, PermDelete, PermNews, PermSettings, PermUsers}

var rolePerms = map[string][]string{
	"admin":     allPerms,
	"operator":  {PermView, PermControl, PermConsole, PermPlayers, PermConfigure, PermBackup, PermCreate, PermNews},
	"moderator": {PermView, PermControl, PermConsole, PermPlayers, PermNews},
	"viewer":    {PermView},
}

type User struct {
	Username string    `json:"username"`
	Role     string    `json:"role"`
	Servers  []string  `json:"servers"` // empty = all servers
	Hash     string    `json:"hash,omitempty"`
	Created  time.Time `json:"created"`
	Builtin  bool      `json:"builtin,omitempty"`
}

func (u *User) Can(perm string) bool {
	for _, p := range rolePerms[u.Role] {
		if p == perm {
			return true
		}
	}
	return false
}

func (u *User) CanServer(name string) bool {
	if len(u.Servers) == 0 {
		return true
	}
	for _, s := range u.Servers {
		if s == name {
			return true
		}
	}
	return false
}

func (u *User) Perms() []string { return append([]string{}, rolePerms[u.Role]...) }

func (u User) public() User { u.Hash = ""; return u }

// ---- password hashing (PBKDF2-SHA256, stdlib) ----------------------------------

const pbkdfIter = 600_000

func hashPassword(pw string) (string, error) {
	salt := []byte(randomHex(16))
	key, err := pbkdf2.Key(sha256.New, pw, salt, pbkdfIter, 32)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("pbkdf2-sha256$%d$%s$%s", pbkdfIter, base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

func checkPassword(hash, pw string) bool {
	parts := strings.Split(hash, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" {
		return false
	}
	iter, err := strconv.Atoi(parts[1])
	if err != nil || iter < 1000 {
		return false
	}
	salt, err1 := base64.RawStdEncoding.DecodeString(parts[2])
	want, err2 := base64.RawStdEncoding.DecodeString(parts[3])
	if err1 != nil || err2 != nil {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, pw, salt, iter, len(want))
	return err == nil && subtle.ConstantTimeCompare(got, want) == 1
}

// ---- Auth ------------------------------------------------------------------------

type session struct {
	user string
	exp  time.Time
}

type Auth struct {
	builtinUser, builtinPass string
	path                     string
	ttl                      time.Duration

	mu       sync.Mutex
	users    map[string]*User
	sessions map[string]session

	auditMu sync.Mutex
	audit   []AuditEntry
}

type AuditEntry struct {
	Time   int64  `json:"time"`
	User   string `json:"user"`
	Action string `json:"action"`
	Target string `json:"target,omitempty"`
	Status int    `json:"status"`
	IP     string `json:"ip,omitempty"`
}

var usernameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]{1,31}$`)

func NewAuth(user, pass, dataDir string) *Auth {
	a := &Auth{builtinUser: user, builtinPass: pass, ttl: 12 * time.Hour, users: map[string]*User{}, sessions: map[string]session{}}
	if dataDir != "" {
		a.path = filepath.Join(dataDir, "users.json")
		if b, err := os.ReadFile(a.path); err == nil {
			var list []*User
			if err := json.Unmarshal(b, &list); err != nil {
				log.Printf("WARNUNG: users.json ist beschädigt: %v", err)
			}
			for _, u := range list {
				if u.Username != user {
					a.users[u.Username] = u
				}
			}
		}
		if b, err := os.ReadFile(filepath.Join(dataDir, "audit.json")); err == nil {
			_ = json.Unmarshal(b, &a.audit)
		}
	}
	return a
}

func (a *Auth) builtin() *User {
	return &User{Username: a.builtinUser, Role: "admin", Builtin: true}
}

func (a *Auth) saveLocked() error {
	if a.path == "" {
		return nil
	}
	list := make([]*User, 0, len(a.users))
	for _, u := range a.users {
		list = append(list, u)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Username < list[j].Username })
	b, _ := json.MarshalIndent(list, "", "  ")
	tmp := a.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, a.path)
}

func (a *Auth) Login(user, pass string) (string, *User, bool) {
	var u *User
	if subtle.ConstantTimeCompare([]byte(user), []byte(a.builtinUser)) == 1 {
		if subtle.ConstantTimeCompare([]byte(pass), []byte(a.builtinPass)) == 1 {
			u = a.builtin()
		}
	} else {
		a.mu.Lock()
		cand := a.users[user]
		a.mu.Unlock()
		if cand != nil && checkPassword(cand.Hash, pass) {
			u = cand
		} else if cand == nil {
			_, _ = hashPassword(pass) // equalise timing for unknown users
		}
	}
	if u == nil {
		return "", nil, false
	}
	tok := randomHex(32)
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	for t, s := range a.sessions {
		if now.After(s.exp) {
			delete(a.sessions, t)
		}
	}
	a.sessions[tok] = session{u.Username, now.Add(a.ttl)}
	return tok, u, true
}

// User returns the logged-in user for the request, or nil.
func (a *Auth) User(r *http.Request) *User {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	s, ok := a.sessions[c.Value]
	if !ok || time.Now().After(s.exp) {
		delete(a.sessions, c.Value)
		return nil
	}
	s.exp = time.Now().Add(a.ttl) // sliding expiry
	a.sessions[c.Value] = s
	if s.user == a.builtinUser {
		return a.builtin()
	}
	u, ok := a.users[s.user]
	if !ok {
		delete(a.sessions, c.Value)
		return nil
	}
	cp := *u
	return &cp
}

func (a *Auth) Logout(r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		a.mu.Lock()
		delete(a.sessions, c.Value)
		a.mu.Unlock()
	}
}

func (a *Auth) dropSessionsLocked(user string) {
	for t, s := range a.sessions {
		if s.user == user {
			delete(a.sessions, t)
		}
	}
}

func (a *Auth) List() []User {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := []User{a.builtin().public()}
	names := make([]string, 0, len(a.users))
	for n := range a.users {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		out = append(out, a.users[n].public())
	}
	return out
}

type UserInput struct {
	Username string   `json:"username"`
	Role     string   `json:"role"`
	Servers  []string `json:"servers"`
	Password string   `json:"password"`
}

func validateUserInput(in *UserInput, needPassword bool) error {
	in.Username = strings.ToLower(strings.TrimSpace(in.Username))
	if _, ok := rolePerms[in.Role]; !ok {
		return badRequest("Unbekannte Rolle %q", in.Role)
	}
	for _, s := range in.Servers {
		if !nameRe.MatchString(s) {
			return badRequest("Ungültiger Servername %q", s)
		}
	}
	if (needPassword || in.Password != "") && len(in.Password) < 10 {
		return badRequest("Das Passwort muss mindestens 10 Zeichen lang sein")
	}
	return nil
}

func (a *Auth) Create(in UserInput) error {
	if err := validateUserInput(&in, true); err != nil {
		return err
	}
	if !usernameRe.MatchString(in.Username) {
		return badRequest("Benutzername: 2–32 Zeichen, a–z, 0–9, . _ -")
	}
	h, err := hashPassword(in.Password)
	if err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if in.Username == a.builtinUser || a.users[in.Username] != nil {
		return conflict("Benutzer „%s“ existiert bereits", in.Username)
	}
	a.users[in.Username] = &User{Username: in.Username, Role: in.Role, Servers: in.Servers, Hash: h, Created: time.Now().UTC()}
	return a.saveLocked()
}

func (a *Auth) Update(name string, in UserInput) error {
	if err := validateUserInput(&in, false); err != nil {
		return err
	}
	var h string
	if in.Password != "" {
		var err error
		if h, err = hashPassword(in.Password); err != nil {
			return err
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if name == a.builtinUser {
		return badRequest("Der eingebaute Administrator wird über CRAFTDOCK_USER/CRAFTDOCK_PASSWORD verwaltet")
	}
	u := a.users[name]
	if u == nil {
		return &APIError{http.StatusNotFound, "Benutzer nicht gefunden"}
	}
	u.Role, u.Servers = in.Role, in.Servers
	if h != "" {
		u.Hash = h
	}
	a.dropSessionsLocked(name) // new role / password takes effect immediately
	return a.saveLocked()
}

func (a *Auth) SetOwnPassword(name, oldPw, newPw string) error {
	if len(newPw) < 10 {
		return badRequest("Das Passwort muss mindestens 10 Zeichen lang sein")
	}
	a.mu.Lock()
	u := a.users[name]
	a.mu.Unlock()
	if u == nil {
		return badRequest("Das Passwort des eingebauten Administrators wird über CRAFTDOCK_PASSWORD gesetzt")
	}
	if !checkPassword(u.Hash, oldPw) {
		return badRequest("Das aktuelle Passwort stimmt nicht")
	}
	h, err := hashPassword(newPw)
	if err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	u.Hash = h
	return a.saveLocked()
}

func (a *Auth) Delete(name string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if name == a.builtinUser {
		return badRequest("Der eingebaute Administrator kann nicht gelöscht werden")
	}
	if a.users[name] == nil {
		return &APIError{http.StatusNotFound, "Benutzer nicht gefunden"}
	}
	delete(a.users, name)
	a.dropSessionsLocked(name)
	return a.saveLocked()
}

// ---- audit log ----------------------------------------------------------------------

const auditMax = 1000

func (a *Auth) Audit(e AuditEntry) {
	a.auditMu.Lock()
	defer a.auditMu.Unlock()
	a.audit = append(a.audit, e)
	if len(a.audit) > auditMax {
		a.audit = a.audit[len(a.audit)-auditMax:]
	}
	if a.path != "" {
		b, _ := json.Marshal(a.audit)
		_ = os.WriteFile(filepath.Join(filepath.Dir(a.path), "audit.json"), b, 0o600)
	}
}

func (a *Auth) AuditLog(limit int) []AuditEntry {
	a.auditMu.Lock()
	defer a.auditMu.Unlock()
	out := []AuditEntry{}
	for i := len(a.audit) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, a.audit[i])
	}
	return out
}

func isHTTPS(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}

func (a *Auth) setCookie(w http.ResponseWriter, r *http.Request, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: value, Path: "/", MaxAge: maxAge,
		HttpOnly: true, Secure: isHTTPS(r), SameSite: http.SameSiteStrictMode,
	})
}
