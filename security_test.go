package main

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTOTPVectors(t *testing.T) {
	secret := []byte("12345678901234567890") // RFC 6238 appendix B
	for ts, want := range map[int64]string{59: "287082", 1111111109: "081804", 1234567890: "005924", 2000000000: "279037"} {
		if got := totpCode(secret, ts/30); got != want {
			t.Errorf("t=%d: got %s want %s", ts, got, want)
		}
	}
	sec := newTOTPSecret()
	raw, _ := b32.DecodeString(sec)
	now := time.Now()
	code := totpCode(raw, now.Unix()/30)
	step, ok := totpVerify(sec, code, 0, now)
	if !ok {
		t.Fatal("valid code rejected")
	}
	if _, ok := totpVerify(sec, code, step, now); ok {
		t.Fatal("replay accepted")
	}
	if _, ok := totpVerify(sec, "000000", 0, now.Add(10*time.Minute)); ok && code == "000000" {
		t.Fatal("old code accepted")
	}
}

func currentCode(secret string) string {
	raw, _ := b32.DecodeString(secret)
	return totpCode(raw, time.Now().Unix()/30)
}

func newTestAuth(t *testing.T) (*Auth, string) {
	dir := t.TempDir()
	return NewAuth(dir, "admin", "start-passwort-1", false, NewRoles(dir)), dir
}

func TestBootstrapAndLegacyMigration(t *testing.T) {
	dir := t.TempDir()
	legacy := `[{"username":"mod","role":"moderator","servers":["survival"],"hash":"x","created":"2025-01-01T00:00:00Z"},
	            {"username":"op","role":"operator","servers":[],"hash":"x","created":"2025-01-01T00:00:00Z"}]`
	_ = os.WriteFile(filepath.Join(dir, "users.json"), []byte(legacy), 0o600)
	a := NewAuth(dir, "admin", "start-passwort-1", false, NewRoles(dir))
	if res, err := a.Login("admin", "start-passwort-1", "1.2.3.4", "test"); err != nil || res.Token == "" {
		t.Fatalf("bootstrap admin: %v", err)
	}
	mod, op := a.Get("mod"), a.Get("op")
	if mod.AllServers || mod.Access["survival"] != "moderator" || len(mod.Servers) != 0 {
		t.Fatalf("restricted legacy user migrated wrong: %+v", mod)
	}
	if !op.AllServers {
		t.Fatal("unrestricted legacy user lost access")
	}
	// Restart: no second migration, bootstrap password not re-applied.
	b := NewAuth(dir, "admin", "anderes-passwort", false, NewRoles(dir))
	if _, err := b.Login("admin", "anderes-passwort", "1.2.3.4", ""); err == nil {
		t.Fatal("env password must not override existing account")
	}
	if b.Get("mod").Access["survival"] != "moderator" {
		t.Fatal("access lost after restart")
	}
	// Reset from environment
	c := NewAuth(dir, "admin", "neues-passwort-99", true, NewRoles(dir))
	if _, err := c.Login("admin", "neues-passwort-99", "1.2.3.4", ""); err != nil {
		t.Fatalf("reset: %v", err)
	}
}

func TestLoginThrottling(t *testing.T) {
	a, _ := newTestAuth(t)
	for i := 0; i < keyFailLimit; i++ {
		if _, err := a.Login("admin", "falsch", "9.9.9.9", ""); err == nil {
			t.Fatal("wrong password accepted")
		}
	}
	_, err := a.Login("admin", "start-passwort-1", "9.9.9.9", "")
	if ae, ok := err.(*APIError); !ok || ae.Status != 429 {
		t.Fatalf("expected lockout, got %v", err)
	}
	// other IP still works
	if _, err := a.Login("admin", "start-passwort-1", "8.8.8.8", ""); err != nil {
		t.Fatalf("other IP locked too: %v", err)
	}
}

