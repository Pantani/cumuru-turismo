package config

import (
	"net/url"
	"strings"
)

// bareURL holds for a URL that carries no credentials, query or fragment: a
// capability is appended to the path and must not collide with anything.
func bareURL(value *url.URL) bool {
	return value != nil &&
		value.User == nil &&
		value.RawQuery == "" &&
		value.Fragment == ""
}

func validateInviteURL(value *url.URL, requireHTTPS bool) error {
	if !bareURL(value) {
		return invalid("INVITE_BASE_URL")
	}
	if requireHTTPS && value.Scheme != "https" {
		return invalid("INVITE_BASE_URL")
	}
	return nil
}

func validateOrigins(origins []string, requireHTTPS bool) error {
	if len(origins) == 0 {
		return invalid("CORS_ALLOWED_ORIGINS")
	}
	for _, origin := range origins {
		if err := validateOrigin(origin, requireHTTPS); err != nil {
			return err
		}
	}
	return nil
}

// An allowed origin is scheme, host and port only; a path would never match a
// browser Origin header.
func originOnly(parsed *url.URL) bool {
	return bareURL(parsed) && parsed.Path == ""
}

func validateOrigin(origin string, requireHTTPS bool) error {
	parsed, err := parseAbsoluteURL("CORS_ALLOWED_ORIGINS", origin)
	if err != nil || !originOnly(parsed) {
		return invalid("CORS_ALLOWED_ORIGINS")
	}
	if requireHTTPS && parsed.Scheme != "https" {
		return invalid("CORS_ALLOWED_ORIGINS")
	}
	return nil
}

func parseAbsoluteURL(field, value string) (*url.URL, error) {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, invalid(field)
	}
	return parsed, nil
}

// splitList reads the comma-separated form every list-valued variable uses,
// dropping blanks so a trailing comma is not a silent empty entry.
func splitList(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}
