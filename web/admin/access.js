'use strict';

/* =====================================================================
   CraftDock 2.0 – accounts, 2FA, users & roles, invitations, hosting,
   alerting. Loaded before app.js; uses its helpers at runtime only.
   ===================================================================== */

const PERM_SHORT = {
  'server.view': 'Sehen', 'server.control': 'Steuern', 'server.console': 'Konsole', 'server.configure': 'Konfigurieren',
  'server.advanced': 'Erweitert', 'server.mods': 'Mods', 'server.files': 'Dateien', 'server.world': 'Welt', 'server.backup': 'Backups',
  'server.delete': 'Löschen', 'players.manage': 'Spieler verwalten', 'players.cheat': 'Spieler-Eingriffe', 'servers.create': 'Server anlegen',
  networks: 'Netzwerke', news: 'Neuigkeiten', settings: 'Einstellungen', alerts: 'Alarmierung', users: 'Benutzer', roles: 'Rollen', audit: 'Audit-Log',
};

const fmtMB = (mb) => mb >= 1024 ? `${(mb / 1024).toFixed(mb % 1024 ? 1 : 0).replace('.', ',')} GB` : `${mb} MB`;
const fmtCPU = (c) => String(Math.round(c * 10) / 10).replace('.', ',');
const ago = (iso) => {
  if (!iso || iso.startsWith('0001')) return 'nie';
  const s = (Date.now() - new Date(iso).getTime()) / 1000;
  if (s < 90) return 'gerade eben';
  if (s < 3600) return `vor ${Math.round(s / 60)} Min.`;
  if (s < 86400) return `vor ${Math.round(s / 3600)} Std.`;
  return new Date(iso).toLocaleDateString('de-DE', { dateStyle: 'medium' });
};
const errBox = (e) => `<div class="banner">${icon('alert')}<div>${esc(e.message || e)}</div></div>`;
const agentName = (ua) => {
  if (!ua) return 'Unbekanntes Gerät';
  const b = /Edg\//.test(ua) ? 'Edge' : /Firefox\//.test(ua) ? 'Firefox' : /Chrome\//.test(ua) ? 'Chrome' : /Safari\//.test(ua) ? 'Safari' : /curl|Go-http|python/i.test(ua) ? 'Skript' : 'Browser';
  const os = /Windows/.test(ua) ? 'Windows' : /Android/.test(ua) ? 'Android' : /iPhone|iPad/.test(ua) ? 'iOS' : /Mac OS/.test(ua) ? 'macOS' : /Linux/.test(ua) ? 'Linux' : '';
  return os ? `${b} auf ${os}` : b;
};

/* ---------- quota display ---------- */
function quotaParts(q, u) {
  if (!q) return [];
  const out = [];
  if (q.servers) out.push(`${u.servers}/${q.servers} Server`);
  if (q.memoryMB) out.push(`${fmtMB(u.memoryMB)} / ${fmtMB(q.memoryMB)} RAM`);
  if (q.cpus) out.push(`${fmtCPU(u.cpus)}/${fmtCPU(q.cpus)} CPU`);
  return out;
}
function quotaLine() {
  const me = state.me;
  if (!me || me.allServers) return '';
  const p = quotaParts(me.quota, me.usage || {});
  return p.length ? `Deine Quote: ${p.join(' · ')}` : '';
}
function quotaHTML() {
  const me = state.me;
  const bar = (used, max, label) => max ? `<div class="q-row"><span>${label}</span><span class="num">${String(used).replace('.', ',')} / ${String(max).replace('.', ',')}</span><div class="bar ${used / max > .9 ? 'hot' : ''}"><i data-w="${Math.min(100, used / max * 100)}"></i></div></div>` : '';
  const q = me.quota || {}, u = me.usage || {};
  const rows = bar(u.servers || 0, q.servers, 'Server') + bar(Math.round((u.memoryMB || 0) / 102.4) / 10, q.memoryMB ? Math.round(q.memoryMB / 102.4) / 10 : 0, 'RAM (GB)') + bar(u.cpus || 0, q.cpus, 'CPU-Kerne');
  return `${rows ? `<div class="quota">${rows}</div>` : ''}<div class="row dim">CraftDock ${esc(state.info?.version || '')} · ${esc(me.user)} (${esc(me.roleName || me.role)})</div>`;
}
function quotaLimits(q = {}) {
  return [q.servers ? `${q.servers} Server` : '', q.memoryMB ? `${fmtMB(q.memoryMB)} RAM` : '', q.cpus ? `${fmtCPU(q.cpus)} CPU` : ''].filter(Boolean).join(' · ');
}
function applyWidths(root) { $$('[data-w]', root).forEach((i) => { i.style.width = `${i.dataset.w}%`; }); }
async function refreshMe() {
  try { const s = await api('GET', '/api/session'); if (s.authenticated) { state.me = s; loadInfo(); } } catch { /* ignore */ }
}

/* =====================================================================
   Registration
   ===================================================================== */
async function showRegister(token) {
  stopTimers?.();
  $('#boot').classList.add('hidden');
  $('#app').classList.add('hidden');
  $('#login').classList.add('hidden');
  const box = $('#register');
  box.classList.remove('hidden');
  let info = { open: false, invite: false };
  try { info = await api('GET', `/api/register?invite=${encodeURIComponent(token || '')}`); } catch { /* */ }
  const card = $('.login-card', box);
  if (!info.invite && !info.open) {
    card.innerHTML = `<div class="brand brand-lg"><span class="logo"></span><span>CraftDock</span></div>
      <h2 class="reg-h">Einladung ungültig</h2><p class="muted">Der Link ist abgelaufen, wurde schon verwendet oder ist falsch. Bitte frag nach einer neuen Einladung.</p>
      <button class="btn block" id="regBack" type="button">Zur Anmeldung</button>`;
    $('#regBack').addEventListener('click', () => { history.replaceState(null, '', '#/'); showLogin(); });
    return;
  }
  card.innerHTML = `<div class="brand brand-lg"><span class="logo"></span><span>CraftDock</span></div>
    <h2 class="reg-h">Konto erstellen</h2>
    <p class="muted">${info.invite ? `Du wurdest eingeladen${info.roleName ? ` – Rolle <b>${esc(info.roleName)}</b>` : ''}.${info.note ? `<br><span class="dim">„${esc(info.note)}“</span>` : ''}` : 'Registriere dich, um eigene Minecraft-Server zu verwalten.'}</p>
    <label class="field"><span>Benutzername</span><input name="username" autocomplete="username" required pattern="[a-zA-Z0-9][a-zA-Z0-9_.\\-]{1,31}" spellcheck="false"></label>
    <label class="field"><span>E-Mail (optional)</span><input name="email" type="email" autocomplete="email"></label>
    <label class="field"><span>Passwort</span><input name="password" type="password" autocomplete="new-password" required minlength="10"><span class="hint">Mindestens 10 Zeichen</span></label>
    <label class="field"><span>Passwort wiederholen</span><input name="password2" type="password" autocomplete="new-password" required minlength="10"></label>
    <div id="regError" class="form-error hidden"></div>
    <button class="btn primary block" type="submit">Konto erstellen</button>
    <button class="btn ghost block" type="button" id="regBack">Ich habe schon ein Konto</button>`;
  $('#regBack').addEventListener('click', () => { history.replaceState(null, '', '#/'); showLogin(); });
  card.onsubmit = async (e) => {
    e.preventDefault();
    const f = card, err = $('#regError');
    err.classList.add('hidden');
    if (f.password.value !== f.password2.value) { err.textContent = 'Die Passwörter stimmen nicht überein'; err.classList.remove('hidden'); return; }
    const btn = $('button[type=submit]', f); btn.disabled = true;
    try {
      const me = await api('POST', '/api/register', { username: f.username.value, email: f.email.value, password: f.password.value, invite: token || '' });
      history.replaceState(null, '', '#/');
      toast('Willkommen bei CraftDock!');
      showApp(me);
    } catch (ex) { err.textContent = ex.message; err.classList.remove('hidden'); btn.disabled = false; }
  };
  setTimeout(() => card.username?.focus(), 30);
}

/* =====================================================================
   Two-factor authentication
   ===================================================================== */
function showRecoveryCodes(codes, { title = 'Wiederherstellungscodes', onDone } = {}) {
  const text = `CraftDock – Wiederherstellungscodes für ${state.me?.user || ''}\n${new Date().toLocaleString('de-DE')}\n\n${codes.join('\n')}\n\nJeder Code funktioniert genau einmal.\n`;
  const m = openOverlay(`<div class="modal" role="dialog" aria-modal="true">
    <header><h2>${esc(title)}</h2></header>
    <div class="body">
      <p>Bewahre diese Codes sicher auf (Passwort-Manager, ausgedruckt). Mit jedem Code kannst du dich <b>einmal</b> anmelden, falls dein Handy weg ist.</p>
      <div class="codes mono">${codes.map((c) => `<span>${esc(c)}</span>`).join('')}</div>
      <div class="btn-row"><button class="btn" id="rcCopy">${icon('copy')}Kopieren</button><button class="btn" id="rcDl">${icon('download')}Als Datei speichern</button></div>
      <label class="chk"><input type="checkbox" id="rcOk"> Ich habe die Codes gesichert</label>
    </div>
    <footer><button class="btn primary" id="rcDone" disabled>Fertig</button></footer></div>`, { onClose: () => onDone && onDone() });
  $('#rcCopy', m.el).addEventListener('click', () => copyText(codes.join('\n')));
  $('#rcDl', m.el).addEventListener('click', () => {
    const a = document.createElement('a');
    a.href = URL.createObjectURL(new Blob([text], { type: 'text/plain' }));
    a.download = `craftdock-recovery-${state.me?.user || 'codes'}.txt`;
    document.body.appendChild(a); a.click(); a.remove();
    setTimeout(() => URL.revokeObjectURL(a.href), 2000);
  });
  $('#rcOk', m.el).addEventListener('change', (e) => { $('#rcDone', m.el).disabled = !e.target.checked; });
  $('#rcDone', m.el).addEventListener('click', () => m.close());
}

