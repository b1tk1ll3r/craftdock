// CraftDock – a web launcher and configurator for Minecraft servers running in Docker.
package main

import (
	"context"
	"embed"
	"flag"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// appVersion is overridden at build time via -ldflags "-X main.appVersion=…".
var appVersion = "2.0.0"

//go:embed web
var webFS embed.FS

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	addr := flag.String("addr", envOr("CRAFTDOCK_ADDR", ":8080"), "Listen-Adresse der Weboberfläche")
	dockerHost := flag.String("docker", os.Getenv("DOCKER_HOST"), "Docker-Endpunkt (unix:///var/run/docker.sock oder tcp://host:2375)")
	network := flag.String("network", os.Getenv("CRAFTDOCK_NETWORK"), "optionales Docker-Netzwerk für neue Server")
	healthcheck := flag.Bool("healthcheck", false, "prüft /healthz der laufenden Instanz (für Docker-HEALTHCHECK) und beendet sich")
	flag.Parse()
	if *healthcheck {
		os.Exit(runHealthcheck(*addr))
	}

	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.SetPrefix("craftdock ")

	d, err := NewDocker(*dockerHost)
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if info, err := d.Info(ctx); err != nil {
		log.Printf("WARNUNG: %v – läuft der Docker-Daemon und ist der Socket eingebunden?", err)
	} else {
		log.Printf("Verbunden mit Docker %s (%s, %d CPUs)", info.ServerVersion, info.OperatingSystem, info.NCPU)
	}
	cancel()

	initTrustedProxies(os.Getenv("CRAFTDOCK_TRUSTED_PROXIES"))

	dataDir := envOr("CRAFTDOCK_DATA", "data")
	store := NewStore(dataDir)
	mgr := NewManager(d, *network, store)
	pctx, pcancel := context.WithTimeout(context.Background(), 10*time.Second)
	mgr.loadProxies(pctx)
	if _, err := mgr.List(pctx); err != nil { // fills the configuration mirror
		log.Printf("WARNUNG: Serverliste: %v", err)
	}
	pcancel()

	roles := NewRoles(dataDir)
	auth := NewAuth(dataDir, envOr("CRAFTDOCK_USER", "admin"), os.Getenv("CRAFTDOCK_PASSWORD"), os.Getenv("CRAFTDOCK_RESET_ADMIN") == "1", roles)
	auth.ownerOf = store.Owner
	auth.SetForce2FA(store.Hosting().Require2FAAll)
	alerts := NewAlerter(mgr, dataDir)
	auth.OnEvent = alerts.AuthEvent
	mgr.OnTaskFailed = func(name, action string, err error) {
		if strings.HasPrefix(name, "~proxy-") {
			alerts.Emit("task_failed", "", "Vorgang fehlgeschlagen – Netzwerk „"+strings.TrimPrefix(name, "~proxy-")+"“", action+": "+err.Error())
			return
		}
		alerts.Emit("task_failed", name, "Vorgang fehlgeschlagen – Server „"+name+"“", action+": "+err.Error())
	}

	adminFS, _ := fs.Sub(webFS, "web/admin")
	publicFS, _ := fs.Sub(webFS, "web/public")
	api := &API{m: mgr, auth: auth, alerts: alerts, admin: adminFS, public: publicFS}

	samplerCtx, stopSampler := context.WithCancel(context.Background())
	go mgr.RunSampler(samplerCtx, time.Minute)
	go alerts.Run(samplerCtx)
	srv := &http.Server{
		Addr:              *addr,
		Handler:           api.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		// No WriteTimeout: log streams and backups are long-lived responses.
	}

	go func() {
		log.Printf("CraftDock %s – öffentliche Seite auf %s/, Verwaltung auf %s/admin/", appVersion, *addr, *addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Print("Beende …")
	stopSampler()
	store.SaveHistory(nil)
	sctx, scancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer scancel()
	_ = srv.Shutdown(sctx)
}

// runHealthcheck queries /healthz of the local instance – the distroless image
// has no curl or wget.
func runHealthcheck(addr string) int {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return 1
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	c := &http.Client{Timeout: 4 * time.Second}
	resp, err := c.Get("http://" + net.JoinHostPort(host, port) + "/healthz")
	if err != nil {
		return 1
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}
