package main

// Users, sessions, two-factor authentication, login throttling and invitations.
//
// All accounts live in users.json. On first start the account from
// CRAFTDOCK_USER / CRAFTDOCK_PASSWORD is created as administrator; afterwards
// it is an ordinary account (CRAFTDOCK_RESET_ADMIN=1 resets its password and
// 2FA from the environment, e.g. after losing the authenticator).

import (
	"crypto/pbkdf2"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
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

// Quota limits what a user may own. Zero means unlimited.
type Quota struct {
	Servers  int     `json:"servers"`
	MemoryMB int     `json:"memoryMB"` // sum of Java heap of owned servers
	CPUs     float64 `json:"cpus"`     // sum of CPU limits of owned servers
}

type User struct {
	Username   string            `json:"username"`
	Role       string            `json:"role"`
	Email      string            `json:"email,omitempty"`
	AllServers bool              `json:"allServers"`        // role applies to every server
	Access     map[string]string `json:"access,omitempty"`  // server → role ID
	Servers    []string          `json:"servers,omitempty"` // ≤ 1.3: restricted server list (migrated)
	Quota      Quota             `json:"quota"`
	Disabled   bool              `json:"disabled,omitempty"`
	Hash       string            `json:"hash,omitempty"`
	TOTP       string            `json:"totp,omitempty"`
	TOTPLast   int64             `json:"totpLast,omitempty"`
	TOTPSetup  string            `json:"totpSetup,omitempty"` // secret waiting for confirmation
	Recovery   []string          `json:"recovery,omitempty"`  // SHA-256 of unused codes
	KnownIPs   []string          `json:"knownIps,omitempty"`
	Created    time.Time         `json:"created"`
	LastLogin  time.Time         `json:"lastLogin,omitempty"`
	CreatedBy  string            `json:"createdBy,omitempty"`
}

// UserView is what the API returns – never secrets.
type UserView struct {
	Username   string            `json:"username"`
	Role       string            `json:"role"`
	Email      string            `json:"email,omitempty"`
	AllServers bool              `json:"allServers"`
	Access     map[string]string `json:"access"`
	Quota      Quota             `json:"quota"`
	Disabled   bool              `json:"disabled"`
	TwoFA      bool              `json:"twofa"`
	Recovery   int               `json:"recoveryLeft"`
	Created    time.Time         `json:"created"`
	LastLogin  time.Time         `json:"lastLogin"`
	CreatedBy  string            `json:"createdBy,omitempty"`
	Sessions   int               `json:"sessions"`
}

func (u *User) view() UserView {
	acc := u.Access
	if acc == nil {
		acc = map[string]string{}
	}
	return UserView{Username: u.Username, Role: u.Role, Email: u.Email, AllServers: u.AllServers || u.Role == "admin",
		Access: acc, Quota: u.Quota, Disabled: u.Disabled, TwoFA: u.TOTP != "", Recovery: len(u.Recovery),
		Created: u.Created, LastLogin: u.LastLogin, CreatedBy: u.CreatedBy}
}

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

func validPassword(pw string) error {
	if len(pw) < 10 || len(pw) > 200 {
		return badRequest("Das Passwort muss 10 bis 200 Zeichen lang sein")
	}
	return nil
}

func tokenHash(tok string) string {
	s := sha256.Sum256([]byte(tok))
	return hex.EncodeToString(s[:])
}

// ---- Auth ------------------------------------------------------------------------

type Session struct {
	ID       string    `json:"id"`
	Hash     string    `json:"hash"`
	User     string    `json:"user"`
	Created  time.Time `json:"created"`
	Expires  time.Time `json:"expires"`
	LastSeen time.Time `json:"lastSeen"`
	IP       string    `json:"ip"`
	Agent    string    `json:"agent"`
}

type ticket struct {
	user     string
	exp      time.Time
	attempts int
}

type failInfo struct {
	count int
	first time.Time
	until time.Time
}

type Invite struct {
	ID         string            `json:"id"`
	Hash       string            `json:"hash"`
	Role       string            `json:"role"`
	AllServers bool              `json:"allServers"`
	Access     map[string]string `json:"access,omitempty"`
	Quota      Quota             `json:"quota"`
	Note       string            `json:"note"`
	CreatedBy  string            `json:"createdBy"`
	Created    time.Time         `json:"created"`
	Expires    time.Time         `json:"expires"`
	MaxUses    int               `json:"maxUses"`
	Uses       int               `json:"uses"`
}

// AuthEvent is reported to the alerting.
type AuthEvent struct {
	Kind       string // login_failed, locked, login
	User, IP   string
	Privileged bool
	NewIP      bool
}

type Auth struct {
	dir   string
	ttl   time.Duration
	roles *Roles

	mu        sync.Mutex
	users     map[string]*User
	sessions  map[string]*Session // by hash
	savedAt   time.Time
	tickets   map[string]*ticket
	failsKey  map[string]*failInfo // user|ip
	failsIP   map[string]*failInfo
	regByIP   map[string][]time.Time
	invites   map[string]*Invite // by hash
	OnEvent   func(AuthEvent)
	force2FA  bool
	ownerOf   func(server string) string
	auditMu   sync.Mutex
	audit     []AuditEntry
	auditPath string
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
var emailRe = regexp.MustCompile(`^[^@\s]{1,64}@[^@\s]{1,190}\.[a-zA-Z]{2,}$`)

func NewAuth(dataDir, bootUser, bootPass string, reset bool, roles *Roles) *Auth {
	a := &Auth{dir: dataDir, ttl: 12 * time.Hour, roles: roles, users: map[string]*User{}, sessions: map[string]*Session{},
		tickets: map[string]*ticket{}, failsKey: map[string]*failInfo{}, failsIP: map[string]*failInfo{},
		regByIP: map[string][]time.Time{}, invites: map[string]*Invite{}, ownerOf: func(string) string { return "" }}
	if dataDir != "" {
		a.auditPath = filepath.Join(dataDir, "audit.json")
		var raw []json.RawMessage
		a.load("users.json", &raw)
		for _, r := range raw {
			var u User
			if err := json.Unmarshal(r, &u); err != nil || u.Username == "" {
				continue
			}
			a.migrate(&u, !strings.Contains(string(r), `"allServers"`))
			a.users[u.Username] = &u
		}
		var sess []*Session
		a.load("sessions.json", &sess)
		now := time.Now()
		for _, s := range sess {
			if now.Before(s.Expires) && a.users[s.User] != nil {
				a.sessions[s.Hash] = s
			}
		}
		var inv []*Invite
		a.load("invites.json", &inv)
		for _, i := range inv {
			if now.Before(i.Expires) {
				a.invites[i.Hash] = i
			}
		}
		a.load("audit.json", &a.audit)
	}
	bootUser = strings.ToLower(strings.TrimSpace(bootUser))
	if bootUser != "" && usernameRe.MatchString(bootUser) {
		u := a.users[bootUser]
		switch {
		case u == nil:
			pass := bootPass
			if pass == "" {
				pass = randomHex(8)
				log.Printf("Kein CRAFTDOCK_PASSWORD gesetzt – Startpasswort für %q: %s (bitte nach dem Login ändern)", bootUser, pass)
			}
			h, _ := hashPassword(pass)
			a.users[bootUser] = &User{Username: bootUser, Role: "admin", AllServers: true, Hash: h, Created: time.Now().UTC(), CreatedBy: "bootstrap"}
			log.Printf("Administrator %q angelegt", bootUser)
			a.mu.Lock()
			_ = a.saveUsersLocked()
			a.mu.Unlock()
		case reset:
			if bootPass == "" {
				log.Printf("CRAFTDOCK_RESET_ADMIN gesetzt, aber kein CRAFTDOCK_PASSWORD – nichts zurückgesetzt")
				break
			}
			h, _ := hashPassword(bootPass)
			u.Hash, u.TOTP, u.TOTPSetup, u.Recovery, u.Disabled, u.Role, u.AllServers = h, "", "", nil, false, "admin", true
			a.mu.Lock()
			a.dropSessionsLocked(bootUser, "")
			_ = a.saveUsersLocked()
			a.mu.Unlock()
			log.Printf("Administrator %q zurückgesetzt (Passwort aus CRAFTDOCK_PASSWORD, 2FA entfernt) – CRAFTDOCK_RESET_ADMIN jetzt wieder entfernen", bootUser)
		}
	}
	return a
}

// migrate converts accounts of CraftDock ≤ 1.3.
func (a *Auth) migrate(u *User, legacy bool) {
	if a.roles.Get(u.Role) == nil {
		log.Printf("WARNUNG: Benutzer %q hat unbekannte Rolle %q – wird Betrachter", u.Username, u.Role)
		u.Role = "viewer"
	}
	if legacy {
		if len(u.Servers) == 0 {
			u.AllServers = true // ≤ 1.3: empty list meant all servers
		} else {
			u.Access = map[string]string{}
			for _, s := range u.Servers {
				u.Access[s] = u.Role
			}
		}
		u.Servers = nil
	}
}

func (a *Auth) load(name string, v any) {
	b, err := os.ReadFile(filepath.Join(a.dir, name))
	if err != nil {
		return
	}
	if err := json.Unmarshal(b, v); err != nil {
		log.Printf("WARNUNG: %s ist beschädigt: %v", name, err)
	}
}

func (a *Auth) write(name string, v any) error {
	if a.dir == "" {
		return nil
	}
	return writeFileAtomic(filepath.Join(a.dir, name), mustJSONIndent(v), 0o600)
}

func (a *Auth) saveUsersLocked() error {
	list := make([]*User, 0, len(a.users))
	for _, u := range a.users {
		list = append(list, u)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Username < list[j].Username })
	return a.write("users.json", list)
}

func (a *Auth) saveSessionsLocked() {
	list := make([]*Session, 0, len(a.sessions))
	now := time.Now()
	for h, s := range a.sessions {
		if now.After(s.Expires) {
			delete(a.sessions, h)
			continue
		}
		list = append(list, s)
	}
	a.savedAt = now
	_ = a.write("sessions.json", list)
}

func (a *Auth) saveInvitesLocked() {
	list := make([]*Invite, 0, len(a.invites))
	for _, i := range a.invites {
		list = append(list, i)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Created.Before(list[j].Created) })
	_ = a.write("invites.json", list)
}

