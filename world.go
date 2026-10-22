package main

// Cloning servers, world reset and world import.

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

func dataBind(cfg ServerConfig) string {
	if cfg.DataPath != "" {
		return cfg.DataPath
	}
	return volumeName(cfg.Name)
}

// ---- clone ---------------------------------------------------------------------------

type CloneInput struct {
	Name     string `json:"name"`
	Port     int    `json:"port"`
	CopyData bool   `json:"copyData"`
	Start    bool   `json:"start"`
}

func (m *Manager) Clone(ctx context.Context, src string, in CloneInput) error {
	c, err := m.find(ctx, src)
	if err != nil {
		return err
	}
	old := configFromLabels(c.Labels)
	cfg := old
	cfg.Name = in.Name
	cfg.Port = in.Port
	cfg.DataPath = ""
	cfg.PublicHidden = true // a clone is usually a staging copy
	cfg.PublicAddress = ""
	cfg.ProxyMember, cfg.Network = false, ""
	cfg.MapPort = 0
	cfg.Ports = nil // extra host ports would collide with the original
	base := old.DisplayName
	if base == "" {
		base = old.Name
	}
	cfg.DisplayName = base + " (Kopie)"
	if len(cfg.DisplayName) > 48 {
		cfg.DisplayName = cfg.DisplayName[:48]
	}
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return err
	}
	if _, err := m.find(ctx, cfg.Name); err == nil {
		return conflict("Ein Server namens „%s“ existiert bereits", cfg.Name)
	}
	if err := m.checkPorts(ctx, cfg); err != nil {
		return err
	}
	cfg.RconPassword = randomHex(16)
	cfg.CreatedAt = time.Now().UTC()
	pending := cfg
	srcID, srcRunning := c.ID, c.State == "running"
	return m.startTask(cfg.Name, "clone", "Wird vorbereitet …", &pending, func(ctx context.Context, p func(string)) error {
		if err := m.ensureImage(ctx, cfg.Image(), p); err != nil {
			return err
		}
		if in.CopyData {
			p("Weltdaten werden kopiert …")
			if srcRunning {
				if _, err := m.rcon(ctx, srcID, "save-off"); err == nil {
					_, _ = m.rcon(ctx, srcID, "save-all flush")
					defer func() {
						sctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
						defer cancel()
						_, _ = m.rcon(sctx, srcID, "save-on")
					}()
				}
			}
			if _, err := m.d.RunHelper(ctx, cfg.Image(), []string{dataBind(old) + ":/src:ro", dataBind(cfg) + ":/dst"},
				[]string{"sh", "-c", "cp -a /src/. /dst/ && rm -f /dst/session.lock /dst/*/session.lock"}); err != nil {
				return fmt.Errorf("Kopieren fehlgeschlagen: %w", err)
			}
		}
		p("Container wird erstellt …")
		id, err := m.d.CreateContainer(ctx, containerName(cfg.Name), m.spec(cfg))
		if err != nil {
			return err
		}
		if in.Start {
			return m.startContainer(ctx, id, false, cfg, p)
		}
		return nil
	})
}

func (a *API) cloneServer(w http.ResponseWriter, r *http.Request) {
	if len(userFrom(r).Servers) > 0 {
		writeErr(w, &APIError{http.StatusForbidden, "Benutzer mit Server-Beschränkung können keine Server anlegen"})
		return
	}
	var in CloneInput
	if err := readJSON(r, &in); err != nil {
		writeErr(w, err)
		return
	}
	ctx, cancel := ctxT(r, 15*time.Second)
	defer cancel()
	if err := a.m.Clone(ctx, r.PathValue("name"), in); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"name": in.Name})
}

// ---- world reset --------------------------------------------------------------------

const wipeWorldScript = `L="$1"; rm -rf "/data/$L" "/data/${L}_nether" "/data/${L}_the_end" /data/bluemap/web/maps; true`

type ResetInput struct {
	Seed      string  `json:"seed"`
	LevelType *string `json:"levelType"`
}

func (m *Manager) ResetWorld(ctx context.Context, name string, in ResetInput) error {
	c, err := m.find(ctx, name)
	if err != nil {
		return err
	}
	old := configFromLabels(c.Labels)
	cfg := old
	cfg.Seed = strings.TrimSpace(in.Seed)
	if in.LevelType != nil {
		cfg.LevelType = *in.LevelType
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	id, running := c.ID, c.State == "running"
	return m.startTask(name, "reset", "Welt wird zurückgesetzt …", nil, func(ctx context.Context, p func(string)) error {
		if running {
			p("Server wird gestoppt …")
			if err := m.d.Stop(ctx, id, stopTimeout); err != nil {
				return err
			}
		}
		if cfg.Seed != old.Seed || cfg.LevelType != old.LevelType {
			newID, err := m.replaceContainer(ctx, id, old, cfg, false, p)
			if err != nil {
				return err
			}
			id = newID
		}
		p("Alte Welt wird gelöscht …")
		if _, err := m.runInServer(ctx, &ContainerSummary{ID: id, Image: cfg.Image(), State: "exited", Labels: map[string]string{labelConfig: mustJSON(cfg)}}, wipeWorldScript, cfg.levelName()); err != nil {
			return err
		}
		if running {
			return m.startContainer(ctx, id, false, cfg, p)
		}
		return nil
	})
}

func (a *API) worldReset(w http.ResponseWriter, r *http.Request) {
	var in ResetInput
	if err := readJSON(r, &in); err != nil {
		writeErr(w, err)
		return
	}
	ctx, cancel := ctxT(r, 15*time.Second)
	defer cancel()
	if err := a.m.ResetWorld(ctx, r.PathValue("name"), in); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]bool{"ok": true})
}