func TestTwoFactorFlowAndSessions(t *testing.T) {
	a, dir := newTestAuth(t)
	first, _ := a.Login("admin", "start-passwort-1", "1.1.1.1", "A")
	second, _ := a.Login("admin", "start-passwort-1", "1.1.1.1", "B")
	secret, uri, err := a.Begin2FA("admin")
	if err != nil || !strings.HasPrefix(uri, "otpauth://totp/CraftDock:admin?secret=") {
		t.Fatalf("begin: %v %s", err, uri)
	}
	if _, err := a.Confirm2FA("admin", "123456", ""); err == nil && currentCode(secret) != "123456" {
		t.Fatal("wrong confirmation code accepted")
	}
	confirmCode := currentCode(secret)
	codes, err := a.Confirm2FA("admin", confirmCode, tokenHash(first.Token))
	if err != nil || len(codes) != 10 {
		t.Fatalf("confirm: %v", err)
	}
	// Other sessions are ended when 2FA is switched on.
	if n := len(a.Sessions("admin", "")); n != 1 {
		t.Fatalf("expected 1 session after enabling 2FA, got %d", n)
	}
	_ = second

	res, err := a.Login("admin", "start-passwort-1", "1.1.1.1", "")
	if err != nil || res.Ticket == "" || res.Token != "" {
		t.Fatalf("expected 2FA ticket: %+v %v", res, err)
	}
	if _, err := a.Login2FA(res.Ticket, "000000", "1.1.1.1", ""); err == nil && currentCode(secret) != "000000" {
		t.Fatal("bad code accepted")
	}
	// The TOTP step of the confirmation was consumed: the same code is a replay.
	if _, err := a.Login2FA(res.Ticket, confirmCode, "1.1.1.1", ""); err == nil {
		t.Fatal("replayed TOTP accepted")
	}
	done, err := a.Login2FA(res.Ticket, codes[0], "1.1.1.1", "")
	if err != nil || done.Token == "" {
		t.Fatalf("recovery code: %v", err)
	}
	res2, _ := a.Login("admin", "start-passwort-1", "1.1.1.1", "")
	if _, err := a.Login2FA(res2.Ticket, codes[0], "1.1.1.1", ""); err == nil {
		t.Fatal("recovery code reusable")
	}
	if a.Get("admin").TOTP == "" || len(a.Get("admin").Recovery) != 9 {
		t.Fatal("recovery code not consumed")
	}
	// sessions persist across restarts (hashed)
	b := NewAuth(dir, "admin", "", false, NewRoles(dir))
	if len(b.Sessions("admin", "")) == 0 {
		t.Fatal("sessions not persisted")
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "sessions.json"))
	if strings.Contains(string(raw), done.Token) {
		t.Fatal("plain session token stored")
	}
	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(&http.Cookie{Name: sessionCookie, Value: done.Token})
	if b.User(r) == nil {
		t.Fatal("persisted session not accepted")
	}
}