func (a *Auth) emit(e AuthEvent) {
	if a.OnEvent != nil {
		go a.OnEvent(e)
	}
}

// ---- permission evaluation --------------------------------------------------------

func (a *Auth) roleOf(u *User) *Role {
	if r := a.roles.Get(u.Role); r != nil {
		return r
	}
	return &Role{ID: "none"}
}

// Can reports a global permission.
func (a *Auth) Can(u *User, perm string) bool { return a.roleOf(u).Has(perm) }

// GlobalPerms lists the panel-wide permissions of u.
func (a *Auth) GlobalPerms(u *User) []string {
	out := []string{}
	for _, p := range a.roleOf(u).Perms {
		if !serverPerm[p] {
			out = append(out, p)
		}
	}
	return out
}

// ServerRole is the role u acts with on a server, or nil without access:
// the own role for all servers (admins, AllServers) or own servers, an
// explicitly assigned role otherwise.
func (a *Auth) ServerRole(u *User, server string) *Role {
	if u.Role == "admin" || u.AllServers {
		return a.roleOf(u)
	}
	if owner := a.ownerOf(server); owner != "" && owner == u.Username {
		return a.roleOf(u)
	}
	if rid, ok := u.Access[server]; ok {
		return a.roles.Get(rid)
	}
	return nil
}

