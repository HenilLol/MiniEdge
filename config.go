// Package config defines MiniEdge configuration types, a strict JSON loader,
// and a full validator. Configuration is parsed into an immutable snapshot and
// atomically activated only after all checks pass (SEC-10, D-04, REL-05).
//
// The active snapshot is stored in an atomic pointer so request handlers can read
// it without locking (REL-04, REL-11).
package config

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"sync/atomic"
	"unicode"

	"miniedge/internal/ssrf"
)

// ──────────────────────────────────────────────────────────
// Limits
// ──────────────────────────────────────────────────────────

const (
	maxConfigFileBytes     = 1 << 20 // 1 MiB (SEC-05)
	maxServiceCount        = 256
	maxUpstreamCount       = 256
	maxRouteCount          = 256
	maxServiceNameLen      = 64
	maxUpstreamNameLen     = 64
	maxRouteIDLen          = 64
	maxRouteMatchLen       = 256
	maxUpstreamURLLen      = 2048
	maxAllowedServicesList = 64
)

// ──────────────────────────────────────────────────────────
// Configuration types
// ──────────────────────────────────────────────────────────

// Timeouts holds per-operation timeout budgets in milliseconds. All must be > 0.
type Timeouts struct {
	HeaderReadMs       int64 `json:"headerReadMs"`
	BodyReadMs         int64 `json:"bodyReadMs"`
	UpstreamConnectMs  int64 `json:"upstreamConnectMs"`
	UpstreamResponseMs int64 `json:"upstreamResponseMs"`
	UpstreamBodyMs     int64 `json:"upstreamBodyMs"`
	TotalRequestMs     int64 `json:"totalRequestMs"`
	ShutdownDrainMs    int64 `json:"shutdownDrainMs"`
}

// DefaultTimeouts returns the safe baseline values from SECURITY_CONFIG_SPEC.md.
func DefaultTimeouts() Timeouts {
	return Timeouts{
		HeaderReadMs:       5_000,
		BodyReadMs:         15_000,
		UpstreamConnectMs:  3_000,
		UpstreamResponseMs: 5_000,
		UpstreamBodyMs:     15_000,
		TotalRequestMs:     30_000,
		ShutdownDrainMs:    10_000,
	}
}

// Limits holds resource-protection bounds.
type Limits struct {
	MaxRequestTargetBytes int64 `json:"maxRequestTargetBytes"`
	MaxHeaderBytes        int64 `json:"maxHeaderBytes"`
	MaxHeaderCount        int   `json:"maxHeaderCount"`
	MaxRequestBodyBytes   int64 `json:"maxRequestBodyBytes"`
	MaxResponseBodyBytes  int64 `json:"maxResponseBodyBytes"`
	MaxActiveRequests     int   `json:"maxActiveRequests"`
	MaxQueuedRequests     int   `json:"maxQueuedRequests"`
	MaxLogEventBytes      int64 `json:"maxLogEventBytes"`
}

// DefaultLimits returns the safe baseline values from SECURITY_CONFIG_SPEC.md.
func DefaultLimits() Limits {
	return Limits{
		MaxRequestTargetBytes: 8_192,
		MaxHeaderBytes:        32_768,
		MaxHeaderCount:        100,
		MaxRequestBodyBytes:   1_048_576,
		MaxResponseBodyBytes:  5_242_880,
		MaxActiveRequests:     64,
		MaxQueuedRequests:     64,
		MaxLogEventBytes:      8_192,
	}
}

// Upstream describes one configured upstream destination.
type Upstream struct {
	// URL is the full base URL of the upstream. Validated through the SSRF policy.
	URL string `json:"url"`
	// AllowPrivate, when true, permits private-range upstreams for local-dev.
	AllowPrivate bool `json:"allowPrivate"`
}

// Service is a named routing target that maps to a configured upstream.
type Service struct {
	Upstream string `json:"upstream"` // must match an Upstreams key
	Enabled  bool   `json:"enabled"`
}

// RouteMethod is an HTTP method allow-list entry.
type RouteMethod = string

// Route maps an incoming path prefix to a service.
type Route struct {
	ID      string        `json:"id"`
	Match   string        `json:"match"`   // path prefix
	Service string        `json:"service"` // must match a Services key
	Methods []RouteMethod `json:"methods"` // empty = allow all defined methods
}

// RateLimit holds the in-memory rate-limit settings.
type RateLimit struct {
	Enabled       bool  `json:"enabled"`
	Requests      int   `json:"requests"`
	WindowSeconds int   `json:"windowSeconds"`
	MaxKeys       int   `json:"maxKeys"`
}

