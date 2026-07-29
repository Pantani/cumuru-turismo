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
	address, err := remoteAddress(request)
	if err != nil {
		return "", err
	}
	if proxyContains(trustedProxies, address) {
		address, err = forwardedAddress(request)
		if err != nil {
			return "", err
		}
	}
	return generalizedAddress(address), nil
}

func remoteAddress(request *http.Request) (netip.Addr, error) {
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err != nil {
		host = strings.TrimSpace(request.RemoteAddr)
	}
	return parseAddress(host)
}

func forwardedAddress(request *http.Request) (netip.Addr, error) {
	values := request.Header.Values("X-Forwarded-For")
	if len(values) != 1 || strings.Contains(values[0], ",") {
		return netip.Addr{}, errInvalidClientAddress
	}
	return parseAddress(values[0])
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
