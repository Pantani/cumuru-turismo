package config

import (
	"net/netip"
	"strings"
)

func loadTrustedProxyCIDRs(process Process, lookup LookupEnv) ([]netip.Prefix, error) {
	if process != ProcessAPI {
		return nil, nil
	}
	return parseTrustedProxyCIDRs(required(lookup, "TRUSTED_PROXY_CIDRS"))
}

func parseTrustedProxyCIDRs(value string) ([]netip.Prefix, error) {
	entries := strings.Split(value, ",")
	prefixes := make([]netip.Prefix, 0, len(entries))
	for _, entry := range entries {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(entry))
		if err != nil {
			return nil, invalid("TRUSTED_PROXY_CIDRS")
		}
		prefix, ok := normalizeProxyPrefix(prefix)
		if !ok {
			return nil, invalid("TRUSTED_PROXY_CIDRS")
		}
		prefixes = append(prefixes, prefix)
	}
	return prefixes, nil
}

func normalizeProxyPrefix(prefix netip.Prefix) (netip.Prefix, bool) {
	address := prefix.Addr()
	if address.Zone() != "" {
		return netip.Prefix{}, false
	}
	if !address.Is4In6() {
		return prefix.Masked(), true
	}
	bits := prefix.Bits() - 96
	if bits < 0 {
		return netip.Prefix{}, false
	}
	return netip.PrefixFrom(address.Unmap(), bits).Masked(), true
}