async function open2FASetup({ forced = false, onDone } = {}) {
  let d;
  try { d = await api('POST', '/api/me/2fa/setup'); } catch (e) { toast(e.message, 'err'); return; }
  const grouped = d.secret.replace(/(.{4})/g, '$1 ').trim();
  const m = openOverlay(`<div class="modal tfa" role="dialog" aria-modal="true">
    <header><h2>${icon('phone')}Zwei-Faktor-Anmeldung einrichten</h2>${forced ? '' : `<button class="btn ghost icon sm" data-close aria-label="Schließen">${icon('x')}</button>`}</header>
    <div class="body">
      ${forced ? `<div class="banner info">${icon('lock')}<div>Für dein Konto ist die Zwei-Faktor-Anmeldung vorgeschrieben. Richte sie jetzt ein, um weiterzumachen.</div></div>` : ''}
      <ol class="steps">
        <li><b>Authenticator-App öffnen</b><span>z. B. Aegis, 2FAS, Google Authenticator, Microsoft Authenticator, 1Password oder Bitwarden.</span></li>
        <li><b>QR-Code scannen</b><div class="qr-wrap"><div class="qr">${d.qr}</div>
          <div class="qr-alt"><span class="dim small">Kein Scanner? Schlüssel manuell eingeben:</span><code class="secret">${esc(grouped)}</code><button class="btn sm" id="tfCopy" type="button">${icon('copy')}Kopieren</button></div></div></li>
        <li><b>Code bestätigen</b><span>Gib den 6-stelligen Code aus der App ein.</span>
          <input id="tfCode" class="code-in" inputmode="numeric" autocomplete="one-time-code" maxlength="7" placeholder="123 456"></li>
      </ol>
      <div id="tfErr" class="form-error hidden"></div>
    </div>
    <footer>${forced ? `<button class="btn ghost" id="tfLogout">${icon('logout')}Abmelden</button>` : '<button class="btn" data-close>Abbrechen</button>'}<button class="btn primary" id="tfGo">${icon('check')}Aktivieren</button></footer></div>`, { sticky: forced });
  $('#tfCopy', m.el).addEventListener('click', () => copyText(d.secret));
  $('#tfLogout', m.el)?.addEventListener('click', async () => { try { await api('POST', '/api/logout'); } catch { /* */ } location.reload(); });
  const go = async () => {
    const code = $('#tfCode', m.el).value.replace(/\s/g, '');
    const err = $('#tfErr', m.el); err.classList.add('hidden');
    try {
      const r = await api('POST', '/api/me/2fa/confirm', { code });
      m.close();
      toast('Zwei-Faktor-Anmeldung ist aktiv');
      showRecoveryCodes(r.recovery, { onDone });
    } catch (e) { err.textContent = e.message; err.classList.remove('hidden'); $('#tfCode', m.el).select(); }
  };
  $('#tfGo', m.el).addEventListener('click', go);
  $('#tfCode', m.el).addEventListener('keydown', (e) => { if (e.key === 'Enter') go(); });
  setTimeout(() => $('#tfCode', m.el).focus(), 50);
}

function forced2FA() {
  if (state.forcing2fa) return;
  state.forcing2fa = true;
  stopTimers();
  $('#main').innerHTML = `${mobileBar()}<div class="empty"><div class="box"><div class="art">${icon('lock')}</div><h2>Zwei-Faktor-Anmeldung erforderlich</h2><p>Bitte richte die Anmeldung mit Authenticator-App ein.</p><button class="btn primary" id="f2Again">Einrichten</button></div></div>`;
  const done = async () => { state.forcing2fa = false; const s = await api('GET', '/api/session'); showApp(s); };
  $('#f2Again').addEventListener('click', () => open2FASetup({ forced: true, onDone: done }));
  open2FASetup({ forced: true, onDone: done });
}

/* =====================================================================
   Account
   ===================================================================== */
async function openAccount() {
  await refreshMe();
  const me = state.me;
  const quota = quotaParts(me.quota, me.usage || {});
  const m = openOverlay(`<div class="modal wide" role="dialog" aria-modal="true">
    <header><h2>Mein Konto</h2><button class="btn ghost icon sm" data-close aria-label="Schließen">${icon('x')}</button></header>
    <div class="body acc">
      <section><dl class="kv"><dt>Benutzer</dt><dd><b>${esc(me.user)}</b>${me.email ? ` <span class="dim">· ${esc(me.email)}</span>` : ''}</dd>
        <dt>Rolle</dt><dd>${esc(me.roleName || me.role)}</dd>
        <dt>Server</dt><dd>${me.allServers ? 'alle Server' : 'eigene und zugewiesene Server'}</dd>
        ${quota.length ? `<dt>Quote</dt><dd>${esc(quota.join(' · '))}</dd>` : ''}</dl></section>

      <section><h3>${icon('phone')}Zwei-Faktor-Anmeldung</h3>
        ${me.twofa ? `<p><span class="tag new">aktiv</span> Noch <b>${me.recoveryLeft}</b> Wiederherstellungscodes übrig.${me.recoveryLeft < 3 ? ' <span class="warn-text">Erzeuge bald neue.</span>' : ''}</p>
          <div class="btn-row"><button class="btn sm" id="acCodes">${icon('key')}Neue Wiederherstellungscodes</button><button class="btn sm danger" id="acOff">Deaktivieren</button></div>`
        : `<p class="muted">Schützt dein Konto zusätzlich mit einem Code aus einer Authenticator-App – auch wenn jemand dein Passwort kennt.</p>
          <button class="btn primary sm" id="acOn">${icon('lock')}Einrichten</button>`}</section>

      <section><h3>${icon('key')}Passwort ändern</h3>
        <div class="fgrid"><label class="field"><span>Aktuelles Passwort</span><input type="password" id="acOld" autocomplete="current-password"></label>
        <label class="field"><span>Neues Passwort</span><input type="password" id="acNew" autocomplete="new-password"><span class="hint">Mindestens 10 Zeichen. Andere Sitzungen werden abgemeldet.</span></label></div>
        <button class="btn sm" id="acPw">${icon('save')}Passwort ändern</button></section>

      <section><h3>${icon('server')}Aktive Sitzungen</h3><div id="acSess"><span class="spinner"></span></div></section>
    </div>
    <footer><button class="btn" data-close>Schließen</button></footer></div>`);
  const reopen = () => { m.close(); openAccount(); };
  $('#acOn', m.el)?.addEventListener('click', () => { m.close(); open2FASetup({ onDone: () => { refreshMe(); } }); });
  $('#acCodes', m.el)?.addEventListener('click', async () => {
    const pw = await askPassword('Neue Wiederherstellungscodes', 'Die alten Codes werden ungültig.');
    if (pw === null) return;
    try { const r = await api('POST', '/api/me/2fa/recovery', { password: pw }); m.close(); showRecoveryCodes(r.recovery, { onDone: refreshMe }); } catch (e) { toast(e.message, 'err'); }
  });
  $('#acOff', m.el)?.addEventListener('click', async () => {
    const r = await askPassword('Zwei-Faktor-Anmeldung deaktivieren', 'Zur Sicherheit brauchen wir dein Passwort und einen aktuellen Code (oder Wiederherstellungscode).', true);
    if (r === null) return;
    try { await api('POST', '/api/me/2fa/disable', r); toast('Zwei-Faktor-Anmeldung deaktiviert'); await refreshMe(); reopen(); } catch (e) { toast(e.message, 'err'); }
  });
  $('#acPw', m.el).addEventListener('click', async () => {
    try { await api('PUT', '/api/me/password', { old: $('#acOld', m.el).value, new: $('#acNew', m.el).value }); toast('Passwort geändert'); $('#acOld', m.el).value = $('#acNew', m.el).value = ''; loadSess(); }
    catch (e) { toast(e.message, 'err'); }
  });
  const loadSess = async () => {
    const box = $('#acSess', m.el);
    try {
      const list = await api('GET', '/api/me/sessions');
      box.innerHTML = `<div class="sess">${list.map((s) => `<div class="sess-row ${s.current ? 'cur' : ''}"><span class="sess-ic">${icon(/Android|iPhone|iPad/.test(s.agent) ? 'phone' : 'server')}</span>
        <div class="grow"><b>${esc(agentName(s.agent))}</b>${s.current ? ' <span class="tag new">diese Sitzung</span>' : ''}<div class="dim small">${esc(s.ip)} · angemeldet ${esc(ago(s.created))} · aktiv ${esc(ago(s.lastSeen))}</div></div>
        ${s.current ? '' : `<button class="btn ghost sm" data-rv="${esc(s.id)}">Abmelden</button>`}</div>`).join('')}</div>
        ${list.length > 1 ? `<button class="btn sm" id="acOthers">${icon('logout')}Alle anderen abmelden</button>` : ''}`;
      $$('[data-rv]', box).forEach((b) => b.addEventListener('click', async () => { try { await api('DELETE', `/api/me/sessions/${b.dataset.rv}`); loadSess(); } catch (e) { toast(e.message, 'err'); } }));
      $('#acOthers', box)?.addEventListener('click', async () => { try { const r = await api('POST', '/api/me/sessions/revoke-others'); toast(`${r.revoked} Sitzung(en) abgemeldet`); loadSess(); } catch (e) { toast(e.message, 'err'); } });
    } catch (e) { box.innerHTML = errBox(e); }
  };
  loadSess();
}

