package main

// Hosting rules for third parties: server ownership, quotas, host-port
// ranges and the settings users without "server.advanced" may not touch.

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type HostingSettings struct {
	PortMin       int    `json:"portMin"` // host ports for users without advanced rights
	PortMax       int    `json:"portMax"`
	Registration  string `json:"registration"` // "off" | "open" – invitations always work
	DefaultRole   string `json:"defaultRole"`  // for open registration
	DefaultQuota  Quota  `json:"defaultQuota"`
	MaxServerMB   int    `json:"maxServerMB"`   // max. Java heap per server (restricted users), 0 = none
	OverheadMB    int    `json:"overheadMB"`    // container limit = heap + overhead (restricted users)
	PanelURL      string `json:"panelUrl"`      // public URL of the panel, used in links and alerts
	Require2FAAll bool   `json:"require2faAll"` // every account must use 2FA
	// CustomerPublic lets restricted users show their servers on the public
	// page (off: the public page stays reserved for the operator's servers).
	CustomerPublic bool `json:"customerPublic"`
}

func defaultHosting() HostingSettings {
	return HostingSettings{Registration: "off", DefaultRole: "customer", DefaultQuota: Quota{Servers: 1, MemoryMB: 4096, CPUs: 2}, OverheadMB: 768}
}

func (h *HostingSettings) Validate(roles *Roles) error {
	h.PanelURL = strings.TrimRight(strings.TrimSpace(h.PanelURL), "/")
	if h.Registration == "" {
		h.Registration = "off"
	}
	switch {
	case (h.PortMin == 0) != (h.PortMax == 0):
		return badRequest("Portbereich: Start und Ende angeben (oder beide leer lassen)")
	case h.PortMin != 0 && (h.PortMin < 1024 || h.PortMax > 65535 || h.PortMin > h.PortMax):
		return badRequest("Portbereich: 1024–65535, Start ≤ Ende")
	case h.Registration != "off" && h.Registration != "open":
		return badRequest("Ungültiger Registrierungsmodus")
	case roles.Get(h.DefaultRole) == nil:
		return badRequest("Unbekannte Standardrolle")
	case h.DefaultRole == "admin":
		return badRequest("Neue Konten dürfen keine Administratoren werden")
	case h.MaxServerMB < 0 || h.OverheadMB < 0 || h.OverheadMB > 16384:
		return badRequest("Ungültige Speichergrenzen")
	case h.PanelURL != "" && !strings.HasPrefix(h.PanelURL, "https://") && !strings.HasPrefix(h.PanelURL, "http://"):
		return badRequest("Panel-Adresse muss mit http(s):// beginnen")
	case h.DefaultQuota.Servers < 0 || h.DefaultQuota.MemoryMB < 0 || h.DefaultQuota.CPUs < 0:
		return badRequest("Ungültige Standardquote")
	}
	return nil
}

func (h HostingSettings) inRange(p int) bool {
	return h.PortMin != 0 && p >= h.PortMin && p <= h.PortMax
}

// Usage sums what a user owns.
type Usage struct {
	Servers  int     `json:"servers"`
	MemoryMB int     `json:"memoryMB"`
	CPUs     float64 `json:"cpus"`
}

func (m *Manager) Usage(owner, except string) Usage {
	var u Usage
	if owner == "" {
		return u
	}
	for _, c := range m.store.Configs() {
		if c.Owner != owner || c.Name == except {
			continue
		}
		u.Servers++
		u.MemoryMB += c.heapMB()
		u.CPUs += c.CPUs
	}
	return u
}

// usedHostPorts collects host ports of all servers and networks.
func (m *Manager) usedHostPorts(ctx context.Context, except string) map[int]bool {
	used := map[int]bool{}
	add := func(c ServerConfig) {
		if c.Name == except {
			return
		}
		for _, p := range c.hostPorts() {
			n, _ := strconv.Atoi(strings.SplitN(p, "/", 2)[0])
			used[n] = true
		}
	}
	for _, c := range m.store.Configs() {
		add(c)
	}
	if cs, err := m.d.ListContainers(ctx, labelManaged+"=true"); err == nil {
		for _, c := range cs {
			add(configFromLabels(c.Labels))
		}
	}
	for _, pc := range m.proxyConfigs() {
		used[pc.Port] = true
	}
	return used
}

