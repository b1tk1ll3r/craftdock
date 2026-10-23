'use strict';

/* =====================================================================
   CraftDock WebUI – vanilla JS, no build step.
   ===================================================================== */

const $ = (s, el = document) => el.querySelector(s);
const $$ = (s, el = document) => [...el.querySelectorAll(s)];
const esc = (v) => String(v ?? '').replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));

/* ---------- Icons (stroke, 24px grid) ---------- */
const ICONS = {
  plus: '<path d="M12 5v14M5 12h14"/>',
  play: '<path d="M7 4.5v15l12.5-7.5z"/>',
  stop: '<rect x="6" y="6" width="12" height="12" rx="2"/>',
  restart: '<path d="M3 12a9 9 0 1 0 3-6.7L3 8"/><path d="M3 3v5h5"/>',
  more: '<circle cx="12" cy="5" r="1"/><circle cx="12" cy="12" r="1"/><circle cx="12" cy="19" r="1"/>',
  download: '<path d="M12 3v12M7 10l5 5 5-5M5 21h14"/>',
  trash: '<path d="M3 6h18M8 6V4h8v2M6 6l1 14h10l1-14M10 11v6M14 11v6"/>',
  terminal: '<path d="m4 17 6-5-6-5M12 19h8"/>',
  sliders: '<path d="M4 21v-7M4 10V3M12 21v-9M12 8V3M20 21v-5M20 12V3M1 14h6M9 8h6M17 16h6"/>',
  file: '<path d="M14 3H6a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9z"/><path d="M14 3v6h6"/>',
  grid: '<rect x="3" y="3" width="7" height="7" rx="1.5"/><rect x="14" y="3" width="7" height="7" rx="1.5"/><rect x="14" y="14" width="7" height="7" rx="1.5"/><rect x="3" y="14" width="7" height="7" rx="1.5"/>',
  copy: '<rect x="9" y="9" width="12" height="12" rx="2"/><path d="M5 15H4a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1h10a1 1 0 0 1 1 1v1"/>',
  users: '<path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 0 0-3-3.9M16 3.1a4 4 0 0 1 0 7.8"/>',
  cpu: '<rect x="6" y="6" width="12" height="12" rx="1.5"/><rect x="9.5" y="9.5" width="5" height="5"/><path d="M9 2v4M15 2v4M9 18v4M15 18v4M2 9h4M2 15h4M18 9h4M18 15h4"/>',
  memory: '<rect x="2" y="7" width="20" height="10" rx="1.5"/><path d="M6 17v3M10 17v3M14 17v3M18 17v3M6 10v4M10 10v4M14 10v4M18 10v4"/>',
  clock: '<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>',
  logout: '<path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4M16 17l5-5-5-5M21 12H9"/>',
  x: '<path d="M18 6 6 18M6 6l12 12"/>',
  check: '<path d="m5 12 5 5L20 7"/>',
  alert: '<path d="M12 9v4M12 17h.01"/><path d="M10.3 3.9 1.8 18a2 2 0 0 0 1.7 3h17a2 2 0 0 0 1.7-3L13.7 3.9a2 2 0 0 0-3.4 0z"/>',
  box: '<path d="M21 16V8l-9-5-9 5v8l9 5z"/><path d="M3.3 7 12 12l8.7-5M12 22V12"/>',
  send: '<path d="M22 2 11 13M22 2l-7 20-4-9-9-4z"/>',
  save: '<path d="M19 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11l5 5v11a2 2 0 0 1-2 2z"/><path d="M17 21v-8H7v8M7 3v5h8"/>',
  globe: '<circle cx="12" cy="12" r="9"/><path d="M3 12h18M12 3a14 14 0 0 1 0 18 14 14 0 0 1 0-18"/>',
  refresh: '<path d="M21 12a9 9 0 0 1-15 6.7L3 16M3 12a9 9 0 0 1 15-6.7L21 8"/><path d="M21 3v5h-5M3 21v-5h5"/>',
  menu: '<path d="M3 6h18M3 12h18M3 18h18"/>',
  server: '<rect x="3" y="3" width="18" height="8" rx="2"/><rect x="3" y="13" width="18" height="8" rx="2"/><path d="M7 7h.01M7 17h.01"/>',
  eraser: '<path d="m7 21-4.3-4.3a1 1 0 0 1 0-1.4l10-10a1 1 0 0 1 1.4 0l5.6 5.6a1 1 0 0 1 0 1.4L12 20.6M22 21H7M5 11l9 9"/>',
  arrowDown: '<path d="M12 5v14M5 12l7 7 7-7"/>',
  puzzle: '<path d="M19.4 13.5a2.5 2.5 0 0 0 0-3H18V7a1 1 0 0 0-1-1h-3.5V4.6a2.5 2.5 0 0 0-5 0V6H5a1 1 0 0 0-1 1v3.5h1.5a2.5 2.5 0 0 1 0 5H4V19a1 1 0 0 0 1 1h3.5v-1.5a2.5 2.5 0 0 1 5 0V20H17a1 1 0 0 0 1-1v-3.5z"/>',
  external: '<path d="M15 3h6v6M10 14 21 3M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/>',
  upload: '<path d="M12 21V9M7 14l5-5 5 5M5 3h14"/>',
  search: '<circle cx="11" cy="11" r="7"/><path d="m21 21-4.3-4.3"/>',
  undo: '<path d="M3 7v6h6"/><path d="M21 17a9 9 0 0 0-15-6.7L3 13"/>',
  map: '<path d="m3 6 6-3 6 3 6-3v15l-6 3-6-3-6 3z"/><path d="M9 3v15M15 6v15"/>',
  earth: '<circle cx="12" cy="12" r="9"/><path d="M3.6 9h16.8M3.6 15h16.8M11.5 3a17 17 0 0 0 0 18M12.5 3a17 17 0 0 1 0 18"/>',
  network: '<rect x="9" y="2" width="6" height="6" rx="1"/><rect x="2" y="16" width="6" height="6" rx="1"/><rect x="16" y="16" width="6" height="6" rx="1"/><path d="M12 8v4M5 16v-2h14v2"/>',
  news: '<path d="M4 4h13a1 1 0 0 1 1 1v14a2 2 0 0 0 2 2H6a2 2 0 0 1-2-2z"/><path d="M18 9h2a1 1 0 0 1 1 1v9a2 2 0 0 1-2 2M8 8h6M8 12h6M8 16h4"/>',
  shield: '<path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/>',
  user: '<circle cx="12" cy="8" r="4"/><path d="M4 21v-1a6 6 0 0 1 6-6h4a6 6 0 0 1 6 6v1"/>',
  code: '<path d="m16 18 6-6-6-6M8 6l-6 6 6 6"/>',
  clone: '<rect x="8" y="8" width="13" height="13" rx="2"/><path d="M4 16V5a1 1 0 0 1 1-1h11"/>',
  ban: '<circle cx="12" cy="12" r="9"/><path d="m5.7 5.7 12.6 12.6"/>',
  star: '<path d="m12 3 2.8 5.7 6.2.9-4.5 4.4 1 6.2-5.5-2.9-5.5 2.9 1-6.2L3 9.6l6.2-.9z"/>',
  door: '<path d="M14 3H6a1 1 0 0 0-1 1v16a1 1 0 0 0 1 1h8"/><path d="M10 12h11M18 9l3 3-3 3"/>',
  calendar: '<rect x="3" y="4" width="18" height="18" rx="2"/><path d="M16 2v4M8 2v4M3 10h18"/>',
  moon: '<path d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z"/>',
  key: '<circle cx="7.5" cy="15.5" r="4.5"/><path d="m10.7 12.3 9.8-9.8M17 6l3 3M14.5 8.5l2 2"/>',
  bell: '<path d="M6 8a6 6 0 1 1 12 0c0 7 3 9 3 9H3s3-2 3-9"/><path d="M10.3 21a1.94 1.94 0 0 0 3.4 0"/>',
  wrench: '<path d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z"/>',
  lock: '<rect x="4" y="11" width="16" height="10" rx="2"/><path d="M8 11V7a4 4 0 0 1 8 0v4"/>',
  phone: '<rect x="6" y="2" width="12" height="20" rx="2"/><path d="M11 18h2"/>',
  link: '<path d="M10 13a5 5 0 0 0 7.5.5l3-3a5 5 0 0 0-7-7l-1.7 1.7"/><path d="M14 11a5 5 0 0 0-7.5-.5l-3 3a5 5 0 0 0 7 7l1.7-1.7"/>',
  mail: '<rect x="3" y="5" width="18" height="14" rx="2"/><path d="m3 7 9 6 9-6"/>',
  send2: '<path d="m22 2-7 20-4-9-9-4z"/><path d="M22 2 11 13"/>',
  pause: '<rect x="6" y="4" width="4" height="16" rx="1"/><rect x="14" y="4" width="4" height="16" rx="1"/>',
};
const icon = (n) => `<svg class="i" viewBox="0 0 24 24" aria-hidden="true">${ICONS[n] || ''}</svg>`;

/* ---------- Static data ---------- */
const TYPES = [
  { id: 'VANILLA', label: 'Vanilla', short: 'VN', desc: 'Offizieller Server von Mojang' },
  { id: 'PAPER', label: 'Paper', short: 'PP', desc: 'Schnell, Bukkit/Spigot-Plugins' },
  { id: 'PURPUR', label: 'Purpur', short: 'PU', desc: 'Paper-Fork mit vielen Optionen' },
  { id: 'FOLIA', label: 'Folia', short: 'FO', desc: 'Paper mit regionalem Multithreading' },
  { id: 'SPIGOT', label: 'Spigot', short: 'SP', desc: 'Klassische Plugin-Plattform' },
  { id: 'FABRIC', label: 'Fabric', short: 'FB', desc: 'Leichtgewichtige Mods' },
  { id: 'QUILT', label: 'Quilt', short: 'QU', desc: 'Fabric-kompatible Mods' },
  { id: 'FORGE', label: 'Forge', short: 'FG', desc: 'Klassische Mod-Plattform' },
  { id: 'NEOFORGE', label: 'NeoForge', short: 'NF', desc: 'Moderner Forge-Nachfolger' },
  { id: 'MODRINTH', label: 'Modrinth-Pack', short: 'MR', desc: 'Komplettes Modpack von Modrinth' },
  { id: 'AUTO_CURSEFORGE', label: 'CurseForge-Pack', short: 'CF', desc: 'Modpack von CurseForge (API-Key)' },
];
const MODPACK_TYPES = ['MODRINTH', 'AUTO_CURSEFORGE'];
const typeOf = (id) => TYPES.find((t) => t.id === id) || { id, label: id, short: id.slice(0, 2) };

const PHASES = {
  online: 'Online', starting: 'Startet', busy: 'Beschäftigt', stopped: 'Gestoppt',
  unhealthy: 'Gestört', restarting: 'Neustart', paused: 'Pausiert', error: 'Fehler', sleeping: 'Schläft', missing: 'Container fehlt',
};
const TASKS = {
  create: 'Wird erstellt', update: 'Wird aktualisiert', delete: 'Wird gelöscht', start: 'Startet',
  stop: 'Stoppt', restart: 'Startet neu', 'update-image': 'Image-Update', clone: 'Wird geklont',
  reset: 'Neue Welt', import: 'Welt-Import', proxy: 'Proxy-Einrichtung', restore: 'Wird wiederhergestellt',
};
const RUNNING = ['online', 'starting', 'unhealthy', 'sleeping'];
const PROXY_TYPES = ['PAPER', 'PURPUR', 'FOLIA', 'FABRIC', 'QUILT'];
const MAP_TYPES = ['PAPER', 'PURPUR', 'FOLIA', 'SPIGOT', 'FABRIC', 'QUILT', 'FORGE', 'NEOFORGE', 'MODRINTH'];
/** Panel-wide permission of the logged-in user. */
const can = (p) => !!state.me && (state.me.perms || []).includes(p);
/** Permission on a server (the role the user holds there). */
const canS = (p, s = current()) => !!s && (s.perms || []).includes(p);
/** Permission the user will have on servers he creates himself. */
const canNew = (p) => !!state.me && (state.me.serverPerms || []).includes(p);
/** May touch host-level settings (ports, env, data path, network) of a server. */
const advFor = (s) => s ? canS('server.advanced', s) : canNew('server.advanced');

const SECTIONS = [
  { id: 'general', title: 'Allgemein', desc: 'Plattform, Version und Name des Servers', fields: [
    { k: 'name', label: 'Name', type: 'text', newOnly: true, placeholder: 'survival', hint: 'Kleinbuchstaben, Ziffern, Bindestriche – Container heißt mc-<name>' },
    { k: 'motd', label: 'MOTD', type: 'text', placeholder: 'Ein Minecraft-Server', hint: 'Nachricht in der Serverliste' },
    { k: 'type', label: 'Plattform', type: 'types', full: true },
    { k: 'modpack', label: 'Modpack', type: 'text', full: true, types: MODPACK_TYPES, placeholder: 'z. B. cobblemon-fabric oder https://www.curseforge.com/minecraft/modpacks/all-the-mods-10', hint: 'Slug, Projekt-ID oder Link des Modpacks. Mods, Loader und Version kommen aus dem Pack.' },
    { k: 'cfApiKey', label: 'CurseForge-API-Key', type: 'secret', full: true, types: ['AUTO_CURSEFORGE'], hint: 'Kostenlos unter console.curseforge.com. Leer lassen, um den gespeicherten bzw. globalen Key (CRAFTDOCK_CF_API_KEY) zu verwenden.' },
    { k: 'version', label: 'Minecraft-Version', type: 'version', hint: 'LATEST = neueste Release (bei Modpacks: vom Pack vorgegeben)' },
    { k: 'javaTag', label: 'Java-Image', type: 'select', options: [['latest', 'latest (empfohlen)'], ['stable', 'stable'], ['java25', 'Java 25'], ['java21', 'Java 21'], ['java17', 'Java 17'], ['java11', 'Java 11'], ['java8', 'Java 8']], hint: 'Tag von itzg/minecraft-server – sehr alte Versionen brauchen Java 8/11' },
  ] },
  { id: 'network', title: 'Netzwerk', desc: 'Erreichbarkeit von außen', fields: [
    { k: 'port', label: 'Host-Port', type: 'number', min: 1, max: 65535, hint: 'Port, unter dem Spieler verbinden (Standard 25565)' },
    { k: 'onlineMode', label: 'Online-Modus', type: 'toggle', hint: 'Mojang/Microsoft-Konten prüfen (empfohlen)' },
    { k: 'network', label: 'Velocity-Netzwerk', type: 'netselect', full: true, adv: true, types: PROXY_TYPES, hint: 'Im Netzwerk ist der Server nur über den Proxy erreichbar (der Host-Port wird nicht veröffentlicht, die Anmeldung prüft der Proxy). Wechsel im Spiel mit /server <name>. Ein Server kann nur in einem Netzwerk sein.' },
  ] },
  { id: 'gameplay', title: 'Gameplay', desc: 'Spielregeln für alle Spieler', fields: [
    { k: 'gamemode', label: 'Spielmodus', type: 'select', options: [['survival', 'Überleben'], ['creative', 'Kreativ'], ['adventure', 'Abenteuer'], ['spectator', 'Zuschauer']] },
    { k: 'difficulty', label: 'Schwierigkeit', type: 'select', options: [['peaceful', 'Friedlich'], ['easy', 'Einfach'], ['normal', 'Normal'], ['hard', 'Schwer']] },
    { k: 'maxPlayers', label: 'Max. Spieler', type: 'number', min: 1, max: 1000 },
    { k: 'viewDistance', label: 'Sichtweite (Chunks)', type: 'number', min: 2, max: 32 },
    { k: 'pvp', label: 'PvP', type: 'toggle', hint: 'Spieler können sich gegenseitig Schaden zufügen' },
    { k: 'hardcore', label: 'Hardcore', type: 'toggle', hint: 'Ein Leben – nach dem Tod Zuschauer' },
  ] },
  { id: 'world', title: 'Welt', desc: 'Wirkt nur, wenn eine neue Welt erzeugt wird', fields: [
    { k: 'seed', label: 'Seed', type: 'text', placeholder: 'zufällig' },
    { k: 'levelType', label: 'Welttyp', type: 'select', options: [['', 'Standard'], ['minecraft:normal', 'Normal'], ['minecraft:flat', 'Flachland'], ['minecraft:large_biomes', 'Große Biome'], ['minecraft:amplified', 'Zerklüftet']] },
  ] },
  { id: 'access', title: 'Zugang', desc: 'Operatoren und Whitelist', fields: [
    { k: 'ops', label: 'Operatoren', type: 'list', placeholder: 'Spielername oder UUID, einer pro Zeile' },
    { k: 'whitelist', label: 'Whitelist', type: 'list', placeholder: 'Leer lassen = keine Whitelist' },
    { k: 'enforceWhitelist', label: 'Whitelist erzwingen', type: 'toggle', hint: 'Nicht gelistete Spieler beim Neuladen kicken', full: true },
  ] },
  { id: 'resources', title: 'Ressourcen', desc: 'Speicher, CPU und Neustartverhalten', fields: [
    { k: 'memory', label: 'Java-Speicher (Heap)', type: 'text', placeholder: '4G', hint: 'z. B. 2G, 4G, 6144M' },
    { k: 'memoryLimitMB', label: 'Container-RAM-Limit (MB)', type: 'number', min: 0, adv: true, hint: '0 = kein hartes Limit; sonst ≥ Heap + 256 MB' },
    { k: 'cpus', label: 'CPU-Limit (Kerne)', type: 'number', min: 0, step: 0.5, hint: '0 = unbegrenzt' },
    { k: 'restartPolicy', label: 'Neustart-Richtlinie', type: 'select', options: [['unless-stopped', 'unless-stopped (empfohlen)'], ['always', 'always'], ['on-failure', 'on-failure'], ['no', 'no']] },
    { k: 'idleMode', label: 'Bei Leerlauf', type: 'select', options: [['', 'Weiterlaufen'], ['pause', 'Pausieren – wacht beim Verbinden auf'], ['stop', 'Container stoppen']], hint: 'Pausieren spart CPU und hält den Server sofort bereit. Stoppen gibt auch den RAM frei, der Server muss dann aber manuell gestartet werden.' },
    { k: 'idleMinutes', label: 'Leerlauf nach (Minuten)', type: 'number', min: 1, max: 1440, hint: 'Ohne Spieler online' },
  ] },
  { id: 'map', title: 'Live-Karte', desc: 'BlueMap: 3D-Karte der Welt im Browser, eingebunden ins öffentliche Dashboard', fields: [
    { k: 'map', label: 'Live-Karte aktivieren', type: 'toggle', full: true, types: MAP_TYPES, hint: 'Installiert BlueMap automatisch. Damit stimmst du zu, dass BlueMap die Minecraft-Client-Ressourcen herunterlädt (Mojang-EULA). Das erste Rendern dauert je nach Weltgröße eine Weile.' },
    { k: 'mapPort', label: 'Host-Port der Karte (optional)', type: 'number', min: 0, max: 65535, types: MAP_TYPES, hint: '0 = nur über CraftDock unter /map/<name>/ erreichbar' },
  ] },
  { id: 'public', title: 'Öffentliche Seite', desc: 'Wie der Server auf der öffentlichen Übersicht und seinem Dashboard erscheint', fields: [
    { k: 'publicHidden', label: 'Öffentlich anzeigen', type: 'toggle', invert: true, full: true, hint: 'Server auf der Startseite und unter /s/<name> zeigen' },
    { k: 'displayName', label: 'Anzeigename', type: 'text', placeholder: 'Survival-Welt' },
    { k: 'publicAddress', label: 'Öffentliche Adresse', type: 'text', placeholder: 'play.example.com', hint: 'Leer = Hostname aus den Einstellungen + Port' },
    { k: 'description', label: 'Beschreibung', type: 'textarea', full: true, placeholder: 'Worum geht es auf diesem Server? Regeln, Besonderheiten, Events …' },
  ] },
  { id: 'advanced', title: 'Erweitert', desc: 'Daten, zusätzliche Ports und Umgebungsvariablen', fields: [
    { k: 'owner', label: 'Besitzer (Kunde)', type: 'owner', hint: 'Der Besitzer verwaltet den Server mit seiner Rolle; der Server zählt auf seine Quote. Leer = Team-Server ohne Besitzer.' },
    { k: 'dataPath', label: 'Datenpfad auf dem Host', type: 'text', newOnly: true, adv: true, placeholder: 'leer = Docker-Volume mc-<name>-data', hint: 'Absoluter Pfad, z. B. /srv/minecraft/survival' },
    { k: 'icon', label: 'Server-Icon (URL)', type: 'text', placeholder: 'https://…/icon.png' },
    { k: 'ports', label: 'Zusätzliche Ports', type: 'ports', full: true, rangeOnly: true, hint: 'z. B. 19132/udp für Geyser oder 8100/tcp für BlueMap' },
    { k: 'extraEnv', label: 'Umgebungsvariablen', type: 'env', full: true, hint: 'Alle Optionen des Images itzg/minecraft-server, z. B. SPAWN_PROTECTION, ALLOW_FLIGHT, USE_AIKAR_FLAGS' },
  ] },
];

const DEFAULTS = {
  name: '', type: 'PAPER', version: 'LATEST', javaTag: 'latest', motd: '', port: 25565, onlineMode: true,
  gamemode: 'survival', difficulty: 'normal', maxPlayers: 20, viewDistance: 10, pvp: true, hardcore: false,
  seed: '', levelType: '', ops: '', whitelist: '', enforceWhitelist: false, memory: '4G', memoryLimitMB: 0,
  cpus: 0, restartPolicy: 'unless-stopped', modrinthProjects: '', icon: '', dataPath: '', extraEnv: {}, ports: [],
  modpack: '', cfApiKey: '', publicHidden: false, displayName: '', description: '', publicAddress: '',
  idleMode: '', idleMinutes: 10, map: false, mapPort: 0, network: '', owner: '',
};

/* ---------- State ---------- */
const state = {
  servers: [], selected: null, tab: 'overview', info: null, versions: null, dockerError: null,
  pollTimer: null, ovTimer: null, logES: null, lastPhase: {}, user: '',
  history: JSON.parse(sessionStorageGet('cd_hist') || '[]'), histIdx: -1, autoscroll: true,
  configDirty: false, fileDirty: false, currentFile: null,
};
function sessionStorageGet(k) { try { return sessionStorage.getItem(k); } catch { return null; } }
function sessionStorageSet(k, v) { try { sessionStorage.setItem(k, v); } catch { /* ignore */ } }
const current = () => state.servers.find((s) => s.name === state.selected);

/* ---------- API ---------- */
async function api(method, path, body) {
  const opt = { method, headers: { 'X-CraftDock': '1' }, credentials: 'same-origin' };
  if (body !== undefined) { opt.headers['Content-Type'] = 'application/json'; opt.body = JSON.stringify(body); }
  let res;
  try { res = await fetch(path, opt); } catch { throw new Error('CraftDock ist nicht erreichbar'); }
  if (res.status === 401 && !path.startsWith('/api/login') && path !== '/api/register') { showLogin(); throw new Error('Sitzung abgelaufen'); }
  const ct = res.headers.get('content-type') || '';
  const data = ct.includes('json') ? await res.json() : await res.text();
  if (!res.ok) {
    if (data && data.code === '2fa_setup' && state.me && !state.forcing2fa) forced2FA();
    throw new Error((data && data.error) || res.statusText);
  }
  return data;
}