func (a *Auth) ServerPerms(u *User, server string) []string {
	r := a.ServerRole(u, server)
	if r == nil {
		return nil
	}
	out := []string{}
	for _, p := range r.Perms {
		if serverPerm[p] {
			out = append(out, p)
		}
	}
	return out
}

func (a *Auth) CanServer(u *User, server, perm string) bool {
	r := a.ServerRole(u, server)
	return r != nil && (perm == "" || r.Has(perm))
}

// Needs2FASetup: the role demands 2FA but the user has not set it up yet.
func (a *Auth) Needs2FASetup(u *User) bool {
	return u.TOTP == "" && a.requires2FA(u)
}

func (a *Auth) requires2FA(u *User) bool { return a.force2FA || a.roleOf(u).Require2FA }

// SetForce2FA makes 2FA mandatory for every account.
func (a *Auth) SetForce2FA(v bool) {
	a.mu.Lock()
	a.force2FA = v
	a.mu.Unlock()
}

// grantableBy: perms the editor holds (admins: all).
func (a *Auth) grantableBy(editor *User) func(string) bool {
	r := a.roleOf(editor)
	return func(p string) bool { return editor.Role == "admin" || r.Has(p) }
}

// roleWithin reports whether role rid grants nothing beyond editor's own role.
func (a *Auth) roleWithin(editor *User, rid string) bool {
	if editor.Role == "admin" {
		return true
	}
	r := a.roles.Get(rid)
	if r == nil {
		return false
	}
	g := a.grantableBy(editor)
	for _, p := range r.Perms {
		if !g(p) {
			return false
		}
	}
	return true
}

func (a *Auth) RoleInUse(rid string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, u := range a.users {
		if u.Role == rid {
			return true
		}
		for _, r := range u.Access {
			if r == rid {
				return true
			}
		}
	}
	for _, i := range a.invites {
		if i.Role == rid {
			return true
		}
	}
	return false
}

// ---- login, throttling, 2FA --------------------------------------------------------

const (
	failWindow   = 15 * time.Minute
	keyFailLimit = 5  // per user + IP
	ipFailLimit  = 20 // per IP
)

type LoginResult struct {
	Token  string
	Ticket string // 2FA step pending
	User   *User
}

func (a *Auth) lockedLocked(key, ip string) time.Duration {
	now := time.Now()
	for _, f := range []*failInfo{a.failsKey[key], a.failsIP[ip]} {
		if f != nil && now.Before(f.until) {
			return f.until.Sub(now)
		}
	}
	return 0
}