// DefaultRateLimit returns a safe, enabled baseline.
func DefaultRateLimit() RateLimit {
	return RateLimit{
		Enabled:       true,
		Requests:      60,
		WindowSeconds: 60,
		MaxKeys:       10_000,
	}
}

// FailureSimulation holds the simulation feature configuration.
type FailureSimulation struct {
	Enabled            bool     `json:"enabled"`
	AllowedServices    []string `json:"allowedServices"`
	MaxDelayMs         int64    `json:"maxDelayMilliseconds"`
	MaxDurationSeconds int64    `json:"maxDurationSeconds"`
	Mode               string   `json:"mode"` // "delay" | "error"
}

// DefaultFailureSimulation returns simulation disabled by default (D-07).
func DefaultFailureSimulation() FailureSimulation {
	return FailureSimulation{
		Enabled:            false,
		AllowedServices:    nil,
		MaxDelayMs:         2_000,
		MaxDurationSeconds: 60,
		Mode:               "delay",
	}
}

// Config is the complete configuration snapshot. It is immutable once built.
type Config struct {
	ListenAddr string `json:"listenAddr"` // e.g. "127.0.0.1:8080"
	AdminAddr  string `json:"adminAddr"`  // e.g. "127.0.0.1:9090"

	Upstreams map[string]Upstream `json:"upstreams"`
	Services  map[string]Service  `json:"services"`
	Routes    []Route             `json:"routes"`

	Timeouts          Timeouts          `json:"timeouts"`
	Limits            Limits            `json:"limits"`
	RateLimit         RateLimit         `json:"rateLimit"`
	FailureSimulation FailureSimulation `json:"failureSimulation"`
}

// ──────────────────────────────────────────────────────────
// Atomic active config snapshot (REL-04, REL-11)
// ──────────────────────────────────────────────────────────

// Store holds the active immutable config snapshot behind an atomic pointer.
// Handlers call Load() without holding a mutex; only the reload path calls Swap().
type Store struct {
	p atomic.Pointer[Config]
}

// NewStore initialises the store with a validated config.
func NewStore(c *Config) *Store {
	s := &Store{}
	s.p.Store(c)
	return s
}

// Load returns the current active snapshot. Never nil after NewStore.
func (s *Store) Load() *Config {
	return s.p.Load()
}

// Swap atomically replaces the active config. The candidate must already be
// fully validated. Returns the previous config.
func (s *Store) Swap(c *Config) *Config {
	return s.p.Swap(c)
}

// ──────────────────────────────────────────────────────────
// JSON loader
// ──────────────────────────────────────────────────────────

// LoadFile reads and fully validates the config file at path.
// On any error the file contents are not returned to the caller, preventing
// accidental disclosure of secrets. The raw parse error is returned only for
// internal logging (never sent to clients).
func LoadFile(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open config file: %w", err)
	}
	defer f.Close()

	// Bound the read to maxConfigFileBytes (SEC-05, SEC-10).
	r := io.LimitReader(f, maxConfigFileBytes+1)
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("cannot read config file: %w", err)
	}
	if int64(len(data)) > maxConfigFileBytes {
		return nil, fmt.Errorf("config file exceeds maximum size of %d bytes", maxConfigFileBytes)
	}

	return LoadBytes(data)
}

// LoadBytes parses and validates a JSON config payload.
func LoadBytes(data []byte) (*Config, error) {
	// Start from safe defaults so missing optional sections are safe.
	c := &Config{
		Timeouts:          DefaultTimeouts(),
		Limits:            DefaultLimits(),
		RateLimit:         DefaultRateLimit(),
		FailureSimulation: DefaultFailureSimulation(),
	}

	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields() // SEC-10: reject unknown fields
	if err := dec.Decode(c); err != nil {
		return nil, fmt.Errorf("config parse error: %w", err)
	}

	if err := Validate(c); err != nil {
		return nil, err
	}
	return c, nil
}

// ──────────────────────────────────────────────────────────
// Validator
// ──────────────────────────────────────────────────────────

// Validate performs complete cross-field validation of a candidate config
// snapshot. Returns an error if ANY field is invalid; the candidate must not
// be activated on error (SEC-10, D-04, REL-05, REL-12).
func Validate(c *Config) error {
	if err := validateAddrs(c); err != nil {
		return err
	}
	if err := validateTimeouts(&c.Timeouts); err != nil {
		return err
	}
	if err := validateLimits(&c.Limits); err != nil {
		return err
	}
	if err := validateUpstreams(c); err != nil {
		return err
	}
	if err := validateServices(c); err != nil {
		return err
	}
	if err := validateRoutes(c); err != nil {
		return err
	}
	if err := validateRateLimit(&c.RateLimit); err != nil {
		return err
	}
	if err := validateSimulation(c); err != nil {
		return err
	}
	return nil
}

