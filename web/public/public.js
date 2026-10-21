'use strict';

/* CraftDock – public landing page and per-server live dashboard. */

const $ = (s, el = document) => el.querySelector(s);
const $$ = (s, el = document) => [...el.querySelectorAll(s)];
const esc = (v) => String(v ?? '').replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
const reduced = matchMedia('(prefers-reduced-motion: reduce)').matches;

const I = {
  copy: '<rect x="9" y="9" width="12" height="12" rx="2"/><path d="M5 15H4a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1h10a1 1 0 0 1 1 1v1"/>',
  users: '<path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 0 0-3-3.9M16 3.1a4 4 0 0 1 0 7.8"/>',
  arrow: '<path d="M5 12h14M13 6l6 6-6 6"/>',
  back: '<path d="M19 12H5M11 18l-6-6 6-6"/>',
  clock: '<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>',
  wifi: '<path d="M5 12.5a10 10 0 0 1 14 0M8.5 16a5 5 0 0 1 7 0M2 9a15 15 0 0 1 20 0M12 20h.01"/>',
  gauge: '<path d="M12 14l4-4"/><path d="M3.3 19a10 10 0 1 1 17.4 0"/>',
  tag: '<path d="M20.6 13.4 13.4 20.6a2 2 0 0 1-2.8 0L3 13V3h10l7.6 7.6a2 2 0 0 1 0 2.8z"/><path d="M7.5 7.5h.01"/>',
  chart: '<path d="M3 3v18h18"/><path d="m7 15 4-4 3 3 6-6"/>',
  puzzle: '<path d="M19.4 13.5a2.5 2.5 0 0 0 0-3H18V7a1 1 0 0 0-1-1h-3.5V4.6a2.5 2.5 0 0 0-5 0V6H5a1 1 0 0 0-1 1v3.5h1.5a2.5 2.5 0 0 1 0 5H4V19a1 1 0 0 0 1 1h3.5v-1.5a2.5 2.5 0 0 1 5 0V20H17a1 1 0 0 0 1-1v-3.5z"/>',
  info: '<circle cx="12" cy="12" r="9"/><path d="M12 16v-4M12 8h.01"/>',
  play: '<path d="M7 4.5v15l12.5-7.5z"/>',
  server: '<rect x="3" y="3" width="18" height="8" rx="2"/><rect x="3" y="13" width="18" height="8" rx="2"/><path d="M7 7h.01M7 17h.01"/>',
  box: '<path d="M21 16V8l-9-5-9 5v8l9 5z"/><path d="M3.3 7 12 12l8.7-5M12 22V12"/>',
  discord: '<path d="M8 12h.01M16 12h.01"/><path d="M7.5 7.5c3-1 6-1 9 0M7 16.5c3.3 1.3 6.7 1.3 10 0"/><path d="M15.5 17l1 2c2-.6 3.5-1.6 5-3-.3-4-1.3-7.5-3-10.5-1.3-.6-2.6-1-4-1.2l-.5 1M8.5 17l-1 2c-2-.6-3.5-1.6-5-3 .3-4 1.3-7.5 3-10.5 1.3-.6 2.6-1 4-1.2l.5 1"/>',
  check: '<path d="m5 12 5 5L20 7"/>',
  calendar: '<rect x="3" y="4" width="18" height="18" rx="2"/><path d="M16 2v4M8 2v4M3 10h18"/>',
  news: '<path d="M4 4h13a1 1 0 0 1 1 1v14a2 2 0 0 0 2 2H6a2 2 0 0 1-2-2z"/><path d="M18 9h2a1 1 0 0 1 1 1v9a2 2 0 0 1-2 2M8 8h6M8 12h6M8 16h4"/>',
  map: '<path d="m3 6 6-3 6 3 6-3v15l-6 3-6-3-6 3z"/><path d="M9 3v15M15 6v15"/>',
  network: '<rect x="9" y="2" width="6" height="6" rx="1"/><rect x="2" y="16" width="6" height="6" rx="1"/><rect x="16" y="16" width="6" height="6" rx="1"/><path d="M12 8v4M5 16v-2h14v2"/>',
  trophy: '<path d="M8 21h8M12 17v4M7 4h10v5a5 5 0 0 1-10 0z"/><path d="M17 5h3v2a3 3 0 0 1-3 3M7 5H4v2a3 3 0 0 0 3 3"/>',
  pulse: '<path d="M3 12h4l3-8 4 16 3-8h4"/>',
  moon: '<path d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z"/>',
  expand: '<path d="M15 3h6v6M9 21H3v-6M21 3l-7 7M3 21l7-7"/>',
  pin: '<path d="M12 17v5M9 3h6l-1 7 4 3H6l4-3z"/>',
};
const icon = (n) => `<svg class="i" viewBox="0 0 24 24" aria-hidden="true">${I[n] || ''}</svg>`;

const TYPES = {
  VANILLA: 'Vanilla', PAPER: 'Paper', PURPUR: 'Purpur', FOLIA: 'Folia', SPIGOT: 'Spigot', FABRIC: 'Fabric',
  QUILT: 'Quilt', FORGE: 'Forge', NEOFORGE: 'NeoForge', MODRINTH: 'Modpack', AUTO_CURSEFORGE: 'Modpack',
};
const STATUS = { online: 'Online', starting: 'Startet', offline: 'Offline', sleeping: 'Bereit' };
const isUp = (st) => st === 'online' || st === 'sleeping';
const MODES = { survival: 'Überleben', creative: 'Kreativ', adventure: 'Abenteuer', spectator: 'Zuschauer' };
const DIFF = { peaceful: 'Friedlich', easy: 'Einfach', normal: 'Normal', hard: 'Schwer' };
const LOADER_LINKS = {
  fabric: ['Fabric', 'https://fabricmc.net/use/installer/'], quilt: ['Quilt', 'https://quiltmc.org/install/'],
  forge: ['Forge', 'https://files.minecraftforge.net/'], neoforge: ['NeoForge', 'https://neoforged.net/'],
};