func (a *Auth) failLocked(user, ip string) {
	now := time.Now()
	bump := func(m map[string]*failInfo, k string, limit int, lock time.Duration) bool {
		f := m[k]
		if f == nil || now.Sub(f.first) > failWindow {
			f = &failInfo{first: now}
			m[k] = f
		}
		f.count++
		if f.count >= limit {
			f.until, f.count, f.first = now.Add(lock), 0, now
			return true
		}
		return false
	}
	lockedKey := bump(a.failsKey, user+"|"+ip, keyFailLimit, 15*time.Minute)
	lockedIP := bump(a.failsIP, ip, ipFailLimit, 30*time.Minute)
	a.emit(AuthEvent{Kind: "login_failed", User: user, IP: ip})
	if lockedKey || lockedIP {
		log.Printf("Login gesperrt für %q von %s nach wiederholten Fehlversuchen", user, ip)
		a.emit(AuthEvent{Kind: "locked", User: user, IP: ip})
	}
}

func lockedErr(d time.Duration) error {
	return &APIError{http.StatusTooManyRequests, fmt.Sprintf("Zu viele Fehlversuche – bitte in %d Minuten erneut versuchen", int(d.Minutes())+1)}
}

func (a *Auth) Login(user, pass, ip, agent string) (*LoginResult, error) {
	user = strings.ToLower(strings.TrimSpace(user))
	a.mu.Lock()
	if d := a.lockedLocked(user+"|"+ip, ip); d > 0 {
		a.mu.Unlock()
		return nil, lockedErr(d)
	}
	cand := a.users[user]
	var hash string
	if cand != nil {
		hash = cand.Hash
	}
	a.mu.Unlock()
	ok := false
	if cand != nil {
		ok = checkPassword(hash, pass)
	} else {
		_, _ = hashPassword(pass) // equalise timing for unknown users
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if !ok {
		a.failLocked(user, ip)
		return nil, &APIError{http.StatusUnauthorized, "Benutzername oder Passwort falsch"}
	}
	if cand.Disabled {
		return nil, &APIError{http.StatusForbidden, "Dieses Konto ist gesperrt"}
	}
	if cand.TOTP != "" {
		t := randomHex(24)
		now := time.Now()
		for k, v := range a.tickets {
			if now.After(v.exp) {
				delete(a.tickets, k)
			}
		}
		a.tickets[tokenHash(t)] = &ticket{user: user, exp: now.Add(5 * time.Minute)}
		cp := *cand
		return &LoginResult{Ticket: t, User: &cp}, nil
	}
	return a.finishLoginLocked(cand, ip, agent), nil
}

// Login2FA completes a login with a TOTP or recovery code.
func (a *Auth) Login2FA(tk, code, ip, agent string) (*LoginResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	t := a.tickets[tokenHash(tk)]
	if t == nil || time.Now().After(t.exp) {
		return nil, &APIError{http.StatusUnauthorized, "Anmeldung abgelaufen – bitte erneut anmelden"}
	}
	if d := a.lockedLocked(t.user+"|"+ip, ip); d > 0 {
		return nil, lockedErr(d)
	}
	u := a.users[t.user]
	if u == nil || u.Disabled {
		return nil, &APIError{http.StatusUnauthorized, "Anmeldung nicht möglich"}
	}
	if !a.checkSecondFactorLocked(u, code) {
		t.attempts++
		if t.attempts >= 5 {
			delete(a.tickets, tokenHash(tk))
		}
		a.failLocked(t.user, ip)
		return nil, &APIError{http.StatusUnauthorized, "Code ungültig"}
	}
	delete(a.tickets, tokenHash(tk))
	return a.finishLoginLocked(u, ip, agent), nil
}

// checkSecondFactorLocked accepts a current TOTP code or an unused recovery code.
func (a *Auth) checkSecondFactorLocked(u *User, code string) bool {
	if step, ok := totpVerify(u.TOTP, code, u.TOTPLast, time.Now()); ok {
		u.TOTPLast = step
		_ = a.saveUsersLocked()
		return true
	}
	h := hashRecovery(code)
	for i, r := range u.Recovery {
		if subtle.ConstantTimeCompare([]byte(r), []byte(h)) == 1 {
			u.Recovery = append(u.Recovery[:i:i], u.Recovery[i+1:]...)
			_ = a.saveUsersLocked()
			log.Printf("Benutzer %q hat einen Wiederherstellungscode verwendet (%d übrig)", u.Username, len(u.Recovery))
			return true
		}
	}
	return false
}

func (a *Auth) finishLoginLocked(u *User, ip, agent string) *LoginResult {
	delete(a.failsKey, u.Username+"|"+ip)
	tok := randomHex(32)
	h := tokenHash(tok)
	now := time.Now()
	if len(agent) > 200 {
		agent = agent[:200]
	}
	a.sessions[h] = &Session{ID: h[:16], Hash: h, User: u.Username, Created: now, Expires: now.Add(a.ttl), LastSeen: now, IP: ip, Agent: agent}
	a.saveSessionsLocked()
	newIP := true
	for _, k := range u.KnownIPs {
		if k == ip {
			newIP = false
		}
	}
	if newIP {
		u.KnownIPs = append(u.KnownIPs, ip)
		if len(u.KnownIPs) > 20 {
			u.KnownIPs = u.KnownIPs[len(u.KnownIPs)-20:]
		}
	}
	u.LastLogin = now.UTC()
	_ = a.saveUsersLocked()
	r := a.roleOf(u)
	priv := r.Has(PermUsers) || r.Has(PermRoles) || r.Has(PermSettings)
	a.emit(AuthEvent{Kind: "login", User: u.Username, IP: ip, Privileged: priv, NewIP: newIP})
	cp := *u
	return &LoginResult{Token: tok, User: &cp}
}

// User returns the logged-in user for the request, or nil.
func (a *Auth) User(r *http.Request) *User {
	u, _ := a.userSession(r)
	return u
}

func (a *Auth) userSession(r *http.Request) (*User, *Session) {
	c, err := r.Cookie(sessionCookie)
	if err != nil || c.Value == "" {
		return nil, nil
	}
	h := tokenHash(c.Value)
	a.mu.Lock()
	defer a.mu.Unlock()
	s, ok := a.sessions[h]
	now := time.Now()
	if !ok || now.After(s.Expires) {
		if ok {
			delete(a.sessions, h)
		}
		return nil, nil
	}
	u, ok := a.users[s.User]
	if !ok || u.Disabled {
		delete(a.sessions, h)
		a.saveSessionsLocked()
		return nil, nil
	}
	s.Expires, s.LastSeen = now.Add(a.ttl), now // sliding expiry
	if now.Sub(a.savedAt) > 5*time.Minute {
		a.saveSessionsLocked()
	}
	cp := *u
	sc := *s
	return &cp, &sc
}

func (a *Auth) Logout(r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		a.mu.Lock()
		delete(a.sessions, tokenHash(c.Value))
		a.saveSessionsLocked()
		a.mu.Unlock()
	}
}

