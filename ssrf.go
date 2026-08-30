// Package ssrf implements SSRF (Server-Side Request Forgery) protection for
// MiniEdge. It validates every upstream destination before any network I/O
// is attempted. A URL-string blacklist is NOT used; instead, the URL is parsed
// structurally, every DNS result is resolved, and each resulting IP address is
// checked against all disallowed special-use ranges (SEC-01, SEC-02, T-01..T-03).
//
// Policy: deny-by-default. Only explicitly configured upstreams are allowed.
// Private network destinations are rejected unless the caller opts in via
// AllowPrivate (which should only be set for local-dev deployments that need to
// reach private-network services).
package ssrf

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// Policy holds the SSRF validation settings.
type Policy struct {
	// AllowedSchemes is the set of URL schemes permitted (normally {"http"}).
	// If empty, defaults to http only.
	AllowedSchemes map[string]bool
	// AllowPrivate permits configured upstreams that resolve to private IPv4/IPv6
	// ranges. Set to true ONLY for local-dev deployments. Default: false.
	AllowPrivate bool
}

// DefaultPolicy returns a deny-by-default policy suitable for local dev.
// It allows http only and rejects all private/loopback/link-local destinations.
func DefaultPolicy() Policy {
	return Policy{
		AllowedSchemes: map[string]bool{"http": true},
		AllowPrivate:   false,
	}
}

// LocalDevPolicy allows http and permits private-range upstreams.
// Appropriate when MiniEdge routes to local microservices (127.x, 192.168.x, etc.)
func LocalDevPolicy() Policy {
	return Policy{
		AllowedSchemes: map[string]bool{"http": true},
		AllowPrivate:   true,
	}
}

// ValidateUpstreamURL validates a fully-qualified upstream URL against the policy.
// It checks: scheme, no userinfo, no fragment, structural host, valid port,
// resolved IP addresses, and disallowed ranges. Returns nil if safe.
//
// This must be called at startup, config reload, and every admin upstream mutation.
func ValidateUpstreamURL(rawURL string, p Policy) error {
	if len(rawURL) == 0 {
		return fmt.Errorf("upstream URL is empty")
	}
	if len(rawURL) > 2048 {
		return fmt.Errorf("upstream URL exceeds maximum length")
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("upstream URL parse error: %w", err)
	}

	// Scheme check (SEC-01 rule 3).
	scheme := strings.ToLower(u.Scheme)
	if len(p.AllowedSchemes) == 0 {
		p.AllowedSchemes = map[string]bool{"http": true}
	}
	if !p.AllowedSchemes[scheme] {
		return fmt.Errorf("upstream scheme %q is not allowed", scheme)
	}

	// Reject credentials/userinfo (SEC-01 rule 4).
	if u.User != nil {
		return fmt.Errorf("upstream URL must not contain userinfo/credentials")
	}

	// Reject fragments (SEC-01 rule 4).
	if u.Fragment != "" {
		return fmt.Errorf("upstream URL must not contain a fragment")
	}

	// Validate host (SEC-01 rule 4).
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("upstream URL has empty host")
	}
	if err := validateHostSyntax(host); err != nil {
		return fmt.Errorf("upstream host syntax error: %w", err)
	}

	// Validate port (SEC-01 rule 4).
	portStr := u.Port()
	if portStr != "" {
		port, err := strconv.Atoi(portStr)
		if err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("upstream port %q is invalid", portStr)
		}
	}

	// Resolve the host and check every returned address (SEC-01 rules 5–6).
	if err := validateResolvedAddresses(host, p); err != nil {
		return err
	}

	return nil
}

// validateHostSyntax rejects obvious syntax problems.
func validateHostSyntax(host string) error {
	// Reject control characters and whitespace.
	for _, r := range host {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("control character in host")
		}
	}
	if strings.Contains(host, "..") {
		return fmt.Errorf("consecutive dots in host")
	}
	return nil
}

