'use strict';
/* CraftDock embeddable status widget: /widget (all servers) or /widget/<name>. */
const q = new URLSearchParams(location.search);
if (q.get('theme') === 'light') document.body.classList.add('light');
if (q.get('bg') === 'transparent') document.body.classList.add('transparent');
const m = location.pathname.match(/^\/widget\/([a-z0-9][a-z0-9-]{0,30})\/?$/);
const esc = (v) => String(v ?? '').replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
const LBL = { online: 'online', sleeping: 'bereit', starting: 'startet', offline: 'offline' };
const TYPES = { VANILLA: 'Vanilla', PAPER: 'Paper', PURPUR: 'Purpur', FOLIA: 'Folia', SPIGOT: 'Spigot', FABRIC: 'Fabric', QUILT: 'Quilt', FORGE: 'Forge', NEOFORGE: 'NeoForge', MODRINTH: 'Modpack', AUTO_CURSEFORGE: 'Modpack' };
const ver = (v) => (/\d+\.\d+(\.\d+)?/.exec(v || '') || [v || ''])[0];
const root = document.getElementById('w');
let first = true;

function fav(s) {
  return s.favicon ? `<img class="fav" src="${esc(s.favicon)}" alt="">` : `<span class="fav">${esc((s.displayName || s.name)[0].toUpperCase())}</span>`;
}
function row(s, i, single) {
  const up = s.status === 'online';
  const pct = up && s.max ? Math.min(100, (s.online / s.max) * 100) : 0;
  return `<a class="row" href="/s/${encodeURIComponent(s.name)}" target="_blank" rel="noopener" data-d="${first ? i * 70 : 0}">
    ${fav(s)}<div class="t"><div class="name"><span class="dot ${s.status}"></span>${esc(s.displayName)}</div>
    <div class="sub">${esc(TYPES[s.type] || s.type)} ${esc(ver(s.version))} · ${LBL[s.status]}</div>
    <div class="bar"><i data-w="${pct.toFixed(1)}"></i></div></div>
    <div class="pl"><b>${up ? s.online : s.status === 'sleeping' ? 0 : '–'}</b><span>/ ${s.max} Spieler</span></div></a>
    ${single ? `<button class="addr" data-copy="${esc(s.address)}"><span>${esc(s.address)}</span><small>kopieren</small></button>` : ''}`;
}
async function tick() {
  try {
    const d = await (await fetch('/api/public/site')).json();
    let list = d.servers || [];
    if (m) list = list.filter((s) => s.name === m[1]);
    if (!list.length) { root.innerHTML = '<div class="row"><div class="t"><div class="name">Server nicht gefunden</div></div></div>'; return; }
    document.body.classList.toggle('single', !!m);
    root.innerHTML = list.map((s, i) => row(s, i, !!m)).join('') +
      `<div class="foot"><span>${esc(m ? '' : d.title || '')}</span><a href="/" target="_blank" rel="noopener">Live-Status ↗</a></div>`;
    root.querySelectorAll('[data-d]').forEach((r) => { r.style.animationDelay = r.dataset.d + 'ms'; });
    requestAnimationFrame(() => requestAnimationFrame(() => root.querySelectorAll('[data-w]').forEach((b) => { b.style.width = b.dataset.w + '%'; })));
    first = false;
  } catch { /* keep last state */ }
}
root.addEventListener('click', async (e) => {
  const b = e.target.closest('[data-copy]');
  if (!b) return;
  e.preventDefault();
  try { await navigator.clipboard.writeText(b.dataset.copy); b.querySelector('small').textContent = 'kopiert ✓'; } catch { /* iframe without clipboard permission */ }
});
tick();
setInterval(() => { if (!document.hidden) tick(); }, 30000);
