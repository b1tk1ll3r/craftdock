package main

// Minimal Docker Engine API client built on net/http only.
// Talks to the daemon over the unix socket (default) or tcp://host:port.

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const dockerAPIVersion = "v1.41" // Docker 20.10+

type Docker struct {
	client *http.Client
	base   string
}

type DockerError struct {
	Status  int
	Message string
}

func (e *DockerError) Error() string {
	return fmt.Sprintf("Docker: %s (HTTP %d)", e.Message, e.Status)
}

func IsNotFound(err error) bool {
	var de *DockerError
	return errors.As(err, &de) && de.Status == http.StatusNotFound
}

func NewDocker(host string) (*Docker, error) {
	if host == "" {
		host = "unix:///var/run/docker.sock"
	}
	tr := &http.Transport{MaxIdleConns: 20, IdleConnTimeout: 90 * time.Second}
	base := "http://docker"
	switch {
	case strings.HasPrefix(host, "unix://"):
		sock := strings.TrimPrefix(host, "unix://")
		tr.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", sock)
		}
	case strings.HasPrefix(host, "tcp://"):
		base = "http://" + strings.TrimPrefix(host, "tcp://")
	default:
		return nil, fmt.Errorf("nicht unterstützter DOCKER_HOST %q (erwartet unix:// oder tcp://)", host)
	}
	return &Docker{client: &http.Client{Transport: tr}, base: base + "/" + dockerAPIVersion}, nil
}

func (d *Docker) request(ctx context.Context, method, path string, query url.Values, body io.Reader, ctype string) (*http.Response, error) {
	u := d.base + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return nil, err
	}
	if ctype != "" {
		req.Header.Set("Content-Type", ctype)
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Docker nicht erreichbar: %w", err)
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		msg := strings.TrimSpace(string(b))
		var m struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(b, &m) == nil && m.Message != "" {
			msg = m.Message
		}
		if msg == "" {
			msg = http.StatusText(resp.StatusCode)
		}
		return nil, &DockerError{Status: resp.StatusCode, Message: msg}
	}
	return resp, nil
}

func (d *Docker) doJSON(ctx context.Context, method, path string, query url.Values, in, out any) error {
	var body io.Reader
	ctype := ""
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
		ctype = "application/json"
	}
	resp, err := d.request(ctx, method, path, query, body, ctype)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

// ---- Types ---------------------------------------------------------------

type ContainerSummary struct {
	ID      string            `json:"Id"`
	Names   []string          `json:"Names"`
	Image   string            `json:"Image"`
	State   string            `json:"State"`
	Status  string            `json:"Status"`
	Labels  map[string]string `json:"Labels"`
	Created int64             `json:"Created"`
}

type ContainerInspect struct {
	ID           string `json:"Id"`
	Name         string `json:"Name"`
	Created      string `json:"Created"`
	RestartCount int    `json:"RestartCount"`
	State        struct {
		Status     string `json:"Status"`
		Running    bool   `json:"Running"`
		StartedAt  string `json:"StartedAt"`
		FinishedAt string `json:"FinishedAt"`
		ExitCode   int    `json:"ExitCode"`
		Health     *struct {
			Status string `json:"Status"`
		} `json:"Health"`
	} `json:"State"`
	Config struct {
		Tty    bool              `json:"Tty"`
		Image  string            `json:"Image"`
		Labels map[string]string `json:"Labels"`
	} `json:"Config"`
	Mounts []struct {
		Type        string `json:"Type"`
		Name        string `json:"Name"`
		Source      string `json:"Source"`
		Destination string `json:"Destination"`
	} `json:"Mounts"`
	NetworkSettings struct {
		Networks map[string]struct {
			IPAddress string `json:"IPAddress"`
		} `json:"Networks"`
	} `json:"NetworkSettings"`
}

type DockerInfo struct {
	ServerVersion     string `json:"ServerVersion"`
	NCPU              int    `json:"NCPU"`
	MemTotal          int64  `json:"MemTotal"`
	OperatingSystem   string `json:"OperatingSystem"`
	Architecture      string `json:"Architecture"`
	ContainersRunning int    `json:"ContainersRunning"`
}

type ContainerStats struct {
	CPUPercent float64 `json:"cpuPercent"`
	MemUsage   uint64  `json:"memUsage"`
	MemLimit   uint64  `json:"memLimit"`
}

// ---- API calls -----------------------------------------------------------

