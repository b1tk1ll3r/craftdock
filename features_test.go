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
	c := ServerConfig{Name: "s", Type: "FABRIC", IdleMode: "stop", RestartPolicy: "unless-stopped", Map: true, Network: "main", ModrinthProjects: "lithium"}
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

func f64(v float64) *float64 { return &v }

func TestPlayerCommands(t *testing.T) {
	ok := []struct {
		in   PlayerActionInput
		want string
	}{
		{PlayerActionInput{Action: "give", Player: "Steve", Item: "diamond_sword", Count: 2}, "give Steve minecraft:diamond_sword 2"},
		{PlayerActionInput{Action: "give", Player: "Steve", Item: "create:wrench", Count: 1}, "give Steve create:wrench 1"},
		{PlayerActionInput{Action: "tp-to", Player: "Steve", Target: "Alex"}, "tp Steve Alex"},
		{PlayerActionInput{Action: "tp-here", Player: "Steve", Target: "Alex"}, "tp Alex Steve"},
		{PlayerActionInput{Action: "tp-coords", Player: "Steve", X: f64(10.5), Y: f64(64), Z: f64(-3), Dimension: "the_nether"}, "execute in minecraft:the_nether run tp Steve 10.5 64 -3"},
		{PlayerActionInput{Action: "gamemode", Player: "Steve", Gamemode: "creative"}, "gamemode creative Steve"},
		{PlayerActionInput{Action: "effect", Player: "Steve", Effect: "speed", Seconds: 60, Amplifier: 1, Hide: true}, "effect give Steve minecraft:speed 60 1 true"},
		{PlayerActionInput{Action: "xp", Player: "Steve", Amount: 5}, "xp add Steve 5 levels"},
		{PlayerActionInput{Action: "message", Player: "Steve", Message: "Hallo\nWelt §cRot"}, "tell Steve Hallo Welt cRot"},
		{PlayerActionInput{Action: "ban", Player: "Steve", Reason: "Grief"}, "ban Steve Grief"},
		{PlayerActionInput{Action: "whitelist-on"}, "whitelist on"},
	}
	for _, tc := range ok {
		cmds, err := buildPlayerCommands(tc.in)
		if err != nil || cmds[0] != tc.want {
			t.Errorf("%s: got %v / %v, want %q", tc.in.Action, cmds, err, tc.want)
		}
	}
	bad := []PlayerActionInput{
		{Action: "give", Player: "Steve", Item: "diamond; op Evil", Count: 1},
		{Action: "give", Player: "Steve", Item: "diamond", Count: 0},
		{Action: "give", Player: "St eve", Item: "diamond", Count: 1},
		{Action: "tp-coords", Player: "Steve", X: f64(1), Y: f64(5000), Z: f64(1)},
		{Action: "tp-coords", Player: "Steve", X: f64(1), Z: f64(1)},
		{Action: "tp-to", Player: "Steve", Target: "Steve"},
		{Action: "gamemode", Player: "Steve", Gamemode: "god"},
		{Action: "effect", Player: "Steve", Effect: "speed", Seconds: 0},
		{Action: "message", Player: "Steve", Message: "  "},
		{Action: "op-all", Player: "Steve"},
	}
	for i, in := range bad {
		if _, err := buildPlayerCommands(in); err == nil {
			t.Errorf("bad case %d accepted: %+v", i, in)
		}
	}
	if !rconFailure("No player was found") || rconFailure("Gave 2 [Diamond Sword] to Steve") {
		t.Error("rconFailure")
	}
	if v, ok := nbtValue("Steve has the following entity data: [12.5d, 64.0d, -3.25d]"); !ok || v != "[12.5d, 64.0d, -3.25d]" {
		t.Error("nbtValue")
	}
}

func TestGameVersion(t *testing.T) {
	cases := map[string]string{
		"1.21.4": "1.21.4", "Paper 1.21.4": "1.21.4", "Purpur 1.21.10": "1.21.10", "1.20": "1.20",
		"Velocity 1.7.2-1.21.10": "1.21.10", "25w14a": "25w14a", "26.1": "26.1", "Unknown": "",
	}
	for in, want := range cases {
		if got := gameVersion(in); got != want {
			t.Errorf("gameVersion(%q) = %q, want %q", in, got, want)
		}
	}
	if !isProxyVersion("Velocity 1.7.2-1.21.10") || !isProxyVersion("BungeeCord 1.8.x-1.21.x") || isProxyVersion("Paper 1.21.4") {
		t.Error("isProxyVersion")
	}
}