function askPassword(title, text, withCode = false) {
  return new Promise((resolve) => {
    let done = false;
    const m = openOverlay(`<div class="modal" role="dialog" aria-modal="true">
      <header><h2>${esc(title)}</h2><button class="btn ghost icon sm" data-close aria-label="Schließen">${icon('x')}</button></header>
      <div class="body"><p>${esc(text)}</p>
        <label class="field"><span>Passwort</span><input type="password" id="apPw" autocomplete="current-password"></label>
        ${withCode ? '<label class="field"><span>Code</span><input id="apCode" inputmode="numeric" autocomplete="one-time-code" placeholder="123456 oder abcd-efgh"></label>' : ''}</div>
      <footer><button class="btn" data-close>Abbrechen</button><button class="btn primary" id="apOk">Weiter</button></footer></div>`, { onClose: () => { if (!done) resolve(null); } });
    $('#apPw', m.el).focus();
    $('#apOk', m.el).addEventListener('click', () => {
      done = true;
      const pw = $('#apPw', m.el).value;
      resolve(withCode ? { password: pw, code: $('#apCode', m.el).value } : pw);
      m.close();
    });
  });
}

/* =====================================================================
   Server maintenance (mutes alerts)
   ===================================================================== */
async function openMaintenance(s) {
  const put = async (min) => {
    try { await api('PUT', `/api/servers/${s.name}/maintenance`, { minutes: min }); toast(min ? 'Wartungsmodus aktiv – Alarme sind stumm' : 'Wartungsmodus beendet'); poll(); } catch (e) { toast(e.message, 'err'); }
  };
  if (s.maintenance) { put(0); return; }
  const m = openOverlay(`<div class="modal" role="dialog" aria-modal="true">
    <header><h2>${icon('wrench')}Wartungsmodus</h2><button class="btn ghost icon sm" data-close aria-label="Schließen">${icon('x')}</button></header>
    <div class="body"><p>Während der Wartung verschickt CraftDock für <b>${esc(s.name)}</b> keine Alarme – praktisch für Updates, Umbauten oder geplante Neustarts.</p>
      <div class="chips pick">${[[15, '15 Min.'], [30, '30 Min.'], [60, '1 Std.'], [120, '2 Std.'], [480, '8 Std.'], [1440, '1 Tag']].map(([v, l]) => `<button class="btn" data-mm="${v}">${l}</button>`).join('')}</div></div></div>`);
  $$('[data-mm]', m.el).forEach((b) => b.addEventListener('click', () => { m.close(); put(+b.dataset.mm); }));
}

/* =====================================================================
   Hosting settings
   ===================================================================== */
async function openHostingSettings() {
  let h, roles;
  try { [h, roles] = await Promise.all([api('GET', '/api/hosting'), api('GET', '/api/roles')]); } catch (e) { toast(e.message, 'err'); return; }
  const q = h.defaultQuota || {};
  const m = openOverlay(`<div class="modal wide" role="dialog" aria-modal="true">
    <header><h2>${icon('sliders')}Hosting</h2><button class="btn ghost icon sm" data-close aria-label="Schließen">${icon('x')}</button></header>
    <div class="body">
      <p class="muted">Regeln für Benutzer ohne die Berechtigung „Erweiterte Einstellungen“ – typischerweise Kunden, die eigene Server verwalten.</p>
      <h3 class="sub">Ports &amp; Ressourcen</h3>
      <div class="fgrid">
        <label class="field"><span>Portbereich von</span><input type="number" id="hoMin" value="${h.portMin || ''}" placeholder="z. B. 25600"></label>
        <label class="field"><span>bis</span><input type="number" id="hoMax" value="${h.portMax || ''}" placeholder="z. B. 25699"><span class="hint">Neue Server bekommen automatisch einen freien Port daraus. Leer = Kunden können keine Ports wählen.</span></label>
        <label class="field"><span>Max. Java-Speicher pro Server (MB)</span><input type="number" id="hoMaxMB" value="${h.maxServerMB || ''}" placeholder="unbegrenzt"></label>
        <label class="field"><span>Container-Aufschlag (MB)</span><input type="number" id="hoOver" value="${h.overheadMB}"><span class="hint">Container-Limit = Heap + max(Aufschlag, 25 %)</span></label>
      </div>
      <h3 class="sub">Konten</h3>
      <div class="fgrid">
        <label class="field"><span>Registrierung</span><select id="hoReg"><option value="off" ${h.registration === 'off' ? 'selected' : ''}>Nur mit Einladung</option><option value="open" ${h.registration === 'open' ? 'selected' : ''}>Offen für alle</option></select></label>
        <label class="field"><span>Rolle neuer Konten</span><select id="hoRole">${roles.roles.filter((r) => r.id !== 'admin').map((r) => `<option value="${esc(r.id)}" ${r.id === h.defaultRole ? 'selected' : ''}>${esc(r.name)}</option>`).join('')}</select></label>
        <label class="field"><span>Standardquote: Server</span><input type="number" id="hoQS" value="${q.servers || 0}" min="0"></label>
        <label class="field"><span>Standardquote: RAM (MB)</span><input type="number" id="hoQM" value="${q.memoryMB || 0}" min="0"></label>
        <label class="field"><span>Standardquote: CPU-Kerne</span><input type="number" id="hoQC" value="${q.cpus || 0}" min="0" step="0.5"><span class="hint">0 = unbegrenzt</span></label>
        <label class="field"><span>Adresse des Panels</span><input id="hoUrl" value="${esc(h.panelUrl)}" placeholder="https://panel.example.com"><span class="hint">Für Einladungslinks und Links in Alarmen</span></label>
      </div>
      <label class="toggle-row"><span class="t"><b>Kundenserver auf der öffentlichen Seite</b><small>Aus: Server von Kunden bleiben unsichtbar, die öffentliche Seite zeigt nur deine eigenen Server</small></span><span class="switch"><input type="checkbox" id="hoPub" ${h.customerPublic ? 'checked' : ''}><span></span></span></label>
      <label class="toggle-row"><span class="t"><b>Zwei-Faktor-Anmeldung für alle Konten</b><small>Sonst pro Rolle festlegbar</small></span><span class="switch"><input type="checkbox" id="ho2fa" ${h.require2faAll ? 'checked' : ''}><span></span></span></label>
    </div>
    <footer><button class="btn" data-close>Abbrechen</button><button class="btn primary" id="hoSave">${icon('save')}Speichern</button></footer></div>`);
  $('#hoSave', m.el).addEventListener('click', async () => {
    const n = (id) => +$(id, m.el).value || 0;
    try {
      await api('PUT', '/api/hosting', { portMin: n('#hoMin'), portMax: n('#hoMax'), maxServerMB: n('#hoMaxMB'), overheadMB: n('#hoOver'),
        registration: $('#hoReg', m.el).value, defaultRole: $('#hoRole', m.el).value, panelUrl: $('#hoUrl', m.el).value.trim(),
        defaultQuota: { servers: n('#hoQS'), memoryMB: n('#hoQM'), cpus: +$('#hoQC', m.el).value || 0 }, require2faAll: $('#ho2fa', m.el).checked, customerPublic: $('#hoPub', m.el).checked });
      m.close(); toast('Hosting-Einstellungen gespeichert'); refreshMe();
    } catch (e) { toast(e.message, 'err'); }
  });
}

/* =====================================================================
   Users, roles, invitations, audit
   ===================================================================== */
async function renderUsersPage(body) {
  const tabs = [['users', 'Benutzer', 'users', can('users')], ['roles', 'Rollen', 'shield', can('users') || can('roles')], ['invites', 'Einladungen', 'link', can('users')], ['audit', 'Audit-Log', 'clock', can('audit')]].filter((t) => t[3]);
  if (!tabs.some((t) => t[0] === state.usersTab)) state.usersTab = tabs[0][0];
  body.innerHTML = `<nav class="subtabs">${tabs.map(([id, l, ic]) => `<button class="${id === state.usersTab ? 'on' : ''}" data-ut="${id}">${icon(ic)}${l}</button>`).join('')}</nav><div id="utBody"><div class="empty-note"><span class="spinner"></span> Lade …</div></div>`;
  $$('[data-ut]', body).forEach((b) => b.addEventListener('click', () => { state.usersTab = b.dataset.ut; renderUsersPage(body); }));
  $('#pgActions').innerHTML = '';
  const tb = $('#utBody', body);
  const rerender = () => { if (state.page === 'users') renderUsersPage(body); };
  try {
    if (state.usersTab === 'users') await renderUsersTab(tb, rerender);
    else if (state.usersTab === 'roles') await renderRolesTab(tb, rerender);
    else if (state.usersTab === 'invites') await renderInvitesTab(tb, rerender);
    else await renderAuditTab(tb);
  } catch (e) { tb.innerHTML = errBox(e); }
}

