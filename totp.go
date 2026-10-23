package main

// Time-based one-time passwords (RFC 6238, SHA-1, 6 digits, 30 s) – the
// variant every authenticator app supports.

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"time"
)

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

func newTOTPSecret() string {
	b := make([]byte, 20)
	_, _ = rand.Read(b)
	return b32.EncodeToString(b)
}

func totpCode(secret []byte, step int64) string {
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(step))
	mac := hmac.New(sha1.New, secret)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	v := binary.BigEndian.Uint32(sum[off:off+4]) & 0x7fffffff
	return fmt.Sprintf("%06d", v%1_000_000)
}

// totpVerify checks code against the steps around now. A step must be newer
// than last so that an intercepted code cannot be replayed. Returns the
// matching step.
func totpVerify(secretB32, code string, last int64, now time.Time) (int64, bool) {
	code = strings.ReplaceAll(strings.TrimSpace(code), " ", "")
	if len(code) != 6 {
		return 0, false
	}
	secret, err := b32.DecodeString(strings.ToUpper(secretB32))
	if err != nil {
		return 0, false
	}
	cur := now.Unix() / 30
	for _, s := range []int64{cur, cur - 1, cur + 1} {
		if s <= last {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(totpCode(secret, s)), []byte(code)) == 1 {
			return s, true
		}
	}
	return 0, false
}

func totpURI(issuer, account, secret string) string {
	label := url.PathEscape(issuer + ":" + account)
	return "otpauth://totp/" + label + "?secret=" + secret + "&issuer=" + url.QueryEscape(issuer)
}

// Recovery codes: 10 × "xxxx-xxxx", stored as SHA-256 (they are random, so
// a fast hash is enough).
func newRecoveryCodes() (plain, hashed []string) {
	const alphabet = "abcdefghjkmnpqrstuvwxyz23456789"
	for i := 0; i < 10; i++ {
		b := make([]byte, 8)
		_, _ = rand.Read(b)
		for j := range b {
			b[j] = alphabet[int(b[j])%len(alphabet)]
		}
		c := string(b[:4]) + "-" + string(b[4:])
		plain = append(plain, c)
		hashed = append(hashed, hashRecovery(c))
	}
	return
}

func hashRecovery(code string) string {
	c := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(code), " ", ""))
	if len(c) == 8 {
		c = c[:4] + "-" + c[4:]
	}
	s := sha256.Sum256([]byte("craftdock-recovery:" + c))
	return hex.EncodeToString(s[:])
}
