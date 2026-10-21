package main

// Minecraft Server List Ping (Java Edition, 1.7+): the same request the
// multiplayer menu sends. Returns real version, MOTD, favicon and players.

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

type SLPStatus struct {
	Version   string
	Protocol  int
	MOTD      string // flattened with §-codes (hex colours as §#rrggbb)
	Favicon   string // data:image/png;base64,…
	Online    int
	Max       int
	Sample    []string
	LatencyMs int
}

func writeVarInt(w *bytes.Buffer, v int32) {
	u := uint32(v)
	for {
		if u&^0x7F == 0 {
			w.WriteByte(byte(u))
			return
		}
		w.WriteByte(byte(u&0x7F | 0x80))
		u >>= 7
	}
}

func readVarInt(r io.ByteReader) (int32, error) {
	var out uint32
	for i := 0; i < 5; i++ {
		b, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		out |= uint32(b&0x7F) << (7 * i)
		if b&0x80 == 0 {
			return int32(out), nil
		}
	}
	return 0, errors.New("VarInt zu lang")
}

func packet(id int32, payload []byte) []byte {
	var body bytes.Buffer
	writeVarInt(&body, id)
	body.Write(payload)
	var out bytes.Buffer
	writeVarInt(&out, int32(body.Len()))
	out.Write(body.Bytes())
	return out.Bytes()
}

// slpPing queries host:port. Timeout comes from ctx.
func slpPing(ctx context.Context, addr string) (*SLPStatus, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	}
	host, portStr, _ := net.SplitHostPort(addr)
	port, _ := strconv.Atoi(portStr)

	var hs bytes.Buffer
	writeVarInt(&hs, -1) // protocol version: "any"
	writeVarInt(&hs, int32(len(host)))
	hs.WriteString(host)
	_ = binary.Write(&hs, binary.BigEndian, uint16(port))
	writeVarInt(&hs, 1) // next state: status
	if _, err := conn.Write(append(packet(0, hs.Bytes()), packet(0, nil)...)); err != nil {
		return nil, err
	}

	br := bufio.NewReader(conn)
	if _, err := readVarInt(br); err != nil { // packet length
		return nil, err
	}
	if id, err := readVarInt(br); err != nil || id != 0 {
		return nil, fmt.Errorf("unerwartete Antwort (%d, %v)", id, err)
	}
	n, err := readVarInt(br)
	if err != nil || n < 0 || n > 1<<21 {
		return nil, fmt.Errorf("ungültige Statuslänge")
	}
	raw := make([]byte, n)
	if _, err := io.ReadFull(br, raw); err != nil {
		return nil, err
	}

	// Latency via ping/pong.
	start := time.Now()
	var pl bytes.Buffer
	_ = binary.Write(&pl, binary.BigEndian, start.UnixMilli())
	latency := -1
	if _, err := conn.Write(packet(1, pl.Bytes())); err == nil {
		if _, err := readVarInt(br); err == nil {
			if id, err := readVarInt(br); err == nil && id == 1 {
				latency = int(time.Since(start).Milliseconds())
			}
		}
	}

	var st struct {
		Version struct {
			Name     string `json:"name"`
			Protocol int    `json:"protocol"`
		} `json:"version"`
		Players struct {
			Max    int `json:"max"`
			Online int `json:"online"`
			Sample []struct {
				Name string `json:"name"`
			} `json:"sample"`
		} `json:"players"`
		Description json.RawMessage `json:"description"`
		Favicon     string          `json:"favicon"`
	}
	if err := json.Unmarshal(raw, &st); err != nil {
		return nil, err
	}
	out := &SLPStatus{
		Version: st.Version.Name, Protocol: st.Version.Protocol, Online: st.Players.Online,
		Max: st.Players.Max, LatencyMs: latency,
	}
	if strings.HasPrefix(st.Favicon, "data:image/png;base64,") {
		out.Favicon = st.Favicon
	}
	for _, s := range st.Players.Sample {
		// Servers sometimes put decorative lines into the sample.
		if s.Name != "" && !strings.Contains(s.Name, "§") && playerRe.MatchString(s.Name) {
			out.Sample = append(out.Sample, s.Name)
		}
	}
	var desc any
	_ = json.Unmarshal(st.Description, &desc)
	out.MOTD = strings.TrimSpace(flattenChat(desc))
	return out, nil
}

var chatColors = map[string]string{
	"black": "0", "dark_blue": "1", "dark_green": "2", "dark_aqua": "3", "dark_red": "4",
	"dark_purple": "5", "gold": "6", "gray": "7", "dark_gray": "8", "blue": "9", "green": "a",
	"aqua": "b", "red": "c", "light_purple": "d", "yellow": "e", "white": "f",
}

// flattenChat converts a chat component (string, object or array) into §-formatted text.
func flattenChat(v any) string {
	var sb strings.Builder
	var walk func(v any)
	walk = func(v any) {
		switch t := v.(type) {
		case string:
			sb.WriteString(t)
		case []any:
			for _, e := range t {
				walk(e)
			}
		case map[string]any:
			if c, ok := t["color"].(string); ok {
				if code, ok := chatColors[c]; ok {
					sb.WriteString("§" + code)
				} else if len(c) == 7 && c[0] == '#' {
					sb.WriteString("§" + c)
				}
			}
			for key, code := range map[string]string{"bold": "l", "italic": "o", "underlined": "n", "strikethrough": "m"} {
				if b, ok := t[key].(bool); ok && b {
					sb.WriteString("§" + code)
				}
			}
			if s, ok := t["text"].(string); ok {
				sb.WriteString(s)
			}
			if ex, ok := t["extra"]; ok {
				walk(ex)
			}
			if _, styled := t["color"]; styled {
				sb.WriteString("§r")
			}
		}
	}
	walk(v)
	return sb.String()
}
