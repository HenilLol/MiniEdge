// Package admin implements the MiniEdge admin API.
//
// Security controls applied here:
//   - SEC-08: admin operations are on a separate listener/addr; no public exposure
//   - SEC-08: every mutation validates the full proposed state before activation
//   - SEC-10: config reload uses atomic snapshot (validates, then swaps)
//   - SEC-11: responses never expose secrets or internal topology
//   - D-07:   simulation control is bounded, service-scoped, expiring
//   - REL-05: failed reload leaves the last known-good config active
package admin

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"miniedge/internal/config"
	edgeerr "miniedge/internal/errors"
	"miniedge/internal/logger"
	"miniedge/internal/proxy"
	"miniedge/internal/simulation"
)

const (
	maxAdminBodyBytes = 1 << 20 // 1 MiB — config payload cap (SEC-05)
	adminTokenHeader  = "X-Admin-Token"
)

// Handler is the admin HTTP handler.
type Handler struct {
	cfgStore   *config.Store
	proxyH     *proxy.Handler
	sim        *simulation.State
	log        *logger.Logger
	adminToken string // optional; empty = token auth disabled (local dev only)
}

// New creates an admin Handler.
// adminToken may be empty to disable token authentication (local-dev only).
func New(
	cfgStore *config.Store,
	proxyH *proxy.Handler,
	sim *simulation.State,
	log *logger.Logger,
	adminToken string,
) *Handler {
	return &Handler{
		cfgStore:   cfgStore,
		proxyH:     proxyH,
		sim:        sim,
		log:        log,
		adminToken: adminToken,
	}
}

// RegisterRoutes mounts all admin endpoints on the given mux.
// All paths are under /admin/ to keep them off the data plane.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/admin/status", h.handleStatus)
	mux.HandleFunc("/admin/config/reload", h.handleConfigReload)
	mux.HandleFunc("/admin/simulation/activate", h.handleSimActivate)
	mux.HandleFunc("/admin/simulation/deactivate", h.handleSimDeactivate)
	mux.HandleFunc("/admin/simulation/status", h.handleSimStatus)
	mux.HandleFunc("/healthz", h.handleHealth)
}

// ──────────────────────────────────────────────────────────
// Auth
// ──────────────────────────────────────────────────────────

func (h *Handler) checkAuth(r *http.Request) bool {
	if h.adminToken == "" {
		return true // auth disabled for local-dev
	}
	provided := r.Header.Get(adminTokenHeader)
	// Constant-time comparison to resist timing attacks.
	return safeEqual(provided, h.adminToken)
}

// safeEqual performs a constant-time string comparison to prevent
// timing-based token enumeration.
func safeEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := 0; i < len(a); i++ {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}

// ──────────────────────────────────────────────────────────
// /healthz — public health endpoint (no auth required)
// ──────────────────────────────────────────────────────────

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAdminError(w, edgeerr.New(edgeerr.ClassInvalidRequest, "method not allowed"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ──────────────────────────────────────────────────────────
// /admin/status — safe operational status (no secrets)
// ──────────────────────────────────────────────────────────

func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAdminError(w, edgeerr.New(edgeerr.ClassInvalidRequest, "method not allowed"))
		return
	}
	if !h.checkAuth(r) {
		writeAdminError(w, edgeerr.New(edgeerr.ClassUnauthorized, "unauthorized"))
		return
	}

	cfg := h.cfgStore.Load()
	writeJSON(w, http.StatusOK, map[string]any{
		"listenAddr":     cfg.ListenAddr,
		"serviceCount":   len(cfg.Services),
		"routeCount":     len(cfg.Routes),
		"activeRequests": h.proxyH.ActiveCount(),
		"timestamp":      time.Now().UTC().Format(time.RFC3339),
	})
}

// ──────────────────────────────────────────────────────────
// /admin/config/reload — atomic config reload (SEC-10, REL-05)
// ──────────────────────────────────────────────────────────

type reloadRequest struct {
	ConfigPath string `json:"configPath"`
}

func (h *Handler) handleConfigReload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAdminError(w, edgeerr.New(edgeerr.ClassInvalidRequest, "method not allowed"))
		return
	}
	if !h.checkAuth(r) {
		writeAdminError(w, edgeerr.New(edgeerr.ClassUnauthorized, "unauthorized"))
		return
	}

	body, err := readBoundedBody(r, maxAdminBodyBytes)
	if err != nil {
		writeAdminError(w, edgeerr.New(edgeerr.ClassLimitExceeded, "request body too large"))
		return
	}

	var req reloadRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeAdminError(w, edgeerr.New(edgeerr.ClassInvalidRequest, "invalid request body"))
		return
	}

	if req.ConfigPath == "" {
		writeAdminError(w, edgeerr.New(edgeerr.ClassInvalidRequest, "configPath is required"))
		return
	}
	// Reject control characters or path traversal in the path.
	if containsControlCharsOrTraversal(req.ConfigPath) {
		writeAdminError(w, edgeerr.New(edgeerr.ClassInvalidRequest, "invalid configPath"))
		return
	}

	newCfg, err := config.LoadFile(req.ConfigPath)
	if err != nil {
		// Log safely; do not return raw parse error to client.
		h.log.Error("config_reload_failed", logger.F("error", sanitizeErr(err)))
		writeAdminError(w, edgeerr.New(edgeerr.ClassInvalidConfig, "configuration validation failed"))
		return
	}

	// Atomically activate. Prior config remains active until this point.
	h.cfgStore.Swap(newCfg)
	h.proxyH.RebuildClient(newCfg)

	h.log.Info("config_reloaded",
		logger.F("listenAddr", newCfg.ListenAddr),
		logger.F("services", fmt.Sprintf("%d", len(newCfg.Services))),
	)
	writeJSON(w, http.StatusOK, map[string]string{"status": "reloaded"})
}