var listenAddrRE = regexp.MustCompile(`^[\w\.\-\[\]:]+:\d+$`)

func validateAddrs(c *Config) error {
	if c.ListenAddr == "" {
		c.ListenAddr = "127.0.0.1:8080"
	}
	if !listenAddrRE.MatchString(c.ListenAddr) {
		return fmt.Errorf("invalid listenAddr %q", c.ListenAddr)
	}
	if c.AdminAddr != "" && !listenAddrRE.MatchString(c.AdminAddr) {
		return fmt.Errorf("invalid adminAddr %q", c.AdminAddr)
	}
	if c.AdminAddr == "" {
		c.AdminAddr = "127.0.0.1:9090"
	}
	return nil
}

func validateTimeouts(t *Timeouts) error {
	check := func(name string, v int64) error {
		if v <= 0 {
			return fmt.Errorf("timeout %s must be a positive number of milliseconds, got %d", name, v)
		}
		return nil
	}
	for _, pair := range []struct {
		name string
		val  int64
	}{
		{"headerReadMs", t.HeaderReadMs},
		{"bodyReadMs", t.BodyReadMs},
		{"upstreamConnectMs", t.UpstreamConnectMs},
		{"upstreamResponseMs", t.UpstreamResponseMs},
		{"upstreamBodyMs", t.UpstreamBodyMs},
		{"totalRequestMs", t.TotalRequestMs},
		{"shutdownDrainMs", t.ShutdownDrainMs},
	} {
		if err := check(pair.name, pair.val); err != nil {
			return err
		}
	}
	return nil
}

func validateLimits(l *Limits) error {
	check := func(name string, v int64) error {
		if v <= 0 {
			return fmt.Errorf("limit %s must be positive, got %d", name, v)
		}
		return nil
	}
	for _, pair := range []struct {
		name string
		val  int64
	}{
		{"maxRequestTargetBytes", l.MaxRequestTargetBytes},
		{"maxHeaderBytes", l.MaxHeaderBytes},
		{"maxRequestBodyBytes", l.MaxRequestBodyBytes},
		{"maxResponseBodyBytes", l.MaxResponseBodyBytes},
		{"maxLogEventBytes", l.MaxLogEventBytes},
	} {
		if err := check(pair.name, pair.val); err != nil {
			return err
		}
	}
	if l.MaxHeaderCount <= 0 {
		return fmt.Errorf("limit maxHeaderCount must be positive, got %d", l.MaxHeaderCount)
	}
	if l.MaxActiveRequests <= 0 {
		return fmt.Errorf("limit maxActiveRequests must be positive, got %d", l.MaxActiveRequests)
	}
	if l.MaxQueuedRequests < 0 {
		return fmt.Errorf("limit maxQueuedRequests must be non-negative, got %d", l.MaxQueuedRequests)
	}
	return nil
}

func validateUpstreams(c *Config) error {
	if len(c.Upstreams) > maxUpstreamCount {
		return fmt.Errorf("too many upstreams: %d (max %d)", len(c.Upstreams), maxUpstreamCount)
	}
	for name, up := range c.Upstreams {
		if err := validateIdentifierStr(name, maxUpstreamNameLen, "upstream"); err != nil {
			return err
		}
		if len(up.URL) > maxUpstreamURLLen {
			return fmt.Errorf("upstream %q URL exceeds maximum length", name)
		}
		policy := ssrf.DefaultPolicy()
		if up.AllowPrivate {
			policy = ssrf.LocalDevPolicy()
		}
		if err := ssrf.ValidateUpstreamURL(up.URL, policy); err != nil {
			return fmt.Errorf("upstream %q failed SSRF validation: %w", name, err)
		}
	}
	return nil
}

func validateServices(c *Config) error {
	if len(c.Services) > maxServiceCount {
		return fmt.Errorf("too many services: %d (max %d)", len(c.Services), maxServiceCount)
	}
	for name, svc := range c.Services {
		if err := validateIdentifierStr(name, maxServiceNameLen, "service"); err != nil {
			return err
		}
		if svc.Upstream == "" {
			return fmt.Errorf("service %q has empty upstream reference", name)
		}
		if _, ok := c.Upstreams[svc.Upstream]; !ok {
			return fmt.Errorf("service %q references unknown upstream %q", name, svc.Upstream)
		}
	}
	return nil
}