async function renderUsersTab(tb, rerender) {
  const d = await api('GET', '/api/users');
  state.userList = d.users.map((u) => u.username);
  const roleName = (id) => d.roles.find((r) => r.id === id)?.name || id;
  $('#pgActions').innerHTML = `<button class="btn primary" id="usNew">${icon('plus')}<span class="lbl">Benutzer anlegen</span></button>`;
  $('#usNew').addEventListener('click', () => openUserEditor(null, d, rerender));
  const owned = (u) => state.servers.filter((s) => s.config.owner === u).length;
  tb.innerHTML = `<div class="panel"><div class="panel-body scroll-x"><table class="tbl users"><thead><tr><th>Benutzer</th><th>Rolle</th><th>Zugriff</th><th>Quote</th><th>Letzte Anmeldung</th><th></th></tr></thead><tbody>
    ${d.users.map((u) => {
      const acc = Object.keys(u.access || {}).length, own = owned(u.username);
      const q = quotaParts(u.quota, d.usage[u.username] || {});
      return `<tr class="${u.disabled ? 'off' : ''}"><td><span class="uav">${icon('user')}</span><b>${esc(u.username)}</b>
          ${u.username === state.me.user ? ' <span class="tag new">du</span>' : ''}${u.disabled ? ' <span class="tag del">gesperrt</span>' : ''}${u.twofa ? ` <span class="tag ok" title="Zwei-Faktor aktiv">2FA</span>` : ''}
          ${u.email ? `<div class="dim small">${esc(u.email)}</div>` : ''}</td>
        <td>${esc(roleName(u.role))}</td>
        <td>${u.allServers ? 'Alle Server' : `${own ? `${own} eigene` : ''}${own && acc ? ' + ' : ''}${acc ? `${acc} zugewiesen` : ''}${!own && !acc ? '<span class="dim">keine</span>' : ''}`}</td>
        <td class="small">${q.length ? esc(q.join(' · ')) : '<span class="dim">—</span>'}</td>
        <td class="dim small">${esc(ago(u.lastLogin))}${u.sessions ? ` · ${u.sessions} aktiv` : ''}</td>
        <td class="right"><button class="btn ghost sm" data-ue="${esc(u.username)}">Bearbeiten</button>${u.username === state.me.user ? '' : `<button class="btn ghost icon sm" data-ud="${esc(u.username)}" title="Löschen">${icon('trash')}</button>`}</td></tr>`;
    }).join('')}</tbody></table></div></div>`;
  $$('[data-ue]', tb).forEach((b) => b.addEventListener('click', () => openUserEditor(d.users.find((u) => u.username === b.dataset.ue), d, rerender)));
  $$('[data-ud]', tb).forEach((b) => b.addEventListener('click', async () => {
    if (!(await confirmDialog({ title: 'Benutzer löschen', body: `<p><b>${esc(b.dataset.ud)}</b> wird gelöscht und sofort abgemeldet.</p>`, ok: 'Löschen', danger: true }))) return;
    try { await api('DELETE', `/api/users/${encodeURIComponent(b.dataset.ud)}`); toast('Benutzer gelöscht'); rerender(); } catch (e) { toast(e.message, 'err'); }
  }));
}

const accessRow = (servers, roles, srv = '', role = 'moderator') => `<div class="row-edit acc" data-row>
  <select data-f="server">${servers.includes(srv) || !srv ? '' : `<option selected>${esc(srv)}</option>`}${servers.map((s) => `<option ${s === srv ? 'selected' : ''}>${esc(s)}</option>`).join('')}</select>
  <select data-f="role">${roles.map((r) => `<option value="${esc(r.id)}" ${r.id === role ? 'selected' : ''}>${esc(r.name)}</option>`).join('')}</select>
  <button type="button" class="btn icon ghost" data-del aria-label="Entfernen">${icon('x')}</button></div>`;

function quotaFields(q = {}, prefix = 'q') {
  return `<div class="fgrid three">
    <label class="field"><span>Server</span><input type="number" min="0" id="${prefix}S" value="${q.servers || 0}"></label>
    <label class="field"><span>RAM (MB, Java-Heap)</span><input type="number" min="0" step="512" id="${prefix}M" value="${q.memoryMB || 0}"></label>
    <label class="field"><span>CPU-Kerne</span><input type="number" min="0" step="0.5" id="${prefix}C" value="${q.cpus || 0}"></label></div>
    <p class="hint-text">0 = unbegrenzt. Die Quote gilt für Server, die dem Benutzer gehören.</p>`;
}
const readQuota = (root, prefix = 'q') => ({ servers: +$(`#${prefix}S`, root).value || 0, memoryMB: +$(`#${prefix}M`, root).value || 0, cpus: +$(`#${prefix}C`, root).value || 0 });

function roleSelect(roles, sel, id) {
  return `<select id="${id}">${roles.map((r) => `<option value="${esc(r.id)}" ${r.id === sel ? 'selected' : ''}>${esc(r.name)}</option>`).join('')}</select>`;
}

function openUserEditor(u, d, rerender) {
  const isNew = !u;
  u = u || { username: '', role: 'customer', allServers: false, access: {}, quota: { ...(d.defaultQuota || { servers: 1, memoryMB: 4096, cpus: 2 }) } };
  const servers = state.servers.map((s) => s.name);
  const usage = d.usage?.[u.username];
  const m = openOverlay(`<div class="modal wide" role="dialog" aria-modal="true">
    <header><h2>${isNew ? 'Benutzer anlegen' : `${esc(u.username)} bearbeiten`}</h2><button class="btn ghost icon sm" data-close aria-label="Schließen">${icon('x')}</button></header>
    <div class="body">
      <div class="fgrid">
        ${isNew ? '<label class="field"><span>Benutzername</span><input id="ueName" autocomplete="off" spellcheck="false" placeholder="z. B. lena"></label>' : ''}
        <label class="field"><span>E-Mail (optional)</span><input id="ueMail" type="email" value="${esc(u.email || '')}"></label>
        <label class="field"><span>Rolle</span>${roleSelect(d.roles, u.role, 'ueRole')}<span class="hint" id="ueRoleDesc"></span></label>
      </div>
      <h3 class="sub">Server-Zugriff</h3>
      <label class="toggle-row"><span class="t"><b>Rolle gilt für alle Server</b><small>Für Team-Mitglieder. Aus: nur eigene Server und die unten zugewiesenen.</small></span><span class="switch"><input type="checkbox" id="ueAll" ${u.allServers ? 'checked' : ''}><span></span></span></label>
      <div id="ueAccWrap"><span class="field-label">Zusätzlich zugewiesene Server (mit eigener Rolle)</span>
        <div class="rows" id="ueAcc">${Object.entries(u.access || {}).map(([s, r]) => accessRow(servers, d.roles, s, r)).join('')}</div>
        <button type="button" class="btn sm" id="ueAddAcc" ${servers.length ? '' : 'disabled'}>${icon('plus')}Server zuweisen</button></div>
      <h3 class="sub">Quote${usage && usage.servers ? ` <span class="dim small">· belegt: ${usage.servers} Server, ${esc(fmtMB(usage.memoryMB))} RAM${usage.cpus ? `, ${esc(fmtCPU(usage.cpus))} CPU` : ''}</span>` : ''}</h3>
      ${quotaFields(u.quota)}
      <h3 class="sub">Anmeldung</h3>
      <div class="fgrid"><label class="field"><span>${isNew ? 'Passwort' : 'Neues Passwort (optional)'}</span><input type="password" id="uePw" autocomplete="new-password"><span class="hint">Mindestens 10 Zeichen</span></label></div>
      ${isNew ? '' : `<label class="toggle-row"><span class="t"><b>Konto gesperrt</b><small>Keine Anmeldung möglich, aktive Sitzungen enden sofort. Server laufen weiter.</small></span><span class="switch"><input type="checkbox" id="ueOff" ${u.disabled ? 'checked' : ''}><span></span></span></label>
      <div class="btn-row">${u.twofa ? `<button class="btn sm" id="ue2fa">${icon('phone')}2FA zurücksetzen</button>` : '<span class="dim small">2FA nicht eingerichtet</span>'}
        <button class="btn sm" id="ueKick" ${u.sessions ? '' : 'disabled'}>${icon('logout')}Abmelden (${u.sessions || 0} Sitzungen)</button></div>`}
    </div>
    <footer><button class="btn" data-close>Abbrechen</button><button class="btn primary" id="ueSave">${icon('save')}Speichern</button></footer></div>`);
  const desc = () => { const r = d.roles.find((x) => x.id === $('#ueRole', m.el).value); $('#ueRoleDesc', m.el).textContent = r ? r.description : ''; };
  $('#ueRole', m.el).addEventListener('change', () => { desc(); if ($('#ueRole', m.el).value === 'admin') $('#ueAll', m.el).checked = true; });
  desc();
  const acc = $('#ueAcc', m.el);
  $('#ueAddAcc', m.el).addEventListener('click', () => acc.insertAdjacentHTML('beforeend', accessRow(servers, d.roles)));
  acc.addEventListener('click', (e) => { const x = e.target.closest('[data-del]'); if (x) x.closest('[data-row]').remove(); });
  $('#ue2fa', m.el)?.addEventListener('click', async () => {
    if (!(await confirmDialog({ title: '2FA zurücksetzen', body: `<p>${esc(u.username)} kann sich danach nur mit Passwort anmelden und muss 2FA neu einrichten.</p>`, ok: 'Zurücksetzen', danger: true }))) return;
    try { await api('POST', `/api/users/${encodeURIComponent(u.username)}/reset-2fa`); toast('2FA zurückgesetzt'); m.close(); rerender(); } catch (e) { toast(e.message, 'err'); }
  });
  $('#ueKick', m.el)?.addEventListener('click', async () => {
    try { const r = await api('DELETE', `/api/users/${encodeURIComponent(u.username)}/sessions`); toast(`${r.revoked} Sitzung(en) beendet`); m.close(); rerender(); } catch (e) { toast(e.message, 'err'); }
  });
  $('#ueSave', m.el).addEventListener('click', async () => {
    const access = {};
    $$('[data-row]', acc).forEach((r) => { access[$('[data-f=server]', r).value] = $('[data-f=role]', r).value; });
    const payload = { role: $('#ueRole', m.el).value, email: $('#ueMail', m.el).value, allServers: $('#ueAll', m.el).checked, access,
      quota: readQuota(m.el), password: $('#uePw', m.el).value, disabled: !!$('#ueOff', m.el)?.checked };
    try {
      if (isNew) await api('POST', '/api/users', { ...payload, username: $('#ueName', m.el).value });
      else await api('PUT', `/api/users/${encodeURIComponent(u.username)}`, payload);
      m.close(); toast('Gespeichert'); state.userList = null; rerender();
    } catch (e) { toast(e.message, 'err'); }
  });
}