// ---- world import -------------------------------------------------------------------

type archiveEntry struct {
	name string
	dir  bool
	size int64
	open func() (io.ReadCloser, error)
}

// archiveWalker iterates over a zip or (gzipped) tar file. It may be called
// several times; each call re-reads the file.
type archiveWalker func(fn func(e archiveEntry) error) error

func openArchive(file string) (archiveWalker, func(), error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, nil, err
	}
	head := make([]byte, 4)
	_, _ = io.ReadFull(f, head)
	if string(head[:2]) == "PK" {
		st, _ := f.Stat()
		zr, err := zip.NewReader(f, st.Size())
		if err != nil {
			f.Close()
			return nil, nil, badRequest("ZIP-Datei ist beschädigt: %v", err)
		}
		return func(fn func(archiveEntry) error) error {
			for _, zf := range zr.File {
				zf := zf
				if zf.Mode()&os.ModeSymlink != 0 {
					continue
				}
				if err := fn(archiveEntry{name: zf.Name, dir: zf.FileInfo().IsDir(), size: int64(zf.UncompressedSize64), open: zf.Open}); err != nil {
					return err
				}
			}
			return nil
		}, func() { f.Close() }, nil
	}
	gz := head[0] == 0x1f && head[1] == 0x8b
	return func(fn func(archiveEntry) error) error {
		if _, err := f.Seek(0, 0); err != nil {
			return err
		}
		var r io.Reader = f
		if gz {
			g, err := gzip.NewReader(f)
			if err != nil {
				return badRequest("Archiv ist beschädigt: %v", err)
			}
			defer g.Close()
			r = g
		}
		tr := tar.NewReader(r)
		for {
			h, err := tr.Next()
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return badRequest("Archiv ist beschädigt oder kein .zip/.tar.gz: %v", err)
			}
			if h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeDir {
				continue
			}
			if err := fn(archiveEntry{name: h.Name, dir: h.Typeflag == tar.TypeDir, size: h.Size,
				open: func() (io.ReadCloser, error) { return io.NopCloser(tr), nil }}); err != nil {
				return err
			}
		}
	}, func() { f.Close() }, nil
}

func cleanArchivePath(n string) (string, bool) {
	n = strings.ReplaceAll(n, "\\", "/")
	n = path.Clean("/" + n)[1:]
	if n == "" || n == "." || strings.HasPrefix(n, "..") {
		return "", false
	}
	return n, true
}

