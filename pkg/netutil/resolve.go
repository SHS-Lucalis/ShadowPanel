package netutil

import (
	"context"
	"log/slog"
	"net"
	"time"
)

const (
	dnsResolveTimeout = 10 * time.Second
	wildcardAddress   = "0.0.0.0"
)

func ResolveBindAddress(ctx context.Context, host string) string {
	if host == "" || host == wildcardAddress || host == "::" {
		return host
	}

	if net.ParseIP(host) != nil {
		return host
	}

	resolveCtx, cancel := context.WithTimeout(ctx, dnsResolveTimeout)
	defer cancel()

	resolvedIPs, err := net.DefaultResolver.LookupHost(resolveCtx, host)
	if err != nil {
		slog.WarnContext(ctx, "Failed to resolve HTTP_HOST domain, using 0.0.0.0",
			slog.String("domain", host),
			slog.String("error", err.Error()),
		)

		return wildcardAddress
	}

	ifaceAddrs, err := net.InterfaceAddrs()
	if err != nil {
		slog.WarnContext(ctx, "Failed to get local interface addresses, using 0.0.0.0",
			slog.String("domain", host),
			slog.String("error", err.Error()),
		)

		return wildcardAddress
	}

	localIPs := make(map[string]struct{}, len(ifaceAddrs))
	for _, addr := range ifaceAddrs {
		if ipNet, ok := addr.(*net.IPNet); ok {
			localIPs[ipNet.IP.String()] = struct{}{}
		}
	}

	for _, ip := range resolvedIPs {
		if _, found := localIPs[ip]; found {
			slog.InfoContext(ctx, "Resolved HTTP_HOST domain to local IP",
				slog.String("domain", host),
				slog.String("ip", ip),
			)

			return ip
		}
	}

	slog.WarnContext(ctx, "Domain IP not found on local interfaces (possibly behind NAT), using 0.0.0.0",
		slog.String("domain", host),
		slog.Any("resolved_ips", resolvedIPs),
	)

	return "0.0.0.0"
}