async function renderRolesTab(tb, rerender) {
  const d = await api('GET', '/api/roles');
  const edit = can('roles');
  if (edit) {
    $('#pgActions').innerHTML = `<button class="btn primary" id="roNew">${icon('plus')}<span class="lbl">Rolle anlegen</span></button>`;
    $('#roNew').addEventListener('click', () => openRoleEditor(null, d, rerender));
  }
  const srvPerms = d.perms.filter((p) => p.server).map((p) => p.id);
  tb.innerHTML = `<div class="grid roles">${d.roles.map((r) => {
    const sp = r.perms.filter((p) => srvPerms.includes(p)), gp = r.perms.filter((p) => !srvPerms.includes(p));
    return `<div class="panel role-card"><div class="panel-body">
      <div class="rc-head"><b>${esc(r.name)}</b>${r.builtin ? ' <span class="tag">eingebaut</span>' : ''}${r.require2fa ? ' <span class="tag ok">2FA Pflicht</span>' : ''}</div>
      <p class="dim small">${esc(r.description || '')}</p>
      <div class="perm-chips"><span class="pc-l">Server</span>${sp.length ? sp.map((p) => `<span class="pchip">${esc(PERM_SHORT[p] || p)}</span>`).join('') : '<span class="dim small">—</span>'}</div>
      ${gp.length ? `<div class="perm-chips"><span class="pc-l">Panel</span>${gp.map((p) => `<span class="pchip g">${esc(PERM_SHORT[p] || p)}</span>`).join('')}</div>` : ''}
      ${r.consoleDeny?.length || r.consoleAllow?.length ? `<div class="dim small mono">${r.consoleAllow?.length ? `nur: ${esc(r.consoleAllow.join(', '))}` : ''} ${r.consoleDeny?.length ? `gesperrt: ${esc(r.consoleDeny.join(', '))}` : ''}</div>` : ''}
      ${edit && !r.locked ? `<div class="btn-row"><button class="btn sm" data-re="${esc(r.id)}">Bearbeiten</button>${r.builtin ? '' : `<button class="btn ghost icon sm" data-rd="${esc(r.id)}" title="Löschen">${icon('trash')}</button>`}</div>` : ''}
    </div></div>`;
  }).join('')}</div>
  <p class="hint-text">Server-Berechtigungen gelten je nach Benutzer für alle Server, für eigene Server (Kunden) oder für einzeln zugewiesene Server. Panel-Berechtigungen gelten für die Verwaltung insgesamt.</p>`;
  $$('[data-re]', tb).forEach((b) => b.addEventListener('click', () => openRoleEditor(d.roles.find((r) => r.id === b.dataset.re), d, rerender)));
  $$('[data-rd]', tb).forEach((b) => b.addEventListener('click', async () => {
    if (!(await confirmDialog({ title: 'Rolle löschen', body: `<p>Die Rolle <b>${esc(b.dataset.rd)}</b> wird gelöscht.</p>`, ok: 'Löschen', danger: true }))) return;
    try { await api('DELETE', `/api/roles/${encodeURIComponent(b.dataset.rd)}`); rerender(); } catch (e) { toast(e.message, 'err'); }
  }));
}

function openRoleEditor(r, d, rerender) {
  const isNew = !r;
  r = r || { id: '', name: '', description: '', perms: ['server.view'], require2fa: false, consoleAllow: [], consoleDeny: [] };
  const mine = new Set([...(state.me.perms || []), ...(state.me.serverPerms || [])]);
  const grantable = (p) => state.me.role === 'admin' || mine.has(p) || r.perms.includes(p);
  const groups = [...new Set(d.perms.map((p) => p.group))];
  const m = openOverlay(`<div class="modal wide" role="dialog" aria-modal="true">
    <header><h2>${isNew ? 'Rolle anlegen' : `Rolle „${esc(r.name)}“`}</h2><button class="btn ghost icon sm" data-close aria-label="Schließen">${icon('x')}</button></header>
    <div class="body">
      <div class="fgrid">
        <label class="field"><span>Name</span><input id="reName" value="${esc(r.name)}" placeholder="z. B. Support"></label>
        <label class="field"><span>ID</span><input id="reId" value="${esc(r.id)}" ${isNew ? '' : 'disabled'} placeholder="support" spellcheck="false"><span class="hint">a–z, 0–9, _ -</span></label>
        <label class="field full"><span>Beschreibung</span><input id="reDesc" value="${esc(r.description)}"></label>
      </div>
      ${groups.map((g) => `<h3 class="sub">${esc(g)}</h3><div class="perm-grid">${d.perms.filter((p) => p.group === g).map((p) => `<label class="chk perm ${grantable(p.id) ? '' : 'na'}" title="${grantable(p.id) ? '' : 'Diese Berechtigung hast du selbst nicht'}">
        <input type="checkbox" value="${esc(p.id)}" ${r.perms.includes(p.id) ? 'checked' : ''} ${grantable(p.id) ? '' : 'disabled'}><span><b>${esc(PERM_SHORT[p.id] || p.id)}</b><small>${esc(p.label)}</small></span></label>`).join('')}</div>`).join('')}
      <h3 class="sub">Konsole &amp; Sicherheit</h3>
      <div class="fgrid">
        <label class="field"><span>Nur diese Befehle erlauben</span><input id="reAllow" value="${esc((r.consoleAllow || []).join(', '))}" placeholder="leer = alle" spellcheck="false"><span class="hint">Erstes Wort des Befehls, z. B. say, list, tp</span></label>
        <label class="field"><span>Diese Befehle sperren</span><input id="reDeny" value="${esc((r.consoleDeny || []).join(', '))}" placeholder="z. B. op, deop, stop" spellcheck="false"></label>
      </div>
      <label class="toggle-row"><span class="t"><b>Zwei-Faktor-Anmeldung vorschreiben</b><small>Benutzer mit dieser Rolle müssen 2FA einrichten, bevor sie weiterarbeiten können.</small></span><span class="switch"><input type="checkbox" id="re2fa" ${r.require2fa ? 'checked' : ''}><span></span></span></label>
    </div>
    <footer><button class="btn" data-close>Abbrechen</button><button class="btn primary" id="reSave">${icon('save')}Speichern</button></footer></div>`);
  const nameIn = $('#reName', m.el), idIn = $('#reId', m.el);
  if (isNew) nameIn.addEventListener('input', () => { if (!idIn.dataset.touched) idIn.value = nameIn.value.toLowerCase().normalize('NFD').replace(/[̀-ͯ]/g, '').replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '').slice(0, 32); });
  idIn.addEventListener('input', () => { idIn.dataset.touched = '1'; });
  $('#reSave', m.el).addEventListener('click', async () => {
    const words = (v) => v.split(/[\s,;]+/).filter(Boolean);
    const payload = { id: idIn.value.trim(), name: nameIn.value, description: $('#reDesc', m.el).value, require2fa: $('#re2fa', m.el).checked,
      perms: $$('.perm-grid input:checked', m.el).map((i) => i.value), consoleAllow: words($('#reAllow', m.el).value), consoleDeny: words($('#reDeny', m.el).value) };
    try {
      if (isNew) await api('POST', '/api/roles', payload); else await api('PUT', `/api/roles/${encodeURIComponent(r.id)}`, payload);
      m.close(); toast('Rolle gespeichert'); rerender();
    } catch (e) { toast(e.message, 'err'); }
  });
}