// validateResolvedAddresses resolves the host (or parses it as a literal IP)
// and checks every resulting address against the disallowed range list.
func validateResolvedAddresses(host string, p Policy) error {
	// If host is a literal IP, check it directly.
	if ip := net.ParseIP(host); ip != nil {
		return checkIP(ip, p)
	}

	// Bracketed IPv6 literal — strip brackets.
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		inner := host[1 : len(host)-1]
		ip := net.ParseIP(inner)
		if ip == nil {
			return fmt.Errorf("invalid bracketed IPv6 literal %q", host)
		}
		return checkIP(ip, p)
	}

	// DNS resolution — check ALL returned addresses (SEC-01 rule 5).
	addrs, err := net.LookupHost(host)
	if err != nil {
		return fmt.Errorf("upstream host %q cannot be resolved: %w", host, err)
	}
	if len(addrs) == 0 {
		return fmt.Errorf("upstream host %q resolved to no addresses", host)
	}
	for _, addr := range addrs {
		ip := net.ParseIP(addr)
		if ip == nil {
			return fmt.Errorf("resolver returned unparseable address %q for host %q", addr, host)
		}
		if err := checkIP(ip, p); err != nil {
			return fmt.Errorf("upstream host %q resolved to disallowed address %s: %w", host, addr, err)
		}
	}
	return nil
}

// checkIP returns an error if the IP is in any disallowed range.
// Disallowed unconditionally: loopback, link-local, multicast, unspecified.
// Disallowed unless AllowPrivate: private ranges.
func checkIP(ip net.IP, p Policy) error {
	// Normalise to 16-byte form so IPv4-mapped IPv6 addresses are caught.
	ip16 := ip.To16()
	if ip16 == nil {
		return fmt.Errorf("IP address cannot be normalised")
	}

	// Loopback — always denied (127.x.x.x, ::1).
	if ip16.IsLoopback() {
		return fmt.Errorf("upstream resolves to loopback address %s", ip)
	}

	// Unspecified (0.0.0.0, ::) — always denied.
	if ip16.IsUnspecified() {
		return fmt.Errorf("upstream resolves to unspecified address %s", ip)
	}

	// Link-local unicast (169.254.x.x, fe80::/10) — always denied.
	if ip16.IsLinkLocalUnicast() {
		return fmt.Errorf("upstream resolves to link-local address %s", ip)
	}

	// Link-local multicast — always denied.
	if ip16.IsLinkLocalMulticast() {
		return fmt.Errorf("upstream resolves to link-local multicast address %s", ip)
	}

	// Multicast (224.0.0.0/4, ff00::/8) — always denied.
	if ip16.IsMulticast() {
		return fmt.Errorf("upstream resolves to multicast address %s", ip)
	}

	// Interface-local multicast — always denied.
	if ip16.IsInterfaceLocalMulticast() {
		return fmt.Errorf("upstream resolves to interface-local multicast %s", ip)
	}

	// Private ranges — denied unless AllowPrivate (T-01, T-02).
	if !p.AllowPrivate {
		if isPrivate(ip16) {
			return fmt.Errorf("upstream resolves to private address %s (enable AllowPrivate for local-dev)", ip)
		}
	}

	// Documentation/TEST-NET ranges — always denied (T-02).
	if isDocumentation(ip16) {
		return fmt.Errorf("upstream resolves to documentation/reserved address %s", ip)
	}

	return nil
}

// isPrivate checks RFC-1918 private IPv4, unique-local IPv6, and
// IPv4-mapped equivalents within those ranges.
func isPrivate(ip net.IP) bool {
	// Use Go 1.17+ IsPrivate which covers 10/8, 172.16/12, 192.168/16,
	// and fc00::/7 for IPv6.
	return ip.IsPrivate()
}

// isDocumentation checks documentation/TEST-NET ranges that should never
// be reachable upstream (192.0.2.0/24, 198.51.100.0/24, 203.0.113.0/24,
// 100.64.0.0/10 Carrier-Grade NAT, 192.88.99.0/24 6to4 relay).
func isDocumentation(ip net.IP) bool {
	for _, cidr := range documentationCIDRs {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

// documentationCIDRs are pre-parsed at init time.
var documentationCIDRs []*net.IPNet

func init() {
	for _, cidr := range []string{
		"192.0.2.0/24",    // TEST-NET-1 (RFC 5737)
		"198.51.100.0/24", // TEST-NET-2 (RFC 5737)
		"203.0.113.0/24",  // TEST-NET-3 (RFC 5737)
		"100.64.0.0/10",   // Carrier-Grade NAT (RFC 6598)
		"192.88.99.0/24",  // 6to4 relay (RFC 7526)
		"198.18.0.0/15",   // Benchmarking (RFC 2544)
		"240.0.0.0/4",     // Reserved (RFC 1112)
		"2001:db8::/32",   // Documentation (RFC 3849)
		"2002::/16",       // 6to4 (RFC 3056)
	} {
		_, network, err := net.ParseCIDR(cidr)
		if err == nil {
			documentationCIDRs = append(documentationCIDRs, network)
		}
	}
}
