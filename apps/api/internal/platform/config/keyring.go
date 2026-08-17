package config

import (
	"bytes"
	"encoding/base64"
	"regexp"
	"strings"
)

const minimumHMACKeyBytes = 32

var keyVersionPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// KeyringConfig is a versioned set of HMAC keys. Every capability domain owns
// its own ring so that rotating or leaking one never reaches another.
type KeyringConfig struct {
	CurrentVersion string
	Keys           map[string][]byte
}

func (k KeyringConfig) Key(version string) ([]byte, bool) {
	key, ok := k.Keys[version]
	if !ok {
		return nil, false
	}
	return append([]byte(nil), key...), true
}

// coreKeyringSpecs lists every ring the core slice owns, in the order loadCore
// assigns them. The document ring is deliberately separate from every other
// one: ADR-038 stores a CPF blind, and sharing a key with invites or cursors
// would let a leak in one of them reverse the other by comparison.
var coreKeyringSpecs = [6]struct {
	versionField string
	keysField    string
}{
	{"INVITE_HMAC_CURRENT_VERSION", "INVITE_HMAC_KEYS"},
	{"ACTOR_HMAC_CURRENT_VERSION", "ACTOR_HMAC_KEYS"},
	{"IDEMPOTENCY_HMAC_CURRENT_VERSION", "IDEMPOTENCY_HMAC_KEYS"},
	{"RATE_LIMIT_HMAC_CURRENT_VERSION", "RATE_LIMIT_HMAC_KEYS"},
	{"CURSOR_HMAC_CURRENT_VERSION", "CURSOR_HMAC_KEYS"},
	{"DOCUMENT_HMAC_CURRENT_VERSION", "DOCUMENT_HMAC_KEYS"},
}

func loadKeyrings(lookup LookupEnv) ([6]KeyringConfig, error) {
	var result [6]KeyringConfig
	for index, spec := range coreKeyringSpecs {
		keyring, err := parseKeyring(
			spec.versionField,
			required(lookup, spec.versionField),
			spec.keysField,
			required(lookup, spec.keysField),
		)
		if err != nil {
			return [6]KeyringConfig{}, err
		}
		result[index] = keyring
	}
	if keyringsOverlap(result[:]) {
		return [6]KeyringConfig{}, invalid("HMAC_KEYRINGS")
	}
	return result, nil
}

func parseKeyring(versionField, currentVersion, keysField, encoded string) (KeyringConfig, error) {
	if !keyVersionPattern.MatchString(currentVersion) {
		return KeyringConfig{}, invalid(versionField)
	}
	keys, err := parseKeyEntries(encoded, keysField)
	if err != nil {
		return KeyringConfig{}, err
	}
	if _, ok := keys[currentVersion]; !ok {
		return KeyringConfig{}, invalid(versionField)
	}
	return KeyringConfig{CurrentVersion: currentVersion, Keys: keys}, nil
}

func parseKeyEntries(encoded, field string) (map[string][]byte, error) {
	keys := make(map[string][]byte)
	for _, entry := range splitList(encoded) {
		version, key, err := parseKeyEntry(entry, field)
		if err != nil {
			return nil, err
		}
		if _, duplicate := keys[version]; duplicate {
			return nil, invalid(field)
		}
		keys[version] = key
	}
	return keys, nil
}

func parseKeyEntry(entry, field string) (string, []byte, error) {
	version, value, ok := strings.Cut(entry, "=")
	if !ok || !keyVersionPattern.MatchString(version) {
		return "", nil, invalid(field)
	}
	key, err := base64.StdEncoding.DecodeString(value)
	if err != nil || len(key) < minimumHMACKeyBytes {
		return "", nil, invalid(field)
	}
	return version, key, nil
}

func keyringsOverlap(keyrings []KeyringConfig) bool {
	seen := make([][]byte, 0)
	for _, keyring := range keyrings {
		for _, key := range keyring.Keys {
			if containsKey(seen, key) {
				return true
			}
			seen = append(seen, key)
		}
	}
	return false
}

func containsKey(keys [][]byte, candidate []byte) bool {
	for _, key := range keys {
		if bytes.Equal(key, candidate) {
			return true
		}
	}
	return false
}