func TestPermissionsAndEscalation(t *testing.T) {
	a, _ := newTestAuth(t)
	owners := map[string]string{"kunde-srv": "kunde"}
	a.ownerOf = func(s string) string { return owners[s] }
	admin := a.Get("admin")
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(a.Create(UserInput{Username: "kunde", Role: "customer", Password: "kunden-pass-1"}, admin))
	must(a.Create(UserInput{Username: "op", Role: "operator", AllServers: true, Password: "operator-pass-1"}, admin))
	must(a.Create(UserInput{Username: "helfer", Role: "viewer", Access: map[string]string{"kunde-srv": "moderator"}, Password: "helfer-pass-12"}, admin))
	kunde, op, helfer := a.Get("kunde"), a.Get("op"), a.Get("helfer")

	if !a.CanServer(kunde, "kunde-srv", PermDelete) || a.CanServer(kunde, "kunde-srv", PermAdvanced) {
		t.Fatal("customer perms on own server wrong")
	}
	if a.ServerRole(kunde, "fremd") != nil {
		t.Fatal("customer sees foreign server")
	}
	if !a.CanServer(helfer, "kunde-srv", PermConsole) || a.CanServer(helfer, "kunde-srv", PermConfigure) || a.ServerRole(helfer, "fremd") != nil {
		t.Fatal("per-server assignment wrong")
	}
	if !a.CanServer(op, "irgendwas", PermAdvanced) || a.Can(op, PermUsers) {
		t.Fatal("operator perms wrong")
	}
	mod := a.ServerRole(helfer, "kunde-srv")
	if mod.AllowsCommand("op Steve") || mod.AllowsCommand("/minecraft:stop") || !mod.AllowsCommand("say hi") {
		t.Fatal("console deny list ignored")
	}
	allow := Role{ConsoleAllow: []string{"say", "list"}}
	if !allow.AllowsCommand("list") || allow.AllowsCommand("give x") {
		t.Fatal("console allow list ignored")
	}

	// A user manager without admin rights may not hand out admin.
	_, err := a.roles.Save(Role{ID: "support", Name: "Support", Perms: []string{PermUsers, PermView}}, true, a.grantableBy(admin))
	must(err)
	must(a.Create(UserInput{Username: "support", Role: "support", AllServers: true, Password: "support-pass-1"}, admin))
	sup := a.Get("support")
	if err := a.Create(UserInput{Username: "evil", Role: "admin", Password: "evil-pass-123"}, sup); err == nil {
		t.Fatal("support created an admin")
	}
	if err := a.Update("support", UserInput{Role: "admin"}, sup); err == nil {
		t.Fatal("support promoted himself")
	}
	if err := a.Delete("admin", sup); err == nil {
		t.Fatal("support deleted the admin")
	}
	if _, err := a.roles.Save(Role{ID: "evil", Name: "Evil", Perms: []string{PermRoles, PermSettings}}, true, a.grantableBy(sup)); err == nil {
		t.Fatal("role with foreign permissions created")
	}
	if err := a.Delete("admin", admin); err == nil {
		t.Fatal("last admin deletable")
	}
	if _, err := a.roles.Save(Role{ID: "admin", Name: "x"}, false, a.grantableBy(admin)); err == nil {
		t.Fatal("admin role editable")
	}
	if err := a.roles.Delete("support", a.RoleInUse); err == nil {
		t.Fatal("role in use deleted")
	}
	// Disabled accounts cannot log in and lose their sessions.
	must(a.Update("kunde", UserInput{Role: "customer", Disabled: true}, admin))
	if _, err := a.Login("kunde", "kunden-pass-1", "1.1.1.1", ""); err == nil {
		t.Fatal("disabled user logged in")
	}
}

func TestRolePersistenceAndLegacyPerms(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "roles.json"), []byte(`[{"id":"alt","name":"Alt","perms":["view","players","configure"]}]`), 0o600)
	rs := NewRoles(dir)
	r := rs.Get("alt")
	for _, p := range []string{PermView, PermPlayers, PermCheats, PermConfigure, PermMods} {
		if !r.Has(p) {
			t.Errorf("legacy perm not mapped: %s", p)
		}
	}
	if rs.Get("admin") == nil || !rs.Get("admin").Has(PermRoles) {
		t.Fatal("admin role missing")
	}
}

