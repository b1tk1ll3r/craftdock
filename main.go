// CraftDock – a web launcher and configurator for Minecraft servers running in Docker.
package main

import (
	"context"
	"embed"
	"flag"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// appVersion is overridden at build time via -ldflags "-X main.appVersion=…".
var appVersion = "1.2.0"

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
	flag.Parse()

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

	user := envOr("CRAFTDOCK_USER", "admin")
	pass := os.Getenv("CRAFTDOCK_PASSWORD")
	if pass == "" {
		pass = randomHex(8)
		log.Printf("Kein CRAFTDOCK_PASSWORD gesetzt – temporäres Passwort für %q: %s", user, pass)
	}

	dataDir := envOr("CRAFTDOCK_DATA", "data")
	store := NewStore(dataDir)
	mgr := NewManager(d, *network, store)
	pctx, pcancel := context.WithTimeout(context.Background(), 5*time.Second)
	mgr.loadProxy(pctx)
	pcancel()
	adminFS, _ := fs.Sub(webFS, "web/admin")
	publicFS, _ := fs.Sub(webFS, "web/public")
	api := &API{m: mgr, auth: NewAuth(user, pass, dataDir), admin: adminFS, public: publicFS}

	samplerCtx, stopSampler := context.WithCancel(context.Background())
	go mgr.RunSampler(samplerCtx, time.Minute)
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
