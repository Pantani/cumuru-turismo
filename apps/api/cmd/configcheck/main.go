// configcheck loads an environment through the real production loaders and
// exits non-zero on the first rejection. It opens no socket and reaches no
// database, so a deployment can prove its runtime.env before anything starts
// and before a bad value becomes a failed rollout.
//
// With a path argument it reads that file directly instead of the process
// environment. Reading it here rather than sourcing it in a shell is what makes
// the check trustworthy: a shell would split a DSN on the & that separates
// sslmode from sslrootcert and validate a truncated value.
package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/Pantani/cumuru/apps/api/internal/platform/config"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	lookup, err := resolveLookup(os.Args[1:])
	if err == nil {
		err = check(lookup)
	}
	if err != nil {
		logger.Error(
			"configuration rejected",
			"error_code",
			"config_invalid",
			"reason",
			err.Error(),
		)
		os.Exit(1)
	}
	fmt.Println("CONFIG_CHECK=PASS")
}

func resolveLookup(arguments []string) (config.LookupEnv, error) {
	if len(arguments) == 0 {
		return os.LookupEnv, nil
	}
	return loadEnvFile(arguments[0])
}

// The seeder is checked alongside the two runtime processes because it reads
// the same file: a profile the environment refuses has to surface here, not
// when the seeder already holds the provisioning role.
func check(lookup config.LookupEnv) error {
	checks := []struct {
		name string
		load func() error
	}{
		{"api", func() error { return loadProcess(config.ProcessAPI, lookup) }},
		{"worker", func() error { return loadProcess(config.ProcessWorker, lookup) }},
		{"seed", func() error { return loadSeed(lookup) }},
	}
	for _, current := range checks {
		if err := current.load(); err != nil {
			return fmt.Errorf("%s: %w", current.name, err)
		}
	}
	return nil
}

func loadProcess(process config.Process, lookup config.LookupEnv) error {
	_, err := config.Load(process, lookup)
	return err
}

func loadSeed(lookup config.LookupEnv) error {
	_, err := config.LoadSeed(lookup)
	return err
}

func loadEnvFile(path string) (config.LookupEnv, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("environment file is unreadable: %w", err)
	}
	values := make(map[string]string)
	for _, line := range strings.Split(string(content), "\n") {
		key, value, ok := parseEnvLine(line)
		if ok {
			values[key] = value
		}
	}
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}, nil
}

func parseEnvLine(line string) (string, string, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return "", "", false
	}
	key, value, found := strings.Cut(trimmed, "=")
	if !found {
		return "", "", false
	}
	return strings.TrimSpace(key), unquote(strings.TrimSpace(value)), true
}

// Compose strips one layer of surrounding quotes from an env_file value, so the
// check has to strip it too; otherwise a quoted display name would validate
// here and reach the container with the quotes attached.
func unquote(value string) string {
	if len(value) < 2 {
		return value
	}
	first := value[0]
	if first != '"' && first != '\'' {
		return value
	}
	if value[len(value)-1] != first {
		return value
	}
	return value[1 : len(value)-1]
}
