package filterhost

import (
	"context"
	"net"
	"os"
	"strings"
	"time"

	"github.com/openSUSE/piirplug/filter"
	"google.golang.org/adk/v2/plugin"
)

// lookupTimeout bounds all name lookups done while building the name list.
const lookupTimeout = 2 * time.Second

type HostPlugin struct {
	*filter.UniqueNamesPlugin
	domain   string
	resolver *net.Resolver
	lookupFn func(domain string) ([]string, error)
}

type HostPluginOption func(*HostPlugin)

// WithReplacement sets a prefilled replacement table for HostPlugin.
func WithReplacement(replacements *map[string]string) HostPluginOption {
	return func(p *HostPlugin) {
		if replacements != nil {
			p.Replacements = replacements
		}
	}
}

// WithDomain sets the local domain manually instead of auto-detecting.
func WithDomain(domain string) HostPluginOption {
	return func(p *HostPlugin) {
		p.domain = domain
	}
}

// WithDNSServer sets the DNS nameserver manually instead of using the system
// resolver configuration. The port defaults to 53 if none is given.
func WithDNSServer(server string) HostPluginOption {
	return func(p *HostPlugin) {
		if server == "" {
			return
		}
		addr := server
		if _, _, err := net.SplitHostPort(addr); err != nil {
			addr = net.JoinHostPort(server, "53")
		}
		p.resolver = &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, network, addr)
			},
		}
	}
}

// WithResolver sets the resolver used for all name lookups.
func WithResolver(resolver *net.Resolver) HostPluginOption {
	return func(p *HostPlugin) {
		if resolver != nil {
			p.resolver = resolver
		}
	}
}

// WithLookupFunc configures a custom function to collect the host names of the
// domain (useful for tests).
func WithLookupFunc(fn func(domain string) ([]string, error)) HostPluginOption {
	return func(p *HostPlugin) {
		p.lookupFn = fn
	}
}

// LookupHostNames collects the host names that can be discovered for the given
// domain with the standard library resolver. It queries the NS and MX records
// of the domain and resolves the addresses of the local interfaces back to
// names. Unlike a zone transfer this does not enumerate the whole zone, it only
// finds the hosts the resolver is willing to report.
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

// NewHostPlugin creates a new instance of the host filter plugin.
func NewHostPlugin(opts ...HostPluginOption) (*plugin.Plugin, error) {
	p := &HostPlugin{
		UniqueNamesPlugin: &filter.UniqueNamesPlugin{},
	}
	for _, opt := range opts {
		opt(p)
	}

	if p.Replacements == nil {
		m := make(map[string]string)
		p.Replacements = &m
	}

	if p.resolver == nil {
		p.resolver = net.DefaultResolver
	}

	if p.lookupFn == nil {
		p.lookupFn = func(domain string) ([]string, error) {
			return LookupHostNames(p.resolver, domain)
		}
	}

	hostname, _ := os.Hostname()

	if p.domain == "" && hostname != "" {
		p.domain = detectDomain(p.resolver, hostname)
	}

	var rawNames []string

	if names, err := p.lookupFn(p.domain); err == nil {
		rawNames = append(rawNames, names...)
	}

	if hostname != "" {
		rawNames = append(rawNames, hostname)
	}

	if p.domain != "" {
		rawNames = append(rawNames, p.domain)
	}

	rawNames = append(rawNames, parseEtcHosts()...)

	cleanedNames := cleanHostAndDomainNames(rawNames)

	if err := p.UniqueNamesPlugin.InitRegex(cleanedNames); err != nil {
		return nil, err
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