async function renderInvitesTab(tb, rerender) {
  const [list, rd] = await Promise.all([api('GET', '/api/invites'), api('GET', '/api/roles')]);
  $('#pgActions').innerHTML = `<button class="btn primary" id="inNew">${icon('plus')}<span class="lbl">Einladung erstellen</span></button>`;
  $('#inNew').addEventListener('click', () => openInviteCreate(rd.roles, rerender));
  const rn = (id) => rd.roles.find((r) => r.id === id)?.name || id;
  tb.innerHTML = `<div class="panel"><div class="panel-body scroll-x">${list.length ? `<table class="tbl"><thead><tr><th>Notiz</th><th>Rolle</th><th>Quote</th><th>Verwendet</th><th>Gültig bis</th><th>Von</th><th></th></tr></thead><tbody>
    ${list.map((i) => `<tr><td>${esc(i.note || '—')}</td><td>${esc(rn(i.role))}${i.allServers ? ' <span class="dim small">(alle Server)</span>' : ''}</td>
      <td class="small">${esc(quotaLimits(i.quota) || '—')}</td>
      <td class="num">${i.uses} / ${i.maxUses}</td><td class="dim">${esc(fmtDate(i.expires))}</td><td class="dim">${esc(i.createdBy)}</td>
      <td class="right"><button class="btn ghost icon sm" data-id="${esc(i.id)}" title="Widerrufen">${icon('trash')}</button></td></tr>`).join('')}</tbody></table>`
    : '<div class="empty-note">Keine offenen Einladungen. Mit einer Einladung legt sich ein Kunde sein Konto selbst an – mit Rolle und Quote, die du vorgibst.</div>'}</div></div>`;
  $$('[data-id]', tb).forEach((b) => b.addEventListener('click', async () => {
    try { await api('DELETE', `/api/invites/${b.dataset.id}`); rerender(); } catch (e) { toast(e.message, 'err'); }
  }));
}

function openInviteCreate(roles, rerender) {
  const m = openOverlay(`<div class="modal wide" role="dialog" aria-modal="true">
    <header><h2>${icon('link')}Einladung erstellen</h2><button class="btn ghost icon sm" data-close aria-label="Schließen">${icon('x')}</button></header>
    <div class="body" id="inBody">
      <div class="fgrid">
        <label class="field"><span>Rolle</span>${roleSelect(roles.filter((r) => r.id !== 'admin' || state.me.role === 'admin'), 'customer', 'inRole')}</label>
        <label class="field"><span>Notiz</span><input id="inNote" placeholder="z. B. Kunde Müller, Paket M"></label>
        <label class="field"><span>Gültig (Tage)</span><input type="number" id="inDays" value="7" min="1" max="90"></label>
        <label class="field"><span>Verwendungen</span><input type="number" id="inUses" value="1" min="1" max="1000"></label>
      </div>
      <label class="toggle-row"><span class="t"><b>Rolle gilt für alle Server</b><small>Nur für Team-Mitglieder sinnvoll</small></span><span class="switch"><input type="checkbox" id="inAll"><span></span></span></label>
      <h3 class="sub">Quote</h3>${quotaFields({ servers: 1, memoryMB: 4096, cpus: 2 }, 'iq')}
    </div>
    <footer><button class="btn" data-close>Schließen</button><button class="btn primary" id="inGo">${icon('link')}Link erzeugen</button></footer></div>`);
  $('#inGo', m.el).addEventListener('click', async () => {
    try {
      const r = await api('POST', '/api/invites', { role: $('#inRole', m.el).value, note: $('#inNote', m.el).value, days: +$('#inDays', m.el).value,
        maxUses: +$('#inUses', m.el).value, allServers: $('#inAll', m.el).checked, quota: readQuota(m.el, 'iq') });
      const link = r.link.startsWith('http') ? r.link : location.origin + r.link;
      $('#inBody', m.el).innerHTML = `<p>Schicke diesen Link an die eingeladene Person. Er ist nur jetzt sichtbar.</p>
        <div class="address"><span class="a mono small">${esc(link)}</span><button class="btn sm" id="inCopy">${icon('copy')}Kopieren</button></div>
        <p class="hint-text">Gültig bis ${esc(fmtDate(r.invite.expires))} · ${r.invite.maxUses}× verwendbar</p>`;
      $('#inGo', m.el).remove();
      $('#inCopy', m.el).addEventListener('click', () => copyText(link));
      rerender();
    } catch (e) { toast(e.message, 'err'); }
  });
}

async function renderAuditTab(tb) {
  const draw = async (user) => {
    const audit = await api('GET', `/api/audit${user ? `?user=${encodeURIComponent(user)}` : ''}`);
    $('#auList', tb).innerHTML = audit.length ? `<table class="tbl compact"><thead><tr><th>Zeit</th><th>Benutzer</th><th>Aktion</th><th>Ziel</th><th>Status</th><th>IP</th></tr></thead><tbody>
      ${audit.map((a) => `<tr class="${a.status >= 400 ? 'bad' : ''}"><td class="dim nowrap">${fmtTime(a.time)}</td><td>${esc(a.user)}</td><td class="mono">${esc(a.action)}</td><td>${esc(a.target || '')}</td><td class="num">${a.status}</td><td class="dim mono">${esc(a.ip || '')}</td></tr>`).join('')}</tbody></table>`
      : '<div class="empty-note">Keine Einträge.</div>';
  };
  tb.innerHTML = `<div class="panel"><div class="panel-head"><h3>${icon('clock')}Protokoll</h3><input id="auUser" class="sm-in" placeholder="Benutzer filtern" spellcheck="false"></div><div class="panel-body audit scroll-x" id="auList"></div></div>`;
  let t;
  $('#auUser', tb).addEventListener('input', (e) => { clearTimeout(t); t = setTimeout(() => draw(e.target.value.trim().toLowerCase()), 300); });
  await draw('');
}

/* =====================================================================
   Alerting
   ===================================================================== */
const CH_TYPES = {
  discord: ['Discord', 'Webhook-URL aus Servereinstellungen → Integrationen → Webhooks'],
  slack: ['Slack / Mattermost', 'Incoming-Webhook-URL'],
  webhook: ['Webhook (JSON)', 'Eigener Endpunkt – bekommt event, level, title, text, server …'],
  ntfy: ['ntfy', 'Topic-URL, z. B. https://ntfy.sh/mein-geheimes-topic'],
  gotify: ['Gotify', 'Server-URL, z. B. https://gotify.example.com'],
  telegram: ['Telegram', 'Bot-Token von @BotFather und Chat-ID'],
  email: ['E-Mail', 'Über den SMTP-Server unten'],
};
const LEVELS = { critical: ['Kritisch', 'error'], warning: ['Warnung', 'unhealthy'], info: ['Info', 'starting'], ok: ['Behoben', 'online'] };