/* ---------- Toasts / modals ---------- */
function toast(msg, kind = 'ok') {
  const el = document.createElement('div');
  el.className = `toast ${kind}`;
  el.innerHTML = `${icon(kind === 'err' ? 'alert' : 'check')}<div>${esc(msg)}</div>`;
  $('#toasts').appendChild(el);
  setTimeout(() => { el.classList.add('leave'); setTimeout(() => el.remove(), 250); }, kind === 'err' ? 6500 : 3500);
}

function openOverlay(inner, { drawer = false, onClose, sticky = false } = {}) {
  const ov = document.createElement('div');
  ov.className = 'overlay' + (drawer ? ' drawer-wrap' : '');
  ov.innerHTML = inner;
  const close = () => { ov.remove(); document.removeEventListener('keydown', onKey); onClose && onClose(); };
  const onKey = (e) => { if (e.key === 'Escape' && !sticky && ov === $('#modalRoot').lastElementChild) close(); };
  ov.addEventListener('mousedown', (e) => { if (e.target === ov && !sticky) close(); });
  document.addEventListener('keydown', onKey);
  $$('[data-close]', ov).forEach((b) => b.addEventListener('click', close));
  $('#modalRoot').appendChild(ov);
  return { el: ov, close };
}

function confirmDialog({ title, body, ok = 'Bestätigen', danger = false }) {
  return new Promise((resolve) => {
    let done = false;
    const m = openOverlay(`<div class="modal" role="dialog" aria-modal="true">
      <header><h2>${esc(title)}</h2><button class="btn ghost icon sm" data-close aria-label="Schließen">${icon('x')}</button></header>
      <div class="body">${body}</div>
      <footer><button class="btn" data-close>Abbrechen</button><button class="btn ${danger ? 'danger solid' : 'primary'}" data-ok>${esc(ok)}</button></footer>
    </div>`, { onClose: () => { if (!done) resolve(false); } });
    $('[data-ok]', m.el).addEventListener('click', () => { done = true; m.close(); resolve(true); });
    $('[data-ok]', m.el).focus();
  });
}

/* ---------- Formatting ---------- */
function fmtBytes(b) {
  if (!b) return '0 B';
  const u = ['B', 'KB', 'MB', 'GB', 'TB']; let i = 0;
  while (b >= 1024 && i < u.length - 1) { b /= 1024; i++; }
  return `${b.toFixed(b >= 100 || i === 0 ? 0 : 1).replace('.', ',')} ${u[i]}`;
}
function fmtUptime(iso) {
  if (!iso) return '—';
  let s = Math.max(0, (Date.now() - new Date(iso).getTime()) / 1000);
  const d = Math.floor(s / 86400); s -= d * 86400;
  const h = Math.floor(s / 3600); s -= h * 3600;
  const m = Math.floor(s / 60);
  if (d) return `${d} T ${h} Std`;
  if (h) return `${h} Std ${m} Min`;
  return `${m} Min`;
}
function fmtDate(iso) {
  if (!iso || iso.startsWith('0001')) return '—';
  return new Date(iso).toLocaleString('de-DE', { dateStyle: 'medium', timeStyle: 'short' });
}
function versionLabel(c) { return c.version === 'LATEST' ? 'neueste' : c.version === 'SNAPSHOT' ? 'Snapshot' : c.version; }
/** Version shown for a server: the one that really runs (if known), else the configured one. */
function runVersion(s) { return s.gameVersion || versionLabel(s.config); }
function versionDetail(s) {
  const c = s.config;
  if (!s.gameVersion) return c.version === 'LATEST' ? 'neueste – wird beim Start ermittelt' : versionLabel(c);
  return c.version === 'LATEST' || c.version === 'SNAPSHOT' ? `${s.gameVersion} (eingestellt: ${versionLabel(c)})` : s.gameVersion;
}
const netName = (id) => { const n = (state.networks || []).find((x) => x.id === id); return n ? (n.name || n.id) : id; };
function hostAddr(port) { return port === 25565 ? location.hostname : `${location.hostname}:${port}`; }
function phaseLabel(s) {
  if (s.phase === 'busy' && s.task) return TASKS[s.task.action] || 'Beschäftigt';
  if (s.task && s.task.error && !s.containerId) return 'Fehlgeschlagen';
  return PHASES[s.phase] || s.phase;
}
function phaseClass(s) { return s.task && s.task.error && !s.containerId ? 'error' : s.phase; }
function hue(str) { let h = 0; for (const c of str) h = (h * 31 + c.charCodeAt(0)) % 360; return h; }

/* =====================================================================
   Boot / auth
   ===================================================================== */
async function boot() {
  $('#btnNew').innerHTML = `${icon('plus')}<span>Neuer Server</span>`;
  $('#btnLogout').innerHTML = `${icon('logout')}<span>Abmelden</span>`;
  $('#btnAccount').innerHTML = `${icon('user')}<span>Konto</span>`;
  $('#btnAccount').addEventListener('click', () => { closeNav(); openAccount(); });
  $('#btnNew').addEventListener('click', () => { closeNav(); openCreate(); });
  $('#btnLogout').addEventListener('click', async () => { try { await api('POST', '/api/logout'); } catch { /* */ } showLogin(); });
  $('#scrim').addEventListener('click', closeNav);
  $('#loginForm').addEventListener('submit', onLogin);
  $('#twofaForm').addEventListener('submit', onLogin2FA);
  $('#twofaBack').addEventListener('click', () => { $('#twofaForm').classList.add('hidden'); $('#loginForm').classList.remove('hidden'); });
  $('#regLink').addEventListener('click', (e) => { e.preventDefault(); showRegister(''); });
  window.addEventListener('hashchange', fromHash);
  window.addEventListener('beforeunload', (e) => { if (state.configDirty || state.fileDirty) { e.preventDefault(); e.returnValue = ''; } });

  const reg = /#register=([0-9a-f]+)/.exec(location.hash);
  try {
    const s = await api('GET', '/api/session');
    if (reg && !s.authenticated) { showRegister(reg[1]); return; }
    state.registration = s.registration;
    s.authenticated ? showApp(s) : showLogin();
  } catch { showLogin(); }
}

function showLogin() {
  stopTimers();
  closeLogs();
  $('#boot').classList.add('hidden');
  $('#app').classList.add('hidden');
  $('#login').classList.remove('hidden');
  $('#register').classList.add('hidden');
  $('#twofaForm').classList.add('hidden');
  $('#loginForm').classList.remove('hidden');
  $('#regLink').classList.toggle('hidden', state.registration !== 'open');
  setTimeout(() => $('#loginForm [name=password]').focus(), 30);
}

async function onLogin(e) {
  e.preventDefault();
  const f = e.target, btn = $('button', f), err = $('#loginError');
  btn.disabled = true; err.classList.add('hidden');
  try {
    const r = await api('POST', '/api/login', { username: f.username.value, password: f.password.value });
    f.password.value = '';
    if (r.twofa) {
      state.ticket = r.ticket;
      f.classList.add('hidden');
      $('#twofaForm').classList.remove('hidden');
      $('#twofaError').classList.add('hidden');
      $('#twofaForm [name=code]').value = '';
      setTimeout(() => $('#twofaForm [name=code]').focus(), 30);
      return;
    }
    showApp(r);
  } catch (ex) {
    err.textContent = ex.message; err.classList.remove('hidden');
  } finally { btn.disabled = false; }
}

async function onLogin2FA(e) {
  e.preventDefault();
  const f = e.target, btn = $('button[type=submit]', f), err = $('#twofaError');
  btn.disabled = true; err.classList.add('hidden');
  try {
    const r = await api('POST', '/api/login/2fa', { ticket: state.ticket, code: f.code.value.trim() });
    state.ticket = null;
    showApp(r);
  } catch (ex) {
    err.textContent = ex.message; err.classList.remove('hidden');
    if (/abgelaufen/.test(ex.message)) setTimeout(() => showLogin(), 1500);
  } finally { btn.disabled = false; }
}