func (d *Docker) Info(ctx context.Context) (DockerInfo, error) {
	var info DockerInfo
	err := d.doJSON(ctx, "GET", "/info", nil, nil, &info)
	return info, err
}

func (d *Docker) ListContainers(ctx context.Context, labels ...string) ([]ContainerSummary, error) {
	f, _ := json.Marshal(map[string][]string{"label": labels})
	q := url.Values{"all": {"1"}, "filters": {string(f)}}
	var out []ContainerSummary
	err := d.doJSON(ctx, "GET", "/containers/json", q, nil, &out)
	return out, err
}

func (d *Docker) Inspect(ctx context.Context, id string) (ContainerInspect, error) {
	var out ContainerInspect
	err := d.doJSON(ctx, "GET", "/containers/"+url.PathEscape(id)+"/json", nil, nil, &out)
	return out, err
}

func (d *Docker) ImageExists(ctx context.Context, ref string) bool {
	return d.doJSON(ctx, "GET", "/images/"+ref+"/json", nil, nil, nil) == nil
}

// PullImage pulls ref and reports human readable progress.
func (d *Docker) PullImage(ctx context.Context, ref string, progress func(string)) error {
	repo, tag := ref, "latest"
	if i := strings.LastIndex(ref, ":"); i > strings.LastIndex(ref, "/") {
		repo, tag = ref[:i], ref[i+1:]
	}
	resp, err := d.request(ctx, "POST", "/images/create", url.Values{"fromImage": {repo}, "tag": {tag}}, nil, "")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	type layer struct{ cur, total int64 }
	layers := map[string]layer{}
	dec := json.NewDecoder(resp.Body)
	last := time.Time{}
	for {
		var msg struct {
			Status         string `json:"status"`
			ID             string `json:"id"`
			Error          string `json:"error"`
			ProgressDetail struct {
				Current int64 `json:"current"`
				Total   int64 `json:"total"`
			} `json:"progressDetail"`
		}
		if err := dec.Decode(&msg); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if msg.Error != "" {
			return errors.New(msg.Error)
		}
		if msg.Status == "Downloading" && msg.ProgressDetail.Total > 0 {
			layers[msg.ID] = layer{msg.ProgressDetail.Current, msg.ProgressDetail.Total}
		} else if msg.Status == "Download complete" || msg.Status == "Pull complete" {
			if l, ok := layers[msg.ID]; ok {
				layers[msg.ID] = layer{l.total, l.total}
			}
		}
		if progress != nil && time.Since(last) > 400*time.Millisecond {
			last = time.Now()
			var cur, total int64
			for _, l := range layers {
				cur += l.cur
				total += l.total
			}
			if total > 0 {
				progress(fmt.Sprintf("Image wird geladen … %d %%", cur*100/total))
			} else {
				progress("Image wird geladen …")
			}
		}
	}
}

func (d *Docker) CreateContainer(ctx context.Context, name string, spec any) (string, error) {
	var out struct {
		ID string `json:"Id"`
	}
	q := url.Values{}
	if name != "" {
		q.Set("name", name)
	}
	err := d.doJSON(ctx, "POST", "/containers/create", q, spec, &out)
	return out.ID, err
}

func (d *Docker) Start(ctx context.Context, id string) error {
	return d.doJSON(ctx, "POST", "/containers/"+url.PathEscape(id)+"/start", nil, nil, nil)
}

func (d *Docker) Stop(ctx context.Context, id string, timeout int) error {
	return d.doJSON(ctx, "POST", "/containers/"+url.PathEscape(id)+"/stop", url.Values{"t": {strconv.Itoa(timeout)}}, nil, nil)
}

func (d *Docker) Restart(ctx context.Context, id string, timeout int) error {
	return d.doJSON(ctx, "POST", "/containers/"+url.PathEscape(id)+"/restart", url.Values{"t": {strconv.Itoa(timeout)}}, nil, nil)
}

func (d *Docker) Remove(ctx context.Context, id string) error {
	return d.doJSON(ctx, "DELETE", "/containers/"+url.PathEscape(id), url.Values{"force": {"1"}}, nil, nil)
}

func (d *Docker) RemoveVolume(ctx context.Context, name string) error {
	return d.doJSON(ctx, "DELETE", "/volumes/"+url.PathEscape(name), nil, nil, nil)
}