func (m *Manager) freePort(ctx context.Context, h HostingSettings, except string, taken map[int]bool) (int, error) {
	used := m.usedHostPorts(ctx, except)
	for p := h.PortMin; p <= h.PortMax && p > 0; p++ {
		if !used[p] && !taken[p] {
			return p, nil
		}
	}
	return 0, conflict("Im Portbereich %d–%d ist kein Port mehr frei", h.PortMin, h.PortMax)
}

// Environment variables restricted users may not set: they would bypass the
// memory policy or break CraftDock's management of the container.
var restrictedEnv = set("MEMORY", "INIT_MEMORY", "MAX_MEMORY", "JVM_OPTS", "JVM_XX_OPTS", "JVM_DD_OPTS",
	"RCON_PASSWORD", "ENABLE_RCON", "RCON_PORT", "SERVER_PORT", "UID", "GID", "EULA", "TYPE", "VERSION",
	"MODRINTH_MODPACK", "CF_API_KEY", "ENABLE_AUTOPAUSE", "ENABLE_AUTOSTOP")

type policyCtx struct {
	user      *User
	advanced  bool // may touch host-level settings
	assign    bool // may choose the owner (user managers with access to all servers)
	selfQuota bool
}

func (a *API) policyFor(u *User, server string) policyCtx {
	pc := policyCtx{user: u}
	if server == "" {
		pc.advanced = a.auth.Can(u, PermAdvanced)
	} else {
		pc.advanced = a.auth.CanServer(u, server, PermAdvanced)
	}
	pc.assign = a.auth.Can(u, PermUsers) && (u.AllServers || u.Role == "admin")
	return pc
}

