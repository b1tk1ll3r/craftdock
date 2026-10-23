package main

// /healthz for load balancers and uptime monitors, /metrics in Prometheus
// text format (enabled with CRAFTDOCK_METRICS_TOKEN, sent as Bearer token).

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

func (a *API) healthz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxT(r, 3*time.Second)
	defer cancel()
	_, err := a.m.d.Info(ctx)
	status := 200
	out := map[string]any{"status": "ok", "docker": err == nil}
	if err != nil {
		status, out["status"] = http.StatusServiceUnavailable, "degraded"
	}
	writeJSON(w, status, out)
}

func promLabel(s string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`).Replace(s)
}

func (a *API) metrics(w http.ResponseWriter, r *http.Request) {
	token := os.Getenv("CRAFTDOCK_METRICS_TOKEN")
	got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if token == "" || subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
		http.NotFound(w, r)
		return
	}
	ctx, cancel := ctxT(r, 20*time.Second)
	defer cancel()
	var b strings.Builder
	metric := func(name, help, typ string) {
		fmt.Fprintf(&b, "# HELP %s %s\n# TYPE %s %s\n", name, help, name, typ)
	}
	metric("craftdock_info", "CraftDock version", "gauge")
	fmt.Fprintf(&b, "craftdock_info{version=%q} 1\n", promLabel(appVersion))
	_, derr := a.m.d.Info(ctx)
	metric("craftdock_docker_up", "Docker daemon reachable", "gauge")
	fmt.Fprintf(&b, "craftdock_docker_up %d\n", b2i(derr == nil))

	cs, err := a.m.d.ListContainers(ctx, labelManaged+"=true")
	if err == nil {
		sort.Slice(cs, func(i, j int) bool { return configFromLabels(cs[i].Labels).Name < configFromLabels(cs[j].Labels).Name })
		type row struct {
			name string
			lv   Live
			run  bool
			own  string
		}
		var rows []row
		for i := range cs {
			cfg := configFromLabels(cs[i].Labels)
			rw := row{name: cfg.Name, run: cs[i].State == "running", own: cfg.Owner}
			if rw.run {
				rw.lv = a.m.Live(ctx, &cs[i], 90*time.Second)
			}
			rows = append(rows, rw)
		}
		series := []struct {
			name, help string
			val        func(row) (float64, bool)
		}{
			{"craftdock_server_running", "Container is running", func(r row) (float64, bool) { return float64(b2i(r.run)), true }},
			{"craftdock_server_reachable", "Server answers Server List Pings", func(r row) (float64, bool) { return float64(b2i(r.lv.Reachable)), true }},
			{"craftdock_server_sleeping", "Server is paused by auto-pause", func(r row) (float64, bool) { return float64(b2i(r.lv.Sleeping)), true }},
			{"craftdock_server_players_online", "Players online", func(r row) (float64, bool) { return float64(r.lv.Online), r.lv.Reachable }},
			{"craftdock_server_players_max", "Player slots", func(r row) (float64, bool) { return float64(r.lv.Max), r.lv.Reachable }},
			{"craftdock_server_tps", "Ticks per second (Paper family)", func(r row) (float64, bool) {
				if r.lv.TPS == nil {
					return 0, false
				}
				return *r.lv.TPS, true
			}},
			{"craftdock_server_ping_latency_ms", "Server List Ping latency", func(r row) (float64, bool) { return float64(r.lv.LatencyMs), r.lv.Reachable }},
		}
		for _, s := range series {
			metric(s.name, s.help, "gauge")
			for _, rw := range rows {
				if v, ok := s.val(rw); ok {
					fmt.Fprintf(&b, "%s{server=%q,owner=%q} %g\n", s.name, promLabel(rw.name), promLabel(rw.own), v)
				}
			}
		}
	}
	if a.alerts != nil {
		metric("craftdock_alerts_firing", "Currently firing alerts", "gauge")
		fmt.Fprintf(&b, "craftdock_alerts_firing %d\n", len(a.alerts.Firing()))
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	_, _ = w.Write([]byte(b.String()))
}

func b2i(v bool) int {
	if v {
		return 1
	}
	return 0
}
