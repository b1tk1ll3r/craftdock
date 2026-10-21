package main

// Mod & plugin management: Modrinth search/metadata, modpack info and the
// jar files in /data/mods or /data/plugins.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var modrinthAPI = envOr("CRAFTDOCK_MODRINTH_API", "https://api.modrinth.com/v2")

var httpc = &http.Client{Timeout: 12 * time.Second}

type ModrinthProject struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Description string `json:"description"`
	IconURL     string `json:"icon_url"`
	Downloads   int    `json:"downloads"`
	ProjectType string `json:"project_type"`
	ServerSide  string `json:"server_side"`
	ClientSide  string `json:"client_side"`
	Author      string `json:"author,omitempty"`
	URL         string `json:"url"`
	Ref         string `json:"ref,omitempty"` // entry as written in MODRINTH_PROJECTS
}

type modrinthCacheEntry struct {
	p  *ModrinthProject // nil = unknown project
	at time.Time
}

type ModFile struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

type modFilesCacheEntry struct {
	files []ModFile
	at    time.Time
}

func modrinthGet(ctx context.Context, path string, q url.Values, out any) error {
	u := modrinthAPI + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)
	req.Header.Set("User-Agent", "craftdock/"+appVersion+" (github.com/b1tk1ll3r/craftdock)")
	resp, err := httpc.Do(req)
	if err != nil {
		return fmt.Errorf("Modrinth nicht erreichbar: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("Modrinth: HTTP %d %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func projectURL(p *ModrinthProject) string {
	t := p.ProjectType
	if t == "" {
		t = "project"
	}
	return "https://modrinth.com/" + t + "/" + p.Slug
}

// refSlug extracts the project slug/id from a MODRINTH_PROJECTS entry
// ("lithium", "lithium:mc1.21-0.13.0", "datapack:terralith").
func refSlug(ref string) string {
	parts := strings.Split(ref, ":")
	if len(parts) > 1 && (parts[0] == "datapack" || parts[0] == "fabric" || parts[0] == "forge" || parts[0] == "neoforge" || parts[0] == "quilt" || parts[0] == "paper") {
		parts = parts[1:]
	}
	return strings.ToLower(parts[0])
}

// ModrinthProjects resolves metadata for refs, cached for six hours.
func (m *Manager) ModrinthProjects(ctx context.Context, refs []string) []ModrinthProject {
	var missing []string
	m.modMu.Lock()
	for _, r := range refs {
		e, ok := m.modrinth[refSlug(r)]
		if !ok || time.Since(e.at) > 6*time.Hour {
			missing = append(missing, refSlug(r))
		}
	}
	m.modMu.Unlock()

	if len(missing) > 0 {
		ids, _ := json.Marshal(missing)
		var got []ModrinthProject
		if err := modrinthGet(ctx, "/projects", url.Values{"ids": {string(ids)}}, &got); err == nil {
			m.modMu.Lock()
			found := map[string]bool{}
			for i := range got {
				p := got[i]
				p.URL = projectURL(&p)
				for _, key := range []string{strings.ToLower(p.Slug), strings.ToLower(p.ID)} {
					m.modrinth[key] = modrinthCacheEntry{&p, time.Now()}
					found[key] = true
				}
			}
			for _, s := range missing {
				if !found[s] {
					m.modrinth[s] = modrinthCacheEntry{nil, time.Now()}
				}
			}
			m.modMu.Unlock()
		}
	}

	out := make([]ModrinthProject, 0, len(refs))
	m.modMu.Lock()
	defer m.modMu.Unlock()
	for _, r := range refs {
		if e, ok := m.modrinth[refSlug(r)]; ok && e.p != nil {
			p := *e.p
			p.Ref = r
			out = append(out, p)
		} else {
			out = append(out, ModrinthProject{Slug: refSlug(r), Title: refSlug(r), Ref: r, URL: "https://modrinth.com/project/" + url.PathEscape(refSlug(r))})
		}
	}
	return out
}

type SearchResult struct {
	Hits        []ModrinthProject `json:"hits"`
	Total       int               `json:"total"`
	Loader      string            `json:"loader"`
	GameVersion string            `json:"gameVersion"`
	ProjectType string            `json:"projectType"`
}

var loaderCategories = map[string][]string{
	"fabric":   {"fabric"},
	"quilt":    {"quilt", "fabric"},
	"forge":    {"forge"},
	"neoforge": {"neoforge"},
	"paper":    {"paper", "spigot", "bukkit"},
	"purpur":   {"purpur", "paper", "spigot", "bukkit"},
	"folia":    {"folia"},
	"spigot":   {"spigot", "bukkit"},
}

func (m *Manager) ModrinthSearch(ctx context.Context, query, loader, gameVersion string, offset int) (SearchResult, error) {
	cats, ok := loaderCategories[loader]
	if !ok {
		return SearchResult{}, badRequest("Unbekannter Loader %q", loader)
	}
	ptype := "mod"
	if loader == "paper" || loader == "purpur" || loader == "folia" || loader == "spigot" {
		ptype = "plugin"
	}
	facets := [][]string{{"project_type:" + ptype}}
	var catFacet []string
	for _, c := range cats {
		catFacet = append(catFacet, "categories:"+c)
	}
	facets = append(facets, catFacet)
	if gameVersion != "" {
		facets = append(facets, []string{"versions:" + gameVersion})
	}
	fj, _ := json.Marshal(facets)
	q := url.Values{"query": {query}, "facets": {string(fj)}, "limit": {"20"}, "offset": {strconv.Itoa(offset)}}
	if query == "" {
		q.Set("index", "downloads")
	}
	var res struct {
		Hits      []ModrinthProject `json:"hits"`
		TotalHits int               `json:"total_hits"`
	}
	if err := modrinthGet(ctx, "/search", q, &res); err != nil {
		return SearchResult{}, &APIError{http.StatusBadGateway, err.Error()}
	}
	out := SearchResult{Hits: []ModrinthProject{}, Total: res.TotalHits, Loader: loader, GameVersion: gameVersion, ProjectType: ptype}
	for i := range res.Hits {
		h := res.Hits[i]
		if h.ServerSide == "unsupported" {
			continue // client-only mods are useless on a server
		}
		if h.ID == "" {
			h.ID = h.Slug
		}
		h.URL = projectURL(&h)
		out.Hits = append(out.Hits, h)
	}
	return out, nil
}

// ---- jar files ---------------------------------------------------------------------------

var jarNameRe = regexp.MustCompile(`^[^/\\\x00-\x1f.][^/\\\x00-\x1f]{0,200}\.jar$`)

// runInServer runs a shell script against the server's /data, via exec when
// the container is running, otherwise via a throwaway helper container.
func (m *Manager) runInServer(ctx context.Context, c *ContainerSummary, script string, args ...string) (string, error) {
	cmd := append([]string{"sh", "-c", script, "sh"}, args...)
	if c.State == "running" {
		out, code, err := m.d.Exec(ctx, c.ID, cmd)
		if err != nil {
			return "", err
		}
		if code != 0 {
			return "", fmt.Errorf("Befehl fehlgeschlagen (%d): %s", code, strings.TrimSpace(out))
		}
		return out, nil
	}
	cfg := configFromLabels(c.Labels)
	bind := volumeName(cfg.Name) + ":/data"
	if cfg.DataPath != "" {
		bind = cfg.DataPath + ":/data"
	}
	return m.d.RunHelper(ctx, c.Image, []string{bind}, cmd)
}

const listJarsScript = `cd "/data/$1" 2>/dev/null || exit 0
for f in *.jar *.jar.disabled; do [ -f "$f" ] && stat -c '%s %n' -- "$f"; done; exit 0`

func (m *Manager) listModFiles(ctx context.Context, c *ContainerSummary, dir string) ([]ModFile, error) {
	out, err := m.runInServer(ctx, c, listJarsScript, dir)
	if err != nil {
		return nil, err
	}
	files := []ModFile{}
	for _, line := range strings.Split(out, "\n") {
		sz, name, ok := strings.Cut(strings.TrimSpace(line), " ")
		if !ok {
			continue
		}
		n, err := strconv.ParseInt(sz, 10, 64)
		if err != nil {
			continue
		}
		files = append(files, ModFile{Name: name, Size: n})
	}
	sort.Slice(files, func(i, j int) bool { return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name) })
	m.modMu.Lock()
	m.modFiles[configFromLabels(c.Labels).Name] = modFilesCacheEntry{files, time.Now()}
	m.modMu.Unlock()
	return files, nil
}

// cachedModFiles is used by public pages: never spawns helper containers.
func (m *Manager) cachedModFiles(ctx context.Context, c *ContainerSummary, dir string) []ModFile {
	name := configFromLabels(c.Labels).Name
	m.modMu.Lock()
	e, ok := m.modFiles[name]
	m.modMu.Unlock()
	if ok && time.Since(e.at) < 5*time.Minute {
		return e.files
	}
	if c.State == "running" && dir != "" {
		if f, err := m.listModFiles(ctx, c, dir); err == nil {
			return f
		}
	}
	if ok {
		return e.files
	}
	return nil
}

type PackInfo struct {
	Source      string `json:"source"` // modrinth | curseforge
	Ref         string `json:"ref"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	IconURL     string `json:"iconUrl,omitempty"`
	URL         string `json:"url"`
}

type ModsInfo struct {
	Kind        string            `json:"kind"` // mods | plugins | none
	Dir         string            `json:"dir"`
	Loader      string            `json:"loader"`
	GameVersion string            `json:"gameVersion"`
	Modpack     *PackInfo         `json:"modpack,omitempty"`
	Projects    []ModrinthProject `json:"projects"`
	Files       []ModFile         `json:"files"`
	FilesError  string            `json:"filesError,omitempty"`
}

var modrinthPackURL = regexp.MustCompile(`modrinth\.com/modpack/([A-Za-z0-9_-]+)`)
var cfPackURL = regexp.MustCompile(`curseforge\.com/minecraft/modpacks/([A-Za-z0-9_-]+)`)

func (m *Manager) packInfo(ctx context.Context, cfg ServerConfig) *PackInfo {
	switch cfg.Type {
	case "MODRINTH":
		ref := cfg.Modpack
		if mm := modrinthPackURL.FindStringSubmatch(ref); mm != nil {
			ref = mm[1]
		}
		pi := &PackInfo{Source: "modrinth", Ref: cfg.Modpack, Title: ref, URL: "https://modrinth.com/modpack/" + url.PathEscape(ref)}
		if !strings.Contains(ref, "/") {
			if ps := m.ModrinthProjects(ctx, []string{ref}); len(ps) == 1 && ps[0].ID != "" {
				pi.Title, pi.Description, pi.IconURL, pi.URL = ps[0].Title, ps[0].Description, ps[0].IconURL, ps[0].URL
			}
		}
		return pi
	case "AUTO_CURSEFORGE":
		slug := cfg.Modpack
		if mm := cfPackURL.FindStringSubmatch(slug); mm != nil {
			slug = mm[1]
		}
		return &PackInfo{Source: "curseforge", Ref: cfg.Modpack, Title: slug, URL: "https://www.curseforge.com/minecraft/modpacks/" + url.PathEscape(slug)}
	}
	return nil
}

// resolvedVersion returns the concrete game version (from the live ping or Mojang).
func (m *Manager) resolvedVersion(ctx context.Context, c *ContainerSummary, cfg ServerConfig) string {
	if lv := m.Live(ctx, c, 60*time.Second); lv.Reachable && versionNumRe.MatchString(lv.Version) {
		return versionNumRe.FindString(lv.Version)
	}
	if cfg.Version != "LATEST" && cfg.Version != "SNAPSHOT" {
		return cfg.Version
	}
	if vl, err := m.Versions(ctx); err == nil {
		if cfg.Version == "SNAPSHOT" {
			return vl.Snapshot
		}
		return vl.Latest
	}
	return ""
}

var versionNumRe = regexp.MustCompile(`\b1\.[0-9]+(\.[0-9]+)?\b|\b[2-9][0-9]\.[0-9]+(\.[0-9]+)?\b`)

func (m *Manager) Mods(ctx context.Context, name string) (ModsInfo, error) {
	c, err := m.find(ctx, name)
	if err != nil {
		return ModsInfo{}, err
	}
	cfg := configFromLabels(c.Labels)
	info := ModsInfo{Dir: cfg.ModDir(), Loader: cfg.Loader(), Projects: []ModrinthProject{}, Files: []ModFile{}}
	switch info.Dir {
	case "plugins":
		info.Kind = "plugins"
	case "mods":
		info.Kind = "mods"
	default:
		info.Kind = "none"
	}
	info.GameVersion = m.resolvedVersion(ctx, c, cfg)
	info.Modpack = m.packInfo(ctx, cfg)
	if cfg.ModrinthProjects != "" {
		info.Projects = m.ModrinthProjects(ctx, strings.Split(cfg.ModrinthProjects, ","))
	}
	if info.Dir != "" {
		if f, err := m.listModFiles(ctx, c, info.Dir); err != nil {
			info.FilesError = err.Error()
		} else {
			info.Files = f
		}
	}
	return info, nil
}

func (m *Manager) SetModrinthProjects(ctx context.Context, name string, refs []string) error {
	c, err := m.find(ctx, name)
	if err != nil {
		return err
	}
	cfg := configFromLabels(c.Labels)
	seen := map[string]bool{}
	var clean []string
	for _, r := range refs {
		r = strings.TrimSpace(r)
		if r == "" || seen[strings.ToLower(r)] {
			continue
		}
		if !modpackRe.MatchString(r) || strings.ContainsAny(r, ",/") {
			return badRequest("Ungültiger Projekteintrag %q", r)
		}
		seen[strings.ToLower(r)] = true
		clean = append(clean, r)
	}
	cfg.ModrinthProjects = strings.Join(clean, ",")
	cfg.CFApiKey = "" // keep stored key
	return m.recreate(ctx, name, &cfg, false)
}

func (m *Manager) UploadModFile(ctx context.Context, name, file string, r io.Reader, size int64) error {
	if !jarNameRe.MatchString(file) {
		return badRequest("Nur .jar-Dateien mit einfachem Dateinamen sind erlaubt")
	}
	c, err := m.find(ctx, name)
	if err != nil {
		return err
	}
	cfg := configFromLabels(c.Labels)
	if cfg.ModDir() == "" {
		return badRequest("Vanilla-Server unterstützen keine Mods oder Plugins")
	}
	err = m.d.PutFile(ctx, c.ID, "/data", cfg.ModDir()+"/"+file, r, size, 1000, 1000)
	m.invalidateModFiles(name)
	return err
}

func (m *Manager) DeleteModFile(ctx context.Context, name, file string) error {
	if !jarNameRe.MatchString(file) && !(strings.HasSuffix(file, ".jar.disabled") && jarNameRe.MatchString(strings.TrimSuffix(file, ".disabled"))) {
		return badRequest("Ungültiger Dateiname")
	}
	c, err := m.find(ctx, name)
	if err != nil {
		return err
	}
	cfg := configFromLabels(c.Labels)
	if cfg.ModDir() == "" {
		return errors.New("keine Mod-Verzeichnis")
	}
	_, err = m.runInServer(ctx, c, `rm -f -- "/data/$1/$2"`, cfg.ModDir(), file)
	m.invalidateModFiles(name)
	return err
}

func (m *Manager) invalidateModFiles(name string) {
	m.modMu.Lock()
	delete(m.modFiles, name)
	m.modMu.Unlock()
}

// ---- update compatibility check ------------------------------------------------------

type CompatItem struct {
	Ref     string `json:"ref"`
	Title   string `json:"title"`
	IconURL string `json:"iconUrl,omitempty"`
	URL     string `json:"url,omitempty"`
	Status  string `json:"status"` // ok | missing | unknown
	Version string `json:"version,omitempty"`
}

type CompatResult struct {
	Loader      string       `json:"loader"`
	GameVersion string       `json:"gameVersion"`
	Items       []CompatItem `json:"items"`
	OK          int          `json:"ok"`
	Missing     int          `json:"missing"`
	Unchecked   []string     `json:"unchecked"` // jars not managed via Modrinth
}

var modLoaders = map[string][]string{
	"fabric": {"fabric"}, "quilt": {"quilt", "fabric"}, "forge": {"forge"}, "neoforge": {"neoforge"},
	"paper": {"paper", "spigot", "bukkit"}, "purpur": {"purpur", "paper", "spigot", "bukkit"},
	"folia": {"folia"}, "spigot": {"spigot", "bukkit"},
}

// CompatCheck asks Modrinth whether each project has a release for the target
// loader and game version.
func (m *Manager) CompatCheck(ctx context.Context, name, targetType, targetVersion string) (CompatResult, error) {
	c, err := m.find(ctx, name)
	if err != nil {
		return CompatResult{}, err
	}
	cfg := configFromLabels(c.Labels)
	target := cfg
	if targetType != "" {
		target.Type = strings.ToUpper(targetType)
	}
	if targetVersion != "" {
		target.Version = targetVersion
	}
	res := CompatResult{Loader: target.Loader(), Items: []CompatItem{}, Unchecked: []string{}}
	switch target.Version {
	case "", "LATEST", "SNAPSHOT":
		if vl, err := m.Versions(ctx); err == nil {
			res.GameVersion = vl.Latest
			if target.Version == "SNAPSHOT" {
				res.GameVersion = vl.Snapshot
			}
		}
	default:
		res.GameVersion = target.Version
	}
	loaders := modLoaders[res.Loader]
	if len(loaders) == 0 || res.GameVersion == "" {
		return res, badRequest("Für diese Plattform/Version ist kein Check möglich")
	}
	refs := splitList(target.effectiveProjects())
	metas := m.ModrinthProjects(ctx, refs)
	res.Items = make([]CompatItem, len(metas))
	lj, _ := json.Marshal(loaders)
	gj, _ := json.Marshal([]string{res.GameVersion})
	sem := make(chan struct{}, 6)
	done := make(chan struct{})
	for i := range metas {
		go func(i int) {
			defer func() { done <- struct{}{} }()
			sem <- struct{}{}
			defer func() { <-sem }()
			p := metas[i]
			it := CompatItem{Ref: p.Ref, Title: p.Title, IconURL: p.IconURL, URL: p.URL, Status: "unknown"}
			var vs []struct {
				VersionNumber string `json:"version_number"`
			}
			err := modrinthGet(ctx, "/project/"+url.PathEscape(refSlug(p.Ref))+"/version", url.Values{"loaders": {string(lj)}, "game_versions": {string(gj)}}, &vs)
			switch {
			case err != nil:
			case len(vs) == 0:
				it.Status = "missing"
			default:
				it.Status, it.Version = "ok", vs[0].VersionNumber
			}
			res.Items[i] = it
		}(i)
	}
	for range metas {
		<-done
	}
	for _, it := range res.Items {
		switch it.Status {
		case "ok":
			res.OK++
		case "missing":
			res.Missing++
		}
	}
	slugs := map[string]bool{}
	for _, p := range metas {
		slugs[strings.ToLower(p.Slug)] = true
	}
	for _, f := range m.cachedModFiles(ctx, c, cfg.ModDir()) {
		lower := strings.ReplaceAll(strings.ToLower(f.Name), "_", "-")
		covered := false
		for s := range slugs {
			if s != "" && strings.Contains(lower, s) {
				covered = true
			}
		}
		if !covered {
			res.Unchecked = append(res.Unchecked, f.Name)
		}
	}
	return res, nil
}
