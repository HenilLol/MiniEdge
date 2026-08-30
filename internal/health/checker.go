package health

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"miniedge/internal/model"
)

// CheckResult represents the outcome of a single service health probe.
type CheckResult struct {
	ServiceID string
	Status    model.HealthStatus
	CheckedAt time.Time
	Latency   time.Duration
}

// Checker executes HTTP GET health checks against configured upstream services.
type Checker struct {
	client        *http.Client
	timeout       time.Duration
	slowThreshold time.Duration
}

// NewChecker initializes a Checker with specified request timeout and slow threshold.
func NewChecker(timeout, slowThreshold time.Duration) *Checker {
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	if slowThreshold <= 0 {
		slowThreshold = 500 * time.Millisecond
	}

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   timeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   timeout,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &Checker{
		client: &http.Client{
			Transport: transport,
			Timeout:   timeout,
		},
		timeout:       timeout,
		slowThreshold: slowThreshold,
	}
}

// BuildHealthURL joins upstream base URL with healthPath cleanly without double slashes.
func BuildHealthURL(upstream, healthPath string) (string, error) {
	base, err := url.Parse(upstream)
	if err != nil {
		return "", err
	}

	if !strings.HasPrefix(healthPath, "/") {
		healthPath = "/" + healthPath
	}

	basePath := strings.TrimRight(base.Path, "/")
	fullPath := basePath + healthPath

	u := *base
	u.Path = fullPath
	return u.String(), nil
}

// Check probes a service and classifies its health status (UP, SLOW, DOWN).
func (c *Checker) Check(ctx context.Context, svc model.Service) CheckResult {
	targetURL, err := BuildHealthURL(svc.Upstream, svc.HealthPath)
	if err != nil {
		return CheckResult{
			ServiceID: svc.ID,
			Status:    model.HealthDown,
			CheckedAt: time.Now(),
			Latency:   0,
		}
	}

	reqCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, targetURL, nil)
	if err != nil {
		return CheckResult{
			ServiceID: svc.ID,
			Status:    model.HealthDown,
			CheckedAt: time.Now(),
			Latency:   0,
		}
	}
	req.Header.Set("User-Agent", "MiniEdge-HealthChecker/1.0")

	start := time.Now()
	resp, err := c.client.Do(req)
	latency := time.Since(start)
	checkedAt := time.Now()

	if err != nil {
		return CheckResult{
			ServiceID: svc.ID,
			Status:    model.HealthDown,
			CheckedAt: checkedAt,
			Latency:   latency,
		}
	}
	defer resp.Body.Close()

	// Safely drain response body
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 2048))

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return CheckResult{
			ServiceID: svc.ID,
			Status:    model.HealthDown,
			CheckedAt: checkedAt,
			Latency:   latency,
		}
	}

	status := model.HealthUp
	if latency > c.slowThreshold {
		status = model.HealthSlow
	}

	return CheckResult{
		ServiceID: svc.ID,
		Status:    status,
		CheckedAt: checkedAt,
		Latency:   latency,
	}
}
