package filterhost

import (
	"context"
	"net"
	"os"
	"strings"
	"time"

	"github.com/openSUSE/piiplugin/filter"
	"google.golang.org/adk/v2/plugin"
)

// lookupTimeout bounds all name lookups done while building the name list.
const lookupTimeout = 2 * time.Second

// HostFilter is the pure Go non-ADK filter engine.
type HostFilter struct {
	*filter.UniqueNamesFilter
	domain   string
	resolver *net.Resolver
	lookupFn func(domain string) ([]string, error)
}

// Option defines the functional option type for HostFilter.
type Option func(*HostFilter)

// HostPluginOption is a type alias for Option to maintain backward compatibility.
type HostPluginOption = Option

// WithReplacement sets a prefilled replacement table for HostFilter.
func WithReplacement(replacements *map[string]string) Option {
	return func(h *HostFilter) {
		if replacements != nil {
			if h.UniqueNamesFilter == nil {
				h.UniqueNamesFilter = &filter.UniqueNamesFilter{}
			}
			h.Replacements = replacements
		}
	}
}

// WithDomain sets the local domain manually instead of auto-detecting.
func WithDomain(domain string) Option {
	return func(h *HostFilter) {
		h.domain = domain
	}
}

// WithDNSServer sets the DNS nameserver manually instead of using the system
// resolver configuration. The port defaults to 53 if none is given.
func WithDNSServer(server string) Option {
	return func(h *HostFilter) {
		if server == "" {
			return
		}
		addr := server
		if _, _, err := net.SplitHostPort(addr); err != nil {
			addr = net.JoinHostPort(server, "53")
		}
		h.resolver = &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, network, addr)
			},
		}
	}
}

// WithResolver sets the resolver used for all name lookups.
func WithResolver(resolver *net.Resolver) Option {
	return func(h *HostFilter) {
		if resolver != nil {
			h.resolver = resolver
		}
	}
}

// WithLookupFunc configures a custom function to collect the host names of the
// domain (useful for tests).
func WithLookupFunc(fn func(domain string) ([]string, error)) Option {
	return func(h *HostFilter) {
		h.lookupFn = fn
	}
}

// LookupHostNames collects the host names that can be discovered for the given
// domain with the standard library resolver. It queries the NS and MX records
// of the domain and resolves the addresses of the local interfaces back to
// names.
func LookupHostNames(resolver *net.Resolver, domain string) ([]string, error) {
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	ctx, cancel := context.WithTimeout(context.Background(), lookupTimeout)
	defer cancel()

	var names []string
	if domain != "" {
		if nameservers, err := resolver.LookupNS(ctx, domain); err == nil {
			for _, ns := range nameservers {
				names = append(names, ns.Host)
			}
		}
		if mailservers, err := resolver.LookupMX(ctx, domain); err == nil {
			for _, mx := range mailservers {
				names = append(names, mx.Host)
			}
		}
	}
	names = append(names, lookupLocalNames(ctx, resolver)...)
	return names, nil
}

// lookupLocalNames resolves the addresses of all local interfaces back to their
// names.
func lookupLocalNames(ctx context.Context, resolver *net.Resolver) []string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	var names []string
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok || ipNet.IP.IsLoopback() || ipNet.IP.IsLinkLocalUnicast() {
			continue
		}
		found, err := resolver.LookupAddr(ctx, ipNet.IP.String())
		if err != nil {
			continue
		}
		names = append(names, found...)
	}
	return names
}

// detectDomain determines the local domain from the hostname, falling back to
// the canonical name reported by the resolver.
func detectDomain(resolver *net.Resolver, hostname string) string {
	if _, domain, found := strings.Cut(hostname, "."); found {
		return domain
	}
	ctx, cancel := context.WithTimeout(context.Background(), lookupTimeout)
	defer cancel()

	fqdn, err := resolver.LookupCNAME(ctx, hostname)
	if err != nil {
		return ""
	}
	if _, domain, found := strings.Cut(strings.TrimSuffix(fqdn, "."), "."); found {
		return domain
	}
	return ""
}

func parseEtcHosts() []string {
	var hostnames []string
	data, err := os.ReadFile("/etc/hosts")
	if err != nil {
		return nil
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		for _, field := range fields[1:] {
			if len(field) > 0 {
				hostnames = append(hostnames, field)
			}
		}
	}
	return hostnames
}

func cleanHostAndDomainNames(names []string) []string {
	var cleaned []string
	seen := make(map[string]bool)

	addName := func(name string) {
		name = strings.TrimSuffix(strings.TrimSpace(name), ".")
		if len(name) < 2 {
			return
		}
		if net.ParseIP(name) != nil {
			return
		}
		lower := strings.ToLower(name)
		if !seen[lower] {
			seen[lower] = true
			cleaned = append(cleaned, name)
		}
	}

	for _, name := range names {
		name = strings.TrimSuffix(strings.TrimSpace(name), ".")
		if net.ParseIP(name) != nil {
			continue
		}
		addName(name)
		if idx := strings.Index(name, "."); idx != -1 {
			short := name[:idx]
			domainPart := name[idx+1:]
			addName(short)
			addName(domainPart)
		}
	}
	return cleaned
}

// NewHostFilter creates a new instance of the decoupled HostFilter.
func NewHostFilter(opts ...Option) (*HostFilter, error) {
	h := &HostFilter{}
	for _, opt := range opts {
		opt(h)
	}

	if h.UniqueNamesFilter == nil {
		m := make(map[string]string)
		f, err := filter.NewUniqueNamesFilter(&m, nil)
		if err != nil {
			return nil, err
		}
		h.UniqueNamesFilter = f
	}

	if h.resolver == nil {
		h.resolver = net.DefaultResolver
	}

	if h.lookupFn == nil {
		h.lookupFn = func(domain string) ([]string, error) {
			return LookupHostNames(h.resolver, domain)
		}
	}

	hostname, _ := os.Hostname()

	if h.domain == "" && hostname != "" {
		h.domain = detectDomain(h.resolver, hostname)
	}

	var rawNames []string

	if names, err := h.lookupFn(h.domain); err == nil {
		rawNames = append(rawNames, names...)
	}

	if hostname != "" {
		rawNames = append(rawNames, hostname)
	}

	if h.domain != "" {
		rawNames = append(rawNames, h.domain)
	}

	rawNames = append(rawNames, parseEtcHosts()...)

	cleanedNames := cleanHostAndDomainNames(rawNames)

	if err := h.UniqueNamesFilter.InitRegex(cleanedNames); err != nil {
		return nil, err
	}

	return h, nil
}

// NewHostPlugin creates a new instance of the host filter plugin.
func NewHostPlugin(opts ...Option) (*plugin.Plugin, error) {
	f, err := NewHostFilter(opts...)
	if err != nil {
		return nil, err
	}
	p := &filter.UniqueNamesPlugin{
		UniqueNamesFilter: *f.UniqueNamesFilter,
	}
	return plugin.New(plugin.Config{
		Name:                 "host_plugin",
		BeforeModelCallback:  p.BeforeModelCallback,
		AfterModelCallback:   p.AfterModelCallback,
		OnModelErrorCallback: p.OnModelErrorCallback,
		BeforeToolCallback:   p.BeforeToolCallback,
		AfterToolCallback:    p.AfterToolCallback,
		OnToolErrorCallback:  p.OnToolErrorCallback,
	})
}