async function renderAlertsPage(body) {
  let d;
  try { d = await api('GET', '/api/alerts'); } catch (e) { body.innerHTML = errBox(e); return; }
  if (state.page !== 'alerts') return;
  const cfg = d.config;
  const save = async (next, msg = 'Gespeichert') => {
    try { await api('PUT', '/api/alerts', next); toast(msg); renderAlertsPage(body); return true; } catch (e) { toast(e.message, 'err'); return false; }
  };
  const evLabel = (id) => d.events.find((e) => e.id === id)?.label || id;
  const chName = (id) => cfg.channels.find((c) => c.id === id)?.name || id;
  const gm = cfg.maintenance?.['*'];
  $('#pgActions').innerHTML = `<button class="btn" id="alSmtp">${icon('mail')}<span class="lbl">E-Mail-Server</span></button>`;
  $('#alSmtp').addEventListener('click', () => openSMTP(cfg, save));
  body.innerHTML = `<div class="stack">
    <div class="grid two">
      <div class="panel"><div class="panel-head"><h3>${icon('bell')}Status</h3></div><div class="panel-body">
        ${d.firing.length ? `<div class="firing">${d.firing.map((f) => `<div class="fire"><span class="dot error"></span><div><b>${esc(f.name)}</b>${f.server ? ` · ${esc(f.server.replace(/^netz:/, 'Netzwerk '))}` : ''}<div class="dim small">${esc(f.detail)} · seit ${esc(fmtTime(f.since))}</div></div></div>`).join('')}</div>`
          : `<div class="all-good">${icon('check')}<span>Keine offenen Alarme</span></div>`}
      </div></div>
      <div class="panel"><div class="panel-head"><h3>${icon('wrench')}Globale Wartung</h3></div><div class="panel-body">
        <p class="muted small">${gm ? `Alle Alarme sind stumm bis <b>${esc(fmtTime(gm))}</b>.` : 'Schaltet alle Alarme vorübergehend stumm – z. B. während eines Host-Updates.'}</p>
        <div class="chips pick">${gm ? '<button class="btn" data-gm="0">Wartung beenden</button>' : [[30, '30 Min.'], [120, '2 Std.'], [480, '8 Std.'], [1440, '1 Tag']].map(([v, l]) => `<button class="btn" data-gm="${v}">${l}</button>`).join('')}</div>
      </div></div>
    </div>

    <div class="panel"><div class="panel-head"><h3>${icon('send2')}Kanäle</h3><button class="btn sm" id="alAddCh">${icon('plus')}Kanal</button></div><div class="panel-body">
      ${cfg.channels.length ? `<div class="ch-list">${cfg.channels.map((c) => `<div class="ch ${c.enabled ? '' : 'off'}"><span class="ch-type">${esc(CH_TYPES[c.type]?.[0] || c.type)}</span>
        <div class="grow"><b>${esc(c.name)}</b><div class="dim small mono">${esc(c.type === 'email' ? c.target : c.type === 'telegram' ? `Chat ${c.target}` : c.url)}</div></div>
        <div class="ch-actions">${c.enabled ? '' : '<span class="tag">aus</span>'}<button class="btn sm" data-test="${esc(c.id)}">${icon('send')}Test</button><button class="btn ghost sm" data-ce="${esc(c.id)}">Bearbeiten</button><button class="btn ghost icon sm" data-cd="${esc(c.id)}" title="Löschen">${icon('trash')}</button></div></div>`).join('')}</div>`
        : '<div class="empty-note">Noch kein Kanal. Lege zuerst fest, wohin Alarme geschickt werden – Discord, Telegram, ntfy, E-Mail …</div>'}
    </div></div>

    <div class="panel"><div class="panel-head"><h3>${icon('alert')}Regeln</h3><div class="btn-group">
      ${cfg.rules.length ? '' : `<button class="btn sm" id="alDefaults" ${cfg.channels.length ? '' : 'disabled title="Erst einen Kanal anlegen"'}>${icon('star')}Empfohlene Regeln</button>`}
      <button class="btn sm" id="alAddRule" ${cfg.channels.length ? '' : 'disabled'}>${icon('plus')}Regel</button></div></div><div class="panel-body scroll-x">
      ${cfg.rules.length ? `<table class="tbl rules"><thead><tr><th></th><th>Regel</th><th>Bedingung</th><th>Server</th><th>Kanäle</th><th></th></tr></thead><tbody>
        ${cfg.rules.map((r) => { const ev = d.events.find((e) => e.id === r.event) || {}; return `<tr class="${r.enabled ? '' : 'off'}">
          <td><label class="switch sm"><input type="checkbox" data-rt="${esc(r.id)}" ${r.enabled ? 'checked' : ''}><span></span></label></td>
          <td><b>${esc(r.name)}</b>${r.name !== ev.label ? `<div class="dim small">${esc(ev.label || r.event)}</div>` : ''}</td>
          <td class="small">${ev.threshold ? `${ev.id === 'tps' || ev.id === 'disk' ? '&lt;' : '≥'} ${r.threshold} ${esc(ev.unit || '')}` : ''}${r.minutes ? ` · ${r.minutes} Min.` : ''}${r.cooldown ? ` · Pause ${r.cooldown} Min.` : ''}${!ev.threshold && !r.minutes && !r.cooldown ? 'sofort' : ''}</td>
          <td class="small">${ev.server ? (r.servers?.length ? esc(r.servers.join(', ')) : 'alle') : '<span class="dim">—</span>'}</td>
          <td class="small">${esc(r.channels.map(chName).join(', ') || '—')}</td>
          <td class="right"><button class="btn ghost sm" data-re="${esc(r.id)}">Bearbeiten</button><button class="btn ghost icon sm" data-rd="${esc(r.id)}" title="Löschen">${icon('trash')}</button></td></tr>`; }).join('')}</tbody></table>`
        : '<div class="empty-note">Noch keine Regeln. „Empfohlene Regeln“ legt sinnvolle Standards für alle Server an.</div>'}
    </div></div>

    <div class="panel"><div class="panel-head"><h3>${icon('clock')}Verlauf</h3><span class="dim small">${d.history.length} Einträge</span></div><div class="panel-body scroll-x">
      ${d.history.length ? `<table class="tbl compact"><thead><tr><th>Zeit</th><th>Stufe</th><th>Alarm</th><th>Zustellung</th></tr></thead><tbody>
        ${d.history.map((h) => `<tr><td class="dim nowrap">${fmtTime(h.time)}</td><td><span class="pill ${LEVELS[h.level]?.[1] || ''}"><span class="dot ${LEVELS[h.level]?.[1] || ''}"></span>${esc(LEVELS[h.level]?.[0] || h.level)}</span></td>
          <td><b>${esc(h.title)}</b><div class="dim small">${esc(h.text)}</div></td><td class="small ${h.muted ? 'dim' : ''}">${(h.results || []).map((x) => `<div class="${/zugestellt/.test(x) ? '' : 'warn-text'}">${esc(x)}</div>`).join('')}</td></tr>`).join('')}</tbody></table>`
        : '<div class="empty-note">Noch keine Alarme ausgelöst.</div>'}
    </div></div>
  </div>`;

  $$('[data-gm]', body).forEach((b) => b.addEventListener('click', async () => {
    try { await api('PUT', '/api/alerts/maintenance', { minutes: +b.dataset.gm }); renderAlertsPage(body); } catch (e) { toast(e.message, 'err'); }
  }));
  $('#alAddCh', body).addEventListener('click', () => openChannelEditor(null, cfg, save));
  $$('[data-ce]', body).forEach((b) => b.addEventListener('click', () => openChannelEditor(cfg.channels.find((c) => c.id === b.dataset.ce), cfg, save)));
  $$('[data-cd]', body).forEach((b) => b.addEventListener('click', async () => {
    const used = cfg.rules.filter((r) => r.channels.includes(b.dataset.cd));
    if (!(await confirmDialog({ title: 'Kanal löschen', body: `<p>${esc(chName(b.dataset.cd))} wird gelöscht${used.length ? ` und aus ${used.length} Regel(n) entfernt` : ''}.</p>`, ok: 'Löschen', danger: true }))) return;
    save({ ...cfg, channels: cfg.channels.filter((c) => c.id !== b.dataset.cd), rules: cfg.rules.map((r) => ({ ...r, channels: r.channels.filter((c) => c !== b.dataset.cd) })) }, 'Kanal gelöscht');
  }));
  $$('[data-test]', body).forEach((b) => b.addEventListener('click', async () => {
    b.disabled = true; b.innerHTML = '<span class="spinner"></span>Sende …';
    try { await api('POST', '/api/alerts/test', { channel: b.dataset.test }); toast('Testnachricht zugestellt'); } catch (e) { toast(e.message, 'err'); }
    b.disabled = false; b.innerHTML = `${icon('send')}Test`;
  }));
  $('#alAddRule', body).addEventListener('click', () => openRuleEditor(null, cfg, d.events, save));
  $$('[data-re]', body).forEach((b) => b.addEventListener('click', () => openRuleEditor(cfg.rules.find((r) => r.id === b.dataset.re), cfg, d.events, save)));
  $$('[data-rd]', body).forEach((b) => b.addEventListener('click', () => save({ ...cfg, rules: cfg.rules.filter((r) => r.id !== b.dataset.rd) }, 'Regel gelöscht')));
  $$('[data-rt]', body).forEach((i) => i.addEventListener('change', () => save({ ...cfg, rules: cfg.rules.map((r) => r.id === i.dataset.rt ? { ...r, enabled: i.checked } : r) }, i.checked ? 'Regel aktiviert' : 'Regel pausiert')));
  $('#alDefaults', body)?.addEventListener('click', () => {
    const ch = cfg.channels.filter((c) => c.enabled).map((c) => c.id);
    const mk = (event, extra = {}) => ({ id: '', name: '', event, servers: [], threshold: 0, minutes: d.events.find((e) => e.id === event).minutes, cooldown: 0, channels: ch, resolve: true, enabled: true, ...extra });
    save({ ...cfg, rules: [mk('down'), mk('crashloop', { cooldown: 30 }), mk('unhealthy'), mk('tps', { cooldown: 30 }), mk('memory', { cooldown: 60 }), mk('disk', { cooldown: 360 }),
      mk('proxy_down'), mk('task_failed', { resolve: false, cooldown: 5 }), mk('login_failed', { resolve: false, cooldown: 30 }), mk('admin_login', { resolve: false })] }, 'Empfohlene Regeln angelegt');
  });
}