// applyPolicy enforces ownership, quota and hosting rules on a configuration
// before it is created (old == nil) or applied.
func (a *API) applyPolicy(ctx context.Context, pc policyCtx, cfg *ServerConfig, old *ServerConfig) error {
	h := a.m.store.Hosting()
	u := pc.user

	// Ownership
	switch {
	case old != nil && !pc.assign:
		cfg.Owner = old.Owner
	case old == nil && !pc.assign:
		cfg.Owner = u.Username
	case cfg.Owner != "":
		cfg.Owner = strings.ToLower(strings.TrimSpace(cfg.Owner))
		if a.auth.Get(cfg.Owner) == nil {
			return badRequest("Besitzer %q existiert nicht", cfg.Owner)
		}
	}

	if !pc.advanced {
		if old != nil {
			cfg.DataPath = old.DataPath
			cfg.Network, cfg.ProxyMember = old.Network, old.ProxyMember
		} else {
			cfg.DataPath = ""
			if cfg.Network != "" {
				return &APIError{http.StatusForbidden, "Velocity-Netzwerke kann nur ein Administrator zuweisen"}
			}
		}
		env := map[string]string{}
		for k, v := range cfg.ExtraEnv {
			if restrictedEnv[strings.ToUpper(k)] {
				if old != nil && old.ExtraEnv[k] == v {
					env[k] = v // set by an administrator – keep it
					continue
				}
				return &APIError{http.StatusForbidden, fmt.Sprintf("Die Variable %s darf nur ein Administrator setzen", k)}
			}
			env[k] = v
		}
		cfg.ExtraEnv = env
		if !h.CustomerPublic {
			cfg.PublicHidden = true
		}

		// Host ports must come from the hosting range.
		portsChanged := old == nil || cfg.Port != old.Port || cfg.MapPort != old.MapPort || mustJSON(cfg.Ports) != mustJSON(old.Ports)
		if portsChanged {
			if h.PortMin == 0 {
				if old == nil {
					if len(cfg.Ports) > 0 || cfg.MapPort != 0 {
						return &APIError{http.StatusForbidden, "Zusätzliche Ports kann nur ein Administrator freigeben"}
					}
				} else if cfg.Port != old.Port || cfg.MapPort != old.MapPort || mustJSON(cfg.Ports) != mustJSON(old.Ports) {
					return &APIError{http.StatusForbidden, "Ports kann nur ein Administrator ändern"}
				}
			} else {
				taken := map[int]bool{}
				except := ""
				if old != nil {
					except = old.Name
				}
				if cfg.Port == 0 && !cfg.ProxyMember {
					p, err := a.m.freePort(ctx, h, except, taken)
					if err != nil {
						return err
					}
					cfg.Port = p
				}
				taken[cfg.Port] = true
				check := []int{}
				if !cfg.ProxyMember {
					check = append(check, cfg.Port)
				}
				if cfg.MapPort != 0 {
					check = append(check, cfg.MapPort)
				}
				for _, p := range cfg.Ports {
					check = append(check, p.Host)
				}
				for _, p := range check {
					if !h.inRange(p) {
						return badRequest("Host-Ports müssen zwischen %d und %d liegen (%d)", h.PortMin, h.PortMax, p)
					}
				}
			}
		}

		// Memory: container limit follows the heap so nobody exceeds their share.
		ccfg := *cfg
		ccfg.Normalize()
		heap := ccfg.heapMB()
		if h.MaxServerMB > 0 && heap > h.MaxServerMB {
			return badRequest("Pro Server sind höchstens %d MB Arbeitsspeicher erlaubt", h.MaxServerMB)
		}
		if heap > 0 {
			over := h.OverheadMB
			if q := heap / 4; q > over {
				over = q
			}
			if over < 256 {
				over = 256
			}
			cfg.MemoryLimitMB = heap + over
		}
	}

	// Quota – checked when users act on their own servers.
	if cfg.Owner != "" && cfg.Owner == u.Username && u.Role != "admin" {
		q := u.Quota
		except := ""
		if old != nil {
			except = old.Name
		}
		use := a.m.Usage(u.Username, except)
		ccfg := *cfg
		ccfg.Normalize()
		if q.CPUs > 0 && cfg.CPUs == 0 {
			cfg.CPUs = minF(1, q.CPUs-use.CPUs)
			ccfg.CPUs = cfg.CPUs
		}
		switch {
		case old == nil && q.Servers > 0 && use.Servers+1 > q.Servers:
			return &APIError{http.StatusForbidden, fmt.Sprintf("Deine Quote erlaubt %d Server – bitte zuerst einen anderen löschen", q.Servers)}
		case q.MemoryMB > 0 && use.MemoryMB+ccfg.heapMB() > q.MemoryMB:
			return &APIError{http.StatusForbidden, fmt.Sprintf("Deine Quote erlaubt insgesamt %d MB Arbeitsspeicher (frei: %d MB)", q.MemoryMB, max(0, q.MemoryMB-use.MemoryMB))}
		case q.CPUs > 0 && (ccfg.CPUs <= 0 || use.CPUs+ccfg.CPUs > q.CPUs+1e-9):
			return &APIError{http.StatusForbidden, fmt.Sprintf("Deine Quote erlaubt insgesamt %.1f CPU-Kerne (frei: %.1f)", q.CPUs, maxF(0, q.CPUs-use.CPUs))}
		}
	}
	return nil
}

func minF(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxF(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// ---- HTTP -----------------------------------------------------------------------------

func (a *API) getHosting(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, a.m.store.Hosting())
}

func (a *API) putHosting(w http.ResponseWriter, r *http.Request) {
	var h HostingSettings
	if err := readJSON(r, &h); err != nil {
		writeErr(w, err)
		return
	}
	if err := h.Validate(a.auth.roles); err != nil {
		writeErr(w, err)
		return
	}
	if err := a.m.store.SetHosting(h); err != nil {
		writeErr(w, err)
		return
	}
	a.auth.SetForce2FA(h.Require2FAAll)
	writeJSON(w, 200, h)
}

// openInvite turns the open-registration defaults into a pseudo invitation.
func (a *API) openInvite() *Invite {
	h := a.m.store.Hosting()
	if h.Registration != "open" {
		return nil
	}
	return &Invite{Role: h.DefaultRole, Quota: h.DefaultQuota, Expires: time.Now().Add(time.Hour), MaxUses: 1}
}
