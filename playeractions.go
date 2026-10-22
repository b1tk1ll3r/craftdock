package main

// Typed player actions (give, teleport, gamemode, effects, …) that are turned
// into RCON commands. Every parameter is validated here – nothing the browser
// sends ends up in a command unchecked.

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

type PlayerActionInput struct {
	Action    string   `json:"action"`
	Player    string   `json:"player"`
	Reason    string   `json:"reason"`
	Target    string   `json:"target"`    // tp-to / tp-here
	Item      string   `json:"item"`      // give
	Count     int      `json:"count"`     // give
	X         *float64 `json:"x"`         // tp-coords
	Y         *float64 `json:"y"`         // tp-coords
	Z         *float64 `json:"z"`         // tp-coords
	Dimension string   `json:"dimension"` // tp-coords
	Gamemode  string   `json:"gamemode"`
	Effect    string   `json:"effect"`
	Seconds   int      `json:"seconds"`
	Amplifier int      `json:"amplifier"`
	Hide      bool     `json:"hide"` // hide effect particles
	Amount    int      `json:"amount"`
	Unit      string   `json:"unit"` // xp: levels | points
	Message   string   `json:"message"`
}

var (
	resourceIDRe = regexp.MustCompile(`^(?:[a-z0-9_.-]{1,40}:)?[a-z0-9_./-]{1,80}$`)
	gamemodeSet  = set("survival", "creative", "adventure", "spectator")
)

// cleanText removes line breaks and formatting codes and limits the length.
func cleanText(s string, max int) string {
	s = strings.TrimSpace(strings.NewReplacer("\n", " ", "\r", " ", "§", "", "\x00", "").Replace(s))
	if r := []rune(s); len(r) > max {
		s = string(r[:max])
	}
	return s
}

