package application

import (
	"crypto/tls"
	"net"
)

func tlsVersionName(v uint16) string {
	switch v {
	case tls.VersionTLS10:
		return "TLS1.0"
	case tls.VersionTLS11:
		return "TLS1.1"
	case tls.VersionTLS12:
		return "TLS1.2"
	case tls.VersionTLS13:
		return "TLS1.3"
	default:
		return "unknown"
	}
}

func ipAddressesToStrings(ips []net.IP) []string {
	if len(ips) == 0 {
		return nil
	}

	out := make([]string, 0, len(ips))
	for _, ip := range ips {
		out = append(out, ip.String())
	}

	return out
}