// dropSessionsLocked ends all sessions of user except the one with hash keep.
func (a *Auth) dropSessionsLocked(user, keep string) int {
	n := 0
	for h, s := range a.sessions {
		if s.User == user && h != keep {
			delete(a.sessions, h)
			n++
		}
	}
	a.saveSessionsLocked()
	return n
}

type SessionView struct {
	ID       string    `json:"id"`
	Created  time.Time `json:"created"`
	LastSeen time.Time `json:"lastSeen"`
	IP       string    `json:"ip"`
	Agent    string    `json:"agent"`
	Current  bool      `json:"current"`
}

func (a *Auth) Sessions(user, currentHash string) []SessionView {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := []SessionView{}
	for h, s := range a.sessions {
		if s.User == user && time.Now().Before(s.Expires) {
			out = append(out, SessionView{ID: s.ID, Created: s.Created, LastSeen: s.LastSeen, IP: s.IP, Agent: s.Agent, Current: h == currentHash})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LastSeen.After(out[j].LastSeen) })
	return out
}

func (a *Auth) RevokeSession(user, id string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	for h, s := range a.sessions {
		if s.User == user && s.ID == id {
			delete(a.sessions, h)
			a.saveSessionsLocked()
			return nil
		}
	}
	return &APIError{http.StatusNotFound, "Sitzung nicht gefunden"}
}

func (a *Auth) RevokeAll(user, keepHash string) int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.dropSessionsLocked(user, keepHash)
}

// ---- 2FA self-service ---------------------------------------------------------------

func (a *Auth) Begin2FA(name string) (secret, uri string, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	u := a.users[name]
	if u == nil {
		return "", "", ErrUserNotFound
	}
	if u.TOTP != "" {
		return "", "", conflict("Die Zwei-Faktor-Anmeldung ist bereits aktiv")
	}
	u.TOTPSetup = newTOTPSecret()
	if err := a.saveUsersLocked(); err != nil {
		return "", "", err
	}
	return u.TOTPSetup, totpURI("CraftDock", u.Username, u.TOTPSetup), nil
}

func (a *Auth) Confirm2FA(name, code, keepHash string) ([]string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	u := a.users[name]
	if u == nil {
		return nil, ErrUserNotFound
	}
	if u.TOTPSetup == "" {
		return nil, badRequest("Bitte die Einrichtung zuerst starten")
	}
	step, ok := totpVerify(u.TOTPSetup, code, 0, time.Now())
	if !ok {
		return nil, badRequest("Der Code stimmt nicht – Uhrzeit des Geräts prüfen und erneut versuchen")
	}
	plain, hashed := newRecoveryCodes()
	u.TOTP, u.TOTPSetup, u.TOTPLast, u.Recovery = u.TOTPSetup, "", step, hashed
	a.dropSessionsLocked(name, keepHash) // other sessions were created without 2FA
	return plain, a.saveUsersLocked()
}

