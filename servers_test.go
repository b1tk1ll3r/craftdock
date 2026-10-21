package main

import (
	"strings"
	"testing"
)

func TestNormalizeAndValidate(t *testing.T) {
	c := ServerConfig{Name: " Survival ", Ops: "Steve, Alex\nNotch", Memory: "4g"}
	c.Normalize()
	if err := c.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Name != "survival" || c.Type != "VANILLA" || c.Version != "LATEST" || c.Port != 25565 || c.Memory != "4G" {
		t.Fatalf("defaults not applied: %+v", c)
	}
	if c.Ops != "Steve,Alex,Notch" {
		t.Fatalf("ops = %q", c.Ops)
	}

	bad := []ServerConfig{
		{Name: "Bad Name"},
		{Name: "ok", Type: "BUKKIT2"},
		{Name: "ok", Memory: "100M"},
		{Name: "ok", Memory: "2G", MemoryLimitMB: 2048},
		{Name: "ok", DataPath: "relative/path"},
		{Name: "ok", ExtraEnv: map[string]string{"bad-key": "x"}},
		{Name: "ok", Ports: []PortMapping{{Host: 1, Container: 25565, Protocol: "tcp"}}},
		{Name: "ok", Ops: "<script>"},
	}
	for i, b := range bad {
		b.Normalize()
		if b.Validate() == nil {
			t.Errorf("case %d: expected validation error for %+v", i, b)
		}
	}
}

func TestEnv(t *testing.T) {
	c := ServerConfig{Name: "s", Type: "PAPER", Whitelist: "Steve", OnlineMode: true,
		ExtraEnv: map[string]string{"USE_AIKAR_FLAGS": "true"}, RconPassword: "pw"}
	c.Normalize()
	env := strings.Join(c.Env(), "\n")
	for _, want := range []string{"EULA=TRUE", "TYPE=PAPER", "ONLINE_MODE=true", "ENABLE_WHITELIST=true",
		"WHITELIST=Steve", "USE_AIKAR_FLAGS=true", "RCON_PASSWORD=pw", "ENABLE_RCON=true"} {
		if !strings.Contains(env, want) {
			t.Errorf("env missing %s:\n%s", want, env)
		}
	}
	if strings.Contains(env, "SEED=") {
		t.Error("empty optional values must not be set")
	}
}

func TestPlayerListParsing(t *testing.T) {
	m := listRe.FindStringSubmatch("There are 2 of a max of 20 players online: Steve, Alex")
	if m == nil || m[1] != "2" || m[2] != "20" || strings.TrimSpace(m[3]) != "Steve, Alex" {
		t.Fatalf("parse failed: %v", m)
	}
	if listRe.FindStringSubmatch("There are 0 of a max of 10 players online:") == nil {
		t.Fatal("empty list not parsed")
	}
}

func TestFlattenChat(t *testing.T) {
	var v any = map[string]any{"text": "", "extra": []any{
		map[string]any{"text": "Hi ", "color": "green"}, "plain", map[string]any{"text": "!", "color": "#ff00aa"},
	}}
	if got := flattenChat(v); got != "§aHi §rplain§#ff00aa!§r" {
		t.Fatalf("flattenChat = %q", got)
	}
}

func TestModHelpers(t *testing.T) {
	if refSlug("lithium:mc1.21-0.13") != "lithium" || refSlug("datapack:terralith") != "terralith" {
		t.Fatal("refSlug")
	}
	if prettyJar("sodium-fabric-0.5.8+mc1.20.4.jar") != "sodium-fabric 0.5.8+mc1.20.4" {
		t.Fatalf("prettyJar = %q", prettyJar("sodium-fabric-0.5.8+mc1.20.4.jar"))
	}
	for _, bad := range []string{"../x.jar", ".hidden.jar", "a/b.jar", "x.zip"} {
		if jarNameRe.MatchString(bad) {
			t.Errorf("jar name %q accepted", bad)
		}
	}
	c := ServerConfig{Type: "AUTO_CURSEFORGE", Modpack: "https://www.curseforge.com/minecraft/modpacks/atm10", Version: "LATEST", CFApiKey: "k"}
	env := strings.Join(c.Env(), "\n")
	if !strings.Contains(env, "CF_PAGE_URL=") || !strings.Contains(env, "CF_API_KEY=k") || strings.Contains(env, "VERSION=") {
		t.Fatalf("curseforge env:\n%s", env)
	}
}

func TestRoutesRegister(t *testing.T) {
	d, _ := NewDocker("unix:///nonexistent")
	a := &API{m: NewManager(d, "", NewStore(t.TempDir())), auth: NewAuth("a", "b", t.TempDir())}
	_ = a.Routes() // panics on conflicting patterns
}
