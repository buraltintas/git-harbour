package server

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type rateWindow struct {
	started time.Time
	count   int
}

type requestLimiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	items  map[string]rateWindow
}

func newRequestLimiter(limit int, window time.Duration) *requestLimiter {
	return &requestLimiter{limit: limit, window: window, items: map[string]rateWindow{}}
}

func (l *requestLimiter) allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	v := l.items[key]
	if v.started.IsZero() || now.Sub(v.started) >= l.window {
		l.items[key] = rateWindow{started: now, count: 1}
		return true
	}
	if v.count >= l.limit {
		return false
	}
	v.count++
	l.items[key] = v
	return true
}

func (s *Server) rateLimitPVP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		pvpMutation := r.Method == http.MethodPost && (strings.HasPrefix(path, "/v1/challenges") || strings.HasSuffix(path, "/shots") || strings.HasSuffix(path, "/rematch"))
		if !pvpMutation {
			next.ServeHTTP(w, r)
			return
		}
		key := bearer(r)
		if key != "" {
			key = fmt.Sprintf("token:%x", digest(key))
		} else {
			host, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				host = r.RemoteAddr
			}
			key = "ip:" + host
		}
		if !s.limits.allow(key, s.now()) {
			w.Header().Set("Retry-After", "60")
			writeError(w, http.StatusTooManyRequests, "rate_limited", "Too many battle actions. Try again shortly.")
			return
		}
		next.ServeHTTP(w, r)
	})
}
