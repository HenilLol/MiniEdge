package proxy_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"miniedge/internal/model"
	"miniedge/internal/proxy"
)

func TestProxyXForwardedHost(t *testing.T) {
	var receivedHost string
	var receivedXForwardedHost string

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHost = r.Host
		receivedXForwardedHost = r.Header.Get("X-Forwarded-Host")
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	sp := proxy.NewServiceProxy(5 * time.Second)
	svc := model.Service{
		ID:       "s1",
		Upstream: upstream.URL,
	}

	req := httptest.NewRequest(http.MethodGet, "http://original.client.com:8080/test", nil)
	req.Host = "original.client.com:8080"
	rec := httptest.NewRecorder()

	sp.Forward(rec, req, svc)

	parsedUpstream, _ := url.Parse(upstream.URL)
	if receivedHost != parsedUpstream.Host {
		t.Errorf("expected forwarded Host to be upstream host '%s', got '%s'", parsedUpstream.Host, receivedHost)
	}

	if receivedXForwardedHost != "original.client.com:8080" {
		t.Errorf("expected X-Forwarded-Host to be 'original.client.com:8080', got '%s'", receivedXForwardedHost)
	}
}

func TestProxyUpstreamSubpath(t *testing.T) {
	var receivedPath string
	var receivedQuery string

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		receivedQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer upstream.Close()

	sp := proxy.NewServiceProxy(5 * time.Second)

	tests := []struct {
		name          string
		upstreamBase  string
		reqURL        string
		expectedPath  string
		expectedQuery string
	}{
		{
			name:         "upstream v1 + incoming /users",
			upstreamBase: upstream.URL + "/v1",
			reqURL:       "http://client.com/users",
			expectedPath: "/v1/users",
		},
		{
			name:         "upstream v1/ + incoming /users",
			upstreamBase: upstream.URL + "/v1/",
			reqURL:       "http://client.com/users",
			expectedPath: "/v1/users",
		},
		{
			name:         "upstream without path + incoming /users",
			upstreamBase: upstream.URL,
			reqURL:       "http://client.com/users",
			expectedPath: "/users",
		},
		{
			name:          "query string preserved",
			upstreamBase:  upstream.URL + "/v1?secret=true",
			reqURL:        "http://client.com/users?page=1",
			expectedPath:  "/v1/users",
			expectedQuery: "secret=true&page=1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := model.Service{
				ID:       tt.name,
				Upstream: tt.upstreamBase,
			}
			req := httptest.NewRequest(http.MethodGet, tt.reqURL, nil)
			rec := httptest.NewRecorder()

			sp.Forward(rec, req, svc)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected status 200, got %d", rec.Code)
			}
			if receivedPath != tt.expectedPath {
				t.Errorf("expected path '%s', got '%s'", tt.expectedPath, receivedPath)
			}
			if tt.expectedQuery != "" && receivedQuery != tt.expectedQuery {
				t.Errorf("expected query '%s', got '%s'", tt.expectedQuery, receivedQuery)
			}
		})
	}
}