func (a *Auth) Disable2FA(name, password, code string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	u := a.users[name]
	if u == nil {
		return ErrUserNotFound
	}
	if a.requires2FA(u) {
		return badRequest("Für dein Konto ist die Zwei-Faktor-Anmeldung vorgeschrieben")
	}
	if !checkPassword(u.Hash, password) || !a.checkSecondFactorLocked(u, code) {
		return badRequest("Passwort oder Code stimmt nicht")
	}
	u.TOTP, u.TOTPLast, u.Recovery = "", 0, nil
	return a.saveUsersLocked()
}

func (a *Auth) NewRecoveryCodes(name, password string) ([]string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	u := a.users[name]
	if u == nil {
		return nil, ErrUserNotFound
	}
	if u.TOTP == "" {
		return nil, badRequest("Die Zwei-Faktor-Anmeldung ist nicht aktiv")
	}
	if !checkPassword(u.Hash, password) {
		return nil, badRequest("Das Passwort stimmt nicht")
	}
	plain, hashed := newRecoveryCodes()
	u.Recovery = hashed
	return plain, a.saveUsersLocked()
}

// ---- user management ----------------------------------------------------------------

var ErrUserNotFound = &APIError{http.StatusNotFound, "Benutzer nicht gefunden"}

func (a *Auth) List() []UserView {
	a.mu.Lock()
	defer a.mu.Unlock()
	count := map[string]int{}
	now := time.Now()
	for _, s := range a.sessions {
		if now.Before(s.Expires) {
			count[s.User]++
		}
	}
	names := make([]string, 0, len(a.users))
	for n := range a.users {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]UserView, 0, len(names))
	for _, n := range names {
		v := a.users[n].view()
		v.Sessions = count[n]
		out = append(out, v)
	}
	return out
}

func (a *Auth) Get(name string) *User {
	a.mu.Lock()
	defer a.mu.Unlock()
	if u := a.users[name]; u != nil {
		cp := *u
		return &cp
	}
	return nil
}

type UserInput struct {
	Username   string            `json:"username"`
	Role       string            `json:"role"`
	Email      string            `json:"email"`
	AllServers bool              `json:"allServers"`
	Access     map[string]string `json:"access"`
	Quota      Quota             `json:"quota"`
	Disabled   bool              `json:"disabled"`
	Password   string            `json:"password"`
}

func (a *Auth) validateInput(in *UserInput, editor *User, needPassword bool) error {
	in.Username = strings.ToLower(strings.TrimSpace(in.Username))
	in.Email = strings.TrimSpace(in.Email)
	if a.roles.Get(in.Role) == nil {
		return badRequest("Unbekannte Rolle %q", in.Role)
	}
	if editor != nil && !a.roleWithin(editor, in.Role) {
		return &APIError{http.StatusForbidden, "Du kannst nur Rollen vergeben, die nicht mehr dürfen als deine eigene"}
	}
	for s, rid := range in.Access {
		if !nameRe.MatchString(s) {
			return badRequest("Ungültiger Servername %q", s)
		}
		if a.roles.Get(rid) == nil {
			return badRequest("Unbekannte Rolle %q für Server %s", rid, s)
		}
		if editor != nil && !a.roleWithin(editor, rid) {
			return &APIError{http.StatusForbidden, "Du kannst nur Rollen vergeben, die nicht mehr dürfen als deine eigene"}
		}
	}
	if in.Email != "" && !emailRe.MatchString(in.Email) {
		return badRequest("Ungültige E-Mail-Adresse")
	}
	if in.Quota.Servers < 0 || in.Quota.MemoryMB < 0 || in.Quota.CPUs < 0 || in.Quota.Servers > 1000 {
		return badRequest("Ungültige Quote")
	}
	if needPassword || in.Password != "" {
		return validPassword(in.Password)
	}
	return nil
}

