package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestPasswordHash(t *testing.T) {
	h, err := hashPassword("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if !checkPassword(h, "correct horse battery") || checkPassword(h, "wrong") || checkPassword("garbage", "x") {
		t.Fatal("password check broken")
	}
}

func TestUsersAndRoles(t *testing.T) {
	dir := t.TempDir()
	a := NewAuth("admin", "builtin-pass", dir)
	if err := a.Create(UserInput{Username: "Mod", Role: "moderator", Password: "short"}); err == nil {
		t.Fatal("short password accepted")
	}
	if err := a.Create(UserInput{Username: "Mod", Role: "moderator", Password: "lang-genug-123", Servers: []string{"survival"}}); err != nil {
		t.Fatal(err)
	}
	if err := a.Create(UserInput{Username: "admin", Role: "viewer", Password: "lang-genug-123"}); err == nil {
		t.Fatal("builtin name accepted")
	}
	_, u, ok := a.Login("mod", "lang-genug-123")
	if !ok || u.Role != "moderator" {
		t.Fatal("login failed")
	}
	if !u.Can(PermConsole) || u.Can(PermConfigure) || u.Can(PermUsers) {
		t.Fatalf("moderator perms wrong: %v", u.Perms())
	}
	if !u.CanServer("survival") || u.CanServer("creative") {
		t.Fatal("server scope wrong")
	}
	// persisted
	b := NewAuth("admin", "builtin-pass", dir)
	if _, _, ok := b.Login("mod", "lang-genug-123"); !ok {
		t.Fatal("user not persisted")
	}
	if _, u, ok := b.Login("admin", "builtin-pass"); !ok || !u.Builtin || !u.Can(PermUsers) {
		t.Fatal("builtin admin")
	}
	if err := b.Delete("admin"); err == nil {
		t.Fatal("builtin deletable")
	}
}

func TestVelocityToml(t *testing.T) {
	pc := ProxyConfig{Motd: `Mein "Netz"`, MaxPlayers: 50, OnlineMode: true, Lobby: "lobby", ForcedHosts: map[string]string{"survival.example.com": "survival", "x.example.com": "gone"}}
	out := velocityToml(pc, []ServerConfig{{Name: "survival"}, {Name: "lobby"}})
	for _, want := range []string{`motd = "Mein \"Netz\""`, `"lobby" = "mc-lobby:25565"`, `try = ["lobby", "survival"]`, `"survival.example.com" = ["survival"]`, `player-info-forwarding-mode = "modern"`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s in\n%s", want, out)
		}
	}
	if strings.Contains(out, "x.example.com") {
		t.Error("forced host to non-member written")
	}
}

func TestEnvFeatures(t *testing.T) {
	c := ServerConfig{Name: "s", Type: "FABRIC", IdleMode: "stop", RestartPolicy: "unless-stopped", Map: true, ProxyMember: true, ModrinthProjects: "lithium"}
	c.Normalize()
	if c.RestartPolicy != "on-failure" || c.IdleMinutes != 10 {
		t.Fatalf("normalize: %+v", c)
	}
	env := strings.Join(c.Env(), "\n")
	for _, want := range []string{"ENABLE_AUTOSTOP=TRUE", "AUTOSTOP_TIMEOUT_EST=600", "MODRINTH_PROJECTS=lithium,bluemap,fabricproxy-lite", "ONLINE_MODE=false"} {
		if !strings.Contains(env, want) {
			t.Errorf("env missing %s:\n%s", want, env)
		}
	}
	if hp := c.hostPorts(); len(hp) != 0 {
		t.Errorf("proxy member must not publish ports: %v", hp)
	}
	v := ServerConfig{Name: "v", Type: "VANILLA", Map: true}
	v.Normalize()
	if v.Validate() == nil {
		t.Error("map on vanilla accepted")
	}
}

func writeZip(t *testing.T, files map[string]string) string {
	f, err := os.CreateTemp(t.TempDir(), "*.zip")
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for n, c := range files {
		w, _ := zw.Create(n)
		_, _ = w.Write([]byte(c))
	}
	zw.Close()
	f.Close()
	return f.Name()
}

func TestWorldImportMapping(t *testing.T) {
	cases := []struct {
		files map[string]string
		want  []string
	}{
		{map[string]string{"MyWorld/level.dat": "x", "MyWorld/region/r.0.0.mca": "r", "MyWorld/session.lock": "l", "__MACOSX/junk": "j"},
			[]string{"world/level.dat", "world/region/r.0.0.mca"}},
		{map[string]string{"level.dat": "x", "DIM-1/region/a.mca": "n"},
			[]string{"world/DIM-1/region/a.mca", "world/level.dat"}},
		{map[string]string{"srv/survival/level.dat": "x", "srv/survival_nether/level.dat": "n", "srv/survival_the_end/DIM1/x": "e", "srv/survival_the_end/level.dat": "e", "srv/plugins/a.jar": "p"},
			[]string{"world/level.dat", "world_nether/level.dat", "world_the_end/DIM1/x", "world_the_end/level.dat"}},
		{map[string]string{"../../evil/level.dat": "x"}, []string{"world/level.dat"}},
	}
	for i, tc := range cases {
		file := writeZip(t, tc.files)
		walk, closeFn, err := openArchive(file)
		if err != nil {
			t.Fatal(err)
		}
		mapping, err := worldMapping(walk, "world")
		if err != nil {
			t.Fatalf("case %d: %v", i, err)
		}
		var buf bytes.Buffer
		if err := writeWorldTar(walk, mapping, &buf); err != nil {
			t.Fatal(err)
		}
		closeFn()
		var got []string
		tr := tar.NewReader(&buf)
		for {
			h, err := tr.Next()
			if err == io.EOF {
				break
			}
			if h.Typeflag == tar.TypeReg {
				got = append(got, h.Name)
			}
		}
		sort.Strings(got)
		if strings.Join(got, ",") != strings.Join(tc.want, ",") {
			t.Errorf("case %d: got %v want %v", i, got, tc.want)
		}
	}
	empty := writeZip(t, map[string]string{"readme.txt": "x"})
	walk, closeFn, _ := openArchive(empty)
	defer closeFn()
	if _, err := worldMapping(walk, "world"); err == nil {
		t.Error("archive without level.dat accepted")
	}
	_ = filepath.Join
}

func TestUptimeStats(t *testing.T) {
	s := NewStore(t.TempDir())
	now := int64(1_800_000_000)
	for i := 0; i < 10; i++ {
		p := 3
		if i < 2 {
			p = -1
		}
		s.AddSample("x", HistPoint{T: now - int64(10-i)*60, P: p}, []string{"Steve"}, 60e9)
	}
	ranks := s.Players("x")
	if len(ranks) != 1 || ranks[0].Minutes != 10 {
		t.Fatalf("ranks %+v", ranks)
	}
}
