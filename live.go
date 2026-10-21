package main

// Live status (Server List Ping + RCON) with caching, and a background
// sampler that records player counts for the public dashboards.

import (
	"context"
	"log"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Live struct {
	ContainerID string   `json:"-"`
	Running     bool     `json:"running"`
	Sleeping    bool     `json:"sleeping"`
	Reachable   bool     `json:"reachable"`
	Version     string   `json:"version,omitempty"`
	MOTD        string   `json:"motd,omitempty"`
	Favicon     string   `json:"favicon,omitempty"`
	Online      int      `json:"online"`
	Max         int      `json:"max"`
	Names       []string `json:"names"`
	LatencyMs   int      `json:"latencyMs"`
	TPS         *float64 `json:"tps,omitempty"`
	StartedAt   string   `json:"startedAt,omitempty"`
	Checked     int64    `json:"checked"`
}

type liveEntry struct {
	mu   sync.Mutex
	val  Live
	at   time.Time
	addr string // last address that answered the ping
}

func (m *Manager) entry(name string) *liveEntry {
	m.liveMu.Lock()
	defer m.liveMu.Unlock()
	if m.live == nil {
		m.live = map[string]*liveEntry{}
	}
	e, ok := m.live[name]
	if !ok {
		e = &liveEntry{}
		m.live[name] = e
	}
	return e
}

var tpsRe = regexp.MustCompile(`:\s*\*?([0-9]+(?:\.[0-9]+)?)`)

func hasTPS(t string) bool { return t == "PAPER" || t == "PURPUR" || t == "SPIGOT" }

// Live returns the cached status of a server, refreshing it if older than maxAge.
func (m *Manager) Live(ctx context.Context, c *ContainerSummary, maxAge time.Duration) Live {
	cfg := configFromLabels(c.Labels)
	running := c.State == "running"
	e := m.entry(cfg.Name)
	e.mu.Lock()
	defer e.mu.Unlock()
	if time.Since(e.at) < maxAge && e.val.ContainerID == c.ID && e.val.Running == running {
		return e.val
	}
	v := Live{ContainerID: c.ID, Running: running, Names: []string{}, LatencyMs: -1, Checked: time.Now().Unix()}
	if running {
		ins, err := m.d.Inspect(ctx, c.ID)
		if err == nil {
			v.StartedAt = ins.State.StartedAt
		}
		// A paused server must not be pinged: the knock would wake it up.
		if cfg.IdleMode == "pause" {
			if paused, err := m.isPaused(ctx, c.ID); err == nil && paused {
				v.Sleeping = true
				m.setSleeping(cfg.Name, true)
				e.val, e.at = v, time.Now()
				return v
			}
		}
		m.setSleeping(cfg.Name, false)
		if st, addr := m.ping(ctx, ins, cfg, e.addr); st != nil {
			e.addr = addr
			v.Reachable = true
			v.Version, v.MOTD, v.Favicon = st.Version, st.MOTD, st.Favicon
			v.Online, v.Max, v.LatencyMs = st.Online, st.Max, st.LatencyMs
			v.Names = append(v.Names, st.Sample...)
			if st.Online > len(st.Sample) || st.Online == 0 {
				if out, err := m.rcon(ctx, c.ID, "list"); err == nil {
					if pl := parsePlayers(out); pl.Online == st.Online && len(pl.Names) >= len(v.Names) {
						v.Names = pl.Names
					}
				}
			}
			if hasTPS(cfg.Type) {
				if out, err := m.rcon(ctx, c.ID, "tps"); err == nil {
					if mm := tpsRe.FindStringSubmatch(out); mm != nil {
						if f, err := strconv.ParseFloat(mm[1], 64); err == nil {
							v.TPS = &f
						}
					}
				}
			}
		}
	}
	e.val, e.at = v, time.Now()
	return v
}

// ping tries the last working address, the container IPs and finally the
// published host port (via CRAFTDOCK_PING_HOST / host.docker.internal / localhost).
func (m *Manager) ping(ctx context.Context, ins ContainerInspect, cfg ServerConfig, last string) (*SLPStatus, string) {
	var cands []string
	if last != "" {
		cands = append(cands, last)
	}
	for _, n := range ins.NetworkSettings.Networks {
		if n.IPAddress != "" {
			cands = append(cands, net.JoinHostPort(n.IPAddress, "25565"))
		}
	}
	port := strconv.Itoa(cfg.Port)
	if h := os.Getenv("CRAFTDOCK_PING_HOST"); h != "" {
		cands = append(cands, net.JoinHostPort(h, port))
	}
	cands = append(cands, net.JoinHostPort("host.docker.internal", port), net.JoinHostPort("127.0.0.1", port))
	seen := map[string]bool{}
	for _, a := range cands {
		if seen[a] {
			continue
		}
		seen[a] = true
		pctx, cancel := context.WithTimeout(ctx, 1200*time.Millisecond)
		st, err := slpPing(pctx, a)
		cancel()
		if err == nil {
			return st, a
		}
	}
	return nil, ""
}

func parsePlayers(out string) PlayerList {
	pl := PlayerList{Names: []string{}}
	mm := listRe.FindStringSubmatch(strings.ReplaceAll(out, "\n", " "))
	if mm == nil {
		return pl
	}
	pl.Online, _ = strconv.Atoi(mm[1])
	pl.Max, _ = strconv.Atoi(mm[2])
	for _, n := range strings.Split(mm[3], ",") {
		if n = strings.TrimSpace(n); n != "" {
			pl.Names = append(pl.Names, n)
		}
	}
	return pl
}

// RunSampler records the player count of every server once per interval.
func (m *Manager) RunSampler(ctx context.Context, interval time.Duration) {
	m.sampleEvery = interval
	t := time.NewTicker(interval)
	defer t.Stop()
	saveEvery := 5
	for i := 0; ; i++ {
		existing := m.sampleOnce(ctx)
		if i%saveEvery == saveEvery-1 && existing != nil {
			m.store.SaveHistory(existing)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (m *Manager) sampleOnce(ctx context.Context) map[string]bool {
	lctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	cs, err := m.d.ListContainers(lctx, labelManaged+"=true")
	if err != nil {
		if ctx.Err() == nil {
			log.Printf("Sampler: %v", err)
		}
		return nil
	}
	existing := map[string]bool{}
	now := time.Now().Unix()
	for i := range cs {
		name := configFromLabels(cs[i].Labels).Name
		existing[name] = true
		p := -1
		var names []string
		if cs[i].State == "running" {
			lv := m.Live(lctx, &cs[i], 20*time.Second)
			switch {
			case lv.Sleeping:
				p = 0 // available, wakes on connect
			case lv.Reachable:
				p, names = lv.Online, lv.Names
			}
		}
		m.store.AddSample(name, HistPoint{T: now, P: p}, names, m.sampleEvery)
	}
	return existing
}

// isPaused reports whether the server's Java process is stopped (SIGSTOP by
// itzg's autopause). Reads /proc only, so it never wakes the server.
const pausedScript = `for p in /proc/[0-9]*; do
  case "$(tr '\000' ' ' < "$p/cmdline" 2>/dev/null)" in
    *java*) cut -d' ' -f3 "$p/stat"; exit 0;;
  esac
done
echo none`

func (m *Manager) isPaused(ctx context.Context, id string) (bool, error) {
	out, code, err := m.d.Exec(ctx, id, []string{"sh", "-c", pausedScript})
	if err != nil || code != 0 {
		return false, err
	}
	return strings.TrimSpace(out) == "T", nil
}

func (m *Manager) setSleeping(name string, v bool) {
	m.liveMu.Lock()
	defer m.liveMu.Unlock()
	if m.sleeping == nil {
		m.sleeping = map[string]bool{}
	}
	m.sleeping[name] = v
}

func (m *Manager) cachedSleeping(name string) bool {
	m.liveMu.Lock()
	defer m.liveMu.Unlock()
	return m.sleeping[name]
}