async function showApp(me) {
  state.me = me;
  state.user = me.user;
  $('#register').classList.add('hidden');
  if (/#register=/.test(location.hash)) history.replaceState(null, '', '#/');
  $('#btnNew').classList.toggle('hidden', !can('servers.create'));
  renderSideNav();
  $('#boot').classList.add('hidden');
  $('#login').classList.add('hidden');
  $('#app').classList.remove('hidden');
  if (me.needs2fa) { forced2FA(); return; }
  loadInfo();
  loadNetworks().then(() => { renderSidebar(); updateHeader(true); });
  api('GET', '/api/versions').then((v) => { state.versions = v; fillVersionLists(); }).catch(() => {});
  state.booted = false;
  await poll();
  state.booted = true;
  fromHash(true);
  stopTimers();
  state.pollTimer = setInterval(poll, 3000);
  state.ovTimer = setInterval(() => { if (state.tab === 'overview' && !document.hidden) refreshOverview(); }, 5000);
}

function stopTimers() { clearInterval(state.pollTimer); clearInterval(state.ovTimer); }

async function loadInfo() {
  try { state.info = await api('GET', '/api/info'); } catch { state.info = null; }
  const el = $('#hostInfo'), i = state.info;
  if (!i) { el.innerHTML = ''; return; }
  if (!i.docker && i.dockerOk) { el.innerHTML = quotaHTML(); applyWidths(el); return; } // customers: no host details
  if (!i.dockerOk) {
    el.innerHTML = `<div class="row"><span class="dot error"></span><b>Docker nicht erreichbar</b></div>`;
    return;
  }
  const d = i.docker;
  el.innerHTML = `
    <div class="row"><span class="dot online"></span><b>Docker ${esc(d.ServerVersion)}</b></div>
    <div class="row">${icon('cpu')} ${d.NCPU} CPUs · ${fmtBytes(d.MemTotal)} RAM</div>
    <div class="row dim">CraftDock ${esc(i.version)} · ${esc(state.user)} (${esc(state.me.roleName || state.me.role)})</div>`;
}

/* =====================================================================
   Polling, routing, sidebar
   ===================================================================== */
async function poll() {
  try {
    state.servers = await api('GET', '/api/servers');
    if (state.dockerError) { state.dockerError = null; loadInfo(); renderMain(); }
  } catch (e) {
    if (e.message === 'Sitzung abgelaufen') return;
    if (!state.dockerError) { state.dockerError = e.message; renderMain(); }
    state.dockerError = e.message;
  }
  renderSidebar();
  if (!state.booted) return; // initial routing happens in showApp via fromHash
  if (state.page) return;
  if (state.selected && !current()) {
    // Server disappeared (deleted) – fall back.
    state.selected = null;
    setHash();
    renderMain();
    return;
  }
  if (!state.selected && state.servers.length && !$('#main').dataset.view) { select(state.servers[0].name); return; }
  if (!state.servers.length && $('#main').dataset.view !== 'empty' && !state.dockerError) { renderMain(); return; }
  updateHeader();
  handlePhaseChanges();
}

function handlePhaseChanges() {
  for (const s of state.servers) {
    const prev = state.lastPhase[s.name];
    if (prev && prev !== s.phase && s.name === state.selected) {
      if (state.tab === 'console' && RUNNING.includes(s.phase) && !state.logES) openLogs();
      if (state.tab === 'overview') refreshOverview();
      if (state.tab === 'files' && prev === 'busy') renderTab();
    }
    if (prev === 'busy' && s.phase !== 'busy' && s.task && s.task.error) toast(`${s.name}: ${s.task.error}`, 'err');
    if (prev === 'busy' && s.phase !== 'busy' && s.name === state.selected && state.tab === 'mods' && !state.modsDirty) renderTab();
    state.lastPhase[s.name] = s.phase;
  }
}

function setHash() {
  const h = state.page ? `#/_${state.page}` : state.selected ? `#/${state.selected}/${state.tab}` : '#/';
  if (location.hash !== h) history.replaceState(null, '', h);
}

function fromHash(initial) {
  const [, name, tab] = location.hash.split('/');
  if (name && name.startsWith('_') && PAGES[name.slice(1)] && pageAllowed(PAGES[name.slice(1)])) {
    if (state.page !== name.slice(1)) openPage(name.slice(1));
    return;
  }
  if (name && state.servers.some((s) => s.name === name)) {
    if (name !== state.selected || (tab && tab !== state.tab)) select(name, tab || 'overview');
  } else if (initial) {
    if (state.servers.length) select(state.servers[0].name); else renderMain();
  }
}

async function guardDirty() {
  if (!state.configDirty && !state.fileDirty && !state.modsDirty) return true;
  const ok = await confirmDialog({ title: 'Ungespeicherte Änderungen', body: '<p>Deine Änderungen gehen verloren. Trotzdem fortfahren?</p>', ok: 'Verwerfen', danger: true });
  if (ok) { state.configDirty = false; state.fileDirty = false; state.modsDirty = false; }
  return ok;
}

async function select(name, tab = 'overview') {
  if ((name !== state.selected || tab !== state.tab || state.page) && !(await guardDirty())) return;
  stopPageTimer();
  const wasPage = !!state.page;
  state.page = null;
  state.selected = name;
  if (wasPage) renderSideNav();
  state.tab = tab;
  setHash();
  closeNav();
  renderSidebar();
  renderMain();
}

function renderSidebar() {
  $('#serverCount').textContent = state.servers.length || '';
  const list = $('#serverList');
  if (!state.servers.length) {
    list.innerHTML = `<div class="side-empty">${state.dockerError ? 'Keine Verbindung zu Docker.' : 'Noch keine Server angelegt.'}</div>`;
    return;
  }
  list.innerHTML = state.servers.map((s) => {
    const c = s.config, t = typeOf(c.type);
    return `<button class="srv-item ${s.name === state.selected && !state.page ? 'active' : ''}" data-name="${esc(s.name)}" title="${esc(phaseLabel(s))}">
      <span class="dot ${phaseClass(s)}"></span>
      <span class="meta"><span class="nm">${esc(s.name)}</span><span class="sub">${esc(t.label)} ${esc(runVersion(s))} · ${c.network ? esc(netName(c.network)) : ':' + c.port}</span></span>
    </button>`;
  }).join('');
  $$('.srv-item', list).forEach((b) => b.addEventListener('click', () => select(b.dataset.name, state.selected === b.dataset.name ? state.tab : 'overview')));
}

function openNav() { $('#app').classList.add('nav-open'); }
function closeNav() { $('#app').classList.remove('nav-open'); }

/* =====================================================================
   Main view
   ===================================================================== */
function mobileBar() {
  return `<div class="mobile-bar"><button class="btn ghost icon" id="btnNav" aria-label="Menü">${icon('menu')}</button><div class="brand"><span class="logo"></span><span>CraftDock</span></div></div>`;
}

function renderMain() {
  const main = $('#main');
  closeLogs();
  if (state.page) { renderPage(); return; }
  const s = current();
  if (!s) {
    main.dataset.view = 'empty';
    const err = state.dockerError;
    main.innerHTML = `${mobileBar()}
      ${err ? `<div class="banner">${icon('alert')}<div><b>Docker ist nicht erreichbar.</b><br>${esc(err)}<br><span class="dim">Ist /var/run/docker.sock in den CraftDock-Container eingebunden?</span></div></div>` : ''}
      <div class="empty"><div class="box">
        <div class="art"><span class="logo"></span></div>
        <h2>${state.servers.length ? 'Server auswählen' : 'Willkommen bei CraftDock'}</h2>
        <p>${state.servers.length ? 'Wähle links einen Server aus.' : can('servers.create') ? 'Lege deinen ersten Minecraft-Server an. CraftDock lädt das Image, erstellt den Container und startet ihn – die Welt liegt sicher in einem eigenen Volume.' : 'Dir ist noch kein Server zugewiesen. Sobald dich jemand zu einem Server hinzufügt, erscheint er hier.'}</p>
        ${state.servers.length || !can('servers.create') ? '' : `<button class="btn primary" id="btnEmptyNew" ${err ? 'disabled' : ''}>${icon('plus')}Ersten Server erstellen</button>`}
      </div></div>`;
    $('#btnEmptyNew')?.addEventListener('click', () => openCreate());
    $('#btnNav')?.addEventListener('click', openNav);
    return;
  }
  main.dataset.view = 'server';
  const tabs = (s.missing ? [['overview', 'Übersicht', 'grid', 'server.view']] : [['overview', 'Übersicht', 'grid', 'server.view'], ['console', 'Konsole', 'terminal', 'server.view'], ['players', 'Spieler', 'users', 'server.view'],
    ['mods', 'Mods & Plugins', 'puzzle', 'server.view'], ['world', 'Welt', 'earth', ['server.world', 'server.backup']], ['config', 'Konfiguration', 'sliders', 'server.configure'], ['files', 'Dateien', 'file', 'server.files']])
    .filter((t) => [].concat(t[3]).some((p) => canS(p, s)));
  if (!tabs.some((t) => t[0] === state.tab)) state.tab = 'overview';
  main.innerHTML = `${mobileBar()}
    <header class="srv-head">
      <div class="srv-title">
        <div class="type-badge" id="hdBadge"></div>
        <div>
          <div class="line"><h1>${esc(s.name)}</h1><span id="hdPill"></span></div>
          <div class="chips" id="hdChips"></div>
        </div>
      </div>
      <div class="btn-group srv-actions" id="hdActions"></div>
    </header>
    <div id="hdTask"></div>
    <nav class="tabs" role="tablist">${tabs.map(([id, l, ic]) => `<button class="tab ${state.tab === id ? 'active' : ''}" data-tab="${id}" role="tab">${icon(ic)}${l}</button>`).join('')}</nav>
    <section class="tab-body" id="tabBody"></section>`;
  $('#btnNav')?.addEventListener('click', openNav);
  $$('.tab', main).forEach((b) => b.addEventListener('click', () => select(state.selected, b.dataset.tab)));
  updateHeader(true);
  renderTab();
}

function updateHeader(force) {
  const s = current();
  if (!s || !$('#hdPill')) return;
  const c = s.config, t = typeOf(c.type);
  const key = JSON.stringify([s.phase, s.task, c, s.health, s.containerId, s.gameVersion, (state.networks || []).length, s.maintenance]);
  if (!force && $('#main').dataset.hk === key) return;
  $('#main').dataset.hk = key;

  $('#hdBadge').textContent = t.short;
  $('#hdPill').innerHTML = `<span class="pill ${phaseClass(s)}"><span class="dot ${phaseClass(s)}"></span>${esc(phaseLabel(s))}</span>`;
  $('#hdChips').innerHTML = [
    `<span class="chip">${icon('box')}${esc(t.label)}</span>`,
    `<span class="chip" title="${esc(versionDetail(s))}">Minecraft ${esc(runVersion(s))}</span>`,
    c.network ? `<span class="chip">${icon('network')}${esc(netName(c.network))}</span>` : `<span class="chip">${icon('globe')}Port ${c.port}</span>`,
    c.map ? `<span class="chip">${icon('map')}Live-Karte</span>` : '',
    c.idleMode ? `<span class="chip">${icon('moon')}${c.idleMode === 'pause' ? 'Auto-Pause' : 'Auto-Stopp'} ${c.idleMinutes} Min</span>` : '',
    `<span class="chip">${icon('memory')}${esc(c.memory)}</span>`,
    c.owner && c.owner !== state.user ? `<span class="chip" title="Besitzer">${icon('user')}${esc(c.owner)}</span>` : '',
    s.maintenance ? `<span class="chip warn" title="Alarme stumm bis ${esc(fmtTime(s.maintenance))}">${icon('wrench')}Wartung bis ${esc(new Date(s.maintenance * 1000).toLocaleTimeString('de-DE', { hour: '2-digit', minute: '2-digit' }))}</span>` : '',
  ].join('');

  const busy = s.phase === 'busy', exists = !!s.containerId, running = RUNNING.includes(s.phase) || s.phase === 'restarting';
  const ctl = canS('server.control');
  $('#hdActions').innerHTML = s.missing ? `
    ${ctl ? `<button class="btn primary" data-act="restore" ${busy ? 'disabled' : ''}>${icon('refresh')}<span class="lbl">Wiederherstellen</span></button>` : ''}
    ${canS('server.delete') ? `<button class="btn danger" id="btnForget" ${busy ? 'disabled' : ''}>${icon('trash')}<span class="lbl">Entfernen</span></button>` : ''}` : `
    ${!ctl ? '' : running
      ? `<button class="btn" data-act="stop" ${busy ? 'disabled' : ''}>${icon('stop')}<span class="lbl">Stoppen</span></button>
         <button class="btn" data-act="restart" ${busy ? 'disabled' : ''}>${icon('restart')}<span class="lbl">Neustart</span></button>`
      : `<button class="btn primary" data-act="start" ${busy || !exists ? 'disabled' : ''}>${icon('play')}<span class="lbl">Starten</span></button>`}
    <div class="menu-wrap">
      <button class="btn icon" id="btnMore" aria-label="Weitere Aktionen" ${exists ? '' : 'disabled'}>${icon('more')}</button>
    </div>`;
  $$('[data-act]', $('#hdActions')).forEach((b) => b.addEventListener('click', () => doAction(b.dataset.act)));
  $('#btnMore')?.addEventListener('click', (e) => { e.stopPropagation(); toggleMenu(); });
  $('#btnForget')?.addEventListener('click', openDelete);

  const task = s.task;
  const tb = $('#hdTask');
  if (task && task.running) {
    tb.innerHTML = `<div class="taskbar running"><span class="spinner"></span><span class="msg">${esc(task.message)}</span></div>`;
  } else if (task && task.error) {
    tb.innerHTML = `<div class="taskbar failed">${icon('alert')}<span class="msg"><b>${esc(TASKS[task.action] || task.action)} fehlgeschlagen:</b> ${esc(task.error)}</span>
      <button class="btn ghost sm" id="btnDismiss">${s.containerId ? 'Ausblenden' : 'Verwerfen'}</button></div>`;
    $('#btnDismiss').addEventListener('click', async () => { await api('DELETE', `/api/servers/${s.name}/task`).catch(() => {}); poll(); });
  } else tb.innerHTML = '';
}

function toggleMenu() {
  const wrap = $('.menu-wrap'), existing = $('.menu', wrap);
  if (existing) { existing.remove(); return; }
  const s = current(), busy = s.phase === 'busy';
  const m = document.createElement('div');
  m.className = 'menu';
  const pub = s.config.publicHidden ? 'disabled title="Server ist nicht öffentlich"' : '';
  m.innerHTML = `
    ${canS('server.backup') ? `<button data-m="backup">${icon('download')}Backup herunterladen</button>` : ''}
    ${canS('server.configure') ? `<button data-m="image" ${busy ? 'disabled' : ''}>${icon('refresh')}Image aktualisieren</button>` : ''}
    ${can('servers.create') && canS('server.configure') ? `<button data-m="clone" ${busy ? 'disabled' : ''}>${icon('clone')}Server klonen</button>` : ''}
    ${canS('server.control') ? `<button data-m="maint">${icon('wrench')}${s.maintenance ? 'Wartungsmodus beenden' : 'Wartungsmodus …'}</button>` : ''}
    <button data-m="copy">${icon('copy')}Adresse kopieren</button>
    <hr>
    <button data-m="public" ${pub}>${icon('external')}Öffentliches Dashboard</button>
    ${s.config.map ? `<button data-m="map">${icon('map')}Live-Karte öffnen</button>` : ''}
    <button data-m="embed" ${pub}>${icon('code')}Widget &amp; Badge einbetten</button>
    ${canS('server.delete') ? `<hr><button data-m="delete" class="danger" ${busy ? 'disabled' : ''}>${icon('trash')}Server löschen</button>` : ''}`;
  wrap.appendChild(m);
  const off = (e) => { if (!m.contains(e.target)) { m.remove(); document.removeEventListener('click', off); } };
  setTimeout(() => document.addEventListener('click', off));
  m.addEventListener('click', (e) => {
    const b = e.target.closest('button'); if (!b || b.disabled) return;
    m.remove(); document.removeEventListener('click', off);
    ({ backup: doBackup, image: doImageUpdate, copy: () => copyText(hostAddr(s.config.port)), delete: openDelete,
      clone: openClone, embed: openEmbed, maint: () => openMaintenance(s), map: () => window.open(`/map/${encodeURIComponent(s.name)}/`, '_blank', 'noopener'),
      public: () => window.open(`/s/${encodeURIComponent(s.name)}`, '_blank', 'noopener') })[b.dataset.m]();
  });
}

async function doAction(act) {
  const s = current();
  try {
    await api('POST', `/api/servers/${s.name}/actions/${act}`);
    poll();
  } catch (e) { toast(e.message, 'err'); }
}

async function doImageUpdate() {
  const s = current();
  const ok = await confirmDialog({
    title: 'Image aktualisieren',
    body: `<p>Die neueste Version von <span class="mono">itzg/minecraft-server:${esc(s.config.javaTag)}</span> wird geladen und der Container neu erstellt. Ein laufender Server wird dafür kurz gestoppt. Die Welt bleibt erhalten.</p>`,
    ok: 'Aktualisieren',
  });
  if (ok) doAction('update-image');
}

function doBackup() {
  const s = current();
  const a = document.createElement('a');
  a.href = `/api/servers/${encodeURIComponent(s.name)}/backup`;
  a.download = '';
  document.body.appendChild(a); a.click(); a.remove();
  toast(RUNNING.includes(s.phase) ? 'Backup wird erstellt – Autosave ist währenddessen pausiert.' : 'Backup wird erstellt …');
}

async function copyText(t) {
  try { await navigator.clipboard.writeText(t); toast(`„${t}“ kopiert`); }
  catch { toast(`Adresse: ${t}`); }
}

function openDelete() {
  const s = current();
  const usesVolume = !s.config.dataPath;
  const m = openOverlay(`<div class="modal" role="dialog" aria-modal="true">
    <header><h2>Server löschen</h2><button class="btn ghost icon sm" data-close aria-label="Schließen">${icon('x')}</button></header>
    <div class="body">
      <p>Der Container <span class="mono">mc-${esc(s.name)}</span> wird gestoppt und entfernt.</p>
      ${usesVolume
        ? `<label class="toggle-row"><span class="t"><b>Weltdaten ebenfalls löschen</b><small>Volume <span class="mono">mc-${esc(s.name)}-data</span> – nicht wiederherstellbar</small></span><span class="switch"><input type="checkbox" id="delData"><span></span></span></label>`
        : `<p class="dim">Die Daten unter <span class="mono">${esc(s.config.dataPath)}</span> bleiben auf dem Host erhalten.</p>`}
      <label class="field"><span>Zur Bestätigung <b class="mono">${esc(s.name)}</b> eingeben</span><input id="delName" autocomplete="off"></label>
    </div>
    <footer><button class="btn" data-close>Abbrechen</button><button class="btn danger solid" id="delOk" disabled>${icon('trash')}Löschen</button></footer>
  </div>`);
  const inp = $('#delName', m.el), ok = $('#delOk', m.el);
  inp.focus();
  inp.addEventListener('input', () => { ok.disabled = inp.value !== s.name; });
  ok.addEventListener('click', async () => {
    const data = $('#delData', m.el)?.checked ? '?data=1' : '';
    ok.disabled = true;
    try {
      await api('DELETE', `/api/servers/${s.name}${data}`);
      m.close();
      toast(`${s.name} wird gelöscht`);
      poll();
    } catch (e) { toast(e.message, 'err'); ok.disabled = false; }
  });
}

/* =====================================================================
   Tabs
   ===================================================================== */
function renderTab() {
  const body = $('#tabBody');
  if (!body) return;
  closeLogs();
  body.className = 'tab-body' + (state.tab === 'console' || state.tab === 'files' ? ' flush' : '');
  ({ overview: renderOverview, console: renderConsole, players: renderPlayersTab, mods: renderMods, world: renderWorld, config: renderConfig, files: renderFiles }[state.tab] || renderOverview)(body);
}

/* ---------- Overview ---------- */
function renderOverview(body) {
  const s = current(), c = s.config;
  body.innerHTML = `
    <div class="stack">
      <div class="grid stats">
        <div class="panel stat"><div class="k">${icon('clock')}Laufzeit</div><div class="v" id="ovUp">—</div><div class="s" id="ovUpS"></div></div>
        <div class="panel stat"><div class="k">${icon('cpu')}CPU</div><div class="v" id="ovCpu">—</div><div class="bar" id="ovCpuB"><i></i></div></div>
        <div class="panel stat"><div class="k">${icon('memory')}Arbeitsspeicher</div><div class="v" id="ovMem">—</div><div class="bar" id="ovMemB"><i></i></div></div>
        <div class="panel stat"><div class="k">${icon('users')}Spieler</div><div class="v" id="ovPl">—</div><div class="bar" id="ovPlB"><i></i></div></div>
      </div>
      <div class="grid two">
        <div class="panel">
          <div class="panel-head"><h3>${icon('globe')}Verbindung</h3><div class="btn-group">
            ${c.map ? `<a class="btn sm" href="/map/${encodeURIComponent(s.name)}/" target="_blank" rel="noopener">${icon('map')}Karte</a>` : ''}
            ${c.publicHidden ? '' : `<a class="btn sm" href="/s/${encodeURIComponent(s.name)}" target="_blank" rel="noopener">${icon('external')}Dashboard</a>`}</div></div>
          <div class="panel-body">
            <div class="address"><span class="a">${esc(hostAddr(c.port))}</span><button class="btn sm" id="ovCopy">${icon('copy')}Kopieren</button></div>
            <dl class="kv">
              <dt>Plattform</dt><dd>${esc(typeOf(c.type).label)}</dd>
              <dt>Version</dt><dd>${esc(versionDetail(s))}</dd>
              <dt>Spielmodus</dt><dd>${esc(SECTIONS[2].fields[0].options.find((o) => o[0] === c.gamemode)?.[1] || c.gamemode)} · ${esc(SECTIONS[2].fields[1].options.find((o) => o[0] === c.difficulty)?.[1] || c.difficulty)}</dd>
              <dt>Online-Modus</dt><dd>${c.onlineMode ? 'aktiv' : '<span>aus (Cracked)</span>'}</dd>
              <dt>Whitelist</dt><dd>${c.whitelist ? `${c.whitelist.split(',').length} Spieler${c.enforceWhitelist ? ', erzwungen' : ''}` : 'aus'}</dd>
            </dl>
          </div>
        </div>
        <div class="panel">
          <div class="panel-head"><h3>${icon('users')}Spieler online</h3><span class="dim num" id="ovPlCount"></span></div>
          <div class="panel-body"><div class="players" id="ovPlayers"><div class="empty-note">—</div></div></div>
        </div>
      </div>
      <div class="panel">
        <div class="panel-head"><h3>${icon('server')}Container</h3></div>
        <div class="panel-body">
          <dl class="kv" id="ovDetails"></dl>
        </div>
      </div>
    </div>`;
  $('#ovCopy').addEventListener('click', () => copyText(hostAddr(c.port)));
  refreshOverview();
}

function setBar(id, pct) {
  const b = $(id); if (!b) return;
  const p = Math.max(0, Math.min(100, pct || 0));
  b.firstElementChild.style.width = `${p}%`;
  b.classList.toggle('warn', p >= 75 && p < 90);
  b.classList.toggle('danger', p >= 90);
}

let ovSeq = 0;
async function refreshOverview() {
  const s = current();
  if (!s || state.tab !== 'overview' || !$('#ovUp')) return;
  const seq = ++ovSeq;
  const running = RUNNING.includes(s.phase);
  const [detail, stats, players] = await Promise.all([
    api('GET', `/api/servers/${s.name}`).catch(() => null),
    running ? api('GET', `/api/servers/${s.name}/stats`).catch(() => null) : null,
    running && s.phase !== 'sleeping' ? api('GET', `/api/servers/${s.name}/players`).catch(() => null) : null,
  ]);
  if (seq !== ovSeq || !$('#ovUp') || state.selected !== s.name) return;
  const c = s.config;

  $('#ovUp').textContent = running && detail?.startedAt ? fmtUptime(detail.startedAt) : '—';
  $('#ovUpS').textContent = running && detail?.startedAt ? `seit ${fmtDate(detail.startedAt)}` : phaseLabel(s);

  if (stats) {
    // Docker reports 100 % per core; show share of the available cores instead.
    const cores = c.cpus || state.info?.docker?.NCPU || 1;
    const pct = stats.cpuPercent / cores;
    $('#ovCpu').innerHTML = `${pct.toFixed(pct < 10 ? 1 : 0).replace('.', ',')}<small>% · ${(stats.cpuPercent / 100).toFixed(1).replace('.', ',')} von ${String(cores).replace('.', ',')} Kernen</small>`;
    setBar('#ovCpuB', pct);
    const limit = c.memoryLimitMB ? stats.memLimit : (state.info?.docker?.MemTotal || stats.memLimit);
    $('#ovMem').innerHTML = `${fmtBytes(stats.memUsage)}<small>/ ${c.memoryLimitMB ? fmtBytes(stats.memLimit) : esc(c.memory) + ' Heap'}</small>`;
    setBar('#ovMemB', c.memoryLimitMB ? (stats.memUsage / limit) * 100 : (stats.memUsage / (parseMem(c.memory) * 1.25)) * 100);
  } else {
    $('#ovCpu').textContent = '—'; $('#ovMem').textContent = '—';
    setBar('#ovCpuB', 0); setBar('#ovMemB', 0);
  }

  if (players) {
    $('#ovPl').innerHTML = `${players.online}<small>/ ${players.max}</small>`;
    setBar('#ovPlB', (players.online / (players.max || 1)) * 100);
    $('#ovPlCount').textContent = `${players.online} / ${players.max}`;
    $('#ovPlayers').innerHTML = players.names.length
      ? players.names.map((n) => `<span class="player"><span class="av" data-h="${hue(n)}">${esc(n.slice(0, 1).toUpperCase())}</span>${esc(n)}</span>`).join('')
      : '<div class="empty-note">Gerade ist niemand online.</div>';
    $$('.av[data-h]').forEach((a) => { a.style.background = `hsl(${a.dataset.h} 55% 62%)`; });
  } else {
    $('#ovPl').innerHTML = running ? '<small>wird abgefragt …</small>' : '—';
    setBar('#ovPlB', 0);
    $('#ovPlCount').textContent = '';
    $('#ovPlayers').innerHTML = `<div class="empty-note">${s.phase === 'sleeping' ? 'Der Server schläft (Auto-Pause) und wacht auf, sobald sich jemand verbindet.' : running ? 'Spielerliste ist verfügbar, sobald der Server vollständig gestartet ist.' : 'Server ist offline.'}</div>`;
    if (s.phase === 'sleeping') { $('#ovPl').innerHTML = '0<small>· schläft</small>'; }
  }

  if (detail) {
    const limits = [c.memoryLimitMB ? `${c.memoryLimitMB} MB RAM` : null, c.cpus ? `${String(c.cpus).replace('.', ',')} CPUs` : null].filter(Boolean).join(' · ') || 'keine';
    $('#ovDetails').innerHTML = `
      <dt>Container</dt><dd class="mono">mc-${esc(s.name)}${s.containerId ? ` <span class="dim">(${esc(s.containerId.slice(0, 12))})</span>` : ''}</dd>
      <dt>Image</dt><dd class="mono">${esc(s.image || `itzg/minecraft-server:${c.javaTag}`)}</dd>
      <dt>Weltdaten</dt><dd class="mono">${esc(detail.dataPath || `Volume ${detail.volume}`)}</dd>
      <dt>Limits</dt><dd>${esc(limits)}</dd>
      <dt>Neustart</dt><dd class="mono">${esc(c.restartPolicy)}</dd>
      <dt>Docker-Status</dt><dd>${esc(s.statusText || '—')}</dd>
      <dt>Angelegt</dt><dd>${esc(fmtDate(c.createdAt))}</dd>`;
  }
}
function parseMem(m) { const r = /^(\d+)([MG])$/.exec(m || ''); return r ? +r[1] * (r[2] === 'G' ? 1 << 30 : 1 << 20) : 1 << 30; }

/* ---------- Console ---------- */
const QUICK = [['list', 'Spieler'], ['save-all', 'Speichern'], ['time set day', 'Tag'], ['weather clear', 'Sonne'], ['tps', 'TPS']];

function renderConsole(body) {
  const s = current();
  body.innerHTML = `
    <div class="console">
      <div class="console-bar">
        <div class="status" id="conStatus"></div>
        <div class="grow"></div>
        <div class="btn-group ${canS('server.console') ? '' : 'hidden'}">${QUICK.map(([c, l]) => `<button class="btn sm" data-q="${esc(c)}" title="/${esc(c)}">${esc(l)}</button>`).join('')}</div>
        <button class="btn sm icon" id="conScroll" title="Automatisch scrollen">${icon('arrowDown')}</button>
        <button class="btn sm icon" id="conClear" title="Ausgabe leeren">${icon('eraser')}</button>
      </div>
      <div class="console-out" id="conOut" role="log" aria-live="polite"></div>
      <form class="console-in ${canS('server.console') ? '' : 'hidden'}" id="conForm" autocomplete="off">
        <span class="prompt">›</span>
        <input id="conIn" placeholder="${RUNNING.includes(s.phase) ? 'Befehl ohne / eingeben, z. B. say Hallo zusammen' : 'Server ist offline'}" spellcheck="false">
        <button class="btn primary" type="submit">${icon('send')}<span class="lbl">Senden</span></button>
      </form>
    </div>`;
  const out = $('#conOut');
  out.addEventListener('scroll', () => {
    state.autoscroll = out.scrollHeight - out.scrollTop - out.clientHeight < 40;
    $('#conScroll').classList.toggle('primary', !state.autoscroll);
  });
  $('#conScroll').addEventListener('click', () => { state.autoscroll = true; out.scrollTop = out.scrollHeight; });
  $('#conClear').addEventListener('click', () => { out.innerHTML = ''; });
  $$('[data-q]', body).forEach((b) => b.addEventListener('click', () => sendCommand(b.dataset.q)));
  $('#conForm').addEventListener('submit', (e) => { e.preventDefault(); const v = $('#conIn').value.trim(); if (v) { sendCommand(v); $('#conIn').value = ''; } });
  $('#conIn').addEventListener('keydown', onHistoryKey);
  state.autoscroll = true;
  openLogs();
  setTimeout(() => $('#conIn').focus(), 20);
}

function onHistoryKey(e) {
  const h = state.history;
  if (e.key === 'ArrowUp' && h.length) {
    e.preventDefault(); state.histIdx = Math.min(h.length - 1, state.histIdx + 1); e.target.value = h[h.length - 1 - state.histIdx];
  } else if (e.key === 'ArrowDown') {
    e.preventDefault(); state.histIdx = Math.max(-1, state.histIdx - 1); e.target.value = state.histIdx < 0 ? '' : h[h.length - 1 - state.histIdx];
  }
}

function conStatus(live, text) {
  const el = $('#conStatus');
  if (el) el.innerHTML = `<span class="dot ${live ? 'online' : 'stopped'}"></span>${esc(text)}`;
}

const ANSI = /\x1b\[[0-9;?]*[A-Za-z]|\x1b\][^\x07]*\x07|[\x00-\x08\x0b\x0c\x0e-\x1f]/g;
function appendLine(text, cls) {
  const out = $('#conOut'); if (!out) return;
  text = text.replace(ANSI, '').replace(/§[0-9a-fk-or]/gi, '');
  if (!cls) {
    if (/\b(ERROR|SEVERE|FATAL)\b|Exception|Caused by:/.test(text)) cls = 'err';
    else if (/\bWARN(ING)?\b/.test(text)) cls = 'warn';
    else if (/joined the game|left the game|logged in with entity/.test(text)) cls = 'join';
  }
  const d = document.createElement('div');
  d.className = 'l' + (cls ? ' ' + cls : '');
  d.textContent = text || ' ';
  out.appendChild(d);
  while (out.childElementCount > 3000) out.firstElementChild.remove();
  if (state.autoscroll) out.scrollTop = out.scrollHeight;
}

function openLogs() {
  closeLogs();
  const s = current(); if (!s || !s.containerId) { conStatus(false, 'Kein Container vorhanden'); return; }
  const es = new EventSource(`/api/servers/${encodeURIComponent(s.name)}/logs`);
  state.logES = es;
  es.onopen = () => { const o = $('#conOut'); if (o) o.innerHTML = ''; conStatus(true, 'Live-Log verbunden'); };
  es.onmessage = (e) => appendLine(e.data);
  es.addEventListener('end', () => {
    es.close(); if (state.logES === es) state.logES = null;
    appendLine('— Server läuft nicht. Das Log wird fortgesetzt, sobald er startet. —', 'sys');
    conStatus(false, 'Server offline');
  });
  es.onerror = () => { if (es.readyState !== EventSource.CLOSED) conStatus(false, 'Verbindung unterbrochen – verbinde neu …'); };
}
function closeLogs() { if (state.logES) { state.logES.close(); state.logES = null; } }

async function sendCommand(cmd) {
  const s = current();
  cmd = cmd.replace(/^\//, '');
  if (state.history[state.history.length - 1] !== cmd) state.history.push(cmd);
  state.history = state.history.slice(-100); state.histIdx = -1;
  sessionStorageSet('cd_hist', JSON.stringify(state.history));
  appendLine(`> ${cmd}`, 'cmd');
  try {
    const r = await api('POST', `/api/servers/${s.name}/command`, { command: cmd });
    (r.output || '(keine Ausgabe)').split('\n').forEach((l) => appendLine(l, 'res'));
  } catch (e) { appendLine(e.message, 'err'); }
}

/* ---------- Config form (shared by tab and create drawer) ---------- */
function versionField(f, v) {
  return `<input type="text" data-k="${f.k}" list="dl-versions" value="${esc(v)}" placeholder="LATEST" spellcheck="false">`;
}
function fillVersionLists() {
  const v = state.versions; if (!v) return;
  $$('datalist#dl-versions').forEach((dl) => {
    dl.innerHTML = ['LATEST', 'SNAPSHOT', ...v.releases.slice(0, 120)].map((r) => `<option value="${esc(r)}">${r === v.latest ? 'aktuelle Release' : ''}</option>`).join('');
  });
}

/** Context of the config form: what the user may change. */
function formCtx(s) {
  const h = state.me?.hosting || {};
  return { adv: advFor(s), assign: can('users') && state.me?.allServers, range: h.portMin ? [h.portMin, h.portMax] : null };
}

function fieldHTML(f, cfg, isNew, ctx = formCtx()) {
  if (f.adv && !ctx.adv) return '';
  if (f.type === 'owner' && !ctx.assign) return '';
  if (!ctx.adv && !state.me?.hosting?.customerPublic && ['publicHidden', 'displayName', 'publicAddress', 'description'].includes(f.k)) return '';
  if (!ctx.adv && (f.k === 'mapPort' || f.rangeOnly) && !ctx.range) return '';
  if (!ctx.adv && ctx.range && (f.k === 'port' || f.k === 'mapPort' || f.rangeOnly)) {
    f = { ...f, hint: `${f.k === 'port' ? 'Leer lassen = automatisch. ' : ''}Erlaubte Host-Ports: ${ctx.range[0]}–${ctx.range[1]}` };
  }
  if (!ctx.adv && f.k === 'memory') {
    const mx = state.me?.hosting?.maxServerMB;
    f = { ...f, hint: `${f.hint}. Das Container-Limit wird automatisch passend gesetzt${mx ? ` – höchstens ${mx} MB pro Server` : ''}.` };
  }
  if (!ctx.adv && f.k === 'extraEnv') f = { ...f, hint: `${f.hint}. Speicher-, RCON- und JVM-Variablen setzt nur ein Administrator.` };
  if (f.type === 'owner') {
    f = { ...f, type: 'text', placeholder: 'kein Besitzer', list: 'dl-users' };
  }
  const html = fieldInner(f, cfg, isNew);
  return f.types ? `<div class="cond ${f.full || f.type === 'types' ? 'full' : ''}" data-for-types="${f.types.join(',')}">${html}</div>` : html;
}

function fieldInner(f, cfg, isNew) {
  if (f.type === 'netselect') {
    const nets = state.networks || [];
    f = { ...f, type: 'select', options: [['', 'Keins – eigener Port'], ...nets.map((n) => [n.id, `${n.name || n.id} (Port ${n.port})`])],
      hint: nets.length ? f.hint : 'Noch kein Netzwerk vorhanden – lege eins unter „Netzwerke“ an.' };
  }
  const v = cfg[f.k];
  const dis = f.newOnly && !isNew ? 'disabled' : '';
  const hint = f.hint ? `<span class="hint">${esc(f.hint)}${f.newOnly && !isNew ? ' – nach dem Anlegen nicht änderbar' : ''}</span>` : (f.newOnly && !isNew ? '<span class="hint">Nach dem Anlegen nicht änderbar</span>' : '');
  const cls = `field${f.full || f.type === 'types' ? ' full' : ''}`;
  switch (f.type) {
    case 'toggle': {
      const on = f.invert ? !v : v;
      return `<label class="toggle-row ${f.full ? 'full' : ''}"><span class="t"><b>${esc(f.label)}</b>${f.hint ? `<small>${esc(f.hint)}</small>` : ''}</span><span class="switch"><input type="checkbox" data-k="${f.k}" ${f.invert ? 'data-invert' : ''} ${on ? 'checked' : ''}><span></span></span></label>`;
    }
    case 'textarea':
      return `<label class="${cls}"><span>${esc(f.label)}</span><textarea data-k="${f.k}" rows="4" placeholder="${esc(f.placeholder || '')}">${esc(v)}</textarea>${hint}</label>`;
    case 'secret':
      return `<label class="${cls}"><span>${esc(f.label)}</span><input type="password" data-k="${f.k}" value="" autocomplete="new-password" placeholder="${cfg._cfKeySet ? '•••••• gespeichert' : 'nicht gesetzt'}">${hint}</label>`;
    case 'select':
      return `<label class="${cls}"><span>${esc(f.label)}</span><select data-k="${f.k}">${f.options.map(([o, l]) => `<option value="${esc(o)}" ${o === v ? 'selected' : ''}>${esc(l)}</option>`).join('')}${f.options.some(([o]) => o === v) ? '' : `<option value="${esc(v)}" selected>${esc(v)}</option>`}</select>${hint}</label>`;
    case 'number':
      return `<label class="${cls}"><span>${esc(f.label)}</span><input type="number" data-k="${f.k}" value="${v === 0 && f.k === 'port' ? '' : esc(v)}" ${f.min !== undefined ? `min="${f.min}"` : ''} ${f.max !== undefined ? `max="${f.max}"` : ''} step="${f.step || 1}">${hint}</label>`;
    case 'list':
      return `<label class="${cls}"><span>${esc(f.label)}</span><textarea data-k="${f.k}" data-list rows="3" placeholder="${esc(f.placeholder || '')}" spellcheck="false">${esc((v || '').split(',').filter(Boolean).join('\n'))}</textarea>${hint}</label>`;
    case 'version':
      return `<label class="${cls}"><span>${esc(f.label)}</span>${versionField(f, v)}${hint}</label>`;
    case 'types':
      return `<div class="${cls}"><span class="field-label">${esc(f.label)}</span><div class="types" data-k="type" data-types>${TYPES.map((t) => `<button type="button" class="type-opt ${t.id === v ? 'sel' : ''}" data-type="${t.id}"><span class="type-badge sm">${t.short}</span><span><b>${t.label}</b><small>${t.desc}</small></span></button>`).join('')}</div>${!isNew ? '<span class="hint">Ein Plattformwechsel bei bestehender Welt sollte vorher mit einem Backup abgesichert werden.</span>' : ''}</div>`;
    case 'ports':
      return `<div class="${cls}"><span class="field-label">${esc(f.label)}</span><div class="rows" data-k="ports" data-rows="ports">
        <div class="row-head row-edit ports"><span>Host-Port</span><span>Container-Port</span><span>Protokoll</span><span></span></div>
        ${(v || []).map(portRow).join('')}</div>
        <div><button type="button" class="btn sm" data-add="ports">${icon('plus')}Port hinzufügen</button></div>${hint}</div>`;
    case 'env':
      return `<div class="${cls}"><span class="field-label">${esc(f.label)}</span><div class="rows" data-k="extraEnv" data-rows="env">
        ${Object.entries(v || {}).map(([k, val]) => envRow(k, val)).join('')}</div>
        <div><button type="button" class="btn sm" data-add="env">${icon('plus')}Variable hinzufügen</button></div>${hint}</div>`;
    default:
      return `<label class="${cls}"><span>${esc(f.label)}</span><input type="text" data-k="${f.k}" value="${esc(v)}" placeholder="${esc(f.placeholder || '')}" ${f.list ? `list="${f.list}"` : ''} ${dis} spellcheck="false">${hint}</label>`;
  }
}
const portRow = (p = { host: '', container: '', protocol: 'tcp' }) => `<div class="row-edit ports" data-row>
  <input type="number" min="1" max="65535" data-f="host" value="${esc(p.host)}" placeholder="19132">
  <input type="number" min="1" max="65535" data-f="container" value="${esc(p.container)}" placeholder="19132">
  <select data-f="protocol"><option ${p.protocol === 'tcp' ? 'selected' : ''}>tcp</option><option ${p.protocol === 'udp' ? 'selected' : ''}>udp</option></select>
  <button type="button" class="btn icon ghost" data-del aria-label="Entfernen">${icon('x')}</button></div>`;
const envRow = (k = '', v = '') => `<div class="row-edit env" data-row>
  <input data-f="key" value="${esc(k)}" placeholder="SPAWN_PROTECTION" spellcheck="false">
  <input data-f="value" value="${esc(v)}" placeholder="0" spellcheck="false">
  <button type="button" class="btn icon ghost" data-del aria-label="Entfernen">${icon('x')}</button></div>`;

function configFormHTML(cfg, isNew, ctx = formCtx()) {
  const secs = SECTIONS.map((sec) => ({ sec, html: sec.fields.map((f) => fieldHTML(f, cfg, isNew, ctx)).join('') })).filter((x) => x.html.trim());
  if (ctx.assign) loadUserList();
  return `<div class="config">
    <nav class="config-nav">${secs.map(({ sec }) => `<a href="#sec-${sec.id}" data-sec="${sec.id}">${sec.title}</a>`).join('')}</nav>
    <div class="config-sections">
      ${secs.map(({ sec, html }) => `<section class="fsec" id="sec-${sec.id}">
        <header><h3>${sec.title}</h3><p>${esc(sec.desc)}</p></header>
        <div class="fgrid">${html}</div>
      </section>`).join('')}
    </div>
    <datalist id="dl-versions"></datalist><datalist id="dl-users">${(state.userList || []).map((u) => `<option value="${esc(u)}">`).join('')}</datalist>
  </div>`;
}

async function loadUserList() {
  if (state.userList) return;
  try {
    const d = await api('GET', '/api/users');
    state.userList = d.users.map((u) => u.username);
    $$('#dl-users').forEach((dl) => { dl.innerHTML = state.userList.map((u) => `<option value="${esc(u)}">`).join(''); });
  } catch { state.userList = []; }
}

function updateConditional(root) {
  const type = $('.type-opt.sel', root)?.dataset.type;
  $$('[data-for-types]', root).forEach((el) => el.classList.toggle('hidden', !el.dataset.forTypes.split(',').includes(type)));
}

function bindConfigForm(root, onChange) {
  fillVersionLists();
  updateConditional(root);
  root.addEventListener('input', onChange);
  root.addEventListener('change', onChange);
  root.addEventListener('click', (e) => {
    const t = e.target.closest('.type-opt');
    if (t) { $$('.type-opt', root).forEach((b) => b.classList.toggle('sel', b === t)); updateConditional(root); onChange(); return; }
    const add = e.target.closest('[data-add]');
    if (add) {
      const rows = $(`[data-rows="${add.dataset.add}"]`, root);
      rows.insertAdjacentHTML('beforeend', add.dataset.add === 'ports' ? portRow() : envRow());
      $('[data-row]:last-child input', rows).focus();
      onChange(); return;
    }
    const del = e.target.closest('[data-del]');
    if (del) { del.closest('[data-row]').remove(); onChange(); return; }
    const nav = e.target.closest('[data-sec]');
    if (nav) { e.preventDefault(); $(`#sec-${nav.dataset.sec}`, root)?.scrollIntoView({ behavior: 'smooth', block: 'start' }); }
  });
}

function collectConfig(root, base) {
  const cfg = { ...base };
  $$('[data-k]', root).forEach((el) => {
    const k = el.dataset.k;
    if (el.hasAttribute('data-types')) { cfg.type = $('.type-opt.sel', el)?.dataset.type || cfg.type; return; }
    if (el.dataset.rows === 'ports') {
      cfg.ports = $$('[data-row]', el).map((r) => ({ host: +$('[data-f=host]', r).value, container: +$('[data-f=container]', r).value, protocol: $('[data-f=protocol]', r).value }))
        .filter((p) => p.host || p.container);
      return;
    }
    if (el.dataset.rows === 'env') {
      cfg.extraEnv = {};
      $$('[data-row]', el).forEach((r) => { const key = $('[data-f=key]', r).value.trim().toUpperCase(); if (key) cfg.extraEnv[key] = $('[data-f=value]', r).value; });
      return;
    }
    if (el.type === 'checkbox') cfg[k] = el.hasAttribute('data-invert') ? !el.checked : el.checked;
    else if (el.type === 'number') cfg[k] = el.value === '' ? 0 : Number(el.value);
    else if (el.hasAttribute('data-list')) cfg[k] = el.value.split(/[\s,;]+/).filter(Boolean).join(',');
    else cfg[k] = el.value.trim();
  });
  if (!PROXY_TYPES.includes(cfg.type)) cfg.network = '';
  delete cfg.proxyMember; // derived from network on the server
  delete cfg._cfKeySet;
  return cfg;
}

async function loadNetworks() {
  try { const v = await api('GET', '/api/proxies'); state.networks = v.networks.map((n) => n.config); state.proxyInfo = v; } catch { /* keep */ }
  return state.networks || [];
}

function renderConfig(body) {
  const s = current();
  const base = { ...DEFAULTS, ...s.config };
  const formCfg = { ...base, _cfKeySet: s.cfKeySet };
  body.innerHTML = `${configFormHTML(formCfg, false, formCtx(s))}
    <div class="savebar">
      <span class="state" id="cfgState">Keine Änderungen</span>
      <button class="btn" id="cfgReset" disabled>Verwerfen</button>
      <button class="btn primary" id="cfgSave" disabled>${icon('save')}Speichern & anwenden</button>
    </div>`;
  const root = $('.config', body);
  const initial = JSON.stringify(collectConfig(root, base));
  const onChange = () => {
    state.configDirty = JSON.stringify(collectConfig(root, base)) !== initial;
    $('#cfgState').textContent = state.configDirty ? 'Ungespeicherte Änderungen' : 'Keine Änderungen';
    $('#cfgState').classList.toggle('dirty', state.configDirty);
    $('#cfgSave').disabled = $('#cfgReset').disabled = !state.configDirty;
  };
  bindConfigForm(root, onChange);
  $('#cfgReset').addEventListener('click', () => { state.configDirty = false; renderTab(); });
  $('#cfgSave').addEventListener('click', async () => {
    const cfg = collectConfig(root, base);
    const running = RUNNING.includes(current().phase);
    let compat = '';
    if ((cfg.version !== base.version || cfg.type !== base.type) && (cfg.modrinthProjects || cfg.map || cfg.network)) {
      $('#cfgSave').disabled = true;
      $('#cfgSave').innerHTML = '<span class="spinner"></span>Prüfe Mods …';
      try {
        const r = await api('GET', `/api/servers/${s.name}/mods/compat?type=${encodeURIComponent(cfg.type)}&version=${encodeURIComponent(cfg.version)}`);
        compat = compatHTML(r, true);
      } catch (e) { compat = `<div class="form-error">Kompatibilitäts-Check nicht möglich: ${esc(e.message)}</div>`; }
      $('#cfgSave').disabled = false;
      $('#cfgSave').innerHTML = `${icon('save')}Speichern & anwenden`;
    }
    const ok = await confirmDialog({
      title: 'Konfiguration anwenden',
      body: `${compat}<p>Der Container wird mit der neuen Konfiguration neu erstellt. ${running ? '<b>Der Server wird dafür gestoppt und anschließend wieder gestartet.</b>' : ''}</p><p class="dim">Die Welt und alle Dateien im Datenverzeichnis bleiben erhalten.</p>`,
      ok: 'Anwenden',
    });
    if (!ok) return;
    $('#cfgSave').disabled = true;
    try {
      await api('PUT', `/api/servers/${s.name}`, cfg);
      state.configDirty = false;
      toast('Konfiguration wird angewendet');
      await poll();
      renderTab();
    } catch (e) { toast(e.message, 'err'); $('#cfgSave').disabled = false; }
  });
}

/* ---------- Create drawer ---------- */
function nextFreePort() {
  const used = new Set(state.servers.map((s) => s.config.port));
  let p = 25565; while (used.has(p)) p++;
  return p;
}

function openCreate() {
  if (!state.versions) api('GET', '/api/versions').then((v) => { state.versions = v; fillVersionLists(); }).catch(() => {});
  const ctx = formCtx(null);
  const cfg = { ...DEFAULTS, port: !ctx.adv && ctx.range ? 0 : nextFreePort(), name: state.servers.length ? '' : 'survival' };
  if (!ctx.adv) cfg.memory = '2G';
  const m = openOverlay(`<div class="drawer" role="dialog" aria-modal="true">
    <header><span class="type-badge sm">${icon('plus')}</span><h2>Neuer Server</h2><button class="btn ghost icon" data-close aria-label="Schließen">${icon('x')}</button></header>
    <div class="body">${configFormHTML(cfg, true, ctx)}</div>
    <footer><span class="grow dim" id="crHint">${quotaLine() || 'EULA wird automatisch akzeptiert (minecraft.net/eula).'}</span><button class="btn" data-close>Abbrechen</button><button class="btn primary" id="crGo">${icon('play')}Erstellen & starten</button></footer>
  </div>`, { drawer: true });
  const root = $('.config', m.el);
  bindConfigForm(root, () => { $('[data-k=name]', root).classList.remove('invalid'); });
  $('[data-k=name]', root).focus();
  $('#crGo', m.el).addEventListener('click', async () => {
    const c = collectConfig(root, cfg);
    if (!/^[a-z0-9][a-z0-9-]{0,30}$/.test(c.name)) {
      const n = $('[data-k=name]', root); n.classList.add('invalid'); n.focus();
      toast('Bitte einen gültigen Namen eingeben (a–z, 0–9, -)', 'err');
      return;
    }
    const btn = $('#crGo', m.el); btn.disabled = true; btn.innerHTML = '<span class="spinner"></span>Wird angelegt …';
    try {
      const r = await api('POST', '/api/servers', c);
      m.close();
      toast(`${r.name} wird erstellt – das Image wird ggf. zuerst geladen.`);
      await poll();
      select(r.name, 'console');
    } catch (e) {
      toast(e.message, 'err');
      btn.disabled = false; btn.innerHTML = `${icon('play')}Erstellen & starten`;
    }
  });
}

/* ---------- Files ---------- */
const FILE_NOTES = {
  'server.properties': 'Werte, die in der Konfiguration gesetzt sind (z. B. MOTD, Spielmodus), werden beim Start aus CraftDock übernommen. server-port im Container bleibt 25565. Änderungen wirken nach einem Neustart.',
  'ops.json': 'Wird vom Server verwaltet – Änderungen wirken nach einem Neustart oder mit „op“/„deop“ in der Konsole.',
  'whitelist.json': 'Nach dem Speichern in der Konsole „whitelist reload“ ausführen.',
};

async function renderFiles(body) {
  const s = current();
  if (!s.containerId) { body.innerHTML = '<div class="empty"><div class="box"><p>Der Container existiert noch nicht.</p></div></div>'; return; }
  body.innerHTML = `<div class="files"><aside class="file-list" id="fList"><div class="empty-note">Lade …</div></aside>
    <div class="editor"><div class="editor-head"><span class="path" id="fPath">Datei auswählen</span><span class="grow"></span>
      <button class="btn sm" id="fReload" disabled>${icon('refresh')}Neu laden</button>
      <button class="btn primary sm" id="fSave" disabled>${icon('save')}Speichern</button></div>
      <textarea id="fEd" spellcheck="false" disabled placeholder="Wähle links eine Datei aus."></textarea>
      <div class="note" id="fNote">Konfigurationsdateien aus dem Datenverzeichnis /data. Ungültiges JSON wird nicht gespeichert.</div></div></div>`;
  let r;
  try { r = await api('GET', `/api/servers/${s.name}/files`); } catch (e) { $('#fList').innerHTML = `<div class="empty-note">${esc(e.message)}</div>`; return; }
  if (!$('#fList')) return;
  const exist = new Set(r.files);
  $('#fList').innerHTML = r.editable.map((f) => `<button class="f" data-f="${esc(f)}" ${exist.has(f) ? '' : 'disabled title="Existiert (noch) nicht"'}>${icon('file')}${esc(f)}</button>`).join('');
  $$('#fList .f').forEach((b) => b.addEventListener('click', () => openFile(b.dataset.f)));
  const ed = $('#fEd');
  ed.addEventListener('input', () => { state.fileDirty = true; $('#fSave').disabled = false; });
  ed.addEventListener('keydown', (e) => {
    if ((e.ctrlKey || e.metaKey) && e.key === 's') { e.preventDefault(); saveFile(); }
    if (e.key === 'Tab') { e.preventDefault(); const p = ed.selectionStart; ed.setRangeText('  ', p, ed.selectionEnd, 'end'); state.fileDirty = true; $('#fSave').disabled = false; }
  });
  $('#fSave').addEventListener('click', saveFile);
  $('#fReload').addEventListener('click', () => openFile(state.currentFile, true));
  if (state.currentFile && exist.has(state.currentFile)) openFile(state.currentFile, true);
  else if (exist.has('server.properties')) openFile('server.properties', true);
}

async function openFile(f, force) {
  if (!force && f !== state.currentFile && !(await guardDirty())) return;
  const s = current();
  state.currentFile = f;
  $$('#fList .f').forEach((b) => b.classList.toggle('active', b.dataset.f === f));
  $('#fPath').textContent = `/data/${f}`;
  const ed = $('#fEd'); ed.disabled = true; ed.value = 'Lade …';
  try {
    const r = await api('GET', `/api/servers/${s.name}/file?path=${encodeURIComponent(f)}`);
    ed.value = r.content; ed.disabled = false; state.fileDirty = false;
    $('#fSave').disabled = true; $('#fReload').disabled = false;
    $('#fNote').textContent = FILE_NOTES[f] || 'Änderungen wirken in der Regel nach einem Neustart des Servers.';
  } catch (e) { ed.value = ''; toast(e.message, 'err'); }
}

async function saveFile() {
  const s = current(), f = state.currentFile;
  if (!f) return;
  $('#fSave').disabled = true;
  try {
    await api('PUT', `/api/servers/${s.name}/file?path=${encodeURIComponent(f)}`, { content: $('#fEd').value });
    state.fileDirty = false;
    toast(`${f} gespeichert`);
  } catch (e) { toast(e.message, 'err'); $('#fSave').disabled = false; }
}

/* ---------- Mods & plugins ---------- */
const LOADERS = [['fabric', 'Fabric'], ['quilt', 'Quilt'], ['forge', 'Forge'], ['neoforge', 'NeoForge'], ['paper', 'Paper (Plugins)'], ['purpur', 'Purpur (Plugins)'], ['folia', 'Folia (Plugins)'], ['spigot', 'Spigot (Plugins)']];
const fmtNum = (n) => n >= 1e6 ? `${(n / 1e6).toFixed(1).replace('.', ',')} Mio.` : n >= 1e3 ? `${Math.round(n / 1e3)} Tsd.` : String(n || 0);
const modIcon = (p) => p.icon_url || p.iconUrl
  ? `<img class="mod-ic" src="${esc(p.icon_url || p.iconUrl)}" alt="" loading="lazy">`
  : `<span class="mod-ic ph">${esc((p.title || '?').slice(0, 1).toUpperCase())}</span>`;

async function renderMods(body) {
  const s = current();
  state.modsDirty = false;
  if (!s.containerId) { body.innerHTML = '<div class="empty"><div class="box"><p>Der Container existiert noch nicht.</p></div></div>'; return; }
  body.innerHTML = '<div class="empty-note"><span class="spinner"></span> Mods werden geladen …</div>';
  let info;
  try { info = await api('GET', `/api/servers/${s.name}/mods`); } catch (e) { body.innerHTML = `<div class="banner">${icon('alert')}<div>${esc(e.message)}</div></div>`; return; }
  if (state.tab !== 'mods' || state.selected !== s.name) return;

  if (info.kind === 'none') {
    body.innerHTML = `<div class="empty"><div class="box"><div class="art">${icon('puzzle')}</div><h2>Vanilla unterstützt keine Mods</h2>
      <p>Wechsle in der Konfiguration auf <b>Fabric</b>, <b>NeoForge</b> oder <b>Forge</b> für Mods, auf <b>Paper</b> oder <b>Purpur</b> für Plugins – oder auf ein komplettes Modpack.</p>
      <button class="btn primary" id="toCfg">${icon('sliders')}Zur Konfiguration</button></div></div>`;
    $('#toCfg').addEventListener('click', () => select(s.name, 'config'));
    return;
  }
  const what = info.kind === 'plugins' ? 'Plugins' : 'Mods';
  const original = info.projects.map((p) => p.ref);
  let refs = [...original];
  const meta = Object.fromEntries(info.projects.map((p) => [p.ref, p]));

  body.innerHTML = `<div class="stack mods-page${canS('server.mods', s) ? '' : ' readonly'}">
    <div class="mods-head">
      <div class="chips">
        <span class="chip">${icon('box')}${esc(typeOf(s.config.type).label)}</span>
        ${info.loader ? `<span class="chip">Loader ${esc(info.loader)}</span>` : ''}
        ${info.gameVersion ? `<span class="chip">Minecraft ${esc(info.gameVersion)}</span>` : ''}
        <span class="chip mono">/data/${esc(info.dir)}</span>
      </div>
    </div>
    ${info.modpack ? `<div class="panel pack">
      ${info.modpack.iconUrl ? `<img class="pack-ic" src="${esc(info.modpack.iconUrl)}" alt="">` : `<span class="pack-ic ph">${icon('box')}</span>`}
      <div class="grow"><div class="dim small">Modpack · ${info.modpack.source === 'modrinth' ? 'Modrinth' : 'CurseForge'}</div><h3>${esc(info.modpack.title)}</h3>${info.modpack.description ? `<p class="muted">${esc(info.modpack.description)}</p>` : ''}</div>
      <a class="btn sm" href="${esc(info.modpack.url)}" target="_blank" rel="noopener">${icon('external')}Ansehen</a>
    </div>` : ''}
    <div class="grid two mods-grid">
      <div class="panel">
        <div class="panel-head"><h3>${icon('puzzle')}Von Modrinth verwaltet</h3><span class="dim num" id="mpCount"></span></div>
        <div class="panel-body"><div class="mod-list" id="mpList"></div>
          <p class="hint-text">CraftDock lädt diese ${what} beim Start automatisch in der passenden Version und hält sie aktuell. Entfernte Einträge werden beim nächsten Start gelöscht.</p></div>
      </div>
      <div class="panel">
        <div class="panel-head"><h3>${icon('search')}Modrinth durchsuchen</h3></div>
        <div class="panel-body">
          <div class="search-row">
            <input id="msQ" placeholder="${what} suchen, z. B. ${info.kind === 'plugins' ? 'LuckPerms' : 'Sodium'}" spellcheck="false">
            <select id="msLoader">${LOADERS.map(([v, l]) => `<option value="${v}" ${v === (info.loader || 'fabric') ? 'selected' : ''}>${l}</option>`).join('')}</select>
            <input id="msVer" value="${esc(info.gameVersion || '')}" placeholder="Version" list="dl-versions">
          </div>
          <datalist id="dl-versions"></datalist>
          <div class="mod-list results" id="msRes"></div>
        </div>
      </div>
    </div>
    <div class="panel">
      <div class="panel-head"><h3>${icon('check')}Update-Check</h3><span class="dim small">Gibt es alle Mods schon für die nächste Version?</span></div>
      <div class="panel-body">
        <div class="search-row compat-row"><input id="cvVer" list="dl-versions" placeholder="Ziel-Version, z. B. ${esc(state.versions?.latest || '1.21.5')}" value="${esc(state.versions?.latest && state.versions.latest !== info.gameVersion ? state.versions.latest : '')}">
          <select id="cvType">${TYPES.filter((t) => !['VANILLA', 'MODRINTH', 'AUTO_CURSEFORGE'].includes(t.id)).map((t) => `<option value="${t.id}" ${t.id === s.config.type ? 'selected' : ''}>${t.label}</option>`).join('')}</select>
          <button class="btn" id="cvGo">${icon('search')}Prüfen</button></div>
        <div id="cvRes" class="cv-res"></div>
      </div>
    </div>
    <div class="panel">
      <div class="panel-head"><h3>${icon('file')}Dateien in /data/${esc(info.dir)}</h3><span class="dim num">${info.files.length} Dateien</span></div>
      <div class="panel-body">
        <label class="drop" id="drop"><input type="file" id="upFile" accept=".jar" multiple hidden>
          ${icon('upload')}<span><b>.jar hierher ziehen</b> oder klicken zum Auswählen</span><small>Eigene ${what} oder solche, die es nicht auf Modrinth gibt · max. 512 MB</small>
          <span class="up-bar hidden" id="upBar"><i></i></span></label>
        ${info.filesError ? `<div class="form-error">${esc(info.filesError)}</div>` : ''}
        <div class="file-table" id="fTable">${info.files.length ? info.files.map((f) => `
          <div class="ft-row"><span class="mono ft-name">${icon('file')}${esc(f.name)}</span><span class="dim num">${fmtBytes(f.size)}</span>
          <button class="btn ghost icon sm" data-delfile="${esc(f.name)}" title="Löschen">${icon('trash')}</button></div>`).join('') : `<div class="empty-note">Noch keine Dateien – ${s.phase === 'stopped' ? 'beim ersten Start werden die verwalteten ' + what + ' heruntergeladen.' : 'lade eigene hoch oder füge welche von Modrinth hinzu.'}</div>`}</div>
      </div>
    </div>
    <div class="savebar">
      <span class="state" id="mpState">Keine Änderungen</span>
      <button class="btn" id="mpReset" disabled>Verwerfen</button>
      <button class="btn primary" id="mpSave" disabled>${icon('save')}Anwenden</button>
    </div>
  </div>`;
  fillVersionLists();

  const renderList = () => {
    const removed = original.filter((r) => !refs.includes(r));
    const added = refs.filter((r) => !original.includes(r));
    const all = [...refs, ...removed];
    $('#mpCount').textContent = `${refs.length}`;
    $('#mpList').innerHTML = all.length ? all.map((r) => {
      const p = meta[r] || { title: r, slug: r };
      const isRemoved = removed.includes(r), isAdded = added.includes(r);
      return `<div class="mod-row ${isRemoved ? 'removed' : ''}">${modIcon(p)}
        <div class="mod-txt"><div class="mod-title"><a href="${esc(p.url || '#')}" target="_blank" rel="noopener">${esc(p.title)}</a>${isAdded ? '<span class="tag new">neu</span>' : ''}${isRemoved ? '<span class="tag del">wird entfernt</span>' : ''}</div>
        <div class="mod-desc">${esc(p.description || r)}</div></div>
        <button class="btn ghost icon sm" data-toggle="${esc(r)}" title="${isRemoved ? 'Behalten' : 'Entfernen'}">${icon(isRemoved ? 'undo' : 'x')}</button></div>`;
    }).join('') : `<div class="empty-note">Noch keine ${what} von Modrinth – suche rechts danach.</div>`;
    state.modsDirty = added.length + removed.length > 0;
    $('#mpState').textContent = state.modsDirty ? `${added.length} hinzugefügt, ${removed.length} entfernt – wird beim Anwenden übernommen` : 'Keine Änderungen';
    $('#mpState').classList.toggle('dirty', state.modsDirty);
    $('#mpSave').disabled = $('#mpReset').disabled = !state.modsDirty;
    $$('#msRes [data-add]').forEach((b) => { const on = refs.some((r) => r.split(':')[0].toLowerCase() === b.dataset.add.toLowerCase()); b.disabled = on; b.innerHTML = on ? `${icon('check')}Drin` : `${icon('plus')}Hinzufügen`; });
  };
  $('#mpList').addEventListener('click', (e) => {
    const b = e.target.closest('[data-toggle]'); if (!b) return;
    const r = b.dataset.toggle;
    if (refs.includes(r)) refs = refs.filter((x) => x !== r); else refs.push(r);
    renderList();
  });
  $('#mpReset').addEventListener('click', () => { refs = [...original]; renderList(); });
  $('#mpSave').addEventListener('click', async () => {
    const ok = await confirmDialog({ title: `${what} anwenden`, ok: 'Anwenden',
      body: `<p>Der Container wird mit der neuen Liste neu erstellt${RUNNING.includes(s.phase) ? ' und der Server dafür kurz neu gestartet' : ''}. Die ${what} werden beim Start von Modrinth geladen.</p>` });
    if (!ok) return;
    $('#mpSave').disabled = true;
    try { await api('PUT', `/api/servers/${s.name}/mods/projects`, { projects: refs }); state.modsDirty = false; toast(`${what} werden angewendet`); poll(); }
    catch (e) { toast(e.message, 'err'); $('#mpSave').disabled = false; }
  });

  let seq = 0;
  const search = async () => {
    const my = ++seq;
    const q = $('#msQ').value.trim(), loader = $('#msLoader').value, ver = $('#msVer').value.trim();
    $('#msRes').innerHTML = '<div class="empty-note"><span class="spinner"></span> Suche …</div>';
    try {
      const r = await api('GET', `/api/servers/${s.name}/mods/search?q=${encodeURIComponent(q)}&loader=${loader}&version=${encodeURIComponent(ver)}`);
      if (my !== seq || !$('#msRes')) return;
      r.hits.forEach((h) => { meta[h.slug] = h; });
      $('#msRes').innerHTML = r.hits.length ? r.hits.map((h) => `<div class="mod-row">${modIcon(h)}
        <div class="mod-txt"><div class="mod-title"><a href="${esc(h.url)}" target="_blank" rel="noopener">${esc(h.title)}</a><span class="dim small">${icon('download')}${fmtNum(h.downloads)}</span></div>
        <div class="mod-desc">${esc(h.description)}</div></div>
        <button class="btn sm" data-add="${esc(h.slug)}"></button></div>`).join('') + (r.total > r.hits.length ? `<div class="empty-note">${r.total} Treffer – Suche verfeinern für mehr.</div>` : '')
        : `<div class="empty-note">Nichts gefunden${ver ? ` für Minecraft ${esc(ver)}` : ''}.</div>`;
      renderList();
    } catch (e) { if (my === seq && $('#msRes')) $('#msRes').innerHTML = `<div class="form-error">${esc(e.message)}</div>`; }
  };
  let t;
  $('#msQ').addEventListener('input', () => { clearTimeout(t); t = setTimeout(search, 300); });
  $('#msLoader').addEventListener('change', search);
  $('#msVer').addEventListener('change', search);
  $('#msRes').addEventListener('click', (e) => {
    const b = e.target.closest('[data-add]'); if (!b || b.disabled) return;
    if (!refs.includes(b.dataset.add)) refs.push(b.dataset.add);
    renderList();
  });

  // Uploads & deletes
  const upload = async (files) => {
    for (const f of files) {
      if (!f.name.toLowerCase().endsWith('.jar')) { toast(`${f.name}: nur .jar-Dateien`, 'err'); continue; }
      const bar = $('#upBar'); bar.classList.remove('hidden');
      try {
        await new Promise((resolve, reject) => {
          const fd = new FormData(); fd.append('file', f);
          const x = new XMLHttpRequest();
          x.open('POST', `/api/servers/${s.name}/mods/files`);
          x.setRequestHeader('X-CraftDock', '1');
          x.upload.onprogress = (e) => { if (e.lengthComputable) bar.firstElementChild.style.width = `${(e.loaded / e.total) * 100}%`; };
          x.onload = () => { let d = {}; try { d = JSON.parse(x.responseText); } catch { /* */ } x.status < 300 ? resolve(d) : reject(new Error(d.error || x.statusText)); };
          x.onerror = () => reject(new Error('Upload fehlgeschlagen'));
          x.send(fd);
        });
        toast(`${f.name} hochgeladen – wirkt nach einem Neustart`);
      } catch (e) { toast(`${f.name}: ${e.message}`, 'err'); }
    }
    if (state.tab === 'mods' && !state.modsDirty) renderTab();
  };
  const drop = $('#drop');
  $('#upFile').addEventListener('change', (e) => upload([...e.target.files]));
  ['dragenter', 'dragover'].forEach((ev) => drop.addEventListener(ev, (e) => { e.preventDefault(); drop.classList.add('over'); }));
  ['dragleave', 'drop'].forEach((ev) => drop.addEventListener(ev, () => drop.classList.remove('over')));
  drop.addEventListener('drop', (e) => { e.preventDefault(); upload([...e.dataTransfer.files]); });
  $('#fTable').addEventListener('click', async (e) => {
    const b = e.target.closest('[data-delfile]'); if (!b) return;
    const ok = await confirmDialog({ title: 'Datei löschen', danger: true, ok: 'Löschen', body: `<p><span class="mono">${esc(b.dataset.delfile)}</span> wird aus /data/${esc(info.dir)} entfernt. Wirkt nach einem Neustart.</p>` });
    if (!ok) return;
    b.disabled = true;
    try { await api('DELETE', `/api/servers/${s.name}/mods/files?file=${encodeURIComponent(b.dataset.delfile)}`); b.closest('.ft-row').remove(); toast('Datei gelöscht'); }
    catch (er) { toast(er.message, 'err'); b.disabled = false; }
  });

  $('#cvGo').addEventListener('click', async () => {
    const ver = $('#cvVer').value.trim();
    if (!ver) { $('#cvVer').focus(); return; }
    $('#cvRes').innerHTML = '<div class="empty-note"><span class="spinner"></span> Frage Modrinth …</div>';
    try {
      const r = await api('GET', `/api/servers/${s.name}/mods/compat?version=${encodeURIComponent(ver)}&type=${$('#cvType').value}`);
      $('#cvRes').innerHTML = compatHTML(r, false);
    } catch (e) { $('#cvRes').innerHTML = `<div class="form-error">${esc(e.message)}</div>`; }
  });

  renderList();
  search();
}

/* ---------- Public site settings ---------- */
async function openSiteSettings() {
  let st;
  try { st = await api('GET', '/api/settings'); } catch (e) { toast(e.message, 'err'); return; }
  const m = openOverlay(`<div class="modal wide" role="dialog" aria-modal="true">
    <header><h2>Öffentliche Seite</h2><a class="btn sm" href="/" target="_blank" rel="noopener">${icon('external')}Öffnen</a><button class="btn ghost icon sm" data-close aria-label="Schließen">${icon('x')}</button></header>
    <div class="body">
      <label class="toggle-row"><span class="t"><b>Öffentliche Seite aktiv</b><small>Startseite unter / und Dashboards unter /s/&lt;name&gt; – ohne Anmeldung erreichbar</small></span><span class="switch"><input type="checkbox" id="stOn" ${st.publicEnabled ? 'checked' : ''}><span></span></span></label>
      <label class="field"><span>Titel</span><input id="stTitle" value="${esc(st.title)}"></label>
      <label class="field"><span>Untertitel</span><input id="stTag" value="${esc(st.tagline)}"></label>
      <label class="field"><span>Öffentlicher Hostname</span><input id="stHost" value="${esc(st.publicHost)}" placeholder="play.example.com"><span class="hint">Wird in den Verbindungsadressen angezeigt. Leer = Hostname, unter dem die Seite aufgerufen wird.</span></label>
      <label class="field"><span>Discord-Einladung</span><input id="stDiscord" value="${esc(st.discordUrl)}" placeholder="https://discord.gg/…"></label>
      <label class="toggle-row"><span class="t"><b>Spielernamen anzeigen</b><small>Sonst nur die Anzahl der Spieler</small></span><span class="switch"><input type="checkbox" id="stNames" ${st.showPlayerNames ? 'checked' : ''}><span></span></span></label>
    </div>
    <footer><button class="btn" data-close>Abbrechen</button><button class="btn primary" id="stSave">${icon('save')}Speichern</button></footer>
  </div>`);
  $('#stSave', m.el).addEventListener('click', async () => {
    try {
      await api('PUT', '/api/settings', {
        publicEnabled: $('#stOn', m.el).checked, title: $('#stTitle', m.el).value, tagline: $('#stTag', m.el).value,
        publicHost: $('#stHost', m.el).value, discordUrl: $('#stDiscord', m.el).value, showPlayerNames: $('#stNames', m.el).checked,
      });
      m.close(); toast('Einstellungen gespeichert');
    } catch (e) { toast(e.message, 'err'); }
  });
}

/* =====================================================================
   v1.2 – side navigation & pages
   ===================================================================== */
const PAGES = {
  network: { label: 'Netzwerke', icon: 'network', perm: () => can('networks') || (state.networks || []).length > 0, render: renderNetworkPage },
  news: { label: 'Neuigkeiten', icon: 'news', perm: 'news', render: renderNewsPage },
  users: { label: 'Benutzer & Rollen', icon: 'shield', perm: () => can('users') || can('roles') || can('audit'), render: renderUsersPage },
  alerts: { label: 'Alarmierung', icon: 'bell', perm: 'alerts', render: renderAlertsPage },
};
const pageAllowed = (p) => typeof p.perm === 'function' ? p.perm() : can(p.perm);

function renderSideNav() {
  const nav = $('#sideNav');
  const items = Object.entries(PAGES).filter(([, p]) => pageAllowed(p))
    .map(([id, p]) => `<button class="side-link ${state.page === id ? 'active' : ''}" data-page="${id}">${icon(p.icon)}${p.label}</button>`);
  if (can('settings')) items.push(`<button class="side-link" data-site>${icon('globe')}Öffentliche Seite</button>`, `<button class="side-link" data-hosting>${icon('sliders')}Hosting</button>`);
  nav.innerHTML = items.join('');
  $$('[data-page]', nav).forEach((b) => b.addEventListener('click', () => { closeNav(); openPage(b.dataset.page); }));
  $('[data-site]', nav)?.addEventListener('click', () => { closeNav(); openSiteSettings(); });
  $('[data-hosting]', nav)?.addEventListener('click', () => { closeNav(); openHostingSettings(); });
}

function stopPageTimer() { clearInterval(state.pageTimer); state.pageTimer = null; if (state.pageES) { state.pageES.close(); state.pageES = null; } }

async function openPage(id) {
  if (state.page !== id && !(await guardDirty())) return;
  stopPageTimer();
  state.page = id;
  setHash();
  renderSidebar();
  renderSideNav();
  renderMain();
}

function renderPage() {
  const main = $('#main');
  main.dataset.view = 'page';
  const pg = PAGES[state.page];
  main.innerHTML = `${mobileBar()}<header class="srv-head"><div class="srv-title"><div class="type-badge">${icon(pg.icon)}</div><div><h1>${pg.label}</h1></div></div><div class="btn-group" id="pgActions"></div></header>
    <section class="tab-body page-body" id="pgBody"><div class="empty-note"><span class="spinner"></span> Lade …</div></section>`;
  $('#btnNav')?.addEventListener('click', openNav);
  pg.render($('#pgBody'));
}

const fmtTime = (unix) => unix ? new Date(unix * 1000).toLocaleString('de-DE', { dateStyle: 'medium', timeStyle: 'short' }) : '—';
const fmtPlay = (min) => min >= 60 ? `${Math.floor(min / 60)} Std ${min % 60} Min` : `${min} Min`;
const avatar = (n, size = 28) => `<img class="pav" src="https://mc-heads.net/avatar/${encodeURIComponent(n)}/${size * 2}" alt="" loading="lazy" width="${size}" height="${size}">`;
document.addEventListener('error', (e) => {
  const img = e.target;
  if (img instanceof HTMLImageElement && img.classList.contains('pav') && !img.dataset.failed) {
    img.dataset.failed = '1';
    img.src = 'data:image/svg+xml,' + encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 8 8"><rect width="8" height="8" fill="#2f3a48"/><rect x="2" y="3" width="1" height="1" fill="#a9b3bf"/><rect x="5" y="3" width="1" height="1" fill="#a9b3bf"/></svg>');
  }
}, true);

/* ---------- compatibility ---------- */
function compatHTML(r, compact) {
  const miss = r.items.filter((i) => i.status === 'missing'), unk = r.items.filter((i) => i.status === 'unknown');
  const head = r.items.length
    ? (miss.length ? `<div class="form-error"><b>${miss.length} von ${r.items.length} Modrinth-Projekten</b> gibt es noch nicht für ${esc(r.loader)} ${esc(r.gameVersion)}.</div>`
      : `<div class="ok-note">${icon('check')} Alle ${r.items.length} Modrinth-Projekte sind für ${esc(r.loader)} ${esc(r.gameVersion)} verfügbar.</div>`)
    : '<p class="dim">Keine Modrinth-Projekte zu prüfen.</p>';
  const rows = (compact ? [...miss, ...unk] : r.items).map((i) => `<div class="cmp-row ${i.status}">
    ${i.iconUrl ? `<img src="${esc(i.iconUrl)}" alt="">` : `<span class="ph">${esc((i.title || '?')[0].toUpperCase())}</span>`}
    <span class="grow"><b>${esc(i.title)}</b>${i.version ? `<small class="mono">${esc(i.version)}</small>` : ''}</span>
    <span class="tag ${i.status === 'ok' ? 'new' : i.status === 'missing' ? 'del' : ''}">${i.status === 'ok' ? 'passt' : i.status === 'missing' ? 'fehlt' : 'unbekannt'}</span></div>`).join('');
  const unchecked = r.unchecked.length ? `<p class="hint-text">${r.unchecked.length} eigene JAR-Datei(en) können nicht geprüft werden: ${r.unchecked.map(esc).join(', ')}</p>` : '';
  return `<div class="compat">${head}${rows ? `<div class="cmp-list">${rows}</div>` : ''}${unchecked}</div>`;
}

/* ---------- players tab ---------- */
async function renderPlayersTab(body) {
  const s = current();
  if (!s.containerId) { body.innerHTML = '<div class="empty"><div class="box"><p>Der Container existiert noch nicht.</p></div></div>'; return; }
  body.innerHTML = '<div class="empty-note"><span class="spinner"></span> Spielerdaten werden geladen …</div>';
  let d;
  try { d = await api('GET', `/api/servers/${s.name}/playerdata`); } catch (e) { body.innerHTML = `<div class="banner">${icon('alert')}<div>${esc(e.message)}</div></div>`; return; }
  if (state.tab !== 'players' || state.selected !== s.name) return;
  const edit = (canS('players.manage') || canS('players.cheat')) && d.running;
  const manage = canS('players.manage') && d.running, cheat = canS('players.cheat') && d.running;
  const act = (a, p, label, cls = '') => manage ? `<button class="btn sm ${cls}" data-pa="${a}" data-p="${esc(p)}">${label}</button>` : '';
  const opsSet = new Set(d.ops.map((o) => o.name.toLowerCase())), wlSet = new Set(d.whitelist.map((n) => n.toLowerCase()));
  const cfgNote = (n, list) => list.map((x) => x.toLowerCase()).includes(n.toLowerCase()) ? '<span class="tag" title="Steht in der Konfiguration und wird beim Neustart wieder gesetzt">Konfig</span>' : '';
  const statusNote = !d.running ? `<div class="banner info">${icon('alert')}<div>Der Server läuft nicht – Listen sind lesbar, Aktionen gehen erst nach dem Start.</div></div>`
    : d.sleeping ? `<div class="banner info">${icon('moon')}<div>Der Server schläft. Eine Aktion weckt ihn auf.</div></div>` : '';
  body.innerHTML = `<div class="stack">
    ${statusNote}
    <div class="grid two">
      <div class="panel"><div class="panel-head"><h3>${icon('users')}Online</h3><span class="dim num">${d.online.length} / ${d.max}</span></div>
        <div class="panel-body"><div class="plist">${d.online.length ? d.online.map((n) => `<div class="prow clickable" data-pm="${esc(n)}" title="Spieler-Aktionen öffnen">${avatar(n)}<b class="grow">${esc(n)}</b>${opsSet.has(n.toLowerCase()) ? '<span class="tag new">OP</span>' : ''}
          ${cheat ? `<button class="btn sm" data-pm="${esc(n)}" data-tab="items">${icon('box')}<span class="lbl">Item</span></button><button class="btn sm" data-pm="${esc(n)}" data-tab="tp">${icon('send')}<span class="lbl">Teleport</span></button>` : ''}
          <button class="btn sm ${edit ? 'primary' : ''}" data-pm="${esc(n)}">${icon('sliders')}<span class="lbl">${edit ? 'Aktionen' : 'Details'}</span></button></div>`).join('') : '<div class="empty-note">Niemand online.</div>'}</div></div></div>
      <div class="panel"><div class="panel-head"><h3>${icon('shield')}Whitelist</h3>
          <div class="btn-group"><span class="tag ${d.whitelistOn ? 'new' : ''}">${d.whitelistOn ? 'aktiv' : 'aus'}</span>${manage ? `<button class="btn sm" data-pa="${d.whitelistOn ? 'whitelist-off' : 'whitelist-on'}" data-p="">${d.whitelistOn ? 'Ausschalten' : 'Einschalten'}</button>` : ''}</div></div>
        <div class="panel-body">${manage ? `<form class="addrow" data-add="whitelist-add"><input placeholder="Spielername" required pattern="[A-Za-z0-9_.\\-]{1,40}"><button class="btn sm primary">${icon('plus')}Hinzufügen</button></form>` : ''}
          <div class="plist">${d.whitelist.length ? d.whitelist.map((n) => `<div class="prow">${avatar(n, 24)}<span class="grow">${esc(n)}</span>${cfgNote(n, d.configWhitelist)}${act('whitelist-remove', n, icon('x'), 'ghost icon')}</div>`).join('') : '<div class="empty-note">Leer.</div>'}</div></div></div>
      <div class="panel"><div class="panel-head"><h3>${icon('star')}Operatoren</h3></div>
        <div class="panel-body">${manage ? `<form class="addrow" data-add="op"><input placeholder="Spielername" required pattern="[A-Za-z0-9_.\\-]{1,40}"><button class="btn sm primary">${icon('plus')}OP geben</button></form>` : ''}
          <div class="plist">${d.ops.length ? d.ops.map((o) => `<div class="prow">${avatar(o.name, 24)}<span class="grow">${esc(o.name)} <small class="dim">Level ${o.level}</small></span>${cfgNote(o.name, d.configOps)}${act('deop', o.name, icon('x'), 'ghost icon')}</div>`).join('') : '<div class="empty-note">Keine Operatoren.</div>'}</div></div></div>
      <div class="panel"><div class="panel-head"><h3>${icon('ban')}Gebannt</h3></div>
        <div class="panel-body">${manage ? `<form class="addrow" data-add="ban"><input placeholder="Spielername" required pattern="[A-Za-z0-9_.\\-]{1,40}"><input placeholder="Grund (optional)" data-reason><button class="btn sm danger">Bannen</button></form>` : ''}
          <div class="plist">${d.banned.length ? d.banned.map((b) => `<div class="prow">${avatar(b.name, 24)}<span class="grow">${esc(b.name)}<small class="dim block">${esc(b.reason || '')}${b.source ? ' · von ' + esc(b.source) : ''}</small></span>${act('pardon', b.name, 'Entbannen')}</div>`).join('') : '<div class="empty-note">Niemand gebannt.</div>'}</div></div></div>
    </div>
    <div class="panel"><div class="panel-head"><h3>${icon('clock')}Spielzeit</h3><input class="mini-search" id="psQ" placeholder="Spieler suchen …"></div>
      <div class="panel-body">${d.stats.length ? `<table class="tbl" id="psTbl"><thead><tr><th>Spieler</th><th>Spielzeit</th><th>Zuletzt online</th><th>Zuerst gesehen</th></tr></thead><tbody>
        ${d.stats.map((r, i) => `<tr data-q="${esc(r.name.toLowerCase())}"><td><span class="rank">${i + 1}</span>${avatar(r.name, 22)}${esc(r.name)}</td><td class="num">${fmtPlay(r.minutes)}</td><td>${fmtTime(r.last)}</td><td>${fmtTime(r.first)}</td></tr>`).join('')}</tbody></table>`
        : '<div class="empty-note">Noch keine Daten – CraftDock zählt die Spielzeit jede Minute mit.</div>'}</div></div>
    ${d.configOps.length || d.configWhitelist.length ? `<p class="hint-text">Einträge mit „Konfig“ stehen in der Server-Konfiguration und werden beim nächsten Start wieder gesetzt. Dauerhaft entfernen: in der Konfiguration unter „Zugang“.</p>` : ''}
  </div>`;
  body.addEventListener('click', (e) => {
    const b = e.target.closest('[data-pm]');
    if (!b) return;
    e.stopPropagation();
    openPlayerModal(s, b.dataset.pm, { online: d.online, ops: opsSet, running: d.running, tab: b.dataset.tab, onChange: () => { if (state.tab === 'players') renderTab(); } });
  });
  const run = async (action, player, reason = '') => {
    try {
      const r = await api('POST', `/api/servers/${s.name}/playeraction`, { action, player, reason });
      toast(r.output || 'Erledigt');
      setTimeout(() => { if (state.tab === 'players') renderTab(); }, 600);
    } catch (e) { toast(e.message, 'err'); }
  };
  body.addEventListener('click', (e) => { const b = e.target.closest('[data-pa]'); if (b) run(b.dataset.pa, b.dataset.p); });
  $$('form.addrow', body).forEach((f) => f.addEventListener('submit', (e) => {
    e.preventDefault();
    const name = $('input', f).value.trim();
    run(f.dataset.add, name, $('[data-reason]', f)?.value.trim() || (f.dataset.add === 'ban' ? ' ' : ''));
  }));
  $('#psQ')?.addEventListener('input', (e) => { const q = e.target.value.toLowerCase(); $$('#psTbl tbody tr').forEach((tr) => tr.classList.toggle('hidden', !tr.dataset.q.includes(q))); });
}

/* ---------- world tab ---------- */
function renderWorld(body) {
  const s = current(), c = s.config;
  const level = c.extraEnv?.LEVEL || 'world';
  const ltOptions = SECTIONS.find((x) => x.id === 'world').fields[1].options;
  body.innerHTML = `<div class="stack">
    <div class="grid two">
      <div class="panel"><div class="panel-head"><h3>${icon('earth')}Aktuelle Welt</h3>${c.map ? `<a class="btn sm" href="/map/${encodeURIComponent(s.name)}/" target="_blank" rel="noopener">${icon('map')}Live-Karte</a>` : ''}</div>
        <div class="panel-body"><dl class="kv">
          <dt>Ordner</dt><dd class="mono">/data/${esc(level)}</dd>
          <dt>Seed</dt><dd class="mono">${esc(c.seed || 'zufällig')}</dd>
          <dt>Welttyp</dt><dd>${esc(ltOptions.find((o) => o[0] === c.levelType)?.[1] || c.levelType || 'Standard')}</dd>
          <dt>Daten</dt><dd class="mono">${esc(c.dataPath || `Volume mc-${s.name}-data`)}</dd></dl>
          ${canS('server.backup') ? `<div class="btn-row"><button class="btn" id="wBackup">${icon('download')}Backup herunterladen</button></div>` : ''}</div></div>
      ${canS('server.world') ? `<div class="panel"><div class="panel-head"><h3>${icon('refresh')}Neue Welt erzeugen</h3></div>
        <div class="panel-body stack-sm">
          <p class="muted small">Löscht die aktuelle Welt (inkl. Nether und End) und erzeugt beim nächsten Start eine neue. Spielerdaten in der Welt gehen verloren, Plugins/Mods und Einstellungen bleiben.</p>
          <label class="field"><span>Seed</span><div class="inline"><input id="wSeed" value="${esc(c.seed)}" placeholder="leer = zufällig" spellcheck="false"><button class="btn" id="wRand" type="button" title="Zufälligen Seed setzen">🎲</button></div></label>
          <label class="field"><span>Welttyp</span><select id="wType">${ltOptions.map(([v, l]) => `<option value="${esc(v)}" ${v === c.levelType ? 'selected' : ''}>${esc(l)}</option>`).join('')}</select></label>
          <div><button class="btn danger" id="wReset" ${s.phase === 'busy' ? 'disabled' : ''}>${icon('trash')}Welt zurücksetzen …</button></div>
        </div></div>` : ''}
    </div>
    ${canS('server.world') ? `<div class="panel"><div class="panel-head"><h3>${icon('upload')}Welt importieren</h3></div>
      <div class="panel-body">
        <label class="drop" id="wDrop"><input type="file" id="wFile" accept=".zip,.tar,.tar.gz,.tgz" hidden>
          ${icon('upload')}<span><b>Welt-Archiv hierher ziehen</b> oder klicken</span>
          <small>.zip oder .tar.gz mit dem Weltordner (level.dat). Singleplayer-Welten, Server-Welten und Bukkit-Layouts (world, world_nether, world_the_end) werden erkannt. Max. 8 GB.</small>
          <span class="up-bar hidden" id="wBar"><i></i></span></label>
        <p class="hint-text">Die aktuelle Welt wird dabei ersetzt. Ein laufender Server wird gestoppt und danach wieder gestartet.</p>
      </div></div>` : ''}
  </div>`;
  $('#wBackup')?.addEventListener('click', doBackup);
  if (!canS('server.world')) return;
  $('#wRand').addEventListener('click', () => {
    const a = new BigInt64Array(1); crypto.getRandomValues(a); $('#wSeed').value = a[0].toString();
  });
  $('#wReset').addEventListener('click', async () => {
    const ok = await typedConfirm({ title: 'Welt zurücksetzen', word: s.name, ok: 'Welt löschen & neu erzeugen',
      body: `<p>Die Welt <span class="mono">${esc(level)}</span> von <b>${esc(s.name)}</b> wird <b>unwiderruflich gelöscht</b>.${canS('server.backup') ? ' Lade vorher am besten ein Backup herunter.' : ''}</p>` });
    if (!ok) return;
    try {
      await api('POST', `/api/servers/${s.name}/world/reset`, { seed: $('#wSeed').value.trim(), levelType: $('#wType').value });
      toast('Neue Welt wird vorbereitet'); poll();
    } catch (e) { toast(e.message, 'err'); }
  });
  const upload = async (file) => {
    if (!file) return;
    if (!/\.(zip|tar|tar\.gz|tgz)$/i.test(file.name)) { toast('Bitte .zip oder .tar.gz auswählen', 'err'); return; }
    const ok = await typedConfirm({ title: 'Welt importieren', word: s.name, ok: 'Importieren',
      body: `<p><b>${esc(file.name)}</b> (${fmtBytes(file.size)}) ersetzt die aktuelle Welt von <b>${esc(s.name)}</b>.</p>` });
    if (!ok) return;
    const bar = $('#wBar'); bar.classList.remove('hidden');
    try {
      await xhrUpload(`/api/servers/${s.name}/world/import`, file, (p) => { bar.firstElementChild.style.width = `${p}%`; });
      toast('Upload fertig – die Welt wird jetzt eingespielt'); poll();
    } catch (e) { toast(e.message, 'err'); }
    bar.classList.add('hidden');
  };
  const drop = $('#wDrop');
  $('#wFile').addEventListener('change', (e) => upload(e.target.files[0]));
  ['dragenter', 'dragover'].forEach((ev) => drop.addEventListener(ev, (e) => { e.preventDefault(); drop.classList.add('over'); }));
  ['dragleave', 'drop'].forEach((ev) => drop.addEventListener(ev, () => drop.classList.remove('over')));
  drop.addEventListener('drop', (e) => { e.preventDefault(); upload(e.dataTransfer.files[0]); });
}

function xhrUpload(url, file, onProgress) {
  return new Promise((resolve, reject) => {
    const fd = new FormData(); fd.append('file', file);
    const x = new XMLHttpRequest();
    x.open('POST', url);
    x.setRequestHeader('X-CraftDock', '1');
    x.upload.onprogress = (e) => { if (e.lengthComputable) onProgress((e.loaded / e.total) * 100); };
    x.onload = () => { let d = {}; try { d = JSON.parse(x.responseText); } catch { /* */ } x.status < 300 ? resolve(d) : reject(new Error(d.error || x.statusText)); };
    x.onerror = () => reject(new Error('Upload fehlgeschlagen'));
    x.send(fd);
  });
}

function typedConfirm({ title, body, word, ok }) {
  return new Promise((resolve) => {
    let done = false;
    const m = openOverlay(`<div class="modal" role="dialog" aria-modal="true">
      <header><h2>${esc(title)}</h2><button class="btn ghost icon sm" data-close aria-label="Schließen">${icon('x')}</button></header>
      <div class="body">${body}<label class="field"><span>Zur Bestätigung <b class="mono">${esc(word)}</b> eingeben</span><input id="tcIn" autocomplete="off"></label></div>
      <footer><button class="btn" data-close>Abbrechen</button><button class="btn danger solid" id="tcOk" disabled>${esc(ok)}</button></footer></div>`,
    { onClose: () => { if (!done) resolve(false); } });
    const inp = $('#tcIn', m.el), b = $('#tcOk', m.el);
    inp.focus();
    inp.addEventListener('input', () => { b.disabled = inp.value !== word; });
    b.addEventListener('click', () => { done = true; m.close(); resolve(true); });
  });
}

/* ---------- clone & embed ---------- */
function openClone() {
  const s = current();
  const m = openOverlay(`<div class="modal" role="dialog" aria-modal="true">
    <header><h2>Server klonen</h2><button class="btn ghost icon sm" data-close aria-label="Schließen">${icon('x')}</button></header>
    <div class="body">
      <p>Legt eine Kopie von <b>${esc(s.name)}</b> mit gleicher Konfiguration, Mods und – auf Wunsch – Welt an. Ideal, um ein Update erst an einer Testkopie auszuprobieren.</p>
      <label class="field"><span>Name der Kopie</span><input id="clName" value="${esc(s.name)}-test" spellcheck="false"></label>
      <label class="field"><span>Host-Port</span><input id="clPort" type="number" value="${advFor(null) || !state.me?.hosting?.portMin ? nextFreePort() : ''}" placeholder="automatisch"></label>
      <label class="toggle-row"><span class="t"><b>Welt und Dateien kopieren</b><small>Sonst startet die Kopie mit einer neuen Welt</small></span><span class="switch"><input type="checkbox" id="clData" checked><span></span></span></label>
      <label class="toggle-row"><span class="t"><b>Nach dem Klonen starten</b></span><span class="switch"><input type="checkbox" id="clStart"><span></span></span></label>
      <p class="hint-text">Die Kopie ist nicht öffentlich und nicht im Velocity-Netzwerk. Zusätzliche Ports werden nicht übernommen.</p>
    </div>
    <footer><button class="btn" data-close>Abbrechen</button><button class="btn primary" id="clGo">${icon('clone')}Klonen</button></footer></div>`);
  $('#clGo', m.el).addEventListener('click', async () => {
    const name = $('#clName', m.el).value.trim().toLowerCase();
    try {
      await api('POST', `/api/servers/${s.name}/clone`, { name, port: +$('#clPort', m.el).value, copyData: $('#clData', m.el).checked, start: $('#clStart', m.el).checked });
      m.close(); toast(`${name} wird angelegt`); await poll(); select(name, 'overview');
    } catch (e) { toast(e.message, 'err'); }
  });
}

function openEmbed() {
  const s = current(), base = location.origin;
  const snippets = (theme) => {
    const q = theme === 'light' ? '?theme=light' : '';
    return [
      ['Widget (iframe)', `<iframe src="${base}/widget/${s.name}${q}" width="380" height="190" style="border:0;border-radius:14px" title="${s.config.displayName || s.name} – Serverstatus" loading="lazy"></iframe>`],
      ['Alle Server (iframe)', `<iframe src="${base}/widget${q}" width="380" height="420" style="border:0;border-radius:14px" title="Serverstatus" loading="lazy"></iframe>`],
      ['Badge (Markdown)', `[![${s.config.displayName || s.name}](${base}/badge/${s.name}.svg)](${base}/s/${s.name})`],
      ['Badge (HTML)', `<a href="${base}/s/${s.name}"><img src="${base}/badge/${s.name}.svg" alt="${s.config.displayName || s.name}"></a>`],
      ['JSON-API', `${base}/api/public/servers/${s.name}`],
    ];
  };
  const m = openOverlay(`<div class="modal wide xl" role="dialog" aria-modal="true">
    <header><h2>Einbetten</h2><button class="btn ghost icon sm" data-close aria-label="Schließen">${icon('x')}</button></header>
    <div class="body">
      <div class="embed-top"><div class="seg" id="emTheme"><button class="on" data-t="dark">Dunkel</button><button data-t="light">Hell</button></div>
        <img class="badge-prev" src="/badge/${encodeURIComponent(s.name)}.svg?ts=${Date.now()}" alt="Badge-Vorschau"></div>
      <iframe class="embed-prev" id="emPrev" src="/widget/${encodeURIComponent(s.name)}" title="Vorschau"></iframe>
      <div id="emSnips"></div>
    </div></div>`);
  const draw = (theme) => {
    $('#emPrev', m.el).src = `/widget/${encodeURIComponent(s.name)}${theme === 'light' ? '?theme=light' : ''}`;
    $('#emSnips', m.el).innerHTML = snippets(theme).map(([l, code], i) => `<div class="snip"><div class="snip-h"><span>${l}</span><button class="btn ghost sm" data-ci="${i}">${icon('copy')}Kopieren</button></div><pre>${esc(code)}</pre></div>`).join('');
    $$('[data-ci]', m.el).forEach((b) => b.addEventListener('click', () => copyText(snippets(theme)[+b.dataset.ci][1])));
  };
  $$('#emTheme button', m.el).forEach((b) => b.addEventListener('click', () => { $$('#emTheme button', m.el).forEach((x) => x.classList.toggle('on', x === b)); draw(b.dataset.t); }));
  draw('dark');
}

/* ---------- networks page ---------- */
async function renderNetworkPage(body) {
  let v;
  try { v = await api('GET', '/api/proxies'); } catch (e) { body.innerHTML = `<div class="banner">${icon('alert')}<div>${esc(e.message)}</div></div>`; return; }
  if (state.page !== 'network') return;
  state.networks = v.networks.map((n) => n.config);
  state.proxyInfo = v;
  const edit = can('networks');
  if (!v.available) {
    $('#pgActions').innerHTML = '';
    body.innerHTML = `<div class="empty"><div class="box"><div class="art">${icon('network')}</div><h2>Velocity-Netzwerke</h2>
      <p>Ein Proxy bündelt mehrere Server hinter einer Adresse – Spieler wechseln im Spiel mit <span class="mono">/server &lt;name&gt;</span>. Dafür müssen alle Server in einem gemeinsamen Docker-Netzwerk laufen.</p>
      <p class="dim">Setze <span class="mono">CRAFTDOCK_NETWORK</span> (in der mitgelieferten docker-compose.yml bereits <span class="mono">minecraft</span>) und starte CraftDock neu.</p></div></div>`;
    return;
  }
  $('#pgActions').innerHTML = edit ? `<button class="btn primary" id="nwNew">${icon('plus')}<span class="lbl">Neues Netzwerk</span></button>` : '';
  $('#nwNew')?.addEventListener('click', () => openNetworkCreate(v));
  if (!v.networks.length) {
    body.innerHTML = `<div class="empty"><div class="box"><div class="art">${icon('network')}</div><h2>Noch kein Netzwerk</h2>
      <p>Ein Netzwerk ist ein eigener Velocity-Proxy mit eigener Adresse und eigenen Servern – z. B. „Survival-Netzwerk“ auf Port 25565 und „Event-Netzwerk“ auf Port 25570. Paper, Purpur und Folia werden automatisch konfiguriert, Fabric und Quilt bekommen FabricProxy-Lite.</p>
      ${edit ? `<button class="btn primary" id="nwFirst">${icon('plus')}Erstes Netzwerk anlegen</button>` : ''}</div></div>`;
    $('#nwFirst')?.addEventListener('click', () => openNetworkCreate(v));
    return;
  }
  if (!v.networks.some((n) => n.config.id === state.netSel)) state.netSel = v.networks[0].config.id;
  const nv = v.networks.find((n) => n.config.id === state.netSel);
  const pc = nv.config;
  const running = nv.state === 'running';
  const status = (n) => n.task?.running ? ['busy', 'Wird eingerichtet'] : n.state === 'running' ? (n.reachable ? ['online', 'Online'] : ['starting', 'Startet']) : n.exists ? ['stopped', 'Gestoppt'] : ['error', 'Kein Container'];
  const addr = (c) => c.publicAddress || (c.port === 25565 ? location.hostname : `${location.hostname}:${c.port}`);
  const capable = v.servers.filter((x) => x.capable);
  const members = v.servers.filter((x) => x.network === pc.id);
  body.innerHTML = `<div class="stack">
    <div class="netlist">${v.networks.map((n) => { const [cls, lbl] = status(n); return `<button class="netcard-a ${n.config.id === pc.id ? 'sel' : ''}" data-net="${esc(n.config.id)}">
      <div class="nc-top"><span class="nc-ic">${icon('network')}</span><span class="grow"><b>${esc(n.config.name || n.config.id)}</b><small class="mono">${esc(n.config.id)}</small></span><span class="pill ${cls}"><span class="dot ${cls}"></span>${lbl}</span></div>
      <div class="nc-addr mono">${esc(addr(n.config))}</div>
      <div class="nc-meta"><span>${icon('users')}${n.reachable ? n.online : '–'} online</span><span>${icon('server')}${n.members.length} Server</span>${n.config.hidden ? `<span>${icon('globe')}nicht öffentlich</span>` : ''}</div></button>`; }).join('')}</div>

    ${nv.task && nv.task.running ? `<div class="taskbar running flat"><span class="spinner"></span><span class="msg">${esc(nv.task.message)}</span></div>` : ''}
    ${nv.task && nv.task.error ? `<div class="taskbar failed flat">${icon('alert')}<span class="msg">${esc(nv.task.error)}</span></div>` : ''}

    <div class="panel net-detail"><div class="panel-head"><h3>${icon('network')}${esc(pc.name || pc.id)}<small class="dim mono">mc-velocity${pc.id === 'main' ? '' : '-' + esc(pc.id)}</small></h3>
      ${edit ? `<div class="btn-group">${nv.exists ? (running ? `<button class="btn sm" data-pact="restart">${icon('restart')}Neustart</button><button class="btn sm" data-pact="stop">${icon('stop')}Stoppen</button>`
        : `<button class="btn sm primary" data-pact="start">${icon('play')}Starten</button>`) : ''}
        ${nv.exists ? `<button class="btn sm icon" id="pxLogs" title="Log anzeigen">${icon('terminal')}</button>` : ''}
        <button class="btn sm icon danger" id="pxDel" title="Netzwerk löschen">${icon('trash')}</button></div>` : ''}</div>
      <div class="grid two nopad-grid">
        <div class="fgrid tight" id="pxForm">
          <label class="field"><span>Name</span><input data-px="name" value="${esc(pc.name)}" placeholder="${esc(pc.id)}" ${edit ? '' : 'disabled'}></label>
          <label class="field"><span>Host-Port</span><input type="number" data-px="port" value="${pc.port}" ${edit ? '' : 'disabled'}></label>
          <label class="field full"><span>MOTD</span><input data-px="motd" value="${esc(pc.motd)}" ${edit ? '' : 'disabled'}><span class="hint">MiniMessage erlaubt, z. B. &lt;green&gt;Willkommen</span></label>
          <label class="field"><span>Lobby (Startserver)</span><select data-px="lobby" ${edit ? '' : 'disabled'}><option value="">automatisch</option>${members.map((x) => `<option ${x.name === pc.lobby ? 'selected' : ''}>${esc(x.name)}</option>`).join('')}</select></label>
          <label class="field"><span>Max. Spieler (Anzeige)</span><input type="number" data-px="maxPlayers" value="${pc.maxPlayers}" ${edit ? '' : 'disabled'}></label>
          <label class="field"><span>Öffentliche Adresse</span><input data-px="publicAddress" value="${esc(pc.publicAddress)}" placeholder="play.example.com" ${edit ? '' : 'disabled'}></label>
          <label class="field"><span>Speicher</span><input data-px="memory" value="${esc(pc.memory)}" ${edit ? '' : 'disabled'}></label>
          <label class="toggle-row full"><span class="t"><b>Online-Modus</b><small>Mojang/Microsoft-Konten am Proxy prüfen (empfohlen)</small></span><span class="switch"><input type="checkbox" data-px="onlineMode" ${pc.onlineMode ? 'checked' : ''} ${edit ? '' : 'disabled'}><span></span></span></label>
          <label class="toggle-row full"><span class="t"><b>Auf der öffentlichen Seite zeigen</b><small>Netzwerk-Adresse auf Startseite und Dashboards</small></span><span class="switch"><input type="checkbox" data-px="shown" ${pc.hidden ? '' : 'checked'} ${edit ? '' : 'disabled'}><span></span></span></label>
          <div class="field full"><span class="field-label">Forced Hosts</span><div class="rows" id="pxHosts">${Object.entries(pc.forcedHosts || {}).map(([h, srv]) => hostRow(h, srv, members)).join('')}</div>
            ${edit ? `<div><button class="btn sm" id="pxAddHost" type="button" ${members.length ? '' : 'disabled title="Erst Server ins Netzwerk aufnehmen"'}>${icon('plus')}Hostname hinzufügen</button></div>` : ''}<span class="hint">z. B. survival.example.com → survival: Spieler landen direkt auf diesem Server.</span></div>
          ${edit ? `<div class="full"><button class="btn primary" id="pxSave">${icon('save')}${nv.exists ? 'Speichern & Proxy neu starten' : 'Speichern & Proxy anlegen'}</button></div>` : ''}
        </div>
        <div class="net-servers">
          <div class="field-label">Server</div>
          <div class="plist">${capable.length ? capable.map((x) => `<div class="prow"><span class="dot ${x.phase}"></span><span class="grow"><b>${esc(x.displayName || x.name)}</b> <small class="dim">${esc(typeOf(x.type).label)}</small></span>
            ${x.network === pc.id ? '<span class="tag new">in diesem Netzwerk</span>' : x.network ? `<span class="tag" title="Ein Server kann nur in einem Netzwerk sein">in ${esc(netName(x.network))}</span>` : '<span class="tag">nicht verbunden</span>'}
            ${canS('server.advanced', state.servers.find((y) => y.name === x.name)) ? `<button class="btn ghost sm" data-move="${esc(x.name)}" data-to="${x.network === pc.id ? '' : esc(pc.id)}">${x.network === pc.id ? 'Entfernen' : 'Aufnehmen'}</button>` : ''}</div>`).join('')
            : '<div class="empty-note">Keine kompatiblen Server. Velocity-Forwarding unterstützen Paper, Purpur, Folia, Fabric und Quilt.</div>'}</div>
          ${v.servers.some((x) => !x.capable) ? `<p class="hint-text">Nicht kompatibel und daher nicht aufgeführt: ${v.servers.filter((x) => !x.capable).map((x) => esc(x.name)).join(', ')}.</p>` : ''}
          <p class="hint-text">Server im Netzwerk veröffentlichen keinen eigenen Port mehr und laufen im Offline-Modus – die Anmeldung prüft der Proxy. Aufnehmen oder Entfernen erstellt den Server-Container neu (ein laufender Server startet kurz neu).</p>
        </div>
      </div></div>
  </div>`;

  $$('[data-net]', body).forEach((b) => b.addEventListener('click', () => { state.netSel = b.dataset.net; renderNetworkPage(body); }));
  $$('[data-pact]', body).forEach((b) => b.addEventListener('click', async () => {
    try { await api('POST', `/api/proxies/${pc.id}/actions/${b.dataset.pact}`); toast('Wird ausgeführt'); setTimeout(() => renderNetworkPage(body), 1500); } catch (e) { toast(e.message, 'err'); }
  }));
  $('#pxLogs')?.addEventListener('click', () => openProxyLogs(pc.id));
  $('#pxDel')?.addEventListener('click', async () => {
    if (members.length) { toast(`Im Netzwerk sind noch Server (${members.map((x) => x.name).join(', ')}). Nimm sie zuerst heraus.`, 'err'); return; }
    const ok = await typedConfirm({ title: 'Netzwerk löschen', word: pc.id, ok: 'Löschen', body: `<p>Der Proxy <span class="mono">${esc(pc.name || pc.id)}</span> wird gestoppt und samt Daten entfernt.</p>` });
    if (!ok) return;
    try { await api('POST', `/api/proxies/${pc.id}/actions/delete`); toast('Netzwerk wird gelöscht'); state.netSel = null; setTimeout(() => renderNetworkPage(body), 1200); } catch (e) { toast(e.message, 'err'); }
  });
  $$('[data-move]', body).forEach((b) => b.addEventListener('click', () => moveToNetwork(b.dataset.move, b.dataset.to, () => renderNetworkPage(body))));
  $('#pxAddHost')?.addEventListener('click', () => $('#pxHosts').insertAdjacentHTML('beforeend', hostRow('', '', members)));
  $('#pxHosts').addEventListener('click', (e) => { const d = e.target.closest('[data-del]'); if (d) d.closest('[data-row]').remove(); });
  $('#pxSave')?.addEventListener('click', async () => {
    const f = $('#pxForm'), val = (k) => $(`[data-px=${k}]`, f);
    const hosts = {};
    $$('#pxHosts [data-row]').forEach((r) => { const h = $('input', r).value.trim().toLowerCase(), sv = $('select', r).value; if (h && sv) hosts[h] = sv; });
    try {
      await api('PUT', `/api/proxies/${pc.id}`, { name: val('name').value, port: +val('port').value, maxPlayers: +val('maxPlayers').value, motd: val('motd').value, lobby: val('lobby').value,
        memory: val('memory').value, publicAddress: val('publicAddress').value, onlineMode: val('onlineMode').checked, hidden: !val('shown').checked, forcedHosts: hosts });
      toast('Proxy wird neu eingerichtet');
      setTimeout(() => renderNetworkPage(body), 1200);
    } catch (e) { toast(e.message, 'err'); }
  });
  if (!state.pageTimer) state.pageTimer = setInterval(() => { if (state.page === 'network' && !document.hidden && !document.activeElement?.closest?.('#pxForm') && !$('.overlay')) renderNetworkPage($('#pgBody')); }, 8000);
}

async function moveToNetwork(name, to, done) {
  const srv = state.servers.find((x) => x.name === name);
  if (!srv) return;
  const running = RUNNING.includes(srv.phase);
  const ok = await confirmDialog({ title: to ? 'Ins Netzwerk aufnehmen' : 'Aus dem Netzwerk entfernen', ok: to ? 'Aufnehmen' : 'Entfernen',
    body: to ? `<p><b>${esc(name)}</b> wird Teil von <b>${esc(netName(to))}</b>${srv.config.network ? ` (und verlässt ${esc(netName(srv.config.network))})` : ''}. Der Server ist danach nur noch über den Proxy erreichbar.${running ? ' Er startet dafür kurz neu.' : ''}</p>`
      : `<p><b>${esc(name)}</b> verlässt das Netzwerk und ist danach wieder direkt über Port ${srv.config.port} erreichbar.${running ? ' Er startet dafür kurz neu.' : ''}</p>` });
  if (!ok) return;
  try {
    await api('PUT', `/api/servers/${name}`, { ...srv.config, network: to, proxyMember: undefined });
    toast(to ? `${name} wird aufgenommen` : `${name} wird entfernt`);
    await poll();
    setTimeout(done, 800);
  } catch (e) { toast(e.message, 'err'); }
}

function openNetworkCreate(v) {
  const used = new Set([...v.networks.map((n) => n.config.port), ...state.servers.filter((x) => !x.config.network).map((x) => x.config.port)]);
  let port = 25565; while (used.has(port)) port++;
  const first = !v.networks.length;
  const m = openOverlay(`<div class="modal" role="dialog" aria-modal="true">
    <header><h2>Neues Netzwerk</h2><button class="btn ghost icon sm" data-close aria-label="Schließen">${icon('x')}</button></header>
    <div class="body">
      <p>Jedes Netzwerk ist ein eigener Velocity-Proxy mit eigener Adresse, eigener Lobby und eigenen Servern.</p>
      <label class="field"><span>Name</span><input id="ncName" placeholder="${first ? 'Hauptnetzwerk' : 'Event-Netzwerk'}" maxlength="48"></label>
      <label class="field"><span>Kennung</span><input id="ncId" placeholder="${first ? 'main' : 'events'}" spellcheck="false" maxlength="31"><span class="hint">Kleinbuchstaben, Ziffern, Bindestriche. Container heißt mc-velocity-&lt;kennung&gt; – nicht änderbar.</span></label>
      <label class="field"><span>Host-Port</span><input id="ncPort" type="number" value="${port}"><span class="hint">Port, über den Spieler das Netzwerk erreichen. Er darf von keinem Server und keinem anderen Netzwerk belegt sein.</span></label>
    </div>
    <footer><button class="btn" data-close>Abbrechen</button><button class="btn primary" id="ncGo">${icon('plus')}Anlegen & starten</button></footer></div>`);
  const name = $('#ncName', m.el), id = $('#ncId', m.el);
  let touched = false;
  id.addEventListener('input', () => { touched = true; });
  name.addEventListener('input', () => {
    if (!touched) id.value = name.value.toLowerCase().normalize('NFD').replace(/[̀-ͯ]/g, '').replace(/ß/g, 'ss').replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 31);
  });
  name.focus();
  $('#ncGo', m.el).addEventListener('click', async () => {
    try {
      const r = await api('POST', '/api/proxies', { id: id.value || (first ? 'main' : ''), name: name.value, port: +$('#ncPort', m.el).value, onlineMode: true, maxPlayers: 100, memory: '512M' });
      m.close(); toast('Netzwerk wird eingerichtet'); state.netSel = r.id;
      await loadNetworks();
      renderNetworkPage($('#pgBody'));
    } catch (e) { toast(e.message, 'err'); }
  });
}

function hostRow(h, srv, members) {
  return `<div class="row-edit env" data-row><input value="${esc(h)}" placeholder="survival.example.com" spellcheck="false">
    <select>${members.map((x) => `<option ${x.name === srv ? 'selected' : ''}>${esc(x.name)}</option>`).join('')}</select>
    <button type="button" class="btn icon ghost" data-del aria-label="Entfernen">${icon('x')}</button></div>`;
}

function openProxyLogs(id) {
  let es;
  const m = openOverlay(`<div class="modal wide xl" role="dialog" aria-modal="true">
    <header><h2>Proxy-Log · ${esc(netName(id))}</h2><button class="btn ghost icon sm" data-close aria-label="Schließen">${icon('x')}</button></header>
    <div class="body"><div class="console-out modal-log" id="pxOut"></div></div></div>`, { onClose: () => es && es.close() });
  const out = $('#pxOut', m.el);
  es = new EventSource(`/api/proxies/${encodeURIComponent(id)}/logs`);
  es.onopen = () => { out.innerHTML = ''; };
  es.onmessage = (e) => { const d = document.createElement('div'); d.className = 'l'; d.textContent = e.data.replace(ANSI, ''); out.appendChild(d); out.scrollTop = out.scrollHeight; };
  es.addEventListener('end', () => es.close());
}

/* ---------- news page ---------- */
async function renderNewsPage(body) {
  let list;
  try { list = await api('GET', '/api/news'); } catch (e) { body.innerHTML = `<div class="banner">${icon('alert')}<div>${esc(e.message)}</div></div>`; return; }
  if (state.page !== 'news') return;
  $('#pgActions').innerHTML = `<button class="btn primary" id="nwNew">${icon('plus')}Neuer Beitrag</button>`;
  $('#nwNew').addEventListener('click', () => openNewsEditor(null));
  const now = Date.now() / 1000;
  body.innerHTML = list.length ? `<div class="news-list">${list.map((n) => {
    const past = n.kind === 'event' && (n.end || n.start + 10800) < now;
    return `<article class="panel news-item ${past ? 'past' : ''}"><div class="panel-body">
      <div class="news-meta"><span class="tag ${n.kind === 'event' ? 'ev' : 'new'}">${n.kind === 'event' ? 'Event' : 'News'}</span>${n.pinned ? '<span class="tag">angeheftet</span>' : ''}
        ${n.server ? `<span class="chip">${esc(n.server)}</span>` : ''}<span class="dim small">${n.kind === 'event' ? `${icon('calendar')} ${fmtTime(n.start)}${n.end ? ' – ' + fmtTime(n.end) : ''}${past ? ' · vorbei' : ''}` : fmtTime(n.published)}${n.author ? ' · ' + esc(n.author) : ''}</span>
        <span class="grow"></span><button class="btn ghost sm" data-ed="${n.id}">Bearbeiten</button><button class="btn ghost icon sm" data-del="${n.id}" title="Löschen">${icon('trash')}</button></div>
      <h3>${esc(n.title)}</h3>${n.body ? `<p class="news-body">${esc(n.body)}</p>` : ''}</div></article>`;
  }).join('')}</div>` : `<div class="empty"><div class="box"><div class="art">${icon('news')}</div><h2>Noch keine Beiträge</h2><p>Neuigkeiten und Events erscheinen auf der öffentlichen Startseite, Events auch im Dashboard des jeweiligen Servers – mit Countdown.</p></div></div>`;
  $$('[data-ed]', body).forEach((b) => b.addEventListener('click', () => openNewsEditor(list.find((n) => n.id === b.dataset.ed))));
  $$('[data-del]', body).forEach((b) => b.addEventListener('click', async () => {
    if (!(await confirmDialog({ title: 'Beitrag löschen', body: '<p>Wirklich löschen?</p>', ok: 'Löschen', danger: true }))) return;
    try { await api('DELETE', `/api/news/${b.dataset.del}`); renderNewsPage(body); } catch (e) { toast(e.message, 'err'); }
  }));
}

const toLocalInput = (unix) => { if (!unix) return ''; const d = new Date(unix * 1000); d.setMinutes(d.getMinutes() - d.getTimezoneOffset()); return d.toISOString().slice(0, 16); };
const fromLocalInput = (v) => v ? Math.floor(new Date(v).getTime() / 1000) : 0;

function openNewsEditor(n) {
  const isNew = !n;
  n = n || { kind: 'news', title: '', body: '', server: '', pinned: false, start: 0, end: 0 };
  const m = openOverlay(`<div class="modal wide" role="dialog" aria-modal="true">
    <header><h2>${isNew ? 'Neuer Beitrag' : 'Beitrag bearbeiten'}</h2><button class="btn ghost icon sm" data-close aria-label="Schließen">${icon('x')}</button></header>
    <div class="body">
      <div class="seg" id="neKind"><button data-k="news" class="${n.kind === 'news' ? 'on' : ''}">Neuigkeit</button><button data-k="event" class="${n.kind === 'event' ? 'on' : ''}">Event</button></div>
      <label class="field"><span>Titel</span><input id="neTitle" value="${esc(n.title)}" maxlength="120"></label>
      <label class="field"><span>Text</span><textarea id="neBody" rows="6" maxlength="5000">${esc(n.body)}</textarea><span class="hint">Einfacher Text, Zeilenumbrüche bleiben erhalten.</span></label>
      <div class="fgrid tight nopad">
        <label class="field"><span>Server</span><select id="neServer"><option value="">Alle / allgemein</option>${state.servers.map((x) => `<option ${x.name === n.server ? 'selected' : ''}>${esc(x.name)}</option>`).join('')}</select></label>
        <label class="toggle-row"><span class="t"><b>Anheften</b><small>Steht oben</small></span><span class="switch"><input type="checkbox" id="nePin" ${n.pinned ? 'checked' : ''}><span></span></span></label>
        <label class="field ev-only"><span>Beginn</span><input type="datetime-local" id="neStart" value="${toLocalInput(n.start)}"></label>
        <label class="field ev-only"><span>Ende (optional)</span><input type="datetime-local" id="neEnd" value="${toLocalInput(n.end)}"></label>
      </div>
    </div>
    <footer><button class="btn" data-close>Abbrechen</button><button class="btn primary" id="neSave">${icon('save')}Veröffentlichen</button></footer></div>`);
  let kind = n.kind;
  const sync = () => $$('.ev-only', m.el).forEach((e) => e.classList.toggle('hidden', kind !== 'event'));
  sync();
  $$('#neKind button', m.el).forEach((b) => b.addEventListener('click', () => { kind = b.dataset.k; $$('#neKind button', m.el).forEach((x) => x.classList.toggle('on', x === b)); sync(); }));
  $('#neSave', m.el).addEventListener('click', async () => {
    const payload = { kind, title: $('#neTitle', m.el).value, body: $('#neBody', m.el).value, server: $('#neServer', m.el).value, pinned: $('#nePin', m.el).checked,
      start: kind === 'event' ? fromLocalInput($('#neStart', m.el).value) : 0, end: kind === 'event' ? fromLocalInput($('#neEnd', m.el).value) : 0 };
    try {
      await api(isNew ? 'POST' : 'PUT', isNew ? '/api/news' : `/api/news/${n.id}`, payload);
      m.close(); toast('Gespeichert'); renderNewsPage($('#pgBody'));
    } catch (e) { toast(e.message, 'err'); }
  });
}

/* ---------- player action modal ---------- */
const ITEMS = [
  ['Werkzeuge & Waffen', [['netherite_sword', 'Netheritschwert'], ['diamond_sword', 'Diamantschwert'], ['iron_sword', 'Eisenschwert'], ['netherite_pickaxe', 'Netheritspitzhacke'], ['diamond_pickaxe', 'Diamantspitzhacke'], ['iron_pickaxe', 'Eisenspitzhacke'],
    ['diamond_axe', 'Diamantaxt'], ['diamond_shovel', 'Diamantschaufel'], ['bow', 'Bogen'], ['crossbow', 'Armbrust'], ['trident', 'Dreizack'], ['mace', 'Streitkolben'], ['arrow', 'Pfeil'], ['shield', 'Schild'], ['fishing_rod', 'Angel'], ['shears', 'Schere'], ['flint_and_steel', 'Feuerzeug']]],
  ['Rüstung', [['netherite_helmet', 'Netherithelm'], ['netherite_chestplate', 'Netheritharnisch'], ['netherite_leggings', 'Netherithose'], ['netherite_boots', 'Netheritstiefel'], ['diamond_helmet', 'Diamanthelm'], ['diamond_chestplate', 'Diamantharnisch'],
    ['diamond_leggings', 'Diamanthose'], ['diamond_boots', 'Diamantstiefel'], ['iron_chestplate', 'Eisenharnisch'], ['elytra', 'Elytren'], ['turtle_helmet', 'Schildkrötenpanzer']]],
  ['Essen', [['cooked_beef', 'Steak'], ['golden_carrot', 'Goldene Karotte'], ['bread', 'Brot'], ['cooked_porkchop', 'Gebratenes Schweinefleisch'], ['baked_potato', 'Ofenkartoffel'], ['golden_apple', 'Goldener Apfel'], ['enchanted_golden_apple', 'Verzauberter goldener Apfel'], ['cake', 'Kuchen']]],
  ['Rohstoffe', [['diamond', 'Diamant'], ['netherite_ingot', 'Netheritbarren'], ['emerald', 'Smaragd'], ['iron_ingot', 'Eisenbarren'], ['gold_ingot', 'Goldbarren'], ['copper_ingot', 'Kupferbarren'], ['coal', 'Kohle'], ['redstone', 'Redstone'],
    ['lapis_lazuli', 'Lapislazuli'], ['quartz', 'Netherquarz'], ['amethyst_shard', 'Amethystscherbe'], ['ender_pearl', 'Enderperle'], ['blaze_rod', 'Lohenrute'], ['slime_ball', 'Schleimball'], ['string', 'Faden'], ['leather', 'Leder'], ['experience_bottle', 'Erfahrungsfläschchen']]],
  ['Blöcke', [['oak_log', 'Eichenstamm'], ['oak_planks', 'Eichenholzbretter'], ['spruce_log', 'Fichtenstamm'], ['cobblestone', 'Bruchstein'], ['stone', 'Stein'], ['stone_bricks', 'Steinziegel'], ['deepslate_bricks', 'Tiefenschieferziegel'], ['glass', 'Glas'],
    ['torch', 'Fackel'], ['lantern', 'Laterne'], ['white_wool', 'Weiße Wolle'], ['sand', 'Sand'], ['dirt', 'Erde'], ['grass_block', 'Grasblock'], ['obsidian', 'Obsidian'], ['crying_obsidian', 'Weinender Obsidian'], ['tnt', 'TNT'], ['chest', 'Truhe'], ['barrel', 'Fass'],
    ['crafting_table', 'Werkbank'], ['furnace', 'Ofen'], ['anvil', 'Amboss'], ['enchanting_table', 'Zaubertisch'], ['bookshelf', 'Bücherregal'], ['beacon', 'Leuchtfeuer'], ['shulker_box', 'Shulker-Kiste'], ['respawn_anchor', 'Seelenanker']]],
  ['Sonstiges', [['totem_of_undying', 'Totem der Unsterblichkeit'], ['name_tag', 'Namensschild'], ['saddle', 'Sattel'], ['lead', 'Leine'], ['bucket', 'Eimer'], ['water_bucket', 'Wassereimer'], ['lava_bucket', 'Lavaeimer'], ['firework_rocket', 'Feuerwerksrakete'],
    ['enchanted_book', 'Verzaubertes Buch'], ['map', 'Leere Karte'], ['compass', 'Kompass'], ['clock', 'Uhr'], ['spyglass', 'Fernrohr'], ['book', 'Buch'], ['writable_book', 'Buch und Feder'], ['white_bed', 'Weißes Bett'], ['oak_boat', 'Eichenboot'], ['minecart', 'Lore'], ['rail', 'Schiene'], ['ender_chest', 'Endertruhe']]],
];
const EFFECTS = [['speed', 'Schnelligkeit'], ['haste', 'Eile'], ['strength', 'Stärke'], ['jump_boost', 'Sprungkraft'], ['regeneration', 'Regeneration'], ['resistance', 'Resistenz'], ['fire_resistance', 'Feuerresistenz'], ['water_breathing', 'Unterwasseratmung'],
  ['invisibility', 'Unsichtbarkeit'], ['night_vision', 'Nachtsicht'], ['health_boost', 'Extraenergie'], ['absorption', 'Absorption'], ['saturation', 'Sättigung'], ['glowing', 'Leuchten'], ['slow_falling', 'Sanfter Fall'], ['conduit_power', 'Meereskraft'],
  ['dolphins_grace', 'Gunst des Delfins'], ['luck', 'Glück'], ['slowness', 'Langsamkeit'], ['mining_fatigue', 'Abbaulähmung'], ['nausea', 'Übelkeit'], ['blindness', 'Blindheit'], ['darkness', 'Dunkelheit'], ['hunger', 'Hunger'], ['weakness', 'Schwäche'], ['poison', 'Vergiftung'], ['levitation', 'Schwebekraft']];
const ENCHANTS = [
  ['Rüstung', [['protection', 'Schutz', 4], ['fire_protection', 'Feuerschutz', 4], ['blast_protection', 'Explosionsschutz', 4], ['projectile_protection', 'Schusssicher', 4], ['feather_falling', 'Federfall', 4],
    ['thorns', 'Dornen', 3], ['respiration', 'Atmung', 3], ['aqua_affinity', 'Wasseraffinität', 1], ['depth_strider', 'Wasserläufer', 3], ['frost_walker', 'Eisläufer', 2], ['soul_speed', 'Seelenläufer', 3], ['swift_sneak', 'Huschen', 3]]],
  ['Waffen', [['sharpness', 'Schärfe', 5], ['smite', 'Bann', 5], ['bane_of_arthropods', 'Nemesis der Gliederfüßer', 5], ['knockback', 'Rückstoß', 2], ['fire_aspect', 'Verbrennung', 2], ['looting', 'Plünderung', 3], ['sweeping_edge', 'Schwungkraft', 3],
    ['density', 'Dichte', 5], ['breach', 'Durchbruch', 4], ['wind_burst', 'Windstoß', 3]]],
  ['Werkzeuge', [['efficiency', 'Effizienz', 5], ['silk_touch', 'Behutsamkeit', 1], ['fortune', 'Glück', 3], ['luck_of_the_sea', 'Glück des Meeres', 3], ['lure', 'Köder', 3]]],
  ['Fernkampf', [['power', 'Stärke', 5], ['punch', 'Schlag', 2], ['flame', 'Flamme', 1], ['infinity', 'Unendlichkeit', 1], ['multishot', 'Mehrfachschuss', 1], ['quick_charge', 'Schnellladen', 3], ['piercing', 'Durchschuss', 4],
    ['loyalty', 'Treue', 3], ['impaling', 'Harpune', 5], ['riptide', 'Sog', 3], ['channeling', 'Entladung', 1]]],
  ['Allgemein', [['unbreaking', 'Haltbarkeit', 3], ['mending', 'Reparatur', 1], ['binding_curse', 'Fluch der Bindung', 1], ['vanishing_curse', 'Fluch des Verschwindens', 1]]],
];
const ENCH_MAX = Object.fromEntries(ENCHANTS.flatMap(([, l]) => l.map(([id, , max]) => [id, max])));
const ENCH_NAME = Object.fromEntries(ENCHANTS.flatMap(([, l]) => l.map(([id, n]) => [id, n])));
// Item presets: sensible enchantments for common gear (max levels).
const ENCH_PRESETS = {
  sword: ['sharpness', 'looting', 'sweeping_edge', 'fire_aspect', 'unbreaking', 'mending'],
  pickaxe: ['efficiency', 'fortune', 'unbreaking', 'mending'],
  axe: ['efficiency', 'sharpness', 'unbreaking', 'mending'],
  shovel: ['efficiency', 'unbreaking', 'mending'],
  helmet: ['protection', 'respiration', 'aqua_affinity', 'unbreaking', 'mending'],
  chestplate: ['protection', 'unbreaking', 'mending'],
  leggings: ['protection', 'swift_sneak', 'unbreaking', 'mending'],
  boots: ['protection', 'feather_falling', 'depth_strider', 'unbreaking', 'mending'],
  bow: ['power', 'punch', 'flame', 'infinity', 'unbreaking'],
  crossbow: ['quick_charge', 'multishot', 'unbreaking', 'mending'],
  trident: ['loyalty', 'impaling', 'channeling', 'unbreaking', 'mending'],
  mace: ['density', 'wind_burst', 'unbreaking', 'mending'],
  fishing_rod: ['luck_of_the_sea', 'lure', 'unbreaking', 'mending'],
  elytra: ['unbreaking', 'mending'],
};
const presetFor = (item) => { const id = item.replace(/^minecraft:/, ''); const k = Object.keys(ENCH_PRESETS).find((p) => id === p || id.endsWith('_' + p)); return k ? ENCH_PRESETS[k] : null; };
/** Same syntax rules as the backend (playeractions.go) – only used for the preview. */
function giveSyntax(item, ench, version) {
  const ids = Object.keys(ench).sort();
  const full = item.includes(':') ? item : `minecraft:${item}`;
  if (!ids.length) return full;
  const book = full === 'minecraft:enchanted_book';
  const v = (/^(\d+)\.(\d+)(?:\.(\d+))?$/.exec(version || '') || []).slice(1).map((n) => +(n || 0));
  const atLeast = (a, b, c) => !v.length || v[0] > a || (v[0] === a && (v[1] > b || (v[1] === b && v[2] >= c)));
  const ns = (id) => (id.includes(':') ? id : `minecraft:${id}`);
  if (!atLeast(1, 20, 5)) return `${full}{${book ? 'StoredEnchantments' : 'Enchantments'}:[${ids.map((id) => `{id:"${ns(id)}",lvl:${ench[id]}s}`).join(',')}]}`;
  const inner = `{${ids.map((id) => `"${ns(id)}":${ench[id]}`).join(',')}}`;
  return `${full}[${book ? 'stored_enchantments' : 'enchantments'}=${atLeast(1, 21, 5) ? inner : `{levels:${inner}}`}]`;
}
const DIMS = [['overworld', 'Oberwelt'], ['the_nether', 'Nether'], ['the_end', 'Ende']];
const GM = [['survival', 'Überleben'], ['creative', 'Kreativ'], ['adventure', 'Abenteuer'], ['spectator', 'Zuschauer']];
const itemName = (id) => { for (const [, list] of ITEMS) for (const [i, n] of list) if (i === id || `minecraft:${i}` === id) return n; return id; };

function openPlayerModal(s, player, { online = [], ops = new Set(), running = true, tab = 'items', onChange } = {}) {
  const edit = (canS('players.manage', s) || canS('players.cheat', s)) && running;
  const others = online.filter((n) => n !== player);
  const tabs = [['items', 'Items', 'box'], ['tp', 'Teleport', 'send'], ['state', 'Zustand', 'star'], ['msg', 'Nachricht', 'news'], ['mod', 'Moderation', 'shield']]
    .filter(([id]) => ['items', 'tp', 'state'].includes(id) ? canS('players.cheat', s) : canS('players.manage', s));
  if (!tabs.some((t) => t[0] === tab)) tab = tabs[0]?.[0] || 'items';
  const isOp = ops.has(player.toLowerCase());
  const m = openOverlay(`<div class="modal pm" role="dialog" aria-modal="true" aria-label="Aktionen für ${esc(player)}">
    <header class="pm-head"><img class="pm-av" src="https://mc-heads.net/head/${encodeURIComponent(player)}/96" alt="">
      <div class="grow"><h2>${esc(player)} ${isOp ? '<span class="tag new">OP</span>' : ''}</h2><div class="pm-info" id="pmInfo"><span class="spinner"></span> Live-Daten …</div></div>
      <button class="btn ghost icon sm" id="pmRefresh" title="Aktualisieren">${icon('refresh')}</button>
      <button class="btn ghost icon sm" data-close aria-label="Schließen">${icon('x')}</button></header>
    ${edit ? `<nav class="pm-tabs">${tabs.map(([id, l, ic]) => `<button class="${id === tab ? 'on' : ''}" data-pt="${id}">${icon(ic)}${l}</button>`).join('')}</nav>` : ''}
    <div class="body pm-body" id="pmBody"></div>
    <div class="pm-foot" id="pmOut" aria-live="polite"></div>
  </div>`);
  const av = $('.pm-av', m.el);
  av.addEventListener('error', () => { const ph = document.createElement('span'); ph.className = 'pm-av ph'; ph.textContent = player[0].toUpperCase(); av.replaceWith(ph); }, { once: true });

  let info = null;
  const out = (msg, err) => { const o = $('#pmOut', m.el); o.className = 'pm-foot ' + (err ? 'err' : 'ok'); o.innerHTML = `${icon(err ? 'alert' : 'check')}<span>${esc(msg)}</span>`; };
  const loadInfo = async () => {
    const el = $('#pmInfo', m.el);
    if (!running) { el.innerHTML = '<span class="dim">Server läuft nicht</span>'; return; }
    try {
      info = await api('GET', `/api/servers/${s.name}/playerinfo?player=${encodeURIComponent(player)}`);
      const dim = DIMS.find(([d]) => info.dimension === `minecraft:${d}`)?.[1] || info.dimension || '';
      el.innerHTML = [
        info.pos ? `<span title="Position">${icon('globe')}<b class="mono">${info.pos.map((v) => Math.round(v)).join(' / ')}</b> ${esc(dim)}</span>` : '',
        info.health != null ? `<span title="Leben">❤ <b>${String(info.health).replace('.', ',')}</b>/20</span>` : '',
        info.food != null ? `<span title="Hunger">🍗 <b>${info.food}</b>/20</span>` : '',
        info.level != null ? `<span title="Erfahrungslevel">✦ Level <b>${info.level}</b></span>` : '',
        info.gamemode ? `<span>${esc(GM.find(([g]) => g === info.gamemode)?.[1] || info.gamemode)}</span>` : '',
      ].join('');
      if (curTab === 'tp' && !$('#tpX', m.el)?.value) draw();
      if (curTab === 'state') $$('[data-gm]', m.el).forEach((b) => b.classList.toggle('on', b.dataset.gm === info.gamemode));
    } catch (e) { el.innerHTML = `<span class="dim">${esc(e.message)}</span>`; }
  };
  const act = async (payload, okMsg, btn) => {
    if (btn) { btn.disabled = true; }
    try {
      const r = await api('POST', `/api/servers/${s.name}/playeraction`, { player, ...payload });
      out(r.output || okMsg || 'Erledigt');
      setTimeout(loadInfo, 400);
      return true;
    } catch (e) { out(e.message, true); return false; } finally { if (btn) btn.disabled = false; }
  };

  let curTab = tab;
  const draw = () => {
    const b = $('#pmBody', m.el);
    if (!edit) {
      b.innerHTML = `<p class="muted">${running ? 'Du kannst die Live-Daten sehen, aber für Aktionen fehlt dir die Berechtigung „Spielerverwaltung“.' : 'Aktionen gehen nur bei laufendem Server.'}</p>`;
      return;
    }
    $$('[data-pt]', m.el).forEach((x) => x.classList.toggle('on', x.dataset.pt === curTab));
    if (curTab === 'items') {
      b.innerHTML = `<div class="pm-row"><input id="itQ" placeholder="Item suchen oder ID eingeben, z. B. diamond oder create:wrench" spellcheck="false" autocomplete="off">
          <div class="count"><button class="btn sm" data-c="1">1</button><button class="btn sm" data-c="16">16</button><button class="btn sm" data-c="64">64</button><input id="itN" type="number" min="1" max="6400" value="1"></div></div>
        <div class="items" id="itList"></div>
        <details class="ench" id="enBox"><summary>${icon('star')}<span>Verzauberungen</span><small class="dim" id="enSum">optional</small></summary>
          <div class="ench-body">
            <div class="ench-rows" id="enRows"></div>
            <div class="pm-row"><select id="enAdd"><option value="">Verzauberung hinzufügen …</option>${ENCHANTS.map(([g, l]) => `<optgroup label="${g}">${l.map(([id, n, max]) => `<option value="${id}">${n} (max. ${max})</option>`).join('')}</optgroup>`).join('')}<option value="__custom">Andere ID (z. B. aus Mods) …</option></select>
              <button class="btn sm" id="enPreset" type="button" disabled title="Passende Verzauberungen in Höchststufe">${icon('star')}Beste für dieses Item</button></div>
            <form class="pm-row hidden" id="enCustomRow"><input id="enCustom" placeholder="z. B. minecraft:sharpness oder mod:enchant" spellcheck="false"><button class="btn sm primary">${icon('plus')}Hinzufügen</button></form>
            <div class="cmd-prev mono" id="enPrev"></div>
          </div></details>
        <div class="pm-actions"><span class="grow dim small" id="itSel">Kein Item gewählt</span><button class="btn primary" id="itGive" disabled>${icon('box')}Geben</button></div>`;
      let sel = '';
      const ench = {}; // id → level
      const drawEnch = () => {
        $('#enRows', m.el).innerHTML = Object.keys(ench).map((id) => `<div class="ench-row" data-en="${esc(id)}"><span class="grow"><b>${esc(ENCH_NAME[id] || id)}</b> <small class="mono dim">${esc(id.includes(':') ? id : 'minecraft:' + id)}</small></span>
          <span class="lvl"><button type="button" class="btn sm icon" data-lv="-1" aria-label="Stufe senken">−</button><input type="number" min="1" max="255" value="${ench[id]}" aria-label="Stufe"><button type="button" class="btn sm icon" data-lv="1" aria-label="Stufe erhöhen">+</button></span>
          ${ench[id] > (ENCH_MAX[id] || 255) ? `<span class="tag ev" title="Über der normalen Höchststufe ${ENCH_MAX[id]} – funktioniert per Befehl trotzdem">über max.</span>` : ''}
          <button type="button" class="btn ghost sm icon" data-rm aria-label="Entfernen">${icon('x')}</button></div>`).join('') || '<div class="dim small">Keine – das Item wird ohne Verzauberung gegeben.</div>';
        const n = Object.keys(ench).length;
        $('#enSum', m.el).textContent = n ? `${n} gewählt` : 'optional';
        const book = sel.replace(/^minecraft:/, '') === 'enchanted_book';
        $('#enPrev', m.el).textContent = sel ? `/give ${player} ${giveSyntax(sel, ench, s.gameVersion)} ${$('#itN', m.el)?.value || 1}` : '';
        $('#enPrev', m.el).title = s.gameVersion ? `Syntax für Minecraft ${s.gameVersion}${book ? ' (Buch: gespeicherte Verzauberungen)' : ''}` : 'Syntax für aktuelle Minecraft-Versionen';
        $('#enPreset', m.el).disabled = !presetFor(sel || '');
      };
      $('#enAdd', m.el).addEventListener('change', (e) => {
        let id = e.target.value;
        e.target.value = '';
        if (id === '__custom') { $('#enCustomRow', m.el).classList.remove('hidden'); $('#enCustom', m.el).focus(); return; }
        if (id && !ench[id]) { ench[id] = ENCH_MAX[id] || 1; drawEnch(); }
      });
      $('#enCustomRow', m.el).addEventListener('submit', (e) => {
        e.preventDefault();
        const id = $('#enCustom', m.el).value.trim().toLowerCase().replace(/^minecraft:/, '');
        if (!/^([a-z0-9_.-]+:)?[a-z0-9_./-]+$/.test(id)) { out('Ungültige Verzauberungs-ID', true); return; }
        if (!ench[id]) ench[id] = ENCH_MAX[id] || 1;
        $('#enCustom', m.el).value = ''; $('#enCustomRow', m.el).classList.add('hidden');
        drawEnch();
      });
      $('#enPreset', m.el).addEventListener('click', () => { (presetFor(sel) || []).forEach((id) => { ench[id] = ENCH_MAX[id]; }); drawEnch(); });
      $('#enRows', m.el).addEventListener('click', (e) => {
        const row = e.target.closest('[data-en]'); if (!row) return;
        const id = row.dataset.en;
        if (e.target.closest('[data-rm]')) delete ench[id];
        else if (e.target.closest('[data-lv]')) ench[id] = Math.min(255, Math.max(1, ench[id] + +e.target.closest('[data-lv]').dataset.lv));
        else return;
        drawEnch();
      });
      $('#enRows', m.el).addEventListener('change', (e) => {
        const row = e.target.closest('[data-en]');
        if (row && e.target.matches('input')) { ench[row.dataset.en] = Math.min(255, Math.max(1, Math.round(+e.target.value) || 1)); drawEnch(); }
      });
      const list = $('#itList', m.el), q = $('#itQ', m.el), n = $('#itN', m.el);
      const pick = (id) => {
        sel = id;
        $$('.it', list).forEach((x) => x.classList.toggle('on', x.dataset.it === id));
        $('#itSel', m.el).innerHTML = id ? `<b>${esc(itemName(id))}</b> <span class="mono">${esc(id.includes(':') ? id : 'minecraft:' + id)}</span>` : 'Kein Item gewählt';
        $('#itGive', m.el).disabled = !id;
        if (id && id.replace(/^minecraft:/, '') === 'enchanted_book') $('#enBox', m.el).open = true;
        drawEnch();
      };
      const render = () => {
        const term = q.value.trim().toLowerCase();
        const groups = ITEMS.map(([g, items]) => [g, items.filter(([id, name]) => !term || id.includes(term.replace(/^minecraft:/, '').replace(/ /g, '_')) || name.toLowerCase().includes(term))]).filter(([, items]) => items.length);
        // Offer the raw input as an ID if it looks like one (namespace or underscore) or nothing matched by name.
        const custom = /^([a-z0-9_.-]+:)?[a-z0-9_./-]+$/.test(term) && (term.includes(':') || term.includes('_') || !groups.length)
          && !groups.some(([, items]) => items.some(([id]) => id === term || `minecraft:${id}` === term));
        list.innerHTML = (custom ? `<div class="it-g">Eigene ID</div><button class="it custom ${sel === term ? 'on' : ''}" data-it="${esc(term)}"><span class="mono">${esc(term)}</span></button>` : '')
          + groups.map(([g, items]) => `<div class="it-g">${g}</div>${items.map(([id, name]) => `<button class="it ${sel === id ? 'on' : ''}" data-it="${id}" title="minecraft:${id}">${esc(name)}</button>`).join('')}`).join('')
          || '<div class="empty-note">Nichts gefunden – gib eine Item-ID ein (z. B. minecraft:diamond).</div>';
      };
      render();
      q.addEventListener('input', render);
      q.addEventListener('keydown', (e) => { if (e.key === 'Enter') { const first = $('.it', list); if (first) { pick(first.dataset.it); $('#itGive', m.el).focus(); } } });
      list.addEventListener('click', (e) => { const x = e.target.closest('.it'); if (x) pick(x.dataset.it); });
      $$('[data-c]', m.el).forEach((x) => x.addEventListener('click', () => { n.value = x.dataset.c; drawEnch(); }));
      n.addEventListener('input', drawEnch);
      $('#itGive', m.el).addEventListener('click', (e) => {
        const k = Object.keys(ench).length;
        act({ action: 'give', item: sel, count: +n.value, enchantments: k ? { ...ench } : undefined }, `${n.value}× ${itemName(sel)}${k ? ` mit ${k} Verzauberung${k > 1 ? 'en' : ''}` : ''} gegeben`, e.currentTarget);
      });
      drawEnch();
      q.focus();
    } else if (curTab === 'tp') {
      const pos = info?.pos || ['', '', ''];
      const dimNow = (info?.dimension || 'minecraft:overworld').replace('minecraft:', '');
      b.innerHTML = `<div class="pm-sec"><h4>Zu einem Spieler</h4>${others.length ? `<div class="pm-row"><select id="tpTarget">${others.map((n) => `<option>${esc(n)}</option>`).join('')}</select>
          <button class="btn" id="tpTo">${icon('send')}${esc(player)} → Spieler</button><button class="btn" id="tpHere">Spieler → ${esc(player)}</button></div>` : '<p class="dim small">Sonst ist niemand online.</p>'}</div>
        <div class="pm-sec"><h4>Zu Koordinaten</h4><div class="coords"><label>X<input id="tpX" type="number" step="any" value="${pos[0] !== '' ? Math.round(pos[0]) : ''}"></label><label>Y<input id="tpY" type="number" step="any" value="${pos[1] !== '' ? Math.round(pos[1]) : ''}"></label>
          <label>Z<input id="tpZ" type="number" step="any" value="${pos[2] !== '' ? Math.round(pos[2]) : ''}"></label><label>Dimension<select id="tpDim">${DIMS.map(([d, l]) => `<option value="${d}" ${d === dimNow ? 'selected' : ''}>${l}</option>`).join('')}</select></label></div>
          <div class="pm-actions"><span class="grow dim small">${info?.pos ? 'Vorausgefüllt mit der aktuellen Position.' : ''}</span><button class="btn primary" id="tpGo">${icon('globe')}Teleportieren</button></div></div>`;
      $('#tpTo', m.el)?.addEventListener('click', (e) => act({ action: 'tp-to', target: $('#tpTarget', m.el).value }, '', e.currentTarget));
      $('#tpHere', m.el)?.addEventListener('click', (e) => act({ action: 'tp-here', target: $('#tpTarget', m.el).value }, '', e.currentTarget));
      $('#tpGo', m.el).addEventListener('click', (e) => {
        const num = (id) => { const v = $(id, m.el).value; return v === '' ? null : Number(v); };
        act({ action: 'tp-coords', x: num('#tpX'), y: num('#tpY'), z: num('#tpZ'), dimension: $('#tpDim', m.el).value }, 'Teleportiert', e.currentTarget);
      });
    } else if (curTab === 'state') {
      b.innerHTML = `<div class="pm-sec"><h4>Spielmodus</h4><div class="seg wide">${GM.map(([g, l]) => `<button data-gm="${g}" class="${info?.gamemode === g ? 'on' : ''}">${l}</button>`).join('')}</div></div>
        <div class="pm-sec"><h4>Schnell</h4><div class="pm-row"><button class="btn" id="stHeal">❤ Heilen &amp; sättigen</button><button class="btn" id="stClearFx">Alle Effekte entfernen</button></div></div>
        <div class="pm-sec"><h4>Effekt geben</h4><div class="pm-grid3">
          <label class="field"><span>Effekt</span><select id="fxId">${EFFECTS.map(([id, l]) => `<option value="${id}">${l}</option>`).join('')}</select></label>
          <label class="field"><span>Dauer (Sekunden)</span><input id="fxSec" type="number" min="1" value="60"></label>
          <label class="field"><span>Stufe</span><select id="fxAmp">${[1, 2, 3, 4, 5, 10].map((n) => `<option value="${n - 1}">${n}</option>`).join('')}</select></label></div>
          <div class="pm-actions"><label class="chk"><input type="checkbox" id="fxHide"> Partikel ausblenden</label><span class="grow"></span><button class="btn primary" id="fxGo">Effekt geben</button></div></div>
        <div class="pm-sec"><h4>Erfahrung</h4><div class="pm-row"><input id="xpN" type="number" value="5" class="narrow"><select id="xpU" class="narrow"><option value="levels">Level</option><option value="points">Punkte</option></select><button class="btn" id="xpGo">Hinzufügen</button><span class="dim small">Negative Werte ziehen ab.</span></div></div>`;
      $$('[data-gm]', m.el).forEach((x) => x.addEventListener('click', async () => { if (await act({ action: 'gamemode', gamemode: x.dataset.gm }, 'Spielmodus geändert')) $$('[data-gm]', m.el).forEach((y) => y.classList.toggle('on', y === x)); }));
      $('#stHeal', m.el).addEventListener('click', (e) => act({ action: 'heal' }, 'Geheilt und gesättigt', e.currentTarget));
      $('#stClearFx', m.el).addEventListener('click', (e) => act({ action: 'effect-clear' }, 'Effekte entfernt', e.currentTarget));
      $('#fxGo', m.el).addEventListener('click', (e) => act({ action: 'effect', effect: $('#fxId', m.el).value, seconds: +$('#fxSec', m.el).value, amplifier: +$('#fxAmp', m.el).value, hide: $('#fxHide', m.el).checked }, 'Effekt gegeben', e.currentTarget));
      $('#xpGo', m.el).addEventListener('click', (e) => act({ action: 'xp', amount: +$('#xpN', m.el).value, unit: $('#xpU', m.el).value }, 'Erfahrung angepasst', e.currentTarget));
    } else if (curTab === 'msg') {
      b.innerHTML = `<div class="pm-sec"><h4>Private Nachricht</h4><textarea id="msgT" rows="3" maxlength="256" placeholder="Nur ${esc(player)} sieht diese Nachricht (als „Server“)."></textarea>
        <div class="pm-actions"><span class="grow dim small" id="msgLen">0 / 256</span><button class="btn primary" id="msgGo">${icon('send')}Senden</button></div></div>`;
      const t = $('#msgT', m.el);
      t.addEventListener('input', () => { $('#msgLen', m.el).textContent = `${t.value.length} / 256`; });
      t.addEventListener('keydown', (e) => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); $('#msgGo', m.el).click(); } });
      $('#msgGo', m.el).addEventListener('click', async (e) => { if (await act({ action: 'message', message: t.value }, 'Nachricht gesendet', e.currentTarget)) { t.value = ''; $('#msgLen', m.el).textContent = '0 / 256'; } });
      t.focus();
    } else if (curTab === 'mod') {
      b.innerHTML = `<div class="pm-sec"><h4>Kicken oder bannen</h4><input id="modReason" placeholder="Grund (optional, wird dem Spieler angezeigt)" maxlength="200">
          <div class="pm-actions"><span class="grow"></span><button class="btn" id="modKick">${icon('door')}Kicken</button><button class="btn danger" id="modBan">${icon('ban')}Bannen</button></div></div>
        <div class="pm-sec"><h4>Rechte</h4><div class="pm-row">${isOp ? `<button class="btn" id="modDeop">OP entziehen</button>` : `<button class="btn" id="modOp">${icon('star')}OP geben</button>`}</div></div>
        <div class="pm-sec danger-zone"><h4>Gefährlich</h4><div class="pm-row"><button class="btn danger" id="modClear">Inventar leeren</button><button class="btn danger" id="modKill">Töten</button></div></div>`;
      const done = () => { onChange && onChange(); };
      $('#modKick', m.el).addEventListener('click', async (e) => { if (await act({ action: 'kick', reason: $('#modReason', m.el).value }, `${player} wurde gekickt`, e.currentTarget)) { done(); setTimeout(m.close, 900); } });
      $('#modBan', m.el).addEventListener('click', async () => {
        if (!(await confirmDialog({ title: `${player} bannen`, danger: true, ok: 'Bannen', body: `<p>${esc(player)} wird dauerhaft vom Server ausgeschlossen. Entbannen geht im Reiter „Spieler“.</p>` }))) return;
        if (await act({ action: 'ban', reason: $('#modReason', m.el)?.value || '' }, `${player} wurde gebannt`)) { done(); setTimeout(m.close, 900); }
      });
      $('#modOp', m.el)?.addEventListener('click', async (e) => { if (await act({ action: 'op' }, 'OP vergeben', e.currentTarget)) done(); });
      $('#modDeop', m.el)?.addEventListener('click', async (e) => { if (await act({ action: 'deop' }, 'OP entzogen', e.currentTarget)) done(); });
      $('#modClear', m.el).addEventListener('click', async () => { if (await confirmDialog({ title: 'Inventar leeren', danger: true, ok: 'Leeren', body: `<p>Das komplette Inventar von ${esc(player)} wird gelöscht. Das lässt sich nicht rückgängig machen.</p>` })) act({ action: 'clear' }, 'Inventar geleert'); });
      $('#modKill', m.el).addEventListener('click', async () => { if (await confirmDialog({ title: `${player} töten`, danger: true, ok: 'Töten', body: `<p>${esc(player)} stirbt sofort und lässt sein Inventar fallen (je nach keepInventory).</p>` })) act({ action: 'kill' }, `${player} wurde getötet`); });
    }
  };
  $$('[data-pt]', m.el).forEach((x) => x.addEventListener('click', () => { curTab = x.dataset.pt; const o = $('#pmOut', m.el); o.className = 'pm-foot'; o.innerHTML = ''; draw(); }));
  $('#pmRefresh', m.el).addEventListener('click', loadInfo);
  draw();
  loadInfo();
}

boot();