func TestPingCandidatesAndLegacyNetwork(t *testing.T) {
	var ins ContainerInspect
	ins.NetworkSettings.Networks = map[string]struct {
		IPAddress string `json:"IPAddress"`
	}{"minecraft": {IPAddress: "172.18.0.5"}}
	normal := pingCandidates(ins, ServerConfig{Port: 25565})
	member := pingCandidates(ins, ServerConfig{Port: 25565, ProxyMember: true, Network: "main"})
	if len(member) != 1 || member[0] != "172.18.0.5:25565" || len(normal) < 3 {
		t.Fatalf("candidates: normal=%v member=%v", normal, member)
	}
	c := configFromLabels(map[string]string{labelConfig: `{"name":"lobby","type":"PAPER","proxyMember":true}`})
	if c.Network != legacyNetworkID || !c.ProxyMember {
		t.Fatalf("legacy migration: %+v", c)
	}
	n := ServerConfig{Name: "x", Network: "", ProxyMember: true}
	n.Normalize()
	if n.ProxyMember {
		t.Fatal("ProxyMember must follow Network")
	}
	if proxyContainerName("main") != "mc-velocity" || proxyContainerName("events") != "mc-velocity-events" {
		t.Fatal("proxy container names")
	}
}

func TestKnownVersion(t *testing.T) {
	s := NewStore(t.TempDir())
	m := &Manager{store: s}
	cfg := ServerConfig{Name: "a", Type: "PAPER", Version: "LATEST"}
	if v := m.knownVersion(cfg); v != "" {
		t.Fatalf("paper LATEST must be unknown before first start, got %q", v)
	}
	s.SetKnownVersion("a", cfg.versionFingerprint(), "1.21.4")
	if v := m.knownVersion(cfg); v != "1.21.4" {
		t.Fatalf("known = %q", v)
	}
	cfg.Type = "PURPUR"
	if v := m.knownVersion(cfg); v != "" {
		t.Fatalf("fingerprint change must reset, got %q", v)
	}
	if v := m.knownVersion(ServerConfig{Name: "b", Type: "PAPER", Version: "1.20.6"}); v != "1.20.6" {
		t.Fatalf("fixed version = %q", v)
	}
}

func TestEnchantments(t *testing.T) {
	cases := []struct {
		item, ver, want string
		ench            map[string]int
	}{
		{"enchanted_book", "1.21.10", `give Steve minecraft:enchanted_book[stored_enchantments={"minecraft:unbreaking":3}] 1`, map[string]int{"unbreaking": 3}},
		{"diamond_sword", "1.21.5", `give Steve minecraft:diamond_sword[enchantments={"minecraft:sharpness":5,"minecraft:unbreaking":3}] 1`, map[string]int{"unbreaking": 3, "sharpness": 5}},
		{"diamond_sword", "", `give Steve minecraft:diamond_sword[enchantments={"minecraft:sharpness":5}] 1`, map[string]int{"sharpness": 5}},
		{"diamond_sword", "26.1", `give Steve minecraft:diamond_sword[enchantments={"minecraft:sharpness":5}] 1`, map[string]int{"sharpness": 5}},
		{"diamond_sword", "1.21.4", `give Steve minecraft:diamond_sword[enchantments={levels:{"minecraft:sharpness":5}}] 1`, map[string]int{"sharpness": 5}},
		{"enchanted_book", "1.20.6", `give Steve minecraft:enchanted_book[stored_enchantments={levels:{"minecraft:mending":1}}] 1`, map[string]int{"mending": 1}},
		{"bow", "1.20.4", `give Steve minecraft:bow{Enchantments:[{id:"minecraft:infinity",lvl:1s}]} 1`, map[string]int{"infinity": 1}},
		{"enchanted_book", "1.19.4", `give Steve minecraft:enchanted_book{StoredEnchantments:[{id:"minecraft:mending",lvl:1s}]} 1`, map[string]int{"mending": 1}},
		{"diamond_pickaxe", "1.21.4", `give Steve minecraft:diamond_pickaxe 1`, nil},
	}
	for _, tc := range cases {
		cmds, err := buildPlayerCommands(PlayerActionInput{Action: "give", Player: "Steve", Item: tc.item, Count: 1, Enchantments: tc.ench, GameVersion: tc.ver})
		if err != nil || cmds[0] != tc.want {
			t.Errorf("%s @%s: got %v / %v\nwant %s", tc.item, tc.ver, cmds, err, tc.want)
		}
	}
	for _, bad := range []map[string]int{{"sharp ness": 1}, {"sharpness": 0}, {"sharpness": 256}, {`x"]}`: 1}} {
		if _, err := buildPlayerCommands(PlayerActionInput{Action: "give", Player: "Steve", Item: "stick", Count: 1, Enchantments: bad}); err == nil {
			t.Errorf("accepted %v", bad)
		}
	}
}