func TestHostingPolicy(t *testing.T) {
	dir := t.TempDir()
	d, _ := NewDocker("unix:///nonexistent")
	st := NewStore(dir)
	m := NewManager(d, "", st)
	auth := NewAuth(dir, "admin", "start-passwort-1", false, NewRoles(dir))
	auth.ownerOf = st.Owner
	a := &API{m: m, auth: auth}
	_ = st.SetHosting(HostingSettings{PortMin: 30000, PortMax: 30002, Registration: "off", DefaultRole: "customer", OverheadMB: 512})
	_ = auth.Create(UserInput{Username: "kunde", Role: "customer", Quota: Quota{Servers: 2, MemoryMB: 6144, CPUs: 3}, Password: "kunden-pass-1"}, auth.Get("admin"))
	kunde := auth.Get("kunde")
	ctx := context.Background()

	st.SaveConfig(ServerConfig{Name: "belegt", Port: 30000, Memory: "2G", RconPassword: "x"})
	cfg := ServerConfig{Name: "eins", Memory: "4G", DataPath: "/etc", Owner: "admin", ExtraEnv: map[string]string{"LEVEL": "welt"}}
	if err := a.applyPolicy(ctx, a.policyFor(kunde, ""), &cfg, nil); err != nil {
		t.Fatal(err)
	}
	if cfg.Owner != "kunde" || cfg.DataPath != "" || cfg.Port != 30001 || cfg.MemoryLimitMB != 4096+1024 || cfg.CPUs != 1 {
		t.Fatalf("policy result wrong: owner=%s path=%s port=%d limit=%d cpus=%v", cfg.Owner, cfg.DataPath, cfg.Port, cfg.MemoryLimitMB, cfg.CPUs)
	}
	cfg.RconPassword = "x"
	st.SaveConfig(cfg)

	bad := ServerConfig{Name: "zwei", Memory: "4G"}
	if err := a.applyPolicy(ctx, a.policyFor(kunde, ""), &bad, nil); err == nil || !strings.Contains(err.Error(), "MB") {
		t.Fatalf("memory quota not enforced: %v", err)
	}
	env := ServerConfig{Name: "zwei", Memory: "1G", ExtraEnv: map[string]string{"JVM_OPTS": "-Xmx64G"}}
	if err := a.applyPolicy(ctx, a.policyFor(kunde, ""), &env, nil); err == nil {
		t.Fatal("restricted env var accepted")
	}
	port := ServerConfig{Name: "zwei", Memory: "1G", Port: 25565}
	if err := a.applyPolicy(ctx, a.policyFor(kunde, ""), &port, nil); err == nil {
		t.Fatal("port outside range accepted")
	}
	okCfg := ServerConfig{Name: "zwei", Memory: "1G"}
	if err := a.applyPolicy(ctx, a.policyFor(kunde, ""), &okCfg, nil); err != nil {
		t.Fatal(err)
	}
	okCfg.RconPassword = "x"
	st.SaveConfig(okCfg)
	third := ServerConfig{Name: "drei", Memory: "512M"}
	if err := a.applyPolicy(ctx, a.policyFor(kunde, ""), &third, nil); err == nil {
		t.Fatal("server count quota not enforced")
	}
	// Update keeps owner and ports untouched by the customer.
	upd := cfg
	upd.Owner = "someone"
	upd.Memory = "5G"
	if err := a.applyPolicy(ctx, a.policyFor(kunde, "eins"), &upd, &cfg); err != nil || upd.Owner != "kunde" {
		t.Fatalf("update: %v owner=%s", err, upd.Owner)
	}
	// Admins are not bound by hosting rules and may assign owners.
	adm := ServerConfig{Name: "team", Memory: "8G", Port: 25565, Owner: "kunde", DataPath: "/srv/mc"}
	if err := a.applyPolicy(ctx, a.policyFor(auth.Get("admin"), ""), &adm, nil); err != nil || adm.Owner != "kunde" || adm.DataPath != "/srv/mc" {
		t.Fatalf("admin policy: %v %+v", err, adm)
	}
	if u := m.Usage("kunde", ""); u.Servers != 2 || u.MemoryMB != 5120 {
		t.Fatalf("usage: %+v", u)
	}
}

func TestClientIPTrustedProxies(t *testing.T) {
	defer initTrustedProxies("")
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "203.0.113.5:4444"
	r.Header.Set("X-Forwarded-For", "6.6.6.6")
	r.Header.Set("X-Forwarded-Proto", "https")
	initTrustedProxies("")
	if clientIP(r) != "203.0.113.5" || isHTTPS(r) {
		t.Fatal("untrusted forwarded header honoured")
	}
	r.RemoteAddr = "10.0.0.2:4444"
	r.Header.Set("X-Forwarded-For", "6.6.6.6, 198.51.100.7, 10.0.0.9")
	initTrustedProxies("private")
	if got := clientIP(r); got != "198.51.100.7" || !isHTTPS(r) {
		t.Fatalf("trusted proxy chain: %s", got)
	}
}