// validMethods is the allow-list for HTTP methods (SEC-03).
var validMethods = map[string]bool{
	"GET": true, "POST": true, "PUT": true, "PATCH": true,
	"DELETE": true, "HEAD": true, "OPTIONS": true,
}

func validateRoutes(c *Config) error {
	if len(c.Routes) > maxRouteCount {
		return fmt.Errorf("too many routes: %d (max %d)", len(c.Routes), maxRouteCount)
	}
	seenIDs := make(map[string]bool)
	seenMatches := make(map[string]bool)
	for i, r := range c.Routes {
		if err := validateIdentifierStr(r.ID, maxRouteIDLen, "route.id"); err != nil {
			return fmt.Errorf("route[%d]: %w", i, err)
		}
		if seenIDs[r.ID] {
			return fmt.Errorf("duplicate route id %q", r.ID)
		}
		seenIDs[r.ID] = true

		if r.Match == "" || len(r.Match) > maxRouteMatchLen {
			return fmt.Errorf("route %q: match pattern is empty or too long", r.ID)
		}
		if containsControlChars(r.Match) {
			return fmt.Errorf("route %q: match pattern contains control characters", r.ID)
		}
		if seenMatches[r.Match] {
			return fmt.Errorf("duplicate route match pattern %q", r.Match)
		}
		seenMatches[r.Match] = true

		if _, ok := c.Services[r.Service]; !ok {
			return fmt.Errorf("route %q references unknown service %q", r.ID, r.Service)
		}
		for _, m := range r.Methods {
			if !validMethods[strings.ToUpper(m)] {
				return fmt.Errorf("route %q: method %q is not in the allow-list", r.ID, m)
			}
		}
	}
	return nil
}

func validateRateLimit(rl *RateLimit) error {
	if !rl.Enabled {
		return nil
	}
	if rl.Requests <= 0 {
		return fmt.Errorf("rateLimit.requests must be positive, got %d", rl.Requests)
	}
	if rl.WindowSeconds <= 0 {
		return fmt.Errorf("rateLimit.windowSeconds must be positive, got %d", rl.WindowSeconds)
	}
	if rl.MaxKeys <= 0 {
		return fmt.Errorf("rateLimit.maxKeys must be positive, got %d", rl.MaxKeys)
	}
	return nil
}

var validSimModes = map[string]bool{"delay": true, "error": true}

func validateSimulation(c *Config) error {
	sim := &c.FailureSimulation
	if !sim.Enabled {
		return nil
	}
	if len(sim.AllowedServices) > maxAllowedServicesList {
		return fmt.Errorf("failureSimulation.allowedServices exceeds maximum count")
	}
	for _, svc := range sim.AllowedServices {
		if _, ok := c.Services[svc]; !ok {
			return fmt.Errorf("failureSimulation.allowedServices: unknown service %q", svc)
		}
	}
	if sim.MaxDelayMs <= 0 || sim.MaxDelayMs > 10_000 {
		return fmt.Errorf("failureSimulation.maxDelayMilliseconds must be 1–10000, got %d", sim.MaxDelayMs)
	}
	if sim.MaxDurationSeconds <= 0 || sim.MaxDurationSeconds > 300 {
		return fmt.Errorf("failureSimulation.maxDurationSeconds must be 1–300, got %d", sim.MaxDurationSeconds)
	}
	if !validSimModes[sim.Mode] {
		return fmt.Errorf("failureSimulation.mode %q is not allowed (use 'delay' or 'error')", sim.Mode)
	}
	return nil
}

// ──────────────────────────────────────────────────────────
// Identifier helpers
// ──────────────────────────────────────────────────────────

// identRE accepts ASCII alphanumeric, hyphen, and underscore only.
// This prevents path-separator injection and control characters (SEC-03).
var identRE = regexp.MustCompile(`^[A-Za-z0-9_\-]+$`)

// validateIdentifier validates a name/identifier string.
func validateIdentifierStr(s string, maxLen int, kind string) error {
	if s == "" {
		return fmt.Errorf("%s name must not be empty", kind)
	}
	if len(s) > maxLen {
		return fmt.Errorf("%s name %q exceeds maximum length %d", kind, s, maxLen)
	}
	if !identRE.MatchString(s) {
		return fmt.Errorf("%s name %q contains invalid characters (only A-Z a-z 0-9 _ - allowed)", kind, s)
	}
	return nil
}

func containsControlChars(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}