// Logs returns the (optionally following) log stream as plain bytes.
func (d *Docker) Logs(ctx context.Context, id string, tail int, follow, tty bool) (io.ReadCloser, error) {
	q := url.Values{"stdout": {"1"}, "stderr": {"1"}, "tail": {strconv.Itoa(tail)}}
	if follow {
		q.Set("follow", "1")
	}
	resp, err := d.request(ctx, "GET", "/containers/"+url.PathEscape(id)+"/logs", q, nil, "")
	if err != nil {
		return nil, err
	}
	if tty {
		return resp.Body, nil
	}
	return demux(resp.Body), nil
}

// Exec runs cmd inside the container and returns combined output and exit code.
func (d *Docker) Exec(ctx context.Context, id string, cmd []string) (string, int, error) {
	var created struct {
		ID string `json:"Id"`
	}
	err := d.doJSON(ctx, "POST", "/containers/"+url.PathEscape(id)+"/exec", nil, map[string]any{
		"Cmd": cmd, "AttachStdout": true, "AttachStderr": true, "Tty": false,
	}, &created)
	if err != nil {
		return "", -1, err
	}
	b, _ := json.Marshal(map[string]any{"Detach": false, "Tty": false})
	resp, err := d.request(ctx, "POST", "/exec/"+created.ID+"/start", nil, bytes.NewReader(b), "application/json")
	if err != nil {
		return "", -1, err
	}
	r := demux(resp.Body)
	out, err := io.ReadAll(io.LimitReader(r, 1<<20))
	r.Close()
	if err != nil {
		return string(out), -1, err
	}
	var info struct {
		ExitCode int `json:"ExitCode"`
	}
	if err := d.doJSON(ctx, "GET", "/exec/"+created.ID+"/json", nil, nil, &info); err != nil {
		return string(out), -1, err
	}
	return string(out), info.ExitCode, nil
}

func (d *Docker) Stats(ctx context.Context, id string) (ContainerStats, error) {
	type cpu struct {
		CPUUsage struct {
			TotalUsage uint64 `json:"total_usage"`
		} `json:"cpu_usage"`
		SystemUsage uint64 `json:"system_cpu_usage"`
		OnlineCPUs  int    `json:"online_cpus"`
	}
	var s struct {
		CPU    cpu `json:"cpu_stats"`
		PreCPU cpu `json:"precpu_stats"`
		Memory struct {
			Usage uint64            `json:"usage"`
			Limit uint64            `json:"limit"`
			Stats map[string]uint64 `json:"stats"`
		} `json:"memory_stats"`
	}
	if err := d.doJSON(ctx, "GET", "/containers/"+url.PathEscape(id)+"/stats", url.Values{"stream": {"0"}}, nil, &s); err != nil {
		return ContainerStats{}, err
	}
	var out ContainerStats
	cpuDelta := float64(s.CPU.CPUUsage.TotalUsage) - float64(s.PreCPU.CPUUsage.TotalUsage)
	sysDelta := float64(s.CPU.SystemUsage) - float64(s.PreCPU.SystemUsage)
	cpus := s.CPU.OnlineCPUs
	if cpus == 0 {
		cpus = 1
	}
	if cpuDelta > 0 && sysDelta > 0 {
		out.CPUPercent = cpuDelta / sysDelta * float64(cpus) * 100
	}
	mem := s.Memory.Usage
	// Subtract page cache like `docker stats` does (cgroup v2 / v1).
	if v, ok := s.Memory.Stats["inactive_file"]; ok && v < mem {
		mem -= v
	} else if v, ok := s.Memory.Stats["total_inactive_file"]; ok && v < mem {
		mem -= v
	}
	out.MemUsage = mem
	out.MemLimit = s.Memory.Limit
	return out, nil
}