func fmtCoord(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

func namespaced(id string) string {
	if strings.Contains(id, ":") {
		return id
	}
	return "minecraft:" + id
}

// buildPlayerCommands returns the RCON command(s) for an action.
func buildPlayerCommands(in PlayerActionInput) ([]string, error) {
	p := in.Player
	needsPlayer := !strings.HasPrefix(in.Action, "whitelist-o") // whitelist-on / -off
	if needsPlayer && !playerRe.MatchString(p) {
		return nil, badRequest("Ungültiger Spielername")
	}
	target := func() (string, error) {
		if !playerRe.MatchString(in.Target) {
			return "", badRequest("Bitte einen gültigen Ziel-Spieler wählen")
		}
		if strings.EqualFold(in.Target, p) {
			return "", badRequest("Spieler und Ziel sind identisch")
		}
		return in.Target, nil
	}
	switch in.Action {
	case "kick", "ban":
		cmd := in.Action + " " + p
		if r := cleanText(in.Reason, 200); r != "" {
			cmd += " " + r
		}
		return []string{cmd}, nil
	case "pardon", "op", "deop":
		return []string{in.Action + " " + p}, nil
	case "whitelist-add":
		return []string{"whitelist add " + p}, nil
	case "whitelist-remove":
		return []string{"whitelist remove " + p}, nil
	case "whitelist-on", "whitelist-off":
		return []string{"whitelist " + strings.TrimPrefix(in.Action, "whitelist-")}, nil

	case "give":
		item := strings.ToLower(strings.TrimSpace(in.Item))
		if !resourceIDRe.MatchString(item) {
			return nil, badRequest("Ungültige Item-ID %q (z. B. diamond oder minecraft:diamond_sword)", in.Item)
		}
		if in.Count < 1 || in.Count > 6400 {
			return nil, badRequest("Anzahl: 1 bis 6400")
		}
		return []string{fmt.Sprintf("give %s %s %d", p, namespaced(item), in.Count)}, nil

	case "tp-to":
		t, err := target()
		if err != nil {
			return nil, err
		}
		return []string{fmt.Sprintf("tp %s %s", p, t)}, nil
	case "tp-here":
		t, err := target()
		if err != nil {
			return nil, err
		}
		return []string{fmt.Sprintf("tp %s %s", t, p)}, nil
	case "tp-coords":
		if in.X == nil || in.Y == nil || in.Z == nil {
			return nil, badRequest("Bitte X, Y und Z angeben")
		}
		x, y, z := *in.X, *in.Y, *in.Z
		for _, v := range []float64{x, z} {
			if math.IsNaN(v) || math.Abs(v) > 29_999_984 {
				return nil, badRequest("X und Z müssen innerhalb von ±29 999 984 liegen")
			}
		}
		if math.IsNaN(y) || y < -2048 || y > 2048 {
			return nil, badRequest("Y muss zwischen -2048 und 2048 liegen")
		}
		dim := strings.ToLower(strings.TrimSpace(in.Dimension))
		if dim == "" {
			dim = "overworld"
		}
		if !resourceIDRe.MatchString(dim) {
			return nil, badRequest("Ungültige Dimension")
		}
		return []string{fmt.Sprintf("execute in %s run tp %s %s %s %s", namespaced(dim), p, fmtCoord(x), fmtCoord(y), fmtCoord(z))}, nil

	case "gamemode":
		if !gamemodeSet[in.Gamemode] {
			return nil, badRequest("Ungültiger Spielmodus")
		}
		return []string{fmt.Sprintf("gamemode %s %s", in.Gamemode, p)}, nil

	case "effect":
		eff := strings.ToLower(strings.TrimSpace(in.Effect))
		if !resourceIDRe.MatchString(eff) {
			return nil, badRequest("Ungültiger Effekt")
		}
		if in.Seconds < 1 || in.Seconds > 1_000_000 {
			return nil, badRequest("Dauer: 1 bis 1 000 000 Sekunden")
		}
		if in.Amplifier < 0 || in.Amplifier > 255 {
			return nil, badRequest("Stärke: 0 bis 255")
		}
		return []string{fmt.Sprintf("effect give %s %s %d %d %t", p, namespaced(eff), in.Seconds, in.Amplifier, in.Hide)}, nil
	case "effect-clear":
		return []string{"effect clear " + p}, nil
	case "heal":
		return []string{
			fmt.Sprintf("effect give %s minecraft:instant_health 1 3 true", p),
			fmt.Sprintf("effect give %s minecraft:saturation 1 20 true", p),
		}, nil

	case "xp":
		if in.Amount == 0 || in.Amount < -100_000 || in.Amount > 100_000 {
			return nil, badRequest("Menge: -100 000 bis 100 000 (ohne 0)")
		}
		unit := in.Unit
		if unit != "points" {
			unit = "levels"
		}
		return []string{fmt.Sprintf("xp add %s %d %s", p, in.Amount, unit)}, nil

	case "message":
		msg := cleanText(in.Message, 256)
		if msg == "" {
			return nil, badRequest("Die Nachricht ist leer")
		}
		return []string{fmt.Sprintf("tell %s %s", p, msg)}, nil

	case "clear":
		return []string{"clear " + p}, nil
	case "kill":
		return []string{"kill " + p}, nil
	}
	return nil, badRequest("Unbekannte Aktion %q", in.Action)
}

// rconFailure recognises error answers that rcon-cli reports with exit code 0.
var rconFailures = []string{
	"No player was found", "No entity was found", "Unknown item", "Unknown effect", "Unknown dimension",
	"Incorrect argument", "Unknown or incomplete command", "Expected", "Invalid", "That player does not exist",
	"Kein Spieler", "Unbekannt",
}

func rconFailure(out string) bool {
	for _, f := range rconFailures {
		if strings.Contains(out, f) {
			return true
		}
	}
	return false
}

func (m *Manager) PlayerAction(ctx context.Context, name string, in PlayerActionInput) (string, error) {
	cmds, err := buildPlayerCommands(in)
	if err != nil {
		return "", err
	}
	id, err := m.runningContainer(ctx, name)
	if err != nil {
		return "", &APIError{http.StatusConflict, "Spieleraktionen gehen nur bei laufendem Server"}
	}
	var outs []string
	for _, cmd := range cmds {
		out, err := m.rcon(ctx, id, cmd)
		if err != nil {
			return "", err
		}
		if rconFailure(out) {
			return "", &APIError{http.StatusUnprocessableEntity, out}
		}
		if out != "" {
			outs = append(outs, out)
		}
	}
	return strings.Join(outs, "\n"), nil
}

// ---- live player info ---------------------------------------------------------------

type PlayerInfo struct {
	Name      string    `json:"name"`
	Pos       []float64 `json:"pos,omitempty"`
	Dimension string    `json:"dimension,omitempty"`
	Health    *float64  `json:"health,omitempty"`
	Food      *int      `json:"food,omitempty"`
	Level     *int      `json:"level,omitempty"`
	Gamemode  string    `json:"gamemode,omitempty"`
}

var (
	nbtValueRe = regexp.MustCompile(`has the following entity data: (.+)$`)
	nbtNumRe   = regexp.MustCompile(`-?[0-9]+(?:\.[0-9]+)?(?:E-?[0-9]+)?`)
	gameTypes  = []string{"survival", "creative", "adventure", "spectator"}
)

// nbtValue extracts the value of a `data get entity` answer.
func nbtValue(out string) (string, bool) {
	mm := nbtValueRe.FindStringSubmatch(strings.TrimSpace(out))
	if mm == nil {
		return "", false
	}
	return strings.TrimSpace(mm[1]), true
}

func nbtFloat(v string) (float64, bool) {
	n := nbtNumRe.FindString(v)
	if n == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(n, 64)
	return f, err == nil
}

func (m *Manager) PlayerInfo(ctx context.Context, name, player string) (PlayerInfo, error) {
	if !playerRe.MatchString(player) {
		return PlayerInfo{}, badRequest("Ungültiger Spielername")
	}
	id, err := m.runningContainer(ctx, name)
	if err != nil {
		return PlayerInfo{}, &APIError{http.StatusConflict, "Der Server läuft nicht"}
	}
	paths := []string{"Pos", "Dimension", "Health", "foodLevel", "XpLevel", "playerGameType"}
	res := make([]string, len(paths))
	var wg sync.WaitGroup
	for i, path := range paths {
		wg.Add(1)
		go func(i int, path string) {
			defer wg.Done()
			if out, err := m.rcon(ctx, id, fmt.Sprintf("data get entity %s %s", player, path)); err == nil {
				res[i] = out
			}
		}(i, path)
	}
	wg.Wait()
	info := PlayerInfo{Name: player}
	found := false
	if v, ok := nbtValue(res[0]); ok {
		for _, n := range nbtNumRe.FindAllString(v, 3) {
			if f, err := strconv.ParseFloat(n, 64); err == nil {
				info.Pos = append(info.Pos, math.Round(f*10)/10)
			}
		}
		found = len(info.Pos) == 3
	}
	if !found {
		return info, &APIError{http.StatusNotFound, player + " ist gerade nicht auf diesem Server"}
	}
	if v, ok := nbtValue(res[1]); ok {
		info.Dimension = strings.Trim(v, `"'`)
	}
	if v, ok := nbtValue(res[2]); ok {
		if f, ok := nbtFloat(v); ok {
			f = math.Round(f*10) / 10
			info.Health = &f
		}
	}
	for idx, dst := range map[int]**int{3: &info.Food, 4: &info.Level} {
		if v, ok := nbtValue(res[idx]); ok {
			if f, ok := nbtFloat(v); ok {
				n := int(f)
				*dst = &n
			}
		}
	}
	if v, ok := nbtValue(res[5]); ok {
		if f, ok := nbtFloat(v); ok && int(f) >= 0 && int(f) < len(gameTypes) {
			info.Gamemode = gameTypes[int(f)]
		}
	}
	return info, nil
}