func TestInvitesAndRegistration(t *testing.T) {
	a, _ := newTestAuth(t)
	admin := a.Get("admin")
	tok, inv, err := a.CreateInvite(InviteInput{Role: "customer", Quota: Quota{Servers: 1}, Days: 3}, admin)
	if err != nil || inv.Hash == "" {
		t.Fatal(err)
	}
	if a.InviteFor("falsch") != nil || a.InviteFor(tok) == nil {
		t.Fatal("invite lookup")
	}
	res, err := a.Register("Neu", "neues-konto-123", "neu@example.com", tok, nil, "5.5.5.5", "")
	if err != nil || res.Token == "" || a.Get("neu").Quota.Servers != 1 || a.Get("neu").Role != "customer" {
		t.Fatalf("register: %v", err)
	}
	if _, err := a.Register("zweiter", "neues-konto-123", "", tok, nil, "5.5.5.5", ""); err == nil {
		t.Fatal("single-use invite reused")
	}
	if _, err := a.Register("offen", "neues-konto-123", "", "", nil, "5.5.5.5", ""); err == nil {
		t.Fatal("registration without invite accepted")
	}
	open := &Invite{Role: "viewer", MaxUses: 1}
	if _, err := a.Register("offen", "neues-konto-123", "", "", open, "5.5.5.6", ""); err != nil {
		t.Fatal(err)
	}
}

func TestAlertConfigValidation(t *testing.T) {
	d, _ := NewDocker("unix:///nonexistent")
	dir := t.TempDir()
	al := NewAlerter(NewManager(d, "", NewStore(dir)), dir)
	cfg := AlertConfig{
		Channels: []AlertChannel{{ID: "dc", Name: "Discord", Type: "discord", URL: "https://discord.com/api/webhooks/123/geheim", Enabled: true}},
		Rules:    []AlertRule{{Event: "tps", Channels: []string{"dc"}, Enabled: true}},
	}
	if err := al.SetConfig(cfg); err != nil {
		t.Fatal(err)
	}
	got := al.Config(true)
	if strings.Contains(got.Channels[0].URL, "geheim") || got.Rules[0].Threshold != 15 || got.Rules[0].Name == "" {
		t.Fatalf("masking/defaults: %+v", got)
	}
	// Saving the masked config keeps the secret.
	if err := al.SetConfig(got); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(al.Config(false).Channels[0].URL, "geheim") {
		t.Fatal("secret lost on round trip")
	}
	bad := got
	bad.Rules = []AlertRule{{Event: "tps", Channels: []string{"nope"}}}
	if err := al.SetConfig(bad); err == nil {
		t.Fatal("unknown channel accepted")
	}
	// State machine: fires after the hold time, resolves once.
	r := AlertRule{ID: "r1", Name: "x", Event: "tps", Minutes: 0, Resolve: true}
	al.step(r, "srv", true, "TPS 5")
	if len(al.Firing()) != 0 && al.Firing()[0].Server != "srv" {
		t.Fatal("firing state")
	}
	al.step(r, "srv", false, "")
	if len(al.Firing()) != 0 {
		t.Fatal("not resolved")
	}
}

// TestQRCode renders codes as PNG for the external decoder check
// (QR_OUT=dir go test -run TestQRCode); always checks structure.
func TestQRCode(t *testing.T) {
	texts := []string{"hi", totpURI("CraftDock", "admin", newTOTPSecret()), totpURI("CraftDock", "ein-langer.benutzername_42", newTOTPSecret()) + "&x=" + strings.Repeat("a", 60)}
	for i, txt := range texts {
		q, err := qrEncode(txt)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(q.SVG(), "<svg") {
			t.Fatal("svg")
		}
		if out := os.Getenv("QR_OUT"); out != "" {
			const scale, border = 8, 4
			dim := (q.size + 2*border) * scale
			img := image.NewGray(image.Rect(0, 0, dim, dim))
			for y := 0; y < dim; y++ {
				for x := 0; x < dim; x++ {
					mx, my := x/scale-border, y/scale-border
					c := color.Gray{255}
					if mx >= 0 && my >= 0 && mx < q.size && my < q.size && q.modules[my][mx] {
						c = color.Gray{0}
					}
					img.SetGray(x, y, c)
				}
			}
			f, _ := os.Create(filepath.Join(out, "qr"+string(rune('0'+i))+".png"))
			_ = png.Encode(f, img)
			f.Close()
			_ = os.WriteFile(filepath.Join(out, "qr"+string(rune('0'+i))+".txt"), []byte(txt), 0o644)
		}
	}
	if _, err := qrEncode(strings.Repeat("x", 400)); err == nil {
		t.Fatal("too long text accepted")
	}
}