// StatPath reports whether a path exists inside the container.
func (d *Docker) StatPath(ctx context.Context, id, path string) (bool, error) {
	resp, err := d.request(ctx, "HEAD", "/containers/"+url.PathEscape(id)+"/archive", url.Values{"path": {path}}, nil, "")
	if err != nil {
		if IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	resp.Body.Close()
	return true, nil
}

// Archive streams a tar of path from the container (works on stopped containers too).
func (d *Docker) Archive(ctx context.Context, id, path string) (io.ReadCloser, error) {
	resp, err := d.request(ctx, "GET", "/containers/"+url.PathEscape(id)+"/archive", url.Values{"path": {path}}, nil, "")
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func (d *Docker) ReadFile(ctx context.Context, id, path string, max int64) ([]byte, error) {
	rc, err := d.Archive(ctx, id, path)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	tr := tar.NewReader(rc)
	hdr, err := tr.Next()
	if err != nil {
		return nil, err
	}
	if hdr.Typeflag != tar.TypeReg {
		return nil, fmt.Errorf("%s ist keine reguläre Datei", path)
	}
	if hdr.Size > max {
		return nil, fmt.Errorf("%s ist zu groß (%d Bytes)", path, hdr.Size)
	}
	return io.ReadAll(tr)
}

func (d *Docker) WriteFile(ctx context.Context, id, dir, name string, data []byte, uid, gid int) error {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := tw.WriteHeader(&tar.Header{
		Name: name, Mode: 0o644, Size: int64(len(data)), Uid: uid, Gid: gid,
		ModTime: time.Now(), Typeflag: tar.TypeReg,
	}); err != nil {
		return err
	}
	if _, err := tw.Write(data); err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return err
	}
	resp, err := d.request(ctx, "PUT", "/containers/"+url.PathEscape(id)+"/archive", url.Values{"path": {dir}}, &buf, "application/x-tar")
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// PutFile streams r (size bytes) to baseDir/relPath inside the container,
// creating missing parent directories. Works on stopped containers.
func (d *Docker) PutFile(ctx context.Context, id, baseDir, relPath string, r io.Reader, size int64, uid, gid int) error {
	pr, pw := io.Pipe()
	go func() {
		tw := tar.NewWriter(pw)
		parts := strings.Split(relPath, "/")
		for i := 1; i < len(parts); i++ {
			if err := tw.WriteHeader(&tar.Header{Name: strings.Join(parts[:i], "/") + "/", Mode: 0o755, Uid: uid, Gid: gid,
				ModTime: time.Now(), Typeflag: tar.TypeDir}); err != nil {
				pw.CloseWithError(err)
				return
			}
		}
		err := tw.WriteHeader(&tar.Header{Name: relPath, Mode: 0o644, Size: size, Uid: uid, Gid: gid, ModTime: time.Now(), Typeflag: tar.TypeReg})
		if err == nil {
			_, err = io.CopyN(tw, r, size)
		}
		if err == nil {
			err = tw.Close()
		}
		pw.CloseWithError(err)
	}()
	resp, err := d.request(ctx, "PUT", "/containers/"+url.PathEscape(id)+"/archive", url.Values{"path": {baseDir}}, pr, "application/x-tar")
	pr.Close()
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// RunHelper runs a short-lived container from image with the given binds
// and returns its output. Used to touch volume contents while a server is stopped.
func (d *Docker) RunHelper(ctx context.Context, image string, binds []string, cmd []string) (string, error) {
	id, err := d.CreateContainer(ctx, "", map[string]any{
		"Image":      image,
		"Entrypoint": cmd[:1],
		"Cmd":        cmd[1:],
		"Labels":     map[string]string{"craftdock.helper": "true"},
		"HostConfig": map[string]any{"Binds": binds, "NetworkMode": "none"},
	})
	if err != nil {
		return "", err
	}
	defer func() {
		rctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		_ = d.Remove(rctx, id)
	}()
	if err := d.Start(ctx, id); err != nil {
		return "", err
	}
	var wait struct {
		StatusCode int `json:"StatusCode"`
	}
	if err := d.doJSON(ctx, "POST", "/containers/"+id+"/wait", nil, nil, &wait); err != nil {
		return "", err
	}
	rc, err := d.Logs(ctx, id, 100000, false, false)
	if err != nil {
		return "", err
	}
	defer rc.Close()
	out, _ := io.ReadAll(io.LimitReader(rc, 4<<20))
	if wait.StatusCode != 0 {
		return string(out), fmt.Errorf("Hilfscontainer beendet mit Code %d: %s", wait.StatusCode, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

// ---- stream demultiplexing -------------------------------------------------

type demuxReader struct {
	*io.PipeReader
	src io.Closer
}

func (r demuxReader) Close() error {
	r.PipeReader.Close()
	return r.src.Close()
}

// demux strips Docker's 8-byte stream headers from non-TTY attach/log streams.
func demux(rc io.ReadCloser) io.ReadCloser {
	pr, pw := io.Pipe()
	go func() {
		hdr := make([]byte, 8)
		for {
			if _, err := io.ReadFull(rc, hdr); err != nil {
				if err == io.ErrUnexpectedEOF {
					err = io.EOF
				}
				pw.CloseWithError(err)
				return
			}
			n := binary.BigEndian.Uint32(hdr[4:])
			if _, err := io.CopyN(pw, rc, int64(n)); err != nil {
				pw.CloseWithError(err)
				return
			}
		}
	}()
	return demuxReader{pr, rc}
}