function openChannelEditor(c, cfg, save) {
  const isNew = !c;
  c = c || { id: '', name: '', type: 'discord', url: '', token: '', target: '', enabled: true };
  const m = openOverlay(`<div class="modal wide" role="dialog" aria-modal="true">
    <header><h2>${isNew ? 'Kanal hinzufügen' : `Kanal „${esc(c.name)}“`}</h2><button class="btn ghost icon sm" data-close aria-label="Schließen">${icon('x')}</button></header>
    <div class="body">
      <div class="ch-types">${Object.entries(CH_TYPES).map(([id, [l]]) => `<button type="button" class="btn ${id === c.type ? 'primary' : ''}" data-ct="${id}">${esc(l)}</button>`).join('')}</div>
      <p class="hint-text" id="chHelp"></p>
      <div class="fgrid">
        <label class="field"><span>Name</span><input id="chName" value="${esc(c.name)}" placeholder="z. B. Team-Discord"></label>
        <label class="field" data-show="discord slack webhook ntfy gotify telegram"><span id="chUrlL">URL</span><input id="chUrl" value="${esc(c.url)}" spellcheck="false"></label>
        <label class="field" data-show="webhook ntfy gotify telegram"><span id="chTokL">Token</span><input id="chTok" value="${esc(c.token)}" type="password" autocomplete="off"></label>
        <label class="field" data-show="telegram email"><span id="chTgtL">Empfänger</span><input id="chTgt" value="${esc(c.target)}" spellcheck="false"></label>
      </div>
      <label class="toggle-row"><span class="t"><b>Aktiv</b></span><span class="switch"><input type="checkbox" id="chOn" ${c.enabled ? 'checked' : ''}><span></span></span></label>
    </div>
    <footer><button class="btn" data-close>Abbrechen</button><button class="btn primary" id="chSave">${icon('save')}Speichern</button></footer></div>`);
  let type = c.type;
  const labels = {
    discord: ['Webhook-URL', '', ''], slack: ['Webhook-URL', '', ''], webhook: ['URL', 'Bearer-Token (optional)', ''],
    ntfy: ['Topic-URL', 'Zugangstoken (optional)', ''], gotify: ['Server-URL', 'App-Token', ''], telegram: ['API-URL (optional)', 'Bot-Token', 'Chat-ID'], email: ['', '', 'Empfänger (kommagetrennt)'],
  };
  const draw = () => {
    $$('[data-ct]', m.el).forEach((b) => b.classList.toggle('primary', b.dataset.ct === type));
    $$('[data-show]', m.el).forEach((el) => el.classList.toggle('hidden', !el.dataset.show.split(' ').includes(type)));
    $('#chHelp', m.el).textContent = CH_TYPES[type][1];
    const [u, t, g] = labels[type];
    $('#chUrlL', m.el).textContent = u; $('#chTokL', m.el).textContent = t; $('#chTgtL', m.el).textContent = g;
    $('#chUrl', m.el).placeholder = type === 'telegram' ? 'https://api.telegram.org' : type === 'ntfy' ? 'https://ntfy.sh/craftdock-xyz' : 'https://…';
  };
  $$('[data-ct]', m.el).forEach((b) => b.addEventListener('click', () => { type = b.dataset.ct; draw(); }));
  draw();
  $('#chSave', m.el).addEventListener('click', async () => {
    const ch = { id: c.id, name: $('#chName', m.el).value.trim() || CH_TYPES[type][0], type, url: $('#chUrl', m.el).value.trim(), token: $('#chTok', m.el).value, target: $('#chTgt', m.el).value.trim(), enabled: $('#chOn', m.el).checked };
    const channels = isNew ? [...cfg.channels, ch] : cfg.channels.map((x) => x.id === c.id ? ch : x);
    if (await save({ ...cfg, channels }, 'Kanal gespeichert')) m.close();
  });
}

function openRuleEditor(r, cfg, events, save) {
  const isNew = !r;
  r = r || { id: '', name: '', event: 'down', servers: [], threshold: 0, minutes: 3, cooldown: 0, channels: cfg.channels.map((c) => c.id), resolve: true, enabled: true };
  const m = openOverlay(`<div class="modal wide" role="dialog" aria-modal="true">
    <header><h2>${isNew ? 'Regel anlegen' : 'Regel bearbeiten'}</h2><button class="btn ghost icon sm" data-close aria-label="Schließen">${icon('x')}</button></header>
    <div class="body">
      <label class="field"><span>Ereignis</span><select id="ruEv">${events.map((e) => `<option value="${e.id}" ${e.id === r.event ? 'selected' : ''}>${esc(e.label)}</option>`).join('')}</select><span class="hint" id="ruDesc"></span></label>
      <div class="fgrid">
        <label class="field"><span>Name</span><input id="ruName" value="${esc(r.name)}" placeholder="Standard: Name des Ereignisses"></label>
        <label class="field" id="ruThrW"><span id="ruThrL">Schwelle</span><input type="number" id="ruThr" value="${r.threshold || ''}" step="any" min="0"></label>
        <label class="field" id="ruMinW"><span>Dauer (Minuten)</span><input type="number" id="ruMin" value="${r.minutes}" min="0" max="1440"><span class="hint">So lange muss der Zustand anhalten</span></label>
        <label class="field"><span>Pause zwischen Meldungen (Min.)</span><input type="number" id="ruCool" value="${r.cooldown}" min="0"></label>
      </div>
      <div id="ruSrvW"><span class="field-label">Server</span><div class="srv-pick"><label class="chk"><input type="checkbox" id="ruAll" ${r.servers?.length ? '' : 'checked'}> Alle Server (auch künftige)</label>
        <div id="ruList" class="${r.servers?.length ? '' : 'hidden'}">${state.servers.map((s) => `<label class="chk"><input type="checkbox" value="${esc(s.name)}" ${r.servers?.includes(s.name) ? 'checked' : ''}> ${esc(s.name)}</label>`).join('')}</div></div></div>
      <span class="field-label">Kanäle</span><div class="srv-pick">${cfg.channels.map((c) => `<label class="chk"><input type="checkbox" data-ch value="${esc(c.id)}" ${r.channels.includes(c.id) ? 'checked' : ''}> ${esc(c.name)} <span class="dim small">${esc(CH_TYPES[c.type]?.[0] || c.type)}</span></label>`).join('')}</div>
      <label class="toggle-row" id="ruResW"><span class="t"><b>Entwarnung senden</b><small>Meldung, sobald das Problem behoben ist</small></span><span class="switch"><input type="checkbox" id="ruRes" ${r.resolve ? 'checked' : ''}><span></span></span></label>
    </div>
    <footer><button class="btn" data-close>Abbrechen</button><button class="btn primary" id="ruSave">${icon('save')}Speichern</button></footer></div>`);
  const draw = (changed) => {
    const ev = events.find((e) => e.id === $('#ruEv', m.el).value);
    $('#ruDesc', m.el).textContent = ev.description;
    $('#ruThrW', m.el).classList.toggle('hidden', !ev.threshold);
    $('#ruThrL', m.el).textContent = `Schwelle (${ev.unit})`;
    if (changed) { $('#ruThr', m.el).value = ev.threshold || ''; $('#ruMin', m.el).value = ev.minutes; }
    $('#ruMinW', m.el).classList.toggle('hidden', !ev.state);
    $('#ruResW', m.el).classList.toggle('hidden', !ev.state);
    $('#ruSrvW', m.el).classList.toggle('hidden', !ev.server);
  };
  $('#ruEv', m.el).addEventListener('change', () => draw(true));
  $('#ruAll', m.el).addEventListener('change', (e) => $('#ruList', m.el).classList.toggle('hidden', e.target.checked));
  draw(false);
  $('#ruSave', m.el).addEventListener('click', async () => {
    const rule = { ...r, event: $('#ruEv', m.el).value, name: $('#ruName', m.el).value.trim(), threshold: +$('#ruThr', m.el).value || 0, minutes: +$('#ruMin', m.el).value || 0,
      cooldown: +$('#ruCool', m.el).value || 0, resolve: $('#ruRes', m.el).checked,
      servers: $('#ruAll', m.el).checked ? [] : $$('#ruList input:checked', m.el).map((i) => i.value), channels: $$('[data-ch]:checked', m.el).map((i) => i.value) };
    if (isNew && !rule.name) rule.name = '';
    const rules = isNew ? [...cfg.rules, rule] : cfg.rules.map((x) => x.id === r.id ? rule : x);
    if (await save({ ...cfg, rules }, 'Regel gespeichert')) m.close();
  });
}

function openSMTP(cfg, save) {
  const s = cfg.smtp || {};
  const m = openOverlay(`<div class="modal wide" role="dialog" aria-modal="true">
    <header><h2>${icon('mail')}E-Mail-Server (SMTP)</h2><button class="btn ghost icon sm" data-close aria-label="Schließen">${icon('x')}</button></header>
    <div class="body"><div class="fgrid">
      <label class="field"><span>Server</span><input id="smHost" value="${esc(s.host || '')}" placeholder="smtp.example.com"></label>
      <label class="field"><span>Port</span><input type="number" id="smPort" value="${s.port || ''}" placeholder="automatisch"></label>
      <label class="field"><span>Verschlüsselung</span><select id="smSec">${[['starttls', 'STARTTLS (587)'], ['tls', 'TLS (465)'], ['none', 'Keine (25, nur intern)']].map(([v, l]) => `<option value="${v}" ${v === s.security ? 'selected' : ''}>${l}</option>`).join('')}</select></label>
      <label class="field"><span>Absender</span><input id="smFrom" value="${esc(s.from || '')}" placeholder="craftdock@example.com"></label>
      <label class="field"><span>Benutzer</span><input id="smUser" value="${esc(s.username || '')}" autocomplete="off"></label>
      <label class="field"><span>Passwort</span><input id="smPw" type="password" value="${esc(s.password || '')}" autocomplete="new-password"></label>
    </div><p class="hint-text">Teste den Versand mit einem E-Mail-Kanal und dem Test-Knopf.</p></div>
    <footer><button class="btn" data-close>Abbrechen</button><button class="btn primary" id="smSave">${icon('save')}Speichern</button></footer></div>`);
  $('#smSave', m.el).addEventListener('click', async () => {
    const smtp = { host: $('#smHost', m.el).value.trim(), port: +$('#smPort', m.el).value || 0, security: $('#smSec', m.el).value, from: $('#smFrom', m.el).value.trim(),
      username: $('#smUser', m.el).value, password: $('#smPw', m.el).value };
    if (await save({ ...cfg, smtp }, 'SMTP gespeichert')) m.close();
  });
}
