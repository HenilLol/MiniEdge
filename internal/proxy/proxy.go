package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"miniedge/internal/model"
)

// ServiceProxy maps ServiceIDs to ReverseProxy instances for forwarding.
type ServiceProxy struct {
	mu             sync.RWMutex
	proxies        map[string]*httputil.ReverseProxy
	requestTimeout time.Duration
}

// NewServiceProxy initializes a ServiceProxy with specified request timeout.
func NewServiceProxy(timeout time.Duration) *ServiceProxy {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &ServiceProxy{
		proxies:        make(map[string]*httputil.ReverseProxy),
		requestTimeout: timeout,
	}
}

// Forward forwards the HTTP request to the target service.
func (sp *ServiceProxy) Forward(w http.ResponseWriter, r *http.Request, svc model.Service) {
	rp, err := sp.getOrCreateProxy(svc)
	if err != nil {
		reqID := r.Header.Get("X-Request-ID")
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-ID", reqID)
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":      string(model.ErrCodeBadGateway),
			"message":    "invalid upstream configuration",
			"request_id": reqID,
		})
		return
	}

	rp.ServeHTTP(w, r)
}

func (sp *ServiceProxy) getOrCreateProxy(svc model.Service) (*httputil.ReverseProxy, error) {
	sp.mu.RLock()
	rp, ok := sp.proxies[svc.ID]
	sp.mu.RUnlock()
	if ok {
		return rp, nil
	}

	sp.mu.Lock()
	defer sp.mu.Unlock()

	if rp, ok := sp.proxies[svc.ID]; ok {
		return rp, nil
	}

	parsedURL, err := url.Parse(svc.Upstream)
	if err != nil {
		return nil, err
	}

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: sp.requestTimeout,
	}

	newRP := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			// Capture original incoming client Host before modifying req.Host
			origHost := req.Host
			if origHost == "" && req.URL != nil {
				origHost = req.URL.Host
			}

			req.URL.Scheme = parsedURL.Scheme
			req.URL.Host = parsedURL.Host
			req.Host = parsedURL.Host

			// Handle upstream base path joining
			if parsedURL.Path != "" {
				if parsedURL.RawPath != "" || req.URL.RawPath != "" {
					targetRaw := parsedURL.RawPath
					if targetRaw == "" {
						targetRaw = url.PathEscape(parsedURL.Path)
					}
					reqRaw := req.URL.RawPath
					if reqRaw == "" {
						reqRaw = url.PathEscape(req.URL.Path)
					}
					req.URL.RawPath = joinPaths(targetRaw, reqRaw)
				}
				req.URL.Path = joinPaths(parsedURL.Path, req.URL.Path)
			}

			// Combine query strings if upstream URL specifies raw query
			if parsedURL.RawQuery != "" {
				if req.URL.RawQuery == "" {
					req.URL.RawQuery = parsedURL.RawQuery
				} else {
					req.URL.RawQuery = parsedURL.RawQuery + "&" + req.URL.RawQuery
				}
			}

			// Forward X-Forwarded-* headers
			if clientIP, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
				if prior, ok := req.Header["X-Forwarded-For"]; ok {
					clientIP = strings.Join(prior, ", ") + ", " + clientIP
				}
				req.Header.Set("X-Forwarded-For", clientIP)
			}
			if origHost != "" {
				req.Header.Set("X-Forwarded-Host", origHost)
			}
			if req.TLS != nil {
				req.Header.Set("X-Forwarded-Proto", "https")
			} else {
				req.Header.Set("X-Forwarded-Proto", "http")
			}
		},
		Transport: transport,
		ErrorHandler: func(w http.ResponseWriter, req *http.Request, err error) {
			reqID := req.Header.Get("X-Request-ID")
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Request-ID", reqID)

			if errors.Is(err, context.DeadlineExceeded) || isTimeoutError(err) {
				w.WriteHeader(http.StatusGatewayTimeout)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"error":      string(model.ErrCodeUpstreamTimeout),
					"message":    "upstream request timed out",
					"request_id": reqID,
				})
				return
			}

			w.WriteHeader(http.StatusBadGateway)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error":      string(model.ErrCodeBadGateway),
				"message":    "upstream service unavailable",
				"request_id": reqID,
			})
		},
	}

	sp.proxies[svc.ID] = newRP
	return newRP, nil
}

func joinPaths(targetPath, reqPath string) string {
	if targetPath == "" || targetPath == "/" {
		return reqPath
	}
	targetHasSuffix := strings.HasSuffix(targetPath, "/")
	reqHasPrefix := strings.HasPrefix(reqPath, "/")

	switch {
	case targetHasSuffix && reqHasPrefix:
		return targetPath + reqPath[1:]
	case !targetHasSuffix && !reqHasPrefix:
		return targetPath + "/" + reqPath
	default:
		return targetPath + reqPath
	}
}

func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded")
}
