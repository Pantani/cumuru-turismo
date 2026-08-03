package httpapi

import (
	"errors"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

var errInvalidClientAddress = errors.New("invalid client address")

func (d Dependencies) inviteCORS(next http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(d.CORSAllowedOrigins))
	for _, origin := range d.CORSAllowedOrigins {
		allowed[origin] = struct{}{}
	}
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		origin := request.Header.Get("Origin")
		if origin == "" {
			next.ServeHTTP(writer, request)
			return
		}
		if _, ok := allowed[origin]; !ok {
			writeProblem(writer, request, http.StatusForbidden, "origin-forbidden", "Origem não permitida")
			return
		}
		writer.Header().Set("Access-Control-Allow-Origin", origin)
		writer.Header().Set("Vary", "Origin")
		if request.Method == http.MethodOptions {
			writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			writer.Header().Set(
				"Access-Control-Allow-Headers",
				"Content-Type, Idempotency-Key, Survey-Capability, X-Request-ID",
			)
			writer.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(writer, request)
	})
}

func rateSubject(request *http.Request, trustedProxies []netip.Prefix) (string, error) {
	address, err := clientAddress(request, trustedProxies)
	if err != nil {
		return "", err
	}
	return generalizedAddress(address), nil
}

func clientAddress(
	request *http.Request,
	trustedProxies []netip.Prefix,
) (netip.Addr, error) {
	address, err := remoteAddress(request)
	if err != nil {
		return netip.Addr{}, err
	}
	if proxyContains(trustedProxies, address) {
		return forwardedAddress(request)
	}
	return address, nil
}

func remoteAddress(request *http.Request) (netip.Addr, error) {
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err != nil {
		host = strings.TrimSpace(request.RemoteAddr)
	}
	return parseAddress(host)
}

func forwardedAddress(request *http.Request) (netip.Addr, error) {
	forwarded, hasForwarded, err := proxyHeaderAddress(request, "X-Forwarded-For")
	if err != nil {
		return netip.Addr{}, errInvalidClientAddress
	}
	realIP, hasRealIP, err := proxyHeaderAddress(request, "X-Real-IP")
	if err != nil {
		return netip.Addr{}, errInvalidClientAddress
	}
	if !hasForwarded && !hasRealIP {
		return netip.Addr{}, errInvalidClientAddress
	}
	if hasForwarded && hasRealIP && forwarded != realIP {
		return netip.Addr{}, errInvalidClientAddress
	}
	if hasForwarded {
		return forwarded, nil
	}
	return realIP, nil
}

func proxyHeaderAddress(
	request *http.Request,
	name string,
) (netip.Addr, bool, error) {
	values := request.Header.Values(name)
	if len(values) == 0 {
		return netip.Addr{}, false, nil
	}
	if len(values) != 1 || strings.Contains(values[0], ",") {
		return netip.Addr{}, false, errInvalidClientAddress
	}
	address, err := parseAddress(values[0])
	if err != nil {
		return netip.Addr{}, false, err
	}
	return address, true, nil
}

func parseAddress(value string) (netip.Addr, error) {
	address, err := netip.ParseAddr(strings.TrimSpace(value))
	if err != nil || address.Zone() != "" {
		return netip.Addr{}, errInvalidClientAddress
	}
	return address.Unmap(), nil
}

func proxyContains(prefixes []netip.Prefix, address netip.Addr) bool {
	for _, prefix := range prefixes {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}

func generalizedAddress(address netip.Addr) string {
	bits := 56
	if address.Is4() {
		bits = 24
	}
	return netip.PrefixFrom(address, bits).Masked().String()
}

func (d Dependencies) requestRateSubject(
	writer http.ResponseWriter,
	request *http.Request,
) (string, bool) {
	subject, err := rateSubject(request, d.TrustedProxyCIDRs)
	if err != nil {
		writeProblem(writer, request, http.StatusBadRequest, "invalid-request", "Requisição inválida")
		return "", false
	}
	return subject, true
}
