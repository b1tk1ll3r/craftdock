# CraftDock

[![Docker image](https://github.com/b1tk1ll3r/craftdock/actions/workflows/docker.yml/badge.svg)](https://github.com/b1tk1ll3r/craftdock/actions/workflows/docker.yml)

Web-Launcher und Konfigurator für Minecraft-Server, die in Docker laufen – mit öffentlicher Server-Übersicht und Live-Dashboard pro Server. Ein einzelnes Go-Binary (nur Standardbibliothek) mit eingebetteter Weboberfläche, das über die Docker-Engine-API Container auf Basis von [`itzg/minecraft-server`](https://github.com/itzg/docker-minecraft-server) verwaltet.

| Adresse | Inhalt |
|---|---|
| `/` | Öffentliche Startseite mit allen öffentlichen Servern |
| `/s/<name>` | Öffentliches Live-Dashboard eines Servers |
| `/admin/` | Verwaltung (Login erforderlich) |
| `/map/<name>/` | BlueMap-Live-Karte eines Servers |
| `/widget`, `/widget/<name>` | Einbettbares Status-Widget (`?theme=light`, `?bg=transparent`) |
| `/badge/<name>.svg` | Status-Badge |

## Funktionen

### Verwaltung
- **Server anlegen** mit Vanilla, Paper, Purpur, Folia, Spigot, Fabric, Quilt, Forge, NeoForge – oder als komplettes **Modrinth- bzw. CurseForge-Modpack**
- **Starten, Stoppen, Neustarten** – Stoppen gibt dem Server 60 s zum Speichern
- **Live-Konsole** mit Log-Stream, RCON-Befehlen, Verlauf und Schnellbefehlen
- **Konfiguration** per Formular (Gameplay, Welt, Zugang, Ressourcen, Ports, Umgebungsvariablen, Darstellung auf der öffentlichen Seite). Speichern erstellt den Container neu, die Welt bleibt erhalten; lehnt Docker die neue Konfiguration ab, wird die alte wiederhergestellt
- **Datei-Editor**, **Backup** als `.tar.gz`, **Image-Update**

### Mods & Plugins
- **Modrinth-Suche** direkt im Browser, gefiltert nach Loader (Fabric, Quilt, Forge, NeoForge bzw. Paper/Purpur/Folia/Spigot für Plugins) und Minecraft-Version. Reine Client-Mods werden ausgeblendet.
- Hinzugefügte Projekte lädt der Container beim Start automatisch in der passenden Version (`MODRINTH_PROJECTS`) und hält sie aktuell; entfernte werden beim nächsten Start gelöscht.
- **Modpacks:** Plattform „Modrinth-Pack“ (Slug, ID oder Link) oder „CurseForge-Pack“ (Slug oder Link + [CurseForge-API-Key](https://console.curseforge.com/)). Loader, Version und Mods kommen aus dem Pack.
- **Eigene JARs** per Drag & Drop hochladen und löschen (`/data/mods` bzw. `/data/plugins`) – auch bei gestopptem Server, dann über einen kurzlebigen Hilfscontainer.

### Spieler, Welt & Betrieb
- **Spielerverwaltung per Klick:** Online-Spieler kicken, bannen, OP geben; Whitelist, Operatoren und Banns verwalten (Listen lesbar auch bei gestopptem Server). Spielzeit-Statistik mit „zuletzt online“ – minütlich erfasst.
- **Auto-Pause bei Leerlauf:** Pausiert den Server nach X Minuten ohne Spieler und weckt ihn beim nächsten Verbindungsversuch automatisch auf – oder stoppt den Container ganz. CraftDock erkennt den Schlafzustand, ohne den Server zu wecken.
- **Kompatibilitäts-Check vor Updates:** Vor einem Versions- oder Loader-Wechsel fragt CraftDock bei Modrinth nach, ob alle Mods/Plugins für die Zielversion existieren – auch jederzeit im Mods-Tab.
- **Server klonen:** Kopie mit Konfiguration, Mods und Welt (konsistent gespeichert), z. B. als Testserver für ein Update.
- **Welt importieren und zurücksetzen:** `.zip`/`.tar.gz` hochladen (Singleplayer-, Server- und Bukkit-Layouts mit Nether/End werden erkannt) oder die Welt mit neuem Seed/Welttyp neu erzeugen.

### Netzwerk, Karte & Community
- **Velocity-Proxy-Netzwerk:** Ein Proxy (`mc-velocity`, Image `itzg/mc-proxy`) bündelt die Server hinter einer Adresse, Wechsel im Spiel per `/server <name>`. Modern Forwarding mit geheimem Schlüssel; Paper, Purpur und Folia werden automatisch konfiguriert, Fabric/Quilt bekommen FabricProxy-Lite. Lobby und Forced Hosts (z. B. `survival.example.com`) einstellbar. Server im Netzwerk veröffentlichen keinen eigenen Port.
- **Live-Karte:** BlueMap wird automatisch installiert und unter `/map/<name>/` über CraftDock ausgeliefert (kein zusätzlicher Port nötig) und im öffentlichen Dashboard eingebettet.
- **Uptime-Statistik:** Verfügbarkeit für 24 h, 7 und 30 Tage plus Tagesbalken, Spitzenwerte und Bestenliste.
- **Neuigkeiten & Events:** Beiträge und Events (mit Countdown) für die Startseite und die Server-Dashboards.
- **Einbettbares Widget und Badge:** `/widget/<name>` bzw. `/widget` als iframe (hell/dunkel), `/badge/<name>.svg` für READMEs und Foren, CORS-fähige JSON-API.

### Benutzer & Rollen
| Rolle | Darf |
|---|---|
| Administrator | alles, inkl. Benutzer, Netzwerk und öffentliche Seite |
| Operator | Server anlegen/klonen, konfigurieren, Mods, Welt, Dateien, Backups, Spieler, Konsole, Neuigkeiten – nicht löschen |
| Moderator | Starten/Stoppen, Konsole, Spielerverwaltung, Neuigkeiten |
| Beobachter | nur ansehen (Status, Logs, Spieler) |

Benutzer lassen sich auf einzelne Server beschränken. Passwörter werden mit PBKDF2-SHA256 (600 000 Runden) gespeichert, Rollen- oder Passwortänderungen melden den Benutzer sofort ab, und jede ändernde Aktion landet im Protokoll. Der Account aus `CRAFTDOCK_USER`/`CRAFTDOCK_PASSWORD` ist immer Administrator.

### Öffentliche Seite & Dashboards
- Startseite mit Kennzahlen (Server online, Spieler, Mods) und animierten Server-Karten: Favicon, farbige MOTD, Spielerbalken, Adresse zum Kopieren
- Dashboard pro Server: Status, Adresse, Spieler (Ring-Anzeige), TPS (Paper/Purpur/Spigot), Antwortzeit, live tickende Laufzeit, Version, **Spielerverlauf der letzten 24 Stunden**, Spieler mit Avataren, Mod-/Plugin-Liste mit Icons und Links, Modpack-Karte, Server-Infos und eine Beitrittsanleitung passend zum Loader
- Echte Live-Daten über den **Minecraft Server List Ping** (dieselbe Abfrage wie im Mehrspieler-Menü), aktualisiert alle 10 Sekunden
- Pro Server einstellbar: öffentlich ja/nein, Anzeigename, Beschreibung, öffentliche Adresse. Global (Verwaltung → „Öffentliche Seite“): Titel, Untertitel, Hostname, Discord-Link, Spielernamen anzeigen, Seite komplett abschalten
- Respektiert `prefers-reduced-motion`, funktioniert auf dem Handy

## Schnellstart mit Docker Compose

```bash
echo "CRAFTDOCK_PASSWORD=ein-sicheres-passwort" > .env
docker compose up -d
```

Die Compose-Datei nutzt das fertige Image `ghcr.io/b1tk1ll3r/craftdock` (amd64 und arm64, z. B. Raspberry Pi). Zum lokalen Bauen `image:` durch `build: .` ersetzen und `docker compose up -d --build` ausführen.

Öffentliche Seite: `http://<host>:8080/` · Verwaltung: `http://<host>:8080/admin/` (Benutzer `admin`).

## Konfiguration

| Variable / Flag | Standard | Bedeutung |
|---|---|---|
| `CRAFTDOCK_ADDR` / `-addr` | `:8080` | Listen-Adresse |
| `CRAFTDOCK_USER` | `admin` | Benutzername |
| `CRAFTDOCK_PASSWORD` | – | Passwort. Fehlt es, steht ein temporäres Passwort im Log |
| `CRAFTDOCK_DATA` | `data` (`/data` im Image) | Ablage für Benutzer, Protokoll, Einstellungen, Neuigkeiten, Statistik |
| `DOCKER_HOST` / `-docker` | `unix:///var/run/docker.sock` | Docker-Endpunkt |
| `CRAFTDOCK_NETWORK` / `-network` | – | Docker-Netzwerk für neue Server (in der Compose-Datei: `minecraft`). Pflicht für das Velocity-Netzwerk |
| `CRAFTDOCK_PING_HOST` | `host.docker.internal` | Host für den Status-Ping über den veröffentlichten Port, falls die Container-IP nicht erreichbar ist |
| `CRAFTDOCK_CF_API_KEY` | – | Globaler CurseForge-API-Key (alternativ pro Server) |
| `TZ` | – | Wird an die Minecraft-Container weitergereicht |

### Live-Status erreichbar machen

CraftDock pingt jeden Server zuerst über dessen Container-IP, danach über `CRAFTDOCK_PING_HOST:<Host-Port>` und `127.0.0.1:<Host-Port>`. Mit der mitgelieferten Compose-Datei funktioniert beides: neue Server landen im Netzwerk `minecraft`, ältere werden über `host.docker.internal` erreicht. Läuft ein Server, ist aber (noch) nicht erreichbar, zeigen die Seiten „Startet“.

## Wie CraftDock Server verwaltet

- Jeder Server ist ein Container `mc-<name>` mit den Labels `craftdock.managed`, `craftdock.name` und `craftdock.config` (komplette Konfiguration als JSON); der Proxy entsprechend `mc-velocity`. Server-Zustand steckt vollständig in Docker; im Datenverzeichnis liegen Benutzer, Protokoll, Seiteneinstellungen, Neuigkeiten, Spielerverlauf (24 h minütlich) und Tagesstatistiken (90 Tage).
- Dateien, die ein Server vor dem Start braucht (BlueMap-`core.conf`, Velocity-Forwarding in `paper-global.yml`, `FabricProxy-Lite.toml`), schreibt CraftDock vor jedem Start; bei gestopptem Server über einen kurzlebigen Hilfscontainer.
- Weltdaten liegen im Volume `mc-<name>-data` oder in einem beim Anlegen angegebenen Host-Verzeichnis.
- Konsole, Spielerliste, TPS und Backup-Vorbereitung laufen über `rcon-cli` im Container – RCON muss nicht nach außen freigegeben werden.
- EULA wird automatisch akzeptiert (`EULA=TRUE`); mit dem Anlegen stimmst du der [Minecraft-EULA](https://www.minecraft.net/eula) zu.

## Sicherheit & Datenschutz

- Zugriff auf den Docker-Socket heißt faktisch Root auf dem Host: starkes Passwort, `/admin/` nicht ungeschützt ins Internet stellen, idealerweise HTTPS über einen Reverse-Proxy (`X-Forwarded-Proto` wird für das `Secure`-Cookie ausgewertet).
- Die öffentliche API (`/api/public/…`) gibt nur Anzeigedaten heraus – keine Umgebungsvariablen, Pfade, RCON-Passwörter, API-Keys, Operatoren oder Zusatzports. Abfragen werden gecacht, damit Besucher den Docker-Daemon nicht belasten.
- Die öffentlichen Seiten laden Spieler-Avatare von `mc-heads.net` und Mod-Icons von `cdn.modrinth.com`. Spielernamen (auch in der Bestenliste) lassen sich in den Einstellungen ausblenden.
- Die Live-Karte wird über CraftDock ausgeliefert; Cookies und Login-Daten werden dabei nie an BlueMap weitergereicht. Für nicht öffentliche Server ist sie nur angemeldeten Benutzern zugänglich.
- Beim Aktivieren der Live-Karte setzt CraftDock in BlueMaps `core.conf` `accept-download: true` – damit wird der Download der Minecraft-Client-Ressourcen gemäß Mojang-EULA akzeptiert.
- Hinweis zum Velocity-Netzwerk: Proxy und Server, die ihren Port weiter veröffentlichen, dürfen nicht denselben Host-Port nutzen (CraftDock prüft das).

## Releases & CI

Der Workflow `.github/workflows/docker.yml` prüft jeden Push und Pull Request (`gofmt`, `go vet`, `go test -race`) und baut danach das Image für `linux/amd64` und `linux/arm64`:

| Auslöser | Tags in `ghcr.io/b1tk1ll3r/craftdock` |
|---|---|
| Push auf `main` | `latest`, `main`, `sha-<kurz>` |
| Git-Tag `v1.2.3` | `1.2.3`, `1.2`, `1`, `sha-<kurz>` |
| Pull Request | wird nur gebaut, nicht veröffentlicht |

Die Versionsnummer aus dem Tag landet im Binary und erscheint in der Verwaltung. Zu jedem veröffentlichten Image wird eine Build-Provenance-Attestierung erzeugt (`gh attestation verify oci://ghcr.io/b1tk1ll3r/craftdock:latest -o b1tk1ll3r`).

Dependabot (`.github/dependabot.yml`) prüft montags Go-Module, die Basis-Images im Dockerfile und die verwendeten GitHub Actions.

## Entwicklung

```
main.go      Start, Flags, HTTP-Server, Sampler
docker.go    Docker-Engine-API-Client (Socket, Logs, Exec, Archive, Stats, Hilfscontainer)
servers.go   Server-Modell, Validierung, Container-Spezifikation, Hintergrundvorgänge
mods.go      Modrinth-Suche/-Metadaten, Modpacks, JAR-Dateien
slp.go       Minecraft Server List Ping
players.go   Spielerverwaltung
world.go     Klonen, Welt-Import und -Reset
bluemap.go   Vorbereitende Dateien (BlueMap, Forwarding) und Karten-Proxy
proxy.go     Velocity-Netzwerk
widget.go    Widget und Badge
live.go      Live-Status-Cache und Spielerverlauf
public.go    Öffentliche API
store.go     Einstellungen und Verlauf (JSON-Dateien)
api.go       REST-API, SSE-Log-Stream, Backup, Uploads
auth.go      Benutzer, Rollen, Sitzungen, Protokoll
web/admin/   Verwaltungsoberfläche
web/public/  Öffentliche Seiten
```

`go test ./...` führt die Unit-Tests aus.