func (a *Auth) Create(in UserInput, editor *User) error {
	if err := a.validateInput(&in, editor, true); err != nil {
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
	if a.users[in.Username] != nil {
		return conflict("Benutzer „%s“ existiert bereits", in.Username)
	}
	by := ""
	if editor != nil {
		by = editor.Username
	}
	a.users[in.Username] = &User{Username: in.Username, Role: in.Role, Email: in.Email, AllServers: in.AllServers, Access: in.Access,
		Quota: in.Quota, Disabled: in.Disabled, Hash: h, Created: time.Now().UTC(), CreatedBy: by}
	return a.saveUsersLocked()
}

func (a *Auth) Update(name string, in UserInput, editor *User) error {
	if err := a.validateInput(&in, editor, false); err != nil {
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
	u := a.users[name]
	if u == nil {
		return ErrUserNotFound
	}
	if editor != nil && !a.roleWithin(editor, u.Role) {
		return &APIError{http.StatusForbidden, "Dieser Benutzer hat mehr Rechte als du"}
	}
	if editor != nil && editor.Username == name && (in.Role != u.Role || in.Disabled) {
		return badRequest("Die eigene Rolle kannst du nicht ändern und dich nicht selbst sperren")
	}
	if u.Role == "admin" && in.Role != "admin" && a.countAdminsLocked() <= 1 {
		return badRequest("Es muss mindestens ein Administrator bleiben")
	}
	changedRights := u.Role != in.Role || u.Disabled != in.Disabled || h != ""
	u.Role, u.Email, u.AllServers, u.Access, u.Quota, u.Disabled = in.Role, in.Email, in.AllServers, in.Access, in.Quota, in.Disabled
	if h != "" {
		u.Hash = h
	}
	if changedRights {
		a.dropSessionsLocked(name, "") // new role / password takes effect immediately
	}
	return a.saveUsersLocked()
}

func (a *Auth) countAdminsLocked() int {
	n := 0
	for _, u := range a.users {
		if u.Role == "admin" && !u.Disabled {
			n++
		}
	}
	return n
}

func (a *Auth) Reset2FA(name string, editor *User) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	u := a.users[name]
	if u == nil {
		return ErrUserNotFound
	}
	if editor != nil && !a.roleWithin(editor, u.Role) {
		return &APIError{http.StatusForbidden, "Dieser Benutzer hat mehr Rechte als du"}
	}
	u.TOTP, u.TOTPSetup, u.TOTPLast, u.Recovery = "", "", 0, nil
	a.dropSessionsLocked(name, "")
	return a.saveUsersLocked()
}

func (a *Auth) SetOwnPassword(name, oldPw, newPw, keepHash string) error {
	if err := validPassword(newPw); err != nil {
		return err
	}
	a.mu.Lock()
	u := a.users[name]
	var hash string
	if u != nil {
		hash = u.Hash
	}
	a.mu.Unlock()
	if u == nil {
		return ErrUserNotFound
	}
	if !checkPassword(hash, oldPw) {
		return badRequest("Das aktuelle Passwort stimmt nicht")
	}
	h, err := hashPassword(newPw)
	if err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	u.Hash = h
	a.dropSessionsLocked(name, keepHash)
	return a.saveUsersLocked()
}

func (a *Auth) Delete(name string, editor *User) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	u := a.users[name]
	if u == nil {
		return ErrUserNotFound
	}
	if editor != nil && !a.roleWithin(editor, u.Role) {
		return &APIError{http.StatusForbidden, "Dieser Benutzer hat mehr Rechte als du"}
	}
	if u.Role == "admin" && a.countAdminsLocked() <= 1 {
		return badRequest("Der letzte Administrator kann nicht gelöscht werden")
	}
	delete(a.users, name)
	a.dropSessionsLocked(name, "")
	return a.saveUsersLocked()
}

// ---- invitations & registration ------------------------------------------------------

type InviteInput struct {
	Role       string            `json:"role"`
	AllServers bool              `json:"allServers"`
	Access     map[string]string `json:"access"`
	Quota      Quota             `json:"quota"`
	Note       string            `json:"note"`
	Days       int               `json:"days"`
	MaxUses    int               `json:"maxUses"`
}

func (a *Auth) CreateInvite(in InviteInput, editor *User) (string, *Invite, error) {
	ui := UserInput{Role: in.Role, AllServers: in.AllServers, Access: in.Access, Quota: in.Quota}
	if err := a.validateInput(&ui, editor, false); err != nil {
		return "", nil, err
	}
	if in.Days <= 0 {
		in.Days = 7
	}
	if in.Days > 90 || in.MaxUses < 0 || in.MaxUses > 1000 || len(in.Note) > 200 {
		return "", nil, badRequest("Gültigkeit max. 90 Tage, max. 1000 Verwendungen, Notiz max. 200 Zeichen")
	}
	if in.MaxUses == 0 {
		in.MaxUses = 1
	}
	tok := randomHex(20)
	h := tokenHash(tok)
	now := time.Now().UTC()
	inv := &Invite{ID: h[:12], Hash: h, Role: in.Role, AllServers: in.AllServers, Access: in.Access, Quota: in.Quota,
		Note: strings.TrimSpace(in.Note), CreatedBy: editor.Username, Created: now, Expires: now.AddDate(0, 0, in.Days), MaxUses: in.MaxUses}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.invites[h] = inv
	a.saveInvitesLocked()
	cp := *inv
	return tok, &cp, nil
}

