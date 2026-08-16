package main

import (
	"os"
	"path/filepath"
	"testing"
)

// A DSN carries an & between sslmode and sslrootcert. Reading the file here
// instead of sourcing it in a shell is the whole point of the parser, so the
// value must arrive whole.
func TestLoadEnvFileKeepsFullDatabaseURL(t *testing.T) {
	t.Parallel()

	dsn := "postgresql://role:secret@host.invalid:5432/cumuru" +
		"?sslmode=verify-full&sslrootcert=/etc/ssl/certs/bundle.pem"
	lookup := writeEnv(t, "DATABASE_URL="+dsn+"\n")
	value, ok := lookup("DATABASE_URL")
	if !ok || value != dsn {
		t.Fatalf("DATABASE_URL = %q, %t; want the full DSN", value, ok)
	}
}

func TestLoadEnvFileMatchesComposeParsing(t *testing.T) {
	t.Parallel()

	content := "# comentário\n" +
		"\n" +
		"APP_ENV=production\n" +
		"  SPACED_KEY = spaced value \n" +
		"QUOTED=\"Administração Cumuru\"\n" +
		"SINGLE='valor'\n" +
		"UNBALANCED=\"aberta\n" +
		"NO_EQUALS\n"
	lookup := writeEnv(t, content)
	tests := map[string]string{
		"APP_ENV":    "production",
		"SPACED_KEY": "spaced value",
		"QUOTED":     "Administração Cumuru",
		"SINGLE":     "valor",
		"UNBALANCED": "\"aberta",
	}
	for key, want := range tests {
		if got, _ := lookup(key); got != want {
			t.Fatalf("%s = %q, want %q", key, got, want)
		}
	}
	if _, ok := lookup("NO_EQUALS"); ok {
		t.Fatal("a line without = must not become a variable")
	}
	if _, ok := lookup("# comentário"); ok {
		t.Fatal("a comment must not become a variable")
	}
}

func TestLoadEnvFileReportsMissingFile(t *testing.T) {
	t.Parallel()

	if _, err := loadEnvFile(filepath.Join(t.TempDir(), "absent.env")); err == nil {
		t.Fatal("loadEnvFile() error = nil, want unreadable")
	}
}

// Without an argument the check reads the process environment, which is what
// the container entrypoint relies on.
// Not parallel: t.Setenv mutates the process environment, which is exactly what
// this case reads back.
func TestResolveLookupDefaultsToTheProcessEnvironment(t *testing.T) {
	lookup, err := resolveLookup(nil)
	if err != nil {
		t.Fatalf("resolveLookup() error = %v", err)
	}
	t.Setenv("CUMURU_CONFIGCHECK_PROBE", "presente")
	if value, ok := lookup("CUMURU_CONFIGCHECK_PROBE"); !ok || value != "presente" {
		t.Fatalf("lookup = %q, %t; want the process environment", value, ok)
	}
}

func writeEnv(t *testing.T, content string) func(string) (string, bool) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "runtime.env")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	lookup, err := loadEnvFile(path)
	if err != nil {
		t.Fatalf("loadEnvFile() error = %v", err)
	}
	return lookup
}
