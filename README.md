# CraftDock

[![Docker image](https://github.com/b1tk1ll3r/craftdock/actions/workflows/docker.yml/badge.svg)](https://github.com/b1tk1ll3r/craftdock/actions/workflows/docker.yml)

Web-Launcher, Konfigurator und Hosting-Panel für Minecraft-Server, die in Docker laufen – für die eigene Community genauso wie für Hosting für Dritte: Kunden mit eigenen Servern und Quoten, frei definierbare Rollen, Zwei-Faktor-Anmeldung, Alarmierung, öffentliche Server-Übersicht und Live-Dashboard pro Server. Ein einzelnes Go-Binary (nur Standardbibliothek) mit eingebetteter Weboberfläche, das über die Docker-Engine-API Container auf Basis von [`itzg/minecraft-server`](https://github.com/itzg/docker-minecraft-server) verwaltet.

| Adresse | Inhalt |
|---|---|
| `/` | Öffentliche Startseite mit allen öffentlichen Servern |
| `/s/<name>` | Öffentliches Live-Dashboard eines Servers |
| `/admin/` | Verwaltung (Login erforderlich) |
| `/map/<name>/` | BlueMap-Live-Karte eines Servers |
| `/widget`, `/widget/<name>` | Einbettbares Status-Widget (`?theme=light`, `?bg=transparent`) |
| `/badge/<name>.svg` | Status-Badge |
| `/healthz` | Health-Check für Load-Balancer und Uptime-Monitore |
| `/metrics` | Prometheus-Metriken (mit `CRAFTDOCK_METRICS_TOKEN`) |

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
- **Spielerverwaltung per Klick:** Whitelist, Operatoren und Banns verwalten (Listen lesbar auch bei gestopptem Server). Spielzeit-Statistik mit „zuletzt online“ – minütlich erfasst.
- **Spieler-Aktionen:** Ein Klick auf einen Online-Spieler öffnet ein Fenster mit Live-Daten (Position, Dimension, Leben, Hunger, Level, Spielmodus) und Aktionen: Items geben (Suche über gängige Items mit deutschen Namen oder beliebige ID, auch von Mods) – optional mit Verzauberungen und Stufen (inkl. „Beste für dieses Item“; die Befehlssyntax wird automatisch an die Minecraft-Version angepasst, z. B. `minecraft:enchanted_book[stored_enchantments={"minecraft:unbreaking":3}]` ab 1.21.5), Teleport zu Spieler/Spieler holen/zu Koordinaten in jeder Dimension, Spielmodus, Heilen, Effekte, Erfahrung, private Nachricht, Kick/Bann mit Grund, OP, Inventar leeren. Alle Eingaben werden serverseitig geprüft, bevor daraus ein Befehl wird.
- **Auto-Pause bei Leerlauf:** Pausiert den Server nach X Minuten ohne Spieler und weckt ihn beim nächsten Verbindungsversuch automatisch auf – oder stoppt den Container ganz. CraftDock erkennt den Schlafzustand, ohne den Server zu wecken.
- **Kompatibilitäts-Check vor Updates:** Vor einem Versions- oder Loader-Wechsel fragt CraftDock bei Modrinth nach, ob alle Mods/Plugins für die Zielversion existieren – auch jederzeit im Mods-Tab.
- **Server klonen:** Kopie mit Konfiguration, Mods und Welt (konsistent gespeichert), z. B. als Testserver für ein Update.
- **Welt importieren und zurücksetzen:** `.zip`/`.tar.gz` hochladen (Singleplayer-, Server- und Bukkit-Layouts mit Nether/End werden erkannt) oder die Welt mit neuem Seed/Welttyp neu erzeugen.

### Netzwerk, Karte & Community
- **Mehrere Velocity-Netzwerke:** Jedes Netzwerk ist ein eigener Proxy (`mc-velocity-<kennung>`, Image `itzg/mc-proxy`) mit eigener Adresse, eigenem Port, eigener Lobby, eigenen Forced Hosts und eigenem Forwarding-Schlüssel – z. B. ein Hauptnetzwerk auf 25565 und ein Event-Netzwerk auf 25570. Wechsel im Spiel per `/server <name>`. Paper, Purpur und Folia werden automatisch konfiguriert, Fabric/Quilt bekommen FabricProxy-Lite. Ein Server gehört zu höchstens einem Netzwerk (ein Backend kann nur einen Schlüssel prüfen); Aufnehmen/Entfernen geht per Klick auf der Seite „Netzwerke“ oder in der Server-Konfiguration. Server im Netzwerk veröffentlichen keinen eigenen Port. Das Netzwerk aus Version 1.2 wird automatisch als „main“ übernommen (Container, Daten und Schlüssel bleiben).
- **Live-Karte:** BlueMap wird automatisch installiert und unter `/map/<name>/` über CraftDock ausgeliefert (kein zusätzlicher Port nötig) und im öffentlichen Dashboard eingebettet.
- **Uptime-Statistik:** Verfügbarkeit für 24 h, 7 und 30 Tage plus Tagesbalken, Spitzenwerte und Bestenliste.
- **Neuigkeiten & Events:** Beiträge und Events (mit Countdown) für die Startseite und die Server-Dashboards.
- **Einbettbares Widget und Badge:** `/widget/<name>` bzw. `/widget` als iframe (hell/dunkel), `/badge/<name>.svg` für READMEs und Foren, CORS-fähige JSON-API.

### Benutzer, Rollen & Mandanten
Jeder Benutzer hat eine **Rolle** – einen frei zusammenstellbaren Satz aus 20 Berechtigungen. Server-Berechtigungen (Sehen, Steuern, Konsole, Konfigurieren, Erweitert, Mods, Dateien, Welt, Backups, Löschen, Spieler verwalten, Spieler-Eingriffe) gelten

- für **alle Server** (Team-Mitglieder, Schalter „Rolle gilt für alle Server“),
- für **eigene Server** – jeder Server kann einem Benutzer gehören; Kunden sehen nur ihre eigenen Server,
- oder **pro Server** mit einer eigenen Rolle (z. B. „tom ist Moderator auf lena-survival“).

Panel-Berechtigungen (Server anlegen, Netzwerke, Neuigkeiten, Einstellungen, Alarmierung, Benutzer, Rollen, Audit-Log) gelten für die Verwaltung insgesamt.

| Eingebaute Rolle | Gedacht für |
|---|---|
| Administrator | alles (nicht veränderbar, mindestens einer bleibt immer erhalten) |
| Operator | betreibt alle Server, legt neue an, schreibt Neuigkeiten – löscht nicht |
| Moderator | Starten/Stoppen, Konsole (ohne `op`, `deop`, `stop`, `reload`), Spieler |
| Kunde | verwaltet eigene Server im Rahmen seiner Quote, ohne Host-Einstellungen |
| Betrachter | nur ansehen |

Eingebaute Rollen (außer Administrator) lassen sich anpassen, eigene kommen dazu. Pro Rolle: **Konsolen-Allow-/Denylist** (erstes Wort des Befehls) und **2FA-Pflicht**. Niemand kann eine Rolle oder einen Benutzer mit mehr Rechten anlegen, als er selbst hat.

- **Quoten pro Benutzer:** Anzahl Server, Summe Java-Speicher, Summe CPU-Kerne. Gilt für Server, die dem Benutzer gehören.
- **Einladungen:** Link mit Rolle, Quote und Ablaufdatum (ein- oder mehrfach verwendbar) – der Kunde legt sein Konto selbst an. Optional offene Registrierung mit Standardrolle und -quote.
- **Konten sperren:** keine Anmeldung mehr, Sitzungen enden sofort, Server laufen weiter.
- **Audit-Log** mit Filter nach Benutzer.

### Hosting-Regeln
Benutzer ohne „Erweiterte Einstellungen“ (z. B. Kunden) …
- bekommen automatisch einen freien **Host-Port aus einem festen Bereich** und können nur Ports daraus wählen,
- können keinen Datenpfad auf dem Host, kein Velocity-Netzwerk und keine Speicher-, JVM- oder RCON-Variablen setzen,
- bekommen ein **Container-RAM-Limit passend zum Heap** (Heap + Aufschlag) und optional eine Obergrenze pro Server,
- erscheinen nur auf der öffentlichen Seite, wenn du das erlaubst.

Einstellbar unter Verwaltung → „Hosting“, zusammen mit Registrierung, Standardquote, Panel-Adresse und 2FA-Pflicht für alle.

### Sicherheit der Anmeldung
- **Zwei-Faktor-Anmeldung (TOTP)** mit QR-Code für jede Authenticator-App, 10 Wiederherstellungscodes, Schutz gegen Wiederverwendung eines Codes. Pflicht pro Rolle oder für alle einstellbar – wer betroffen ist, muss 2FA einrichten, bevor er weiterarbeiten kann.
- **Login-Sperre:** 5 Fehlversuche pro Benutzer und IP → 15 Minuten, 20 pro IP → 30 Minuten.
- **Sitzungen** überstehen Neustarts (gespeichert als Hash), sind unter „Konto“ einsehbar und einzeln oder gesammelt abmeldbar; Admins können alle Sitzungen eines Benutzers beenden. Passwort- und Rollenwechsel melden andere Sitzungen ab.
- Passwörter mit PBKDF2-SHA256 (600 000 Runden).

### Alarmierung
- **Kanäle:** Discord, Slack/Mattermost, Webhook (JSON, optional Bearer-Token), ntfy, Gotify, Telegram, E-Mail (SMTP mit STARTTLS/TLS). Test-Knopf pro Kanal.
- **Regeln:** Server ausgefallen (Absturz oder antwortet nicht), Absturzschleife, Healthcheck „unhealthy“, niedrige TPS, hohe RAM-Auslastung, wenig Speicherplatz, Velocity-Netzwerk ausgefallen, fehlgeschlagener Vorgang, Angriff auf die Anmeldung, Admin-Anmeldung von neuer IP. Mit Schwellwert, Mindestdauer, Pause zwischen Meldungen, Serverauswahl und **Entwarnung**, sobald das Problem behoben ist. „Empfohlene Regeln“ legt sinnvolle Standards an.
- **Wartungsmodus** pro Server (Menü „⋮“) oder global – Alarme bleiben dann stumm. Absichtliche Stopps und Neustarts lösen nie einen Alarm aus.
- **Verlauf** mit Zustellstatus je Kanal.

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
| `CRAFTDOCK_USER` | `admin` | Erster Administrator – wird beim ersten Start angelegt |
| `CRAFTDOCK_PASSWORD` | – | Dessen Startpasswort. Fehlt es, steht ein temporäres Passwort im Log. Danach wird das Passwort in der Oberfläche verwaltet |
| `CRAFTDOCK_RESET_ADMIN` | – | `1` setzt Passwort (aus `CRAFTDOCK_PASSWORD`), 2FA und Sperre des Administrators zurück – z. B. nach Verlust des Handys. Danach wieder entfernen |
| `CRAFTDOCK_DATA` | `data` (`/data` im Image) | Ablage für Benutzer, Rollen, Sitzungen, Protokoll, Einstellungen, Server-Konfigurationen, Alarme, Statistik |
| `CRAFTDOCK_TRUSTED_PROXIES` | – | Reverse-Proxys, deren `X-Forwarded-For`/`-Proto` gelten: IPs, CIDRs, `private` oder `loopback`, z. B. `172.16.0.0/12`. Ohne Eintrag werden die Header ignoriert |
| `CRAFTDOCK_METRICS_TOKEN` | – | Aktiviert `/metrics`; Abruf mit `Authorization: Bearer <token>` |
| `CRAFTDOCK_LOG_MAX_SIZE` / `CRAFTDOCK_LOG_MAX_FILE` | `20m` / `5` | Log-Rotation der Minecraft- und Proxy-Container (`off` = Docker-Standard) |
| `DOCKER_HOST` / `-docker` | `unix:///var/run/docker.sock` | Docker-Endpunkt |
| `CRAFTDOCK_NETWORK` / `-network` | – | Docker-Netzwerk für neue Server (in der Compose-Datei: `minecraft`). Pflicht für Velocity-Netzwerke |
| `CRAFTDOCK_PING_HOST` | `host.docker.internal` | Host für den Status-Ping über den veröffentlichten Port, falls die Container-IP nicht erreichbar ist |
| `CRAFTDOCK_CF_API_KEY` | – | Globaler CurseForge-API-Key (alternativ pro Server) |
| `TZ` | – | Wird an die Minecraft-Container weitergereicht |

### Live-Status erreichbar machen

CraftDock pingt jeden Server zuerst über dessen Container-IP, danach über `CRAFTDOCK_PING_HOST:<Host-Port>` und `127.0.0.1:<Host-Port>`. Mit der mitgelieferten Compose-Datei funktioniert beides: neue Server landen im Netzwerk `minecraft`, ältere werden über `host.docker.internal` erreicht. Läuft ein Server, ist aber (noch) nicht erreichbar, zeigen die Seiten „Startet“.

## Wie CraftDock Server verwaltet

- Jeder Server ist ein Container `mc-<name>` mit den Labels `craftdock.managed`, `craftdock.name` und `craftdock.config` (komplette Konfiguration als JSON); der Proxy entsprechend `mc-velocity`. Zusätzlich spiegelt CraftDock jede Konfiguration nach `servers.json` im Datenverzeichnis: Wird ein Container außerhalb von CraftDock gelöscht, erscheint der Server als „Container fehlt“ und lässt sich mit einem Klick **wiederherstellen** (die Welt im Volume bleibt ohnehin erhalten).
- Container-Logs werden rotiert (20 MB × 5), damit Minecraft-Logs nicht die Platte füllen.
- Dateien, die ein Server vor dem Start braucht (BlueMap-`core.conf`, Velocity-Forwarding in `paper-global.yml`, `FabricProxy-Lite.toml`), schreibt CraftDock vor jedem Start; bei gestopptem Server über einen kurzlebigen Hilfscontainer.
- Weltdaten liegen im Volume `mc-<name>-data` oder in einem beim Anlegen angegebenen Host-Verzeichnis.
- Konsole, Spielerliste, TPS und Backup-Vorbereitung laufen über `rcon-cli` im Container – RCON muss nicht nach außen freigegeben werden.
- EULA wird automatisch akzeptiert (`EULA=TRUE`); mit dem Anlegen stimmst du der [Minecraft-EULA](https://www.minecraft.net/eula) zu.

## Sicherheit & Datenschutz

- Zugriff auf den Docker-Socket heißt faktisch Root auf dem Host: starkes Passwort, 2FA für Administratoren (Rolle → „2FA vorschreiben“), HTTPS über einen Reverse-Proxy. Dessen Adresse gehört in `CRAFTDOCK_TRUSTED_PROXIES` – nur dann werden `X-Forwarded-For` (Login-Sperre, Audit-Log) und `X-Forwarded-Proto` (`Secure`-Cookie, HSTS) ausgewertet.
- Kunden sehen keine Host-Details (Docker-Version, RAM des Hosts), keine fremden Server und keine Netzwerke, in denen sie keinen Server haben. Plugins und Mods laufen innerhalb des Containers des Kunden; die Grenze bilden Container, RAM-/CPU-Limits und die Hosting-Regeln.
- Sensible Dateien im Datenverzeichnis (`users.json`, `sessions.json`, `servers.json`, `alerts.json`, …) werden mit Rechten `0600` geschrieben. Geheimnisse von Alarm-Kanälen werden in der Oberfläche maskiert.
- Die öffentliche API (`/api/public/…`) gibt nur Anzeigedaten heraus – keine Umgebungsvariablen, Pfade, RCON-Passwörter, API-Keys, Operatoren oder Zusatzports. Abfragen werden gecacht, damit Besucher den Docker-Daemon nicht belasten.
- Die öffentlichen Seiten laden Spieler-Avatare von `mc-heads.net` und Mod-Icons von `cdn.modrinth.com`. Spielernamen (auch in der Bestenliste) lassen sich in den Einstellungen ausblenden.
- Die Live-Karte wird über CraftDock ausgeliefert; Cookies und Login-Daten werden dabei nie an BlueMap weitergereicht. Für nicht öffentliche Server ist sie nur angemeldeten Benutzern zugänglich.
- Beim Aktivieren der Live-Karte setzt CraftDock in BlueMaps `core.conf` `accept-download: true` – damit wird der Download der Minecraft-Client-Ressourcen gemäß Mojang-EULA akzeptiert.
- Hinweis zu Velocity-Netzwerken: Proxys und Server, die ihren Port veröffentlichen, dürfen keinen Host-Port doppelt nutzen (CraftDock prüft das).
- Angezeigt wird immer die Minecraft-Version, die tatsächlich läuft (Status-Ping, sonst zuletzt gesehene Version). Bei „LATEST“ auf Paper & Co. wird keine Version geraten, weil diese Plattformen der neuesten Mojang-Version oft hinterherhinken.

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
proxy.go     Velocity-Netzwerke
playeractions.go  Spieler-Aktionen (Items, Teleport, Effekte …) und Live-Spielerdaten
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