// worldMapping finds the world folders in the archive (by level.dat) and maps
// them onto <level>, <level>_nether and <level>_the_end.
func worldMapping(walk archiveWalker, level string) (map[string]string, error) {
	var roots []string
	err := walk(func(e archiveEntry) error {
		n, ok := cleanArchivePath(e.name)
		if ok && !e.dir && path.Base(n) == "level.dat" {
			d := path.Dir(n)
			if d == "." {
				d = ""
			}
			roots = append(roots, d)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(roots) == 0 {
		return nil, badRequest("Keine level.dat gefunden – enthält das Archiv eine Minecraft-Welt?")
	}
	depth := func(r string) int {
		if r == "" {
			return -1
		}
		return strings.Count(r, "/")
	}
	main, found := "", false
	for _, r := range roots {
		if strings.HasSuffix(r, "_nether") || strings.HasSuffix(r, "_the_end") {
			continue
		}
		if !found || depth(r) < depth(main) || (depth(r) == depth(main) && len(r) < len(main)) {
			main, found = r, true
		}
	}
	if !found {
		main = roots[0]
	}
	mapping := map[string]string{main: level}
	for _, r := range roots {
		switch r {
		case main + "_nether":
			mapping[r] = level + "_nether"
		case main + "_the_end":
			mapping[r] = level + "_the_end"
		}
	}
	return mapping, nil
}

func mapEntry(n string, mapping map[string]string) (string, bool) {
	best := ""
	found := false
	for src := range mapping {
		if src == "" || n == src || strings.HasPrefix(n, src+"/") {
			if !found || len(src) > len(best) {
				best, found = src, true
			}
		}
	}
	if !found {
		return "", false
	}
	rel := strings.TrimPrefix(strings.TrimPrefix(n, best), "/")
	if best == "" && strings.Contains(rel, "/") {
		// Archive with level.dat at its root: skip foreign top-level folders
		// that look like other worlds.
		top := strings.SplitN(rel, "/", 2)[0]
		if _, isRoot := mapping[top]; isRoot {
			return "", false
		}
	}
	if rel == "" {
		return mapping[best], true
	}
	return mapping[best] + "/" + rel, true
}

// writeWorldTar streams the mapped world entries as a tar to w.
func writeWorldTar(walk archiveWalker, mapping map[string]string, w io.Writer) error {
	tw := tar.NewWriter(w)
	dirs := map[string]bool{}
	mkdirs := func(p string) error {
		parts := strings.Split(p, "/")
		for i := 1; i <= len(parts); i++ {
			d := strings.Join(parts[:i], "/")
			if dirs[d] {
				continue
			}
			dirs[d] = true
			if err := tw.WriteHeader(&tar.Header{Name: d + "/", Typeflag: tar.TypeDir, Mode: 0o755, Uid: 1000, Gid: 1000, ModTime: time.Now()}); err != nil {
				return err
			}
		}
		return nil
	}
	err := walk(func(e archiveEntry) error {
		n, ok := cleanArchivePath(e.name)
		if !ok || e.dir || path.Base(n) == "session.lock" {
			return nil
		}
		dst, ok := mapEntry(n, mapping)
		if !ok {
			return nil
		}
		if d := path.Dir(dst); d != "." {
			if err := mkdirs(d); err != nil {
				return err
			}
		}
		rc, err := e.open()
		if err != nil {
			return err
		}
		defer rc.Close()
		if err := tw.WriteHeader(&tar.Header{Name: dst, Typeflag: tar.TypeReg, Mode: 0o644, Size: e.size, Uid: 1000, Gid: 1000, ModTime: time.Now()}); err != nil {
			return err
		}
		_, err = io.CopyN(tw, rc, e.size)
		return err
	})
	if err != nil {
		return err
	}
	return tw.Close()
}

// ImportWorld replaces the server's world with the archive in file (which the
// task deletes when done).
func (m *Manager) ImportWorld(ctx context.Context, name, file string) error {
	c, err := m.find(ctx, name)
	if err != nil {
		os.Remove(file)
		return err
	}
	cfg := configFromLabels(c.Labels)
	walk, closeFn, err := openArchive(file)
	if err != nil {
		os.Remove(file)
		return err
	}
	mapping, err := worldMapping(walk, cfg.levelName())
	if err != nil {
		closeFn()
		os.Remove(file)
		return err
	}
	id, running := c.ID, c.State == "running"
	err = m.startTask(name, "import", "Welt wird importiert …", nil, func(ctx context.Context, p func(string)) error {
		defer os.Remove(file)
		defer closeFn()
		if running {
			p("Server wird gestoppt …")
			if err := m.d.Stop(ctx, id, stopTimeout); err != nil {
				return err
			}
		}
		p("Alte Welt wird gelöscht …")
		if _, err := m.runInServer(ctx, &ContainerSummary{ID: id, Image: c.Image, State: "exited", Labels: c.Labels}, wipeWorldScript, cfg.levelName()); err != nil {
			return err
		}
		p("Neue Welt wird hochgeladen …")
		pr, pw := io.Pipe()
		go func() { pw.CloseWithError(writeWorldTar(walk, mapping, pw)) }()
		resp, err := m.d.request(ctx, "PUT", "/containers/"+id+"/archive", map[string][]string{"path": {"/data"}}, pr, "application/x-tar")
		pr.Close()
		if err != nil {
			return err
		}
		resp.Body.Close()
		log.Printf("[%s] Welt importiert (%d Ordner)", name, len(mapping))
		if running {
			return m.startContainer(ctx, id, false, cfg, p)
		}
		return nil
	})
	if err != nil {
		closeFn()
		os.Remove(file)
	}
	return err
}

const maxWorldUpload = 8 << 30

func (a *API) worldImport(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxWorldUpload+1<<20)
	mr, err := r.MultipartReader()
	if err != nil {
		writeErr(w, badRequest("Erwarte multipart/form-data"))
		return
	}
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			writeErr(w, badRequest("Keine Datei übermittelt"))
			return
		}
		if err != nil {
			writeErr(w, badRequest("Upload fehlgeschlagen: %v", err))
			return
		}
		if part.FormName() != "file" {
			continue
		}
		tmp, err := os.CreateTemp("", "craftdock-world-*")
		if err != nil {
			writeErr(w, err)
			return
		}
		n, err := io.Copy(tmp, part)
		tmp.Close()
		if err != nil {
			os.Remove(tmp.Name())
			writeErr(w, badRequest("Upload abgebrochen: %v", err))
			return
		}
		log.Printf("[%s] Welt-Archiv empfangen (%s)", r.PathValue("name"), strconv.FormatInt(n, 10)+" Bytes")
		ctx, cancel := ctxT(r, 2*time.Minute)
		defer cancel()
		if err := a.m.ImportWorld(ctx, r.PathValue("name"), tmp.Name()); err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]bool{"ok": true})
		return
	}
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