func (a *Auth) Invites() []Invite {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := []Invite{}
	for _, i := range a.invites {
		if time.Now().Before(i.Expires) {
			cp := *i
			cp.Hash = ""
			out = append(out, cp)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Created.After(out[j].Created) })
	return out
}

func (a *Auth) DeleteInvite(id string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	for h, i := range a.invites {
		if i.ID == id {
			delete(a.invites, h)
			a.saveInvitesLocked()
			return nil
		}
	}
	return &APIError{http.StatusNotFound, "Einladung nicht gefunden"}
}

// InviteFor returns a valid invitation for a token, or nil.
func (a *Auth) InviteFor(tok string) *Invite {
	if tok == "" {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	i := a.invites[tokenHash(tok)]
	if i == nil || time.Now().After(i.Expires) || i.Uses >= i.MaxUses {
		return nil
	}
	cp := *i
	return &cp
}

// Register creates an account from an invitation (or, with open registration,
// from the defaults) and logs it in.
func (a *Auth) Register(username, password, email, inviteTok string, open *Invite, ip, agent string) (*LoginResult, error) {
	a.mu.Lock()
	now := time.Now()
	var recent []time.Time
	for _, t := range a.regByIP[ip] {
		if now.Sub(t) < time.Hour {
			recent = append(recent, t)
		}
	}
	a.regByIP[ip] = recent
	if len(recent) >= 5 {
		a.mu.Unlock()
		return nil, &APIError{http.StatusTooManyRequests, "Zu viele Registrierungen von dieser Adresse – bitte später erneut versuchen"}
	}
	a.mu.Unlock()

	inv := a.InviteFor(inviteTok)
	if inv == nil {
		inv = open
	}
	if inv == nil {
		return nil, &APIError{http.StatusForbidden, "Die Einladung ist ungültig oder abgelaufen"}
	}
	in := UserInput{Username: username, Password: password, Email: email, Role: inv.Role, AllServers: inv.AllServers, Access: inv.Access, Quota: inv.Quota}
	if err := a.validateInput(&in, nil, true); err != nil {
		return nil, err
	}
	if !usernameRe.MatchString(in.Username) {
		return nil, badRequest("Benutzername: 2–32 Zeichen, a–z, 0–9, . _ -")
	}
	h, err := hashPassword(in.Password)
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.users[in.Username] != nil {
		return nil, conflict("Der Benutzername ist bereits vergeben")
	}
	by := "registrierung"
	if inv.Hash != "" {
		live := a.invites[inv.Hash]
		if live == nil || live.Uses >= live.MaxUses {
			return nil, &APIError{http.StatusForbidden, "Die Einladung ist ungültig oder abgelaufen"}
		}
		live.Uses++
		if live.Uses >= live.MaxUses {
			delete(a.invites, inv.Hash)
		}
		a.saveInvitesLocked()
		by = "einladung:" + inv.CreatedBy
	}
	u := &User{Username: in.Username, Role: in.Role, Email: in.Email, AllServers: in.AllServers, Access: in.Access, Quota: in.Quota,
		Hash: h, Created: now.UTC(), CreatedBy: by}
	a.users[u.Username] = u
	a.regByIP[ip] = append(a.regByIP[ip], now)
	if err := a.saveUsersLocked(); err != nil {
		return nil, err
	}
	log.Printf("Neues Konto %q registriert (%s, Rolle %s)", u.Username, by, u.Role)
	return a.finishLoginLocked(u, ip, agent), nil
}

// ---- audit log ----------------------------------------------------------------------

const auditMax = 2000

func (a *Auth) Audit(e AuditEntry) {
	a.auditMu.Lock()
	defer a.auditMu.Unlock()
	a.audit = append(a.audit, e)
	if len(a.audit) > auditMax {
		a.audit = a.audit[len(a.audit)-auditMax:]
	}
	if a.auditPath != "" {
		b, _ := json.Marshal(a.audit)
		_ = writeFileAtomic(a.auditPath, b, 0o600)
	}
}

func (a *Auth) AuditLog(limit int, user string) []AuditEntry {
	a.auditMu.Lock()
	defer a.auditMu.Unlock()
	out := []AuditEntry{}
	for i := len(a.audit) - 1; i >= 0 && len(out) < limit; i-- {
		if user == "" || a.audit[i].User == user {
			out = append(out, a.audit[i])
		}
	}
	return out
}

func (a *Auth) setCookie(w http.ResponseWriter, r *http.Request, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: value, Path: "/", MaxAge: maxAge,
		HttpOnly: true, Secure: isHTTPS(r), SameSite: http.SameSiteStrictMode,
	})
}