/* ---------- helpers ---------- */
async function getJSON(url) {
  const r = await fetch(url, { headers: { Accept: 'application/json' } });
  const d = await r.json().catch(() => ({}));
  if (!r.ok) { const e = new Error(d.error || r.statusText); e.status = r.status; throw e; }
  return d;
}

let toastT;
function toast(msg) {
  const t = $('#toast'); t.textContent = msg; t.classList.add('show');
  clearTimeout(toastT); toastT = setTimeout(() => t.classList.remove('show'), 2200);
}
async function copy(text) {
  try { await navigator.clipboard.writeText(text); toast(`„${text}“ kopiert`); }
  catch {
    const ta = document.createElement('textarea'); ta.value = text; document.body.appendChild(ta); ta.select();
    try { document.execCommand('copy'); toast(`„${text}“ kopiert`); } catch { toast(text); }
    ta.remove();
  }
}

/** Render §-formatted Minecraft text as DOM (safe: textContent only). */
function motdNode(text) {
  const root = document.createElement('span');
  let color = null, hex = null, fmt = new Set();
  let buf = '';
  const flush = () => {
    if (!buf) return;
    const sp = document.createElement('span');
    sp.textContent = buf;
    if (color) sp.classList.add(`mc-${color}`);
    if (hex) sp.style.color = hex;
    fmt.forEach((f) => sp.classList.add(`mc-${f}`));
    root.appendChild(sp); buf = '';
  };
  for (let i = 0; i < text.length; i++) {
    if (text[i] === '§' && i + 1 < text.length) {
      flush();
      if (text[i + 1] === '#' && /^#[0-9a-fA-F]{6}$/.test(text.slice(i + 1, i + 8))) { hex = text.slice(i + 1, i + 8); color = null; fmt = new Set(); i += 7; continue; }
      const c = text[i + 1].toLowerCase(); i++;
      if (/[0-9a-f]/.test(c)) { color = c; hex = null; fmt = new Set(); } else if (c === 'r') { color = null; hex = null; fmt = new Set(); } else if (/[klmno]/.test(c)) fmt.add(c);
      continue;
    }
    buf += text[i];
  }
  flush();
  if (!root.childNodes.length) root.textContent = ' ';
  return root;
}
function fillMotds(root) {
  $$('[data-motd]', root).forEach((el) => { el.replaceChildren(motdNode(el.dataset.motd || 'Ein Minecraft-Server')); delete el.dataset.motd; });
}

function favHTML(s, cls = 'fav') {
  return s.favicon ? `<img class="${cls}" src="${esc(s.favicon)}" alt="">` : `<span class="${cls}">${esc((s.displayName || s.name).slice(0, 1).toUpperCase())}</span>`;
}

/** Animate numbers in [data-count] from their previous value. */
function countUp(root = document) {
  $$('[data-count]', root).forEach((el) => {
    const to = parseFloat(el.dataset.count), dec = +(el.dataset.dec || 0);
    const from = parseFloat(el.dataset.from ?? (reduced ? to : 0));
    el.dataset.from = to;
    const fmt = (v) => v.toLocaleString('de-DE', { minimumFractionDigits: dec, maximumFractionDigits: dec });
    if (from === to || reduced) { el.textContent = fmt(to); return; }
    const t0 = performance.now(), dur = 1100;
    const step = (t) => {
      const p = Math.min(1, (t - t0) / dur), e = 1 - Math.pow(1 - p, 3);
      el.textContent = fmt(from + (to - from) * e);
      if (p < 1) requestAnimationFrame(step);
    };
    requestAnimationFrame(step);
  });
}
function stagger(root, base = 0) {
  $$('.rise', root).forEach((el, i) => { if (!el.style.animationDelay) el.style.animationDelay = `${base + i * 70}ms`; });
}
function bars(root) {
  requestAnimationFrame(() => requestAnimationFrame(() => $$('[data-w]', root).forEach((b) => { b.style.width = `${b.dataset.w}%`; })));
}
function fmtUptime(iso) {
  if (!iso) return '–';
  let s = Math.max(0, Math.floor((Date.now() - new Date(iso).getTime()) / 1000));
  const d = Math.floor(s / 86400); s %= 86400;
  const h = Math.floor(s / 3600); s %= 3600;
  const m = Math.floor(s / 60); s %= 60;
  const p = (n) => String(n).padStart(2, '0');
  return d ? `${d}T ${p(h)}:${p(m)}` : `${p(h)}:${p(m)}:${p(s)}`;
}
const fmtPct = (v) => v == null ? '–' : `${v >= 99.95 ? '100' : v.toLocaleString('de-DE', { maximumFractionDigits: 1 })} %`;
const fmtDT = (unix) => new Date(unix * 1000).toLocaleString('de-DE', { weekday: 'short', day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' });
const fmtPlay = (min) => min >= 60 ? `${Math.floor(min / 60)} Std` : `${min} Min`;
function countdown(unix) {
  let s = Math.floor(unix - Date.now() / 1000);
  if (s <= 0) return 'läuft gerade';
  const d = Math.floor(s / 86400); s %= 86400;
  const h = Math.floor(s / 3600); s %= 3600;
  const m = Math.floor(s / 60); s %= 60;
  if (d) return `in ${d} T ${h} Std`;
  if (h) return `in ${h} Std ${m} Min`;
  return `in ${m}:${String(s).padStart(2, '0')}`;
}
setInterval(() => $$('[data-cd]').forEach((el) => { el.textContent = countdown(+el.dataset.cd); }), 1000);
function eventsHTML(events) {
  return events.map((e) => `<article class="event rise"><div class="ev-date"><b>${new Date(e.start * 1000).getDate()}</b><span>${new Date(e.start * 1000).toLocaleString('de-DE', { month: 'short' })}</span></div>
    <div class="ev-body"><div class="ev-meta"><span class="cd" data-cd="${e.start}">${countdown(e.start)}</span>${e.server ? `<span class="chip">${esc(e.server)}</span>` : ''}</div>
    <h3>${esc(e.title)}</h3><div class="ev-when">${icon('calendar')}${fmtDT(e.start)}${e.end ? ` – ${new Date(e.end * 1000).toLocaleTimeString('de-DE', { hour: '2-digit', minute: '2-digit' })}` : ''}</div>
    ${e.body ? `<p>${esc(e.body)}</p>` : ''}</div></article>`).join('');
}
function newsHTML(news) {
  return news.map((n) => `<article class="news rise">${n.pinned ? `<span class="pin">${icon('pin')}</span>` : ''}<div class="news-date">${new Date(n.published * 1000).toLocaleDateString('de-DE', { day: '2-digit', month: 'long', year: 'numeric' })}${n.server ? ` · ${esc(n.server)}` : ''}</div>
    <h3>${esc(n.title)}</h3>${n.body ? `<p>${esc(n.body)}</p>` : ''}</article>`).join('');
}

function loaderLabel(s) {
  if (s.type === 'MODRINTH' || s.type === 'AUTO_CURSEFORGE') return 'Modpack';
  return TYPES[s.type] || s.type;
}

/* =====================================================================
   Landing page
   ===================================================================== */
let site = null;

function applySite(d) {
  site = d;
  $('#siteTitle').textContent = d.title;
  const dc = $('#discord');
  if (d.discordUrl) { dc.href = d.discordUrl; dc.innerHTML = `${icon('discord')}Discord`; dc.classList.remove('hidden'); dc.classList.add('discord'); }
}

async function renderHome(first) {
  let d;
  try { d = await getJSON('/api/public/site'); } catch (e) { if (first) showError(e); return; }
  applySite(d);
  document.title = d.title;
  const root = $('#root');
  const online = d.servers.filter((s) => isUp(s.status));
  const players = online.reduce((a, s) => a + s.online, 0);

  if (first) {
    root.innerHTML = `
      <section class="hero">
        <span class="eyebrow rise">${icon('server')}<span id="hEyebrow"></span></span>
        <h1 class="rise">${esc(d.title)}</h1>
        ${d.tagline ? `<p class="tag rise">${esc(d.tagline)}</p>` : ''}
        <div class="kpis">
          <div class="kpi g rise"><div class="v"><span id="kOn"></span><small> / <span id="kAll"></span></small></div><div class="l">Server online</div></div>
          <div class="kpi rise"><div class="v" id="kPl"></div><div class="l">Spieler gerade online</div></div>
          <div class="kpi rise"><div class="v" id="kMods"></div><div class="l">Mods &amp; Plugins</div></div>
        </div>
      </section>
      <section id="netBox"></section>
      <section id="evBox"></section>
      <div class="section-title rise"><h2>Server</h2><span>Klicke auf einen Server für das Live-Dashboard</span></div>
      <div class="cards" id="cards"></div>
      <section id="newsBox"></section>`;
    stagger(root);
  }
  $('#hEyebrow').textContent = online.length ? `${online.length} ${online.length === 1 ? 'Server ist' : 'Server sind'} gerade online` : 'Gerade ist kein Server online';
  setCount($('#kOn'), online.length); setCount($('#kAll'), d.servers.length);
  setCount($('#kPl'), players); setCount($('#kMods'), d.servers.reduce((a, s) => a + s.modCount, 0));

  const nb = $('#netBox');
  if (d.network) {
    const key = JSON.stringify(d.network);
    if (nb.dataset.key !== key) {
      nb.dataset.key = key;
      nb.innerHTML = `<div class="netcard ${first ? 'rise' : ''}"><div class="net-ic">${icon('network')}</div>
        <div class="grow"><div class="net-k">Ein Netzwerk – eine Adresse</div><div class="net-addr">${esc(d.network.address)}</div>
        <div class="net-sub">${d.network.online ? `<span class="status online"><span class="d"></span>${d.network.players} online</span>` : '<span class="status offline"><span class="d"></span>Proxy offline</span>'}<span>Im Spiel mit <code>/server &lt;name&gt;</code> wechseln</span></div></div>
        <button class="btn primary" data-copy="${esc(d.network.address)}">${icon('copy')}Adresse kopieren</button></div>`;
    }
  } else nb.innerHTML = '';
  const evKey = JSON.stringify(d.events);
  if ($('#evBox').dataset.key !== evKey) {
    $('#evBox').dataset.key = evKey;
    $('#evBox').innerHTML = d.events.length ? `<div class="section-title"><h2>${icon('calendar')}Nächste Events</h2></div><div class="events">${eventsHTML(d.events.slice(0, 4))}</div>` : '';
    stagger($('#evBox'), 150);
  }
  const nwKey = JSON.stringify(d.news);
  if ($('#newsBox').dataset.key !== nwKey) {
    $('#newsBox').dataset.key = nwKey;
    $('#newsBox').innerHTML = d.news.length ? `<div class="section-title"><h2>${icon('news')}Neuigkeiten</h2></div><div class="newsgrid">${newsHTML(d.news)}</div>` : '';
    stagger($('#newsBox'), 100);
  }

  const cards = $('#cards');
  if (!d.servers.length) { cards.innerHTML = '<div class="empty-public">Noch keine öffentlichen Server.</div>'; return; }
  const keyOf = (s) => JSON.stringify([s.name, s.status, s.online, s.max, s.version, s.motd, s.favicon, s.address, s.uptime30 != null ? Math.round(s.uptime30 * 10) : null]);
  const existing = new Map($$('.card', cards).map((c) => [c.dataset.name, c]));
  const frag = document.createDocumentFragment();
  d.servers.forEach((s, i) => {
    let card = existing.get(s.name);
    if (!card || card.dataset.key !== keyOf(s)) {
      const fresh = document.createElement('a');
      fresh.className = `card ${s.status}` + (card ? '' : ' rise');
      fresh.href = `/s/${encodeURIComponent(s.name)}`;
      fresh.dataset.name = s.name; fresh.dataset.key = keyOf(s);
      const pct = s.max ? Math.min(100, (s.online / s.max) * 100) : 0;
      fresh.innerHTML = `
        <div class="card-top">${favHTML(s)}
          <div class="t"><h3>${esc(s.displayName)}</h3><div class="sub">${esc(loaderLabel(s))} · ${esc(shortVersion(s.version))}</div></div>
          <span class="status ${s.status}"><span class="d"></span>${STATUS[s.status]}</span></div>
        <div class="motd" data-motd="${esc(s.motd)}"></div>
        <div class="pbar-wrap"><div class="pbar-label"><span>${icon('users')} Spieler</span><b>${s.status === 'online' ? `${s.online} / ${s.max}` : s.status === 'sleeping' ? 'schläft · wacht beim Verbinden auf' : '–'}</b></div>
          <div class="pbar"><i data-w="${s.status === 'online' ? pct.toFixed(1) : 0}"></i></div></div>
        <div class="chips">
          ${s.modCount ? `<span class="chip">${icon('puzzle')}${s.modCount} ${s.type === 'PAPER' || s.type === 'PURPUR' || s.type === 'SPIGOT' || s.type === 'FOLIA' ? 'Plugins' : 'Mods'}</span>` : ''}
          ${s.modpack ? `<span class="chip">${icon('box')}Modpack</span>` : ''}
          <span class="chip">${esc(MODES[s.gamemode] || s.gamemode)}</span>
          ${s.whitelist ? '<span class="chip">Whitelist</span>' : ''}
          ${s.status === 'online' && s.latencyMs >= 0 ? `<span class="chip">${icon('wifi')}${s.latencyMs} ms</span>` : ''}
          ${s.uptime30 != null ? `<span class="chip" title="Verfügbarkeit der letzten 30 Tage">${icon('pulse')}${fmtPct(s.uptime30)}</span>` : ''}
          ${s.map ? `<span class="chip">${icon('map')}Karte</span>` : ''}
          ${s.network ? `<span class="chip" title="Im Spiel mit /server ${esc(s.name)}">${icon('network')}/server ${esc(s.name)}</span>` : ''}
        </div>
        <div class="card-foot"><button class="btn addr-btn" data-copy="${esc(s.address)}" title="Adresse kopieren"><span>${esc(s.address)}</span>${icon('copy')}</button>
          <span class="btn primary" aria-hidden="true">${icon('arrow')}</span></div>`;
      if (!card) fresh.style.animationDelay = `${250 + i * 90}ms`;
      fillMotds(fresh);
      if (card) {
        // keep the bar animating from its old width
        const old = $('.pbar i', card)?.style.width;
        if (old) $('.pbar i', fresh).style.width = old;
      }
      card = fresh;
    }
    frag.appendChild(card);
  });
  cards.replaceChildren(frag);
  bars(cards);
}

function setCount(el, v, dec = 0) { if (!el) return; el.dataset.count = v; el.dataset.dec = dec; countUp(el.parentElement); }

/* =====================================================================
   Server dashboard
   ===================================================================== */
let uptimeTimer;

async function renderServer(name, first) {
  let s;
  try {
    if (!site) applySite(await getJSON('/api/public/site'));
    s = await getJSON(`/api/public/servers/${encodeURIComponent(name)}`);
  } catch (e) { if (first) showError(e); return; }
  document.title = `${s.displayName} · ${site.title}`;
  const root = $('#root');

  if (first) {
    root.innerHTML = `
      <a class="back rise" href="/">${icon('back')}Alle Server</a>
      <section class="s-hero rise">
        <div id="sFav"></div>
        <div class="grow">
          <div class="row1"><h1>${esc(s.displayName)}</h1><span id="sStatus"></span></div>
          <div class="motd" id="sMotd"></div>
          <div class="chips" id="sChips"></div>
        </div>
        <div class="join">
          <span class="lbl">Server-Adresse</span>
          <div class="addr">${esc(s.address)}</div>
          <button class="btn primary" data-copy="${esc(s.address)}">${icon('copy')}Adresse kopieren</button>
        </div>
      </section>
      <section class="tiles" id="tiles"></section>
      <div class="cols">
        <section class="panel rise"><div class="panel-h"><h2>${icon('chart')}Spieler – letzte 24 Stunden</h2><span id="chPeak"></span></div><div class="chart" id="chart"></div></section>
        <section class="panel rise"><div class="panel-h"><h2>${icon('users')}Spieler</h2><div class="seg" id="plSeg"><button class="on" data-v="online">Online <span id="plCount"></span></button><button data-v="top">${icon('trophy')}Top</button></div></div><div id="players"></div><div id="top" class="hidden"></div></section>
      </div>
      <section class="panel rise ${s.map ? '' : 'hidden'}" id="mapPanel"><div class="panel-h"><h2>${icon('map')}Live-Karte</h2><a class="btn ghost" href="/map/${encodeURIComponent(s.name)}/" target="_blank" rel="noopener">${icon('expand')}Vollbild</a></div>
        <div class="mapwrap" id="mapWrap"><button class="map-cover" id="mapLoad">${icon('map')}<b>Karte laden</b><span>3D-Ansicht der Welt (BlueMap)</span></button></div></section>
      <div class="cols">
        <section class="panel rise"><div class="panel-h"><h2>${icon('pulse')}Verfügbarkeit</h2><span id="upSub"></span></div><div id="uptime"></div></section>
        <section class="panel rise" id="evPanel"><div class="panel-h"><h2>${icon('calendar')}Events</h2></div><div id="events"></div></section>
      </div>
      <div class="cols">
        <section class="panel rise" id="modsPanel"></section>
        <section class="panel rise"><div class="panel-h"><h2>${icon('info')}Über den Server</h2></div><div id="about"></div></section>
      </div>
      <section class="panel rise"><div class="panel-h"><h2>${icon('play')}So kommst du drauf</h2></div><div class="steps" id="steps"></div></section>`;
    stagger(root);
    $('#modsPanel').classList.toggle('hidden', false);
    $$('#plSeg button').forEach((b) => b.addEventListener('click', () => {
      $$('#plSeg button').forEach((x) => x.classList.toggle('on', x === b));
      $('#players').classList.toggle('hidden', b.dataset.v !== 'online');
      $('#top').classList.toggle('hidden', b.dataset.v !== 'top');
    }));
    $('#mapLoad')?.addEventListener('click', () => {
      $('#mapWrap').innerHTML = `<iframe src="/map/${encodeURIComponent(s.name)}/" title="Live-Karte" allow="fullscreen" loading="lazy"></iframe>`;
    });
  }

  $('#sFav').innerHTML = favHTML(s);
  $('#sStatus').innerHTML = `<span class="status ${s.status}"><span class="d"></span>${STATUS[s.status]}</span>`;
  $('#sMotd').replaceChildren(motdNode(s.motd || 'Ein Minecraft-Server'));
  $('#sChips').innerHTML = `<span class="chip">${icon('box')}${esc(loaderLabel(s))}</span><span class="chip">${icon('tag')}Minecraft ${esc(shortVersion(s.version))}</span>`
    + (s.pack ? `<span class="chip">${icon('puzzle')}${esc(s.pack.title)}</span>` : '');

  renderTiles(s, first);
  lastServer = s;
  renderChart(s);
  renderPlayers(s);
  renderMods(s, first);
  renderAbout(s);
  renderSteps(s);
  renderUptime(s);
  renderTop(s);
  renderEvents(s);

  clearInterval(uptimeTimer);
  if (isUp(s.status) && s.startedAt) {
    const el = $('#upV');
    uptimeTimer = setInterval(() => { el.textContent = fmtUptime(s.startedAt); }, 1000);
  }
}

function ring(pct, cls = '') {
  const r = 22, c = 2 * Math.PI * r;
  return `<svg class="ring ${cls}" viewBox="0 0 54 54"><circle class="bg" cx="27" cy="27" r="${r}"/><circle class="fg" cx="27" cy="27" r="${r}" stroke-dasharray="${c.toFixed(1)}" stroke-dashoffset="${c.toFixed(1)}" data-off="${(c * (1 - Math.max(0, Math.min(1, pct)))).toFixed(1)}"/></svg>`;
}

function renderTiles(s, first) {
  const on = s.status === 'online';
  const sleeping = s.status === 'sleeping';
  const tps = s.tps;
  const tpsCls = tps == null ? '' : tps >= 18 ? 'good' : tps >= 14 ? 'warn' : 'bad';
  const lat = s.latencyMs;
  const tiles = $('#tiles');
  const prev = Object.fromEntries($$('[data-count]', tiles).map((e) => [e.id, e.dataset.from]));
  const prevOff = $$('.ring .fg', tiles).map((c) => c.getAttribute('stroke-dashoffset'));
  tiles.innerHTML = `
    <div class="tile rise"><div class="k">${icon('users')}Spieler</div>${ring(on && s.max ? s.online / s.max : 0)}
      <div class="v">${on ? `<span id="tPl" data-count="${s.online}"></span><small>/ ${s.max}</small>` : sleeping ? `0<small>/ ${s.max}</small>` : '–'}</div><div class="s">${on ? `${Math.round((s.online / (s.max || 1)) * 100)} % belegt` : sleeping ? 'schläft – wacht beim Verbinden auf' : 'Server offline'}</div></div>
    <div class="tile rise"><div class="k">${icon('gauge')}TPS</div>${tps != null ? ring(tps / 20, tps >= 18 ? '' : tps >= 14 ? 'amber' : 'red') : ''}
      <div class="v ${tpsCls}">${tps != null ? `<span id="tTps" data-count="${tps}" data-dec="1"></span>` : '–'}</div><div class="s">${tps != null ? (tps >= 18 ? 'läuft flüssig' : tps >= 14 ? 'leicht ausgelastet' : 'stark ausgelastet') : 'nicht verfügbar'}</div></div>
    <div class="tile rise"><div class="k">${icon('wifi')}Antwortzeit</div>
      <div class="v">${on && lat >= 0 ? `<span id="tLat" data-count="${lat}"></span><small>ms</small>` : '–'}</div><div class="s">${on && lat >= 0 ? 'vom Server gemessen' : '–'}</div></div>
    <div class="tile rise"><div class="k">${icon('clock')}Laufzeit</div>
      <div class="v" id="upV">${(on || sleeping) && s.startedAt ? fmtUptime(s.startedAt) : '–'}</div><div class="s">${on && s.startedAt ? `seit ${new Date(s.startedAt).toLocaleString('de-DE', { dateStyle: 'short', timeStyle: 'short' })}` : STATUS[s.status]}</div></div>
    <div class="tile rise"><div class="k">${icon('tag')}Version</div>
      <div class="v ver">${esc(shortVersion(s.version))}</div><div class="s">${esc(loaderLabel(s))}</div></div>`;
  $$('[data-count]', tiles).forEach((e) => { if (prev[e.id] !== undefined) e.dataset.from = prev[e.id]; });
  $$('.ring .fg', tiles).forEach((c, i) => { if (prevOff[i]) c.setAttribute('stroke-dashoffset', prevOff[i]); });
  if (first) stagger(tiles, 200); else $$('.rise', tiles).forEach((e) => e.classList.remove('rise'));
  countUp(tiles);
  requestAnimationFrame(() => requestAnimationFrame(() => $$('.ring .fg', tiles).forEach((c) => c.setAttribute('stroke-dashoffset', c.dataset.off))));
}
function shortVersion(v) { const m = /\d+\.\d+(\.\d+)?/.exec(v || ''); return m ? m[0] : (v || '–'); }

/* ---------- player chart ---------- */
let chartDrawn = false;
function renderChart(s) {
  const el = $('#chart');
  let h = s.history || [];
  const pts = h.filter((p) => p.p >= 0);
  const peak = pts.reduce((a, p) => Math.max(a, p.p), 0);
  $('#chPeak').textContent = pts.length ? `Spitze: ${peak} Spieler` : '';
  if (h.length < 2) { el.innerHTML = '<div class="chart-empty">Noch keine Daten – der Verlauf wird jede Minute aufgezeichnet.</div>'; return; }

  const W = Math.max(300, Math.round(el.clientWidth || 800)), H = 240, L = 34, R = 8, T = 12, B = 26;
  const now = Date.now() / 1000, t0 = now - 24 * 3600;
  h = bucket(h, W < 520 ? 1200 : 600);
  const ymax = Math.max(4, Math.ceil(Math.max(peak, 1) * 1.25));
  const x = (t) => L + ((t - t0) / (now - t0)) * (W - L - R);
  const y = (p) => T + (1 - p / ymax) * (H - T - B);

  // Split into online segments; offline stretches become shaded bands.
  const segs = []; let cur = [];
  const offBands = []; let offStart = null;
  for (const p of h) {
    if (p.t < t0) continue;
    if (p.p >= 0) { cur.push(p); if (offStart !== null) { offBands.push([offStart, p.t]); offStart = null; } }
    else { if (cur.length) segs.push(cur); cur = []; if (offStart === null) offStart = p.t; }
  }
  if (cur.length) segs.push(cur);
  if (offStart !== null) offBands.push([offStart, now]);

  const line = segs.map((sg) => sg.map((p, i) => `${i ? 'L' : 'M'}${x(p.t).toFixed(1)},${y(p.p).toFixed(1)}`).join('')).join('');
  const area = segs.map((sg) => `M${x(sg[0].t).toFixed(1)},${y(0)}` + sg.map((p) => `L${x(p.t).toFixed(1)},${y(p.p).toFixed(1)}`).join('') + `L${x(sg[sg.length - 1].t).toFixed(1)},${y(0)}Z`).join('');
  const grid = [0, .5, 1].map((f) => { const v = Math.round(ymax * f); return `<line class="gl" x1="${L}" x2="${W - R}" y1="${y(v)}" y2="${y(v)}"/><text class="ax" x="${L - 8}" y="${y(v) + 4}" text-anchor="end">${v}</text>`; }).join('');
  const xt = (W < 520 ? [0, 12, 24] : [0, 6, 12, 18, 24]).map((hh) => { const t = t0 + hh * 3600; return `<text class="ax" x="${x(t)}" y="${H - 6}" text-anchor="${hh === 0 ? 'start' : hh === 24 ? 'end' : 'middle'}">${hh === 24 ? 'jetzt' : new Date(t * 1000).toLocaleTimeString('de-DE', { hour: '2-digit', minute: '2-digit' })}</text>`; }).join('');
  const bands = offBands.map(([a, b]) => `<rect class="off" x="${x(a)}" y="${T}" width="${Math.max(1, x(b) - x(a))}" height="${H - T - B}"/>`).join('');

  el.innerHTML = `<svg viewBox="0 0 ${W} ${H}" role="img" aria-label="Spielerverlauf der letzten 24 Stunden">
    <defs>
      <linearGradient id="lg" x1="0" x2="1"><stop offset="0" stop-color="#22c55e"/><stop offset="1" stop-color="#38bdf8"/></linearGradient>
      <linearGradient id="ag" x1="0" x2="0" y1="0" y2="1"><stop offset="0" stop-color="#4ade80" stop-opacity=".35"/><stop offset="1" stop-color="#4ade80" stop-opacity="0"/></linearGradient>
    </defs>
    ${bands}${grid}${xt}
    <path class="area" d="${area}"/>
    <path class="line" d="${line}"/>
    <line class="cursor hidden" y1="${T}" y2="${H - B}"/>
    <circle class="dot hidden" r="5"/>
  </svg><div class="chart-tip"></div>`;

  const path = $('.line', el);
  if (!chartDrawn && !reduced) {
    const len = path.getTotalLength();
    path.style.strokeDasharray = `${len}`; path.style.strokeDashoffset = `${len}`;
    path.classList.add('draw');
    chartDrawn = true;
  } else { $('.area', el).style.animation = 'none'; $('.area', el).style.opacity = 1; }

  // Hover tooltip
  const svg = $('svg', el), tip = $('.chart-tip', el), cursor = $('.cursor', el), dot = $('.dot', el);
  const flat = h.filter((p) => p.t >= t0);
  svg.addEventListener('mousemove', (e) => {
    const r = svg.getBoundingClientRect();
    const vx = ((e.clientX - r.left) / r.width) * W;
    const t = t0 + ((vx - L) / (W - L - R)) * (now - t0);
    let best = flat[0];
    for (const p of flat) if (Math.abs(p.t - t) < Math.abs(best.t - t)) best = p;
    if (!best) return;
    const px = x(best.t);
    cursor.setAttribute('x1', px); cursor.setAttribute('x2', px); cursor.classList.remove('hidden');
    if (best.p >= 0) { dot.setAttribute('cx', px); dot.setAttribute('cy', y(best.p)); dot.classList.remove('hidden'); } else dot.classList.add('hidden');
    tip.innerHTML = `${new Date(best.t * 1000).toLocaleTimeString('de-DE', { hour: '2-digit', minute: '2-digit' })} · ${best.p >= 0 ? `<b>${Math.round(best.p)}</b> Spieler${best.max > Math.round(best.p) ? ` (max. ${best.max})` : ''}` : 'offline'}`;
    tip.style.left = `${(px / W) * r.width}px`; tip.style.top = `${((best.p >= 0 ? y(best.p) : T) / H) * r.height}px`; tip.style.opacity = 1;
  });
  svg.addEventListener('mouseleave', () => { tip.style.opacity = 0; cursor.classList.add('hidden'); dot.classList.add('hidden'); });
}

/** Average history into buckets of `sec` seconds for a calmer line. */
function bucket(h, sec) {
  const out = []; let cur = null;
  for (const p of h) {
    const k = Math.floor(p.t / sec);
    if (!cur || cur.k !== k) { if (cur) out.push(fin(cur)); cur = { k, t: p.t, sum: 0, n: 0, off: 0, max: 0 }; }
    if (p.p >= 0) { cur.sum += p.p; cur.n++; cur.max = Math.max(cur.max, p.p); } else cur.off++;
  }
  if (cur) out.push(fin(cur));
  return out;
  function fin(c) { return { t: c.t, p: c.n && c.n >= c.off ? Math.round((c.sum / c.n) * 10) / 10 : -1, max: c.max }; }
}
let resizeT;
window.addEventListener('resize', () => { clearTimeout(resizeT); resizeT = setTimeout(() => { if (lastServer) { chartDrawn = true; renderChart(lastServer); } }, 200); });
let lastServer = null;

function renderPlayers(s) {
  const el = $('#players');
  $('#plCount').textContent = s.status === 'online' ? `${s.online} / ${s.max}` : '';
  if (s.status === 'sleeping') { el.innerHTML = `<div class="sleep-note">${icon('moon')}<div><b>Der Server schläft.</b><br>Niemand ist online – sobald du dich verbindest, wacht er in wenigen Sekunden auf.</div></div>`; return; }
  if (s.status !== 'online') { el.innerHTML = `<div class="muted-note">${s.status === 'starting' ? 'Der Server startet gerade …' : 'Der Server ist gerade offline.'}</div>`; return; }
  if (!s.online) { el.innerHTML = '<div class="muted-note">Gerade ist niemand online – sei die/der Erste!</div>'; return; }
  if (!s.names.length) { el.innerHTML = `<div class="muted-note">${s.online} ${s.online === 1 ? 'Spieler ist' : 'Spieler sind'} online.</div>`; return; }
  const key = s.names.join(',');
  if (el.dataset.key === key) return;
  el.dataset.key = key;
  el.innerHTML = `<div class="players">${s.names.map((n) => `<div class="player rise"><img src="https://mc-heads.net/avatar/${encodeURIComponent(n)}/64" alt="" loading="lazy"><span>${esc(n)}</span></div>`).join('')}</div>`;
  stagger(el);
}

let modFilter = '';
function renderMods(s, first) {
  const el = $('#modsPanel');
  const total = s.mods.length + s.extraFiles.length;
  if (!s.pack && !total) { el.classList.add('hidden'); return; }
  el.classList.remove('hidden');
  const what = s.modKind === 'plugins' ? 'Plugins' : 'Mods';
  if (first || !el.dataset.ready) {
    el.innerHTML = `<div class="panel-h"><h2>${icon('puzzle')}${what}<span>${total ? `${total}` : ''}</span></h2>
      ${total > 8 ? `<input class="mods-filter" id="modQ" placeholder="${what} filtern …" spellcheck="false">` : ''}</div>
      ${s.pack ? `<a class="pack-card" href="${esc(s.pack.url)}" target="_blank" rel="noopener">
        ${s.pack.iconUrl ? `<img src="${esc(s.pack.iconUrl)}" alt="">` : `<span class="ph">${icon('box')}</span>`}
        <div><div class="k">Modpack · ${s.pack.source === 'modrinth' ? 'Modrinth' : 'CurseForge'}</div><h3>${esc(s.pack.title)}</h3>${s.pack.description ? `<p>${esc(s.pack.description)}</p>` : ''}</div></a>` : ''}
      <div class="mods" id="modList">${s.mods.map((m) => `<a class="mod rise" href="${esc(m.url || '#')}" target="_blank" rel="noopener" data-q="${esc((m.title + ' ' + (m.description || '')).toLowerCase())}">
        ${m.iconUrl ? `<img src="${esc(m.iconUrl)}" alt="" loading="lazy">` : `<span class="ph">${esc(m.title.slice(0, 1).toUpperCase())}</span>`}
        <div class="t"><b>${esc(m.title)}</b><small>${esc(m.description || '')}</small></div></a>`).join('')}</div>
      ${s.extraFiles.length ? `<div class="files" id="fileList">${s.extraFiles.map((f) => `<span class="chip" data-q="${esc(f.toLowerCase())}">${esc(f)}</span>`).join('')}</div>` : ''}`;
    el.dataset.ready = '1';
    stagger(el);
    $('#modQ')?.addEventListener('input', (e) => {
      modFilter = e.target.value.trim().toLowerCase();
      $$('[data-q]', el).forEach((n) => n.classList.toggle('hidden', !!modFilter && !n.dataset.q.includes(modFilter)));
    });
  }
}

function renderAbout(s) {
  const yes = (b) => (b ? 'Ja' : 'Nein');
  $('#about').innerHTML = `
    ${s.description ? `<p class="desc">${esc(s.description)}</p><div class="spacer"></div>` : ''}
    <div class="facts">
      <div class="fact"><div class="k">Spielmodus</div><div class="v">${esc(MODES[s.gamemode] || s.gamemode)}</div></div>
      <div class="fact"><div class="k">Schwierigkeit</div><div class="v">${esc(DIFF[s.difficulty] || s.difficulty)}${s.hardcore ? ' · Hardcore' : ''}</div></div>
      <div class="fact"><div class="k">PvP</div><div class="v">${yes(s.pvp)}</div></div>
      <div class="fact"><div class="k">Whitelist</div><div class="v">${s.whitelist ? 'Nur freigeschaltete Spieler' : 'Offen für alle'}</div></div>
      <div class="fact"><div class="k">Plattform</div><div class="v">${esc(loaderLabel(s))}</div></div>
      <div class="fact"><div class="k">Account</div><div class="v">${s.onlineMode ? 'Minecraft Java (Premium)' : 'Ohne Kontoprüfung'}</div></div>
    </div>`;
  if (s.description) $('#about .desc').classList.add('rise');
}

function renderSteps(s) {
  const ver = shortVersion(s.version);
  const steps = [];
  if (s.pack) {
    steps.push(['Modpack installieren', `Installiere <a href="${esc(s.pack.url)}" target="_blank" rel="noopener">${esc(s.pack.title)}</a> über die ${s.pack.source === 'modrinth' ? 'Modrinth App' : 'CurseForge App'} oder einen Launcher wie Prism.`]);
  } else if (s.modKind === 'mods' && LOADER_LINKS[s.loader]) {
    const [n, url] = LOADER_LINKS[s.loader];
    steps.push([`${n} installieren`, `Lade den <a href="${url}" target="_blank" rel="noopener">${n}-Installer</a> und richte ihn für Minecraft ${esc(ver)} ein.`]);
    steps.push(['Mods hinzufügen', s.mods.length ? `Kopiere die oben gelisteten Mods in deinen <code>mods</code>-Ordner – reine Server-Mods brauchst du nicht.` : 'Hol dir die Mods vom Server-Team.']);
  } else {
    steps.push(['Minecraft starten', `Öffne Minecraft: Java Edition${ver !== '–' ? ` in Version <b>${esc(ver)}</b>` : ''}.`]);
  }
  steps.push(['Server hinzufügen', `Mehrspieler → Server hinzufügen → Adresse <b>${esc(s.address)}</b> eintragen.${s.network ? ` Im Netzwerk wechselst du mit <code>/server ${esc(s.name)}</code> hierher.` : ''}`]);
  if (steps.length < 3) steps.push(['Loslegen', s.whitelist ? 'Die Whitelist ist aktiv – lass dich vorher freischalten.' : 'Verbinden und Spaß haben!']);
  $('#steps').innerHTML = steps.slice(0, 3).map(([t, d]) => `<div class="step"><b>${t}</b><p>${d}</p></div>`).join('');
}

/* ---------- uptime, top players, events ---------- */
function renderUptime(s) {
  const u = s.uptime || { days: [] };
  $('#upSub').textContent = s.uniquePlayers ? `${s.uniquePlayers} verschiedene Spieler` : '';
  const key = JSON.stringify(u);
  const el = $('#uptime');
  if (el.dataset.key === key) return;
  el.dataset.key = key;
  const cls = (v) => v == null ? 'none' : v >= 99 ? 'good' : v >= 95 ? 'ok' : v >= 80 ? 'warn' : 'bad';
  el.innerHTML = `<div class="up-kpis">
      <div><span class="up-v ${cls(u.h24)}">${fmtPct(u.h24)}</span><span class="up-l">24 Stunden</span></div>
      <div><span class="up-v ${cls(u.d7)}">${fmtPct(u.d7)}</span><span class="up-l">7 Tage</span></div>
      <div><span class="up-v ${cls(u.d30)}">${fmtPct(u.d30)}</span><span class="up-l">30 Tage</span></div></div>
    <div class="up-bars">${u.days.map((d, i) => `<span class="ub ${cls(d.uptime)}" data-i="${i}" title="${new Date(d.date).toLocaleDateString('de-DE', { weekday: 'short', day: '2-digit', month: '2-digit' })}: ${d.uptime == null ? 'keine Daten' : fmtPct(d.uptime) + ' online · Spitze ' + d.peak + ' Spieler'}"></span>`).join('')}</div>
    <div class="up-axis"><span>vor 30 Tagen</span><span>heute</span></div>`;
  $$('.ub', el).forEach((b, i) => { b.style.animationDelay = `${i * 18}ms`; });
}

function renderTop(s) {
  const el = $('#top');
  if (!s.top.length) { el.innerHTML = '<div class="muted-note">Noch keine Spielzeit erfasst – die Bestenliste füllt sich, sobald gespielt wird.</div>'; return; }
  const key = JSON.stringify(s.top.map((t) => [t.name, t.minutes]));
  if (el.dataset.key === key) return;
  el.dataset.key = key;
  const max = s.top[0].minutes || 1;
  el.innerHTML = `<ol class="toplist">${s.top.map((t, i) => `<li><span class="tp-rank ${i < 3 ? 'r' + (i + 1) : ''}">${i + 1}</span><img src="https://mc-heads.net/avatar/${encodeURIComponent(t.name)}/48" alt="" loading="lazy">
    <span class="tp-name">${esc(t.name)}<i class="tp-bar"><i data-w="${((t.minutes / max) * 100).toFixed(1)}"></i></i></span><span class="tp-time">${fmtPlay(t.minutes)}</span></li>`).join('')}</ol>`;
  bars(el);
}

function renderEvents(s) {
  const panel = $('#evPanel');
  const key = JSON.stringify(s.events);
  if (panel.dataset.key === key) return;
  panel.dataset.key = key;
  $('#events').innerHTML = s.events.length ? `<div class="events col">${eventsHTML(s.events.slice(0, 3))}</div>` : '<div class="muted-note">Gerade sind keine Events geplant.</div>';
  stagger(panel);
}

/* ---------- errors & boot ---------- */
function showError(e) {
  const nf = e.status === 404;
  $('#root').innerHTML = `<div class="notfound"><div><h1>${nf ? 'Nicht gefunden' : 'Gerade nicht erreichbar'}</h1>
    <p>${esc(nf ? 'Diesen Server gibt es nicht – oder er ist nicht öffentlich.' : e.message)}</p>
    <a class="btn primary" href="/">${icon('back')}Zur Übersicht</a></div></div>`;
}

document.addEventListener('click', (e) => {
  const b = e.target.closest('[data-copy]');
  if (b) { e.preventDefault(); e.stopPropagation(); copy(b.dataset.copy); }
});
document.addEventListener('error', (e) => {
  const img = e.target;
  if (!(img instanceof HTMLImageElement) || img.dataset.failed) return;
  img.dataset.failed = '1';
  const ph = document.createElement('span');
  ph.className = img.className + ' ph';
  const label = img.closest('.player')?.textContent.trim() || img.closest('li')?.querySelector('.tp-name')?.textContent.trim() || img.closest('.mod, .card, .s-hero, .pack-card')?.querySelector('b, h3, h1')?.textContent.trim() || '?';
  ph.textContent = label.slice(0, 1).toUpperCase();
  img.replaceWith(ph);
}, true);
document.addEventListener('pointermove', (e) => {
  const c = e.target.closest?.('.card');
  if (!c) return;
  const r = c.getBoundingClientRect();
  c.style.setProperty('--mx', `${e.clientX - r.left}px`);
  c.style.setProperty('--my', `${e.clientY - r.top}px`);
});

(function boot() {
  const m = location.pathname.match(/^\/s\/([a-z0-9][a-z0-9-]{0,30})\/?$/);
  const tick = m ? (first) => renderServer(m[1], first) : (first) => renderHome(first);
  tick(true);
  setInterval(() => { if (!document.hidden) tick(false); }, 10000);
  document.addEventListener('visibilitychange', () => { if (!document.hidden) tick(false); });
})();
