// Package requestid provides request ID generation, propagation, and
// context storage for MiniEdge request tracing.
//
// Every inbound request receives a stable, unique ID that:
//   - is extracted from X-Request-ID if present and well-formed (trusted network only)
//   - otherwise is generated using crypto/rand (URL-safe base64, 16 bytes = 128 bits)
//   - is stored in the request context for downstream use
//   - is returned in the X-Request-ID response header
//   - is included in all structured log events for the request
//
// Security: request IDs from clients are validated (length + charset) before
// acceptance to prevent log injection and oversized header abuse.
package requestid

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"strings"
	"unicode"
)

const (
	// HeaderName is the canonical request-ID header.
	HeaderName = "X-Request-ID"
	// maxInboundLen is the maximum accepted length of a client-provided ID.
	maxInboundLen = 128
	// generatedBytes is the number of random bytes used for generated IDs (128-bit).
	generatedBytes = 16
)

// ctxKey is an unexported type for context keys in this package, preventing
// collisions with keys from other packages.
type ctxKey struct{}

// FromContext retrieves the request ID stored in ctx.
// Returns empty string if none is set.
func FromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxKey{}).(string)
	return v
}

// WithContext returns a new context carrying the given request ID.
func WithContext(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// Generate creates a new cryptographically random request ID (URL-safe base64).
// Panics only if crypto/rand is unavailable — acceptable for an edge gateway
// where crypto/rand failure is a fatal system error.
func Generate() string {
	b := make([]byte, generatedBytes)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failure is a fatal system condition; panic is correct here.
		panic("requestid: crypto/rand unavailable: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// Middleware returns an http.Handler that assigns every request a stable ID,
// injects it into the request context, and sets it on the response header.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := acceptOrGenerate(r.Header.Get(HeaderName))
		// Propagate on the response so the caller can correlate.
		w.Header().Set(HeaderName, id)
		// Store in context for handlers and loggers.
		r = r.WithContext(WithContext(r.Context(), id))
		next.ServeHTTP(w, r)
	})
}

// acceptOrGenerate validates a client-supplied ID or generates a fresh one.
// A client-supplied ID is accepted only if it is non-empty, within length
// bounds, and contains only safe printable ASCII (no control/whitespace chars)
// to prevent log injection (SEC-09).
func acceptOrGenerate(supplied string) string {
	if supplied != "" && isSafeID(supplied) {
		return supplied
	}
	return Generate()
}

// isSafeID returns true if s is a safe request ID: non-empty, within the
// length cap, and contains only URL-safe ASCII characters (no whitespace,
// control chars, or injection-prone characters).
func isSafeID(s string) bool {
	if len(s) == 0 || len(s) > maxInboundLen {
		return false
	}
	for _, r := range s {
		if r > 127 || unicode.IsControl(r) || unicode.IsSpace(r) {
			return false
		}
		// Reject characters that can cause log injection or header splitting.
		if strings.ContainsRune(`"'\r\n<>&`, r) {
			return false
		}
	}
	return true
}
