package main

// Player management: lists from the server's JSON files (work while stopped)
// and actions through RCON.

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type OpEntry struct {
	Name  string `json:"name"`
	Level int    `json:"level"`
}

type BanEntry struct {
	Name    string `json:"name"`
	Reason  string `json:"reason"`
	Source  string `json:"source"`
	Created string `json:"created"`
	Expires string `json:"expires"`
}

type PlayerData struct {
	Running         bool         `json:"running"`
	Sleeping        bool         `json:"sleeping"`
	Online          []string     `json:"online"`
	Max             int          `json:"max"`
	WhitelistOn     bool         `json:"whitelistOn"`
	Whitelist       []string     `json:"whitelist"`
	Ops             []OpEntry    `json:"ops"`
	Banned          []BanEntry   `json:"banned"`
	Stats           []PlayerRank `json:"stats"`
	ConfigOps       []string     `json:"configOps"`
	ConfigWhitelist []string     `json:"configWhitelist"`
}

func splitList(s string) []string {
	out := []string{}
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func (m *Manager) readJSONFile(ctx context.Context, id, file string, v any) {
	b, err := m.d.ReadFile(ctx, id, "/data/"+file, 4<<20)
	if err == nil {
		_ = json.Unmarshal(b, v)
	}
}

func (m *Manager) PlayerData(ctx context.Context, name string) (PlayerData, error) {
	c, err := m.find(ctx, name)
	if err != nil {
		return PlayerData{}, err
	}
	cfg := configFromLabels(c.Labels)
	pd := PlayerData{
		Running: c.State == "running", Online: []string{}, Max: cfg.MaxPlayers, Whitelist: []string{},
		Ops: []OpEntry{}, Banned: []BanEntry{}, Stats: m.store.Players(name),
		ConfigOps: splitList(cfg.Ops), ConfigWhitelist: splitList(cfg.Whitelist),
	}
	var wl []struct {
		Name string `json:"name"`
	}
	m.readJSONFile(ctx, c.ID, "whitelist.json", &wl)
	for _, e := range wl {
		pd.Whitelist = append(pd.Whitelist, e.Name)
	}
	m.readJSONFile(ctx, c.ID, "ops.json", &pd.Ops)
	m.readJSONFile(ctx, c.ID, "banned-players.json", &pd.Banned)
	if b, err := m.d.ReadFile(ctx, c.ID, "/data/server.properties", 1<<20); err == nil {
		pd.WhitelistOn = strings.Contains(string(b), "\nwhite-list=true")
	}
	if pd.Running && cfg.IdleMode == "pause" {
		if paused, err := m.isPaused(ctx, c.ID); err == nil {
			pd.Sleeping = paused
		}
	}
	if pd.Running && !pd.Sleeping {
		if out, err := m.rcon(ctx, c.ID, "list"); err == nil {
			pl := parsePlayers(out)
			pd.Online, pd.Max = pl.Names, pl.Max
		}
	}
	return pd, nil
}

func (a *API) playerData(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxT(r, 20*time.Second)
	defer cancel()
	pd, err := a.m.PlayerData(ctx, r.PathValue("name"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, pd)
}

func (a *API) playerAction(w http.ResponseWriter, r *http.Request) {
	var in PlayerActionInput
	if err := readJSON(r, &in); err != nil {
		writeErr(w, err)
		return
	}
	ctx, cancel := ctxT(r, 20*time.Second)
	defer cancel()
	out, err := a.m.PlayerAction(ctx, r.PathValue("name"), in)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"output": out})
}

func (a *API) playerInfo(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxT(r, 20*time.Second)
	defer cancel()
	info, err := a.m.PlayerInfo(ctx, r.PathValue("name"), r.URL.Query().Get("player"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, info)
}
