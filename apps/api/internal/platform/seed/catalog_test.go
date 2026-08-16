package seed

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validCatalog = `{
  "organization": {
    "id": "019fb000-0000-7000-8000-000000000001",
    "name": "Organização de exemplo",
    "accommodations": [
      {
        "id": "019fb001-0000-7000-8000-000000000001",
        "name": "Pousada de exemplo",
        "category": "formal_lodging",
        "capacity": 24,
        "public_area_code": "cumuruxatiba",
        "cadastur_id": "EXEMPLO"
      }
    ]
  }
}`

func TestLoadCatalogAcceptsAValidFile(t *testing.T) {
	t.Parallel()

	catalog, err := loadCatalog(writeCatalog(t, validCatalog))
	if err != nil {
		t.Fatalf("loadCatalog() error = %v", err)
	}
	if catalog.Name != "Organização de exemplo" {
		t.Fatalf("Name = %q", catalog.Name)
	}
	if len(catalog.Accommodations) != 1 {
		t.Fatalf("Accommodations = %d, want 1", len(catalog.Accommodations))
	}
	if catalog.Accommodations[0].Capacity == nil ||
		*catalog.Accommodations[0].Capacity != 24 {
		t.Fatalf("Capacity = %v, want 24", catalog.Accommodations[0].Capacity)
	}
}

func TestLoadCatalogRejectsMalformedInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name:    "unknown key",
			content: strings.Replace(validCatalog, `"name"`, `"nome"`, 1),
			wantErr: "malformed",
		},
		{
			name:    "organization id is not a uuid",
			content: strings.Replace(validCatalog, "019fb000-0000-7000-8000-000000000001", "primeira", 1),
			wantErr: "organization id",
		},
		{
			name:    "unknown category",
			content: strings.Replace(validCatalog, "formal_lodging", "hotel", 1),
			wantErr: "unknown category",
		},
		{
			name:    "non positive capacity",
			content: strings.Replace(validCatalog, "24", "0", 1),
			wantErr: "non positive capacity",
		},
		{
			name:    "no accommodation",
			content: `{"organization":{"id":"019fb000-0000-7000-8000-000000000001","name":"Vazia","accommodations":[]}}`,
			wantErr: "no accommodation",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := loadCatalog(writeCatalog(t, test.content))
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("loadCatalog() error = %v, want %q", err, test.wantErr)
			}
		})
	}
}

// A repeated identifier would apply the last entry and silently drop the
// earlier one, so the catalog is refused instead.
func TestLoadCatalogRejectsRepeatedIdentifier(t *testing.T) {
	t.Parallel()

	repeated := strings.Replace(
		validCatalog,
		`"cadastur_id": "EXEMPLO"
      }`,
		`"cadastur_id": "EXEMPLO"
      },
      {
        "id": "019fb001-0000-7000-8000-000000000001",
        "name": "Outra pousada",
        "category": "camping",
        "capacity": 10,
        "public_area_code": "cumuruxatiba",
        "cadastur_id": null
      }`,
		1,
	)
	_, err := loadCatalog(writeCatalog(t, repeated))
	if err == nil || !strings.Contains(err.Error(), "repeats accommodation") {
		t.Fatalf("loadCatalog() error = %v, want repeated identifier", err)
	}
}

// Reusing a fixture range silently overwrites a local-demo row and breaks the
// next demo run, so the catalog is refused while it is read.
func TestLoadCatalogRejectsReservedIdentifiers(t *testing.T) {
	t.Parallel()

	reserved := strings.Replace(
		validCatalog,
		"019fb001-0000-7000-8000-000000000001",
		"019fae11-0000-7000-8000-000000000001",
		1,
	)
	_, err := loadCatalog(writeCatalog(t, reserved))
	if err == nil || !strings.Contains(err.Error(), "reserved for the local-demo") {
		t.Fatalf("loadCatalog() error = %v, want reserved range", err)
	}
}

func TestLoadCatalogReportsMissingFile(t *testing.T) {
	t.Parallel()

	_, err := loadCatalog(filepath.Join(t.TempDir(), "absent.json"))
	if err == nil || !strings.Contains(err.Error(), "unreadable") {
		t.Fatalf("loadCatalog() error = %v, want unreadable", err)
	}
}

func writeCatalog(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "catalog.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}
