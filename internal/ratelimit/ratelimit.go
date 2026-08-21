package ratelimit

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const keyPrefix = "rate:analyze"

// checkAndIncrement rejects atomically when the counter is at the limit,
// otherwise increments and starts the window on first hit.
var checkAndIncrement = redis.NewScript(`
local count = redis.call("GET", KEYS[1])
if count and tonumber(count) >= tonumber(ARGV[1]) then
    return {0, tonumber(count), redis.call("TTL", KEYS[1])}
end

count = redis.call("INCR", KEYS[1])
if count == 1 then
    redis.call("EXPIRE", KEYS[1], ARGV[2])
end

return {1, count, redis.call("TTL", KEYS[1])}
`)

type Config struct {
	Enabled          bool
	MaxRequests      int
	WindowSeconds    int
	DetailMultiplier int
	CookieName       string
	CookieMaxAge     int
	CookieSecure     bool
	CookieSameSite   http.SameSite
	Salt             string
}

// Error is a rate-limit rejection or storage failure, carrying the API error
// contract: status, code, message, and Retry-After seconds when known.
type Error struct {
	Status     int
	Code       string
	Message    string
	RetryAfter int
}

func (e *Error) Error() string { return e.Message }

type Limiter struct {
	client *redis.Client
	cfg    Config
}

func New(client *redis.Client, cfg Config) *Limiter {
	return &Limiter{client: client, cfg: cfg}
}

// Enforce counts this request against both the caller's hashed IP and its
// client cookie (issued here on first contact). Returns *Error on rejection
// or Redis failure; nil means the request may proceed.
func (l *Limiter) Enforce(w http.ResponseWriter, r *http.Request, endpointName string) error {
	if !l.cfg.Enabled {
		return nil
	}

	maxRequests := l.cfg.MaxRequests
	if strings.HasSuffix(endpointName, "-detail") {
		maxRequests *= l.cfg.DetailMultiplier
	}

	ipHash := hashIP(clientIP(r), l.cfg.Salt)
	if err := l.increment(r, fmt.Sprintf("%s:%s:ip:%s", keyPrefix, endpointName, ipHash), maxRequests); err != nil {
		return err
	}

	clientID := l.clientIDFromCookie(r)
	if clientID == "" {
		clientID = uuid.NewString()
		http.SetCookie(w, &http.Cookie{
			Name:     l.cfg.CookieName,
			Value:    clientID,
			MaxAge:   l.cfg.CookieMaxAge,
			HttpOnly: true,
			Secure:   l.cfg.CookieSecure,
			SameSite: l.cfg.CookieSameSite,
			Path:     "/",
		})
	}
	return l.increment(r, fmt.Sprintf("%s:%s:client:%s", keyPrefix, endpointName, clientID), maxRequests)
}

func (l *Limiter) increment(r *http.Request, key string, maxRequests int) error {
	result, err := checkAndIncrement.Run(r.Context(), l.client, []string{key},
		maxRequests, l.cfg.WindowSeconds).Int64Slice()
	if err != nil || len(result) != 3 {
		return &Error{
			Status: http.StatusServiceUnavailable, Code: "rate_limit_unavailable",
			Message: "Rate limit storage is unavailable. Please try again later.",
		}
	}
	if result[0] == 0 {
		return &Error{
			Status: http.StatusTooManyRequests, Code: "rate_limit_exceeded",
			Message:    "Too many analyze requests. Please try again later.",
			RetryAfter: int(result[2]),
		}
	}
	return nil
}

func (l *Limiter) clientIDFromCookie(r *http.Request) string {
	cookie, err := r.Cookie(l.cfg.CookieName)
	if err != nil {
		return ""
	}
	id, err := uuid.Parse(cookie.Value)
	if err != nil {
		return ""
	}
	return id.String()
}

func clientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		first, _, _ := strings.Cut(forwarded, ",")
		return strings.TrimSpace(first)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return "unknown"
	}
	return host
}

func hashIP(ip, salt string) string {
	mac := hmac.New(sha256.New, []byte(salt))
	mac.Write([]byte(ip))
	return hex.EncodeToString(mac.Sum(nil))
}
