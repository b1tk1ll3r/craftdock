package main

// Client address handling behind reverse proxies. X-Forwarded-For and
// X-Forwarded-Proto are only honoured when the direct peer is listed in
// CRAFTDOCK_TRUSTED_PROXIES – otherwise anybody could fake their address
// (and dodge the login throttling) by sending the header themselves.

import (
	"log"
	"net"
	"net/http"
	"strings"
)

var trustedNets []*net.IPNet

func initTrustedProxies(spec string) {
	trustedNets = nil
	for _, p := range strings.FieldsFunc(spec, func(r rune) bool { return r == ',' || r == ' ' || r == ';' }) {
		switch p {
		case "private":
			for _, c := range []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "fc00::/7"} {
				_, n, _ := net.ParseCIDR(c)
				trustedNets = append(trustedNets, n)
			}
			continue
		case "loopback":
			p = "127.0.0.0/8"
			_, n, _ := net.ParseCIDR("::1/128")
			trustedNets = append(trustedNets, n)
		}
		if !strings.Contains(p, "/") {
			if ip := net.ParseIP(p); ip != nil {
				if ip.To4() != nil {
					p += "/32"
				} else {
					p += "/128"
				}
			}
		}
		_, n, err := net.ParseCIDR(p)
		if err != nil {
			log.Printf("WARNUNG: CRAFTDOCK_TRUSTED_PROXIES: %q ignoriert (%v)", p, err)
			continue
		}
		trustedNets = append(trustedNets, n)
	}
	if len(trustedNets) > 0 {
		log.Printf("Vertrauenswürdige Reverse-Proxys: %s", spec)
	}
}

func isTrustedProxy(ip string) bool {
	p := net.ParseIP(ip)
	if p == nil {
		return false
	}
	for _, n := range trustedNets {
		if n.Contains(p) {
			return true
		}
	}
	return false
}

func remoteIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// clientIP walks X-Forwarded-For from the right and returns the first address
// that is not a trusted proxy.
func clientIP(r *http.Request) string {
	ip := remoteIP(r)
	if !isTrustedProxy(ip) {
		return ip
	}
	var hops []string
	for _, h := range r.Header.Values("X-Forwarded-For") {
		hops = append(hops, strings.Split(h, ",")...)
	}
	for i := len(hops) - 1; i >= 0; i-- {
		c := strings.TrimSpace(hops[i])
		if net.ParseIP(c) == nil {
			break
		}
		ip = c
		if !isTrustedProxy(c) {
			break
		}
	}
	return ip
}

func isHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return isTrustedProxy(remoteIP(r)) && strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}
