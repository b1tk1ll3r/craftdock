package main

// Embeddable status widget (/widget, /widget/<name>) and SVG badge
// (/badge/<name>.svg) for other websites, forums and READMEs.

import (
	"fmt"
	"html"
	"io/fs"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

func (a *API) widgetPage(w http.ResponseWriter, r *http.Request) {
	s := a.m.store.Settings()
	if !s.PublicEnabled {
		http.NotFound(w, r)
		return
	}
	if name := r.PathValue("name"); name != "" && !nameRe.MatchString(name) {
		http.NotFound(w, r)
		return
	}
	b, err := fs.ReadFile(a.public, "widget.html")
	if err != nil {
		http.Error(w, "missing widget", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=60")
	_, _ = w.Write(b)
}

// textWidth approximates Verdana 11px glyph widths like shields.io does.
func textWidth(s string) int {
	w := 0.0
	for _, r := range s {
		switch {
		case strings.ContainsRune("iljt.,:;|!'I ", r):
			w += 3.6
		case strings.ContainsRune("mwMW@%", r):
			w += 10
		case r >= 'A' && r <= 'Z':
			w += 7.6
		case r > 127:
			w += 7.5
		default:
			w += 6.6
		}
	}
	return int(w + 0.5)
}

func (a *API) badge(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSuffix(r.PathValue("file"), ".svg")
	s := a.m.store.Settings()
	if !s.PublicEnabled || !nameRe.MatchString(name) {
		http.NotFound(w, r)
		return
	}
	ctx, cancel := ctxT(r, 15*time.Second)
	defer cancel()
	c, err := a.m.find(ctx, name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	cfg := configFromLabels(c.Labels)
	if cfg.PublicHidden {
		http.NotFound(w, r)
		return
	}
	ps := a.m.toPublic(ctx, c, cfg)
	label := ps.DisplayName
	if q := r.URL.Query().Get("label"); q != "" && utf8.RuneCountInString(q) <= 40 {
		label = q
	}
	var value, color string
	switch ps.Status {
	case "online":
		value, color = fmt.Sprintf("%d/%d online", ps.Online, ps.Max), "#2ea043"
	case "sleeping":
		value, color = "bereit", "#3b82f6"
	case "starting":
		value, color = "startet", "#d29922"
	default:
		value, color = "offline", "#6e7681"
	}
	lw, vw := textWidth(label)+16, textWidth(value)+16
	tw := lw + vw
	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%[1]d" height="20" role="img" aria-label="%[2]s: %[3]s">
<title>%[2]s: %[3]s</title>
<linearGradient id="s" x2="0" y2="100%%"><stop offset="0" stop-color="#bbb" stop-opacity=".1"/><stop offset="1" stop-opacity=".1"/></linearGradient>
<clipPath id="r"><rect width="%[1]d" height="20" rx="3" fill="#fff"/></clipPath>
<g clip-path="url(#r)"><rect width="%[4]d" height="20" fill="#3b4250"/><rect x="%[4]d" width="%[5]d" height="20" fill="%[6]s"/><rect width="%[1]d" height="20" fill="url(#s)"/></g>
<g fill="#fff" text-anchor="middle" font-family="Verdana,Geneva,DejaVu Sans,sans-serif" font-size="11">
<text x="%[7]d" y="15" fill="#010101" fill-opacity=".3">%[2]s</text><text x="%[7]d" y="14">%[2]s</text>
<text x="%[8]d" y="15" fill="#010101" fill-opacity=".3">%[3]s</text><text x="%[8]d" y="14">%[3]s</text></g></svg>`,
		tw, html.EscapeString(label), html.EscapeString(value), lw, vw, color, lw/2, lw+vw/2)
	w.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=30")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	_, _ = w.Write([]byte(svg))
}
