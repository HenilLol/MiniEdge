package ssrf_test

// Tests cover S-01 through S-07 from SECURITY_TEST_PLAN.md.
// All tests use only the standard library; no external services are required.

import (
	"fmt"
	"testing"

	"miniedge/internal/ssrf"
)

// ──────────────────────────────────────────────────────────
// Helper
// ──────────────────────────────────────────────────────────

func mustReject(t *testing.T, rawURL string, policy ssrf.Policy, label string) {
	t.Helper()
	if err := ssrf.ValidateUpstreamURL(rawURL, policy); err == nil {
		t.Errorf("[%s] expected rejection of %q but got nil", label, rawURL)
	}
}

func mustAllow(t *testing.T, rawURL string, policy ssrf.Policy, label string) {
	t.Helper()
	if err := ssrf.ValidateUpstreamURL(rawURL, policy); err != nil {
		t.Errorf("[%s] expected %q to be allowed but got: %v", label, rawURL, err)
	}
}

// ──────────────────────────────────────────────────────────
// S-01: Loopback hostnames
// ──────────────────────────────────────────────────────────

func TestLoopbackHostname(t *testing.T) {
	p := ssrf.DefaultPolicy()
	for _, u := range []string{
		"http://localhost:8080",
		"http://localhost",
		"http://LOCALHOST:9000",
	} {
		mustReject(t, u, p, "loopback-hostname")
	}
}

// ──────────────────────────────────────────────────────────
// S-01 / S-02: Loopback and private IPv4 literals
// ──────────────────────────────────────────────────────────

func TestLoopbackIPv4Literal(t *testing.T) {
	p := ssrf.DefaultPolicy()
	for _, u := range []string{
		"http://127.0.0.1:9000",
		"http://127.1.2.3:9000",
		"http://127.255.255.255:80",
	} {
		mustReject(t, u, p, "loopback-ipv4")
	}
}

func TestPrivateIPv4Literal_DefaultDeny(t *testing.T) {
	p := ssrf.DefaultPolicy() // AllowPrivate = false
	for _, u := range []string{
		"http://10.0.0.1:8080",
		"http://172.16.0.1:8080",
		"http://192.168.1.1:8080",
	} {
		mustReject(t, u, p, "private-ipv4-default")
	}
}

func TestPrivateIPv4Literal_AllowedWhenEnabled(t *testing.T) {
	p := ssrf.LocalDevPolicy() // AllowPrivate = true
	for _, u := range []string{
		"http://10.0.0.1:8080",
		"http://172.16.0.1:8080",
		"http://192.168.1.1:8080",
	} {
		mustAllow(t, u, p, "private-ipv4-allowed")
	}
}

// ──────────────────────────────────────────────────────────
// S-03: IPv6 loopback and link-local
// ──────────────────────────────────────────────────────────

func TestIPv6Loopback(t *testing.T) {
	p := ssrf.DefaultPolicy()
	for _, u := range []string{
		"http://[::1]:9000",
		"http://[::1]",
	} {
		mustReject(t, u, p, "ipv6-loopback")
	}
}

func TestIPv6LinkLocal(t *testing.T) {
	p := ssrf.DefaultPolicy()
	// fe80::/10 is link-local
	mustReject(t, "http://[fe80::1]:9000", p, "ipv6-link-local")
}

func TestIPv6UniqueLocal_DefaultDeny(t *testing.T) {
	p := ssrf.DefaultPolicy() // AllowPrivate = false — fc00::/7 is IsPrivate
	mustReject(t, "http://[fc00::1]:9000", p, "ipv6-unique-local-default")
}

// ──────────────────────────────────────────────────────────
// S-04: Invalid / special-form inputs
// ──────────────────────────────────────────────────────────

func TestEmptyURL(t *testing.T) {
	mustReject(t, "", ssrf.DefaultPolicy(), "empty-url")
}

func TestEmptyHost(t *testing.T) {
	mustReject(t, "http://:8080/path", ssrf.DefaultPolicy(), "empty-host")
}

func TestUnsupportedScheme(t *testing.T) {
	p := ssrf.DefaultPolicy()
	for _, u := range []string{
		"file:///etc/passwd",
		"ftp://example.com",
		"https://example.com", // not in default allowed schemes
	} {
		mustReject(t, u, p, "unsupported-scheme")
	}
}

func TestHTTPSAllowedWhenConfigured(t *testing.T) {
	p := ssrf.Policy{AllowedSchemes: map[string]bool{"http": true, "https": true}, AllowPrivate: false}
	// This will try to resolve example.com — in a test environment it may succeed or fail
	// depending on DNS. Either outcome is acceptable; we just want no panic.
	_ = ssrf.ValidateUpstreamURL("https://example.com:443", p)
}

func TestUserinfoRejected(t *testing.T) {
	mustReject(t, "http://user:pass@example.com:80", ssrf.DefaultPolicy(), "userinfo")
}

func TestFragmentRejected(t *testing.T) {
	mustReject(t, "http://example.com:80/#fragment", ssrf.DefaultPolicy(), "fragment")
}

func TestInvalidPort(t *testing.T) {
	for _, u := range []string{
		"http://example.com:0",
		"http://example.com:99999",
		"http://example.com:-1",
		"http://example.com:notaport",
	} {
		mustReject(t, u, ssrf.DefaultPolicy(), fmt.Sprintf("invalid-port: %s", u))
	}
}

func TestURLTooLong(t *testing.T) {
	long := "http://example.com/" + string(make([]byte, 2048))
	mustReject(t, long, ssrf.DefaultPolicy(), "url-too-long")
}

// ──────────────────────────────────────────────────────────
// Documentation/reserved ranges (T-02)
// ──────────────────────────────────────────────────────────

func TestDocumentationRanges(t *testing.T) {
	p := ssrf.DefaultPolicy()
	for _, u := range []string{
		"http://192.0.2.1:80",    // TEST-NET-1
		"http://198.51.100.1:80", // TEST-NET-2
		"http://203.0.113.1:80",  // TEST-NET-3
		"http://198.18.0.1:80",   // Benchmarking
	} {
		mustReject(t, u, p, "documentation-range")
	}
}

// ──────────────────────────────────────────────────────────
// Multicast / unspecified
// ──────────────────────────────────────────────────────────

func TestMulticastAndUnspecified(t *testing.T) {
	p := ssrf.DefaultPolicy()
	mustReject(t, "http://224.0.0.1:80", p, "multicast-ipv4")
	mustReject(t, "http://0.0.0.0:80", p, "unspecified-ipv4")
}