// ──────────────────────────────────────────────────────────
// /admin/simulation/* — bounded failure simulation (D-07, REL-10)
// ──────────────────────────────────────────────────────────

type activateSimRequest struct {
	ServiceID   string `json:"serviceId"`
	Mode        string `json:"mode"`
	DelayMs     int64  `json:"delayMs"`
	DurationSec int64  `json:"durationSeconds"`
}

func (h *Handler) handleSimActivate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAdminError(w, edgeerr.New(edgeerr.ClassInvalidRequest, "method not allowed"))
		return
	}
	if !h.checkAuth(r) {
		writeAdminError(w, edgeerr.New(edgeerr.ClassUnauthorized, "unauthorized"))
		return
	}

	cfg := h.cfgStore.Load()
	if !cfg.FailureSimulation.Enabled {
		writeAdminError(w, edgeerr.New(edgeerr.ClassInvalidSimulation, "failure simulation is disabled in configuration"))
		return
	}

	body, err := readBoundedBody(r, maxAdminBodyBytes)
	if err != nil {
		writeAdminError(w, edgeerr.New(edgeerr.ClassLimitExceeded, "request body too large"))
		return
	}

	var req activateSimRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeAdminError(w, edgeerr.New(edgeerr.ClassInvalidRequest, "invalid request body"))
		return
	}

	// Build the allowed-services set from config.
	allowed := make(map[string]bool, len(cfg.FailureSimulation.AllowedServices))
	for _, svc := range cfg.FailureSimulation.AllowedServices {
		allowed[svc] = true
	}

	if err := h.sim.Activate(
		req.ServiceID,
		simulation.Mode(req.Mode),
		req.DelayMs,
		req.DurationSec,
		cfg.FailureSimulation.MaxDelayMs,
		cfg.FailureSimulation.MaxDurationSeconds,
		allowed,
	); err != nil {
		h.log.Warn("simulation_activate_rejected", logger.F("error", sanitizeErr(err)))
		writeAdminError(w, edgeerr.Newf(edgeerr.ClassInvalidSimulation, "%s", sanitizeErr(err)))
		return
	}

	h.log.Info("simulation_activated",
		logger.F("service", req.ServiceID),
		logger.F("mode", req.Mode),
	)
	writeJSON(w, http.StatusOK, map[string]string{"status": "activated", "service": req.ServiceID})
}

type deactivateSimRequest struct {
	ServiceID string `json:"serviceId"`
}

func (h *Handler) handleSimDeactivate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAdminError(w, edgeerr.New(edgeerr.ClassInvalidRequest, "method not allowed"))
		return
	}
	if !h.checkAuth(r) {
		writeAdminError(w, edgeerr.New(edgeerr.ClassUnauthorized, "unauthorized"))
		return
	}

	body, err := readBoundedBody(r, maxAdminBodyBytes)
	if err != nil {
		writeAdminError(w, edgeerr.New(edgeerr.ClassLimitExceeded, "request body too large"))
		return
	}

	var req deactivateSimRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeAdminError(w, edgeerr.New(edgeerr.ClassInvalidRequest, "invalid request body"))
		return
	}

	h.sim.Deactivate(req.ServiceID)
	h.log.Info("simulation_deactivated", logger.F("service", req.ServiceID))
	writeJSON(w, http.StatusOK, map[string]string{"status": "deactivated"})
}

func (h *Handler) handleSimStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAdminError(w, edgeerr.New(edgeerr.ClassInvalidRequest, "method not allowed"))
		return
	}
	if !h.checkAuth(r) {
		writeAdminError(w, edgeerr.New(edgeerr.ClassUnauthorized, "unauthorized"))
		return
	}

	active := h.sim.Active()
	type simEntry struct {
		ServiceID string `json:"serviceId"`
		Mode      string `json:"mode"`
		DelayMs   int64  `json:"delayMs"`
		ExpiresAt string `json:"expiresAt"`
	}
	entries := make([]simEntry, 0, len(active))
	for _, s := range active {
		entries = append(entries, simEntry{
			ServiceID: s.ServiceID,
			Mode:      string(s.Mode),
			DelayMs:   s.DelayMs,
			ExpiresAt: s.ExpiresAt.UTC().Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"simulations": entries})
}

// ──────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────

func readBoundedBody(r *http.Request, max int64) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	// Read one extra byte so we can detect a body that exceeds the limit.
	data, err := io.ReadAll(io.LimitReader(r.Body, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, fmt.Errorf("request body exceeds maximum %d bytes", max)
	}
	return data, nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		http.Error(w, `{"error":"internal_error"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

func writeAdminError(w http.ResponseWriter, e *edgeerr.EdgeError) {
	writeJSON(w, e.Class.HTTPStatus(), map[string]string{
		"error": e.PublicMsg,
		"class": string(e.Class),
	})
}

func sanitizeErr(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	if len(s) > 256 {
		s = s[:256] + "...[truncated]"
	}
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	return s
}

func containsControlCharsOrTraversal(s string) bool {
	if strings.Contains(s, "..") {
		return true
	}
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}
