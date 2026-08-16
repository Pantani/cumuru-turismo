package document_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Pantani/cumuru/apps/api/internal/document"
)

// Synthetic numbers: structurally valid check digits, not issued to anyone.
const (
	validCPF  = "52998224725"
	validCNPJ = "11222333000181"
)

func testKey() []byte {
	return bytes.Repeat([]byte("k"), 32)
}

func TestParseAccepts(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		kind document.Kind
		raw  string
	}{
		"cpf bare":       {document.KindCPF, validCPF},
		"cpf formatted":  {document.KindCPF, "529.982.247-25"},
		"cpf spaced":     {document.KindCPF, " 529 982 247 25 "},
		"cnpj bare":      {document.KindCNPJ, validCNPJ},
		"cnpj formatted": {document.KindCNPJ, "11.222.333/0001-81"},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			parsed, err := document.Parse(testCase.kind, testCase.raw)
			if err != nil {
				t.Fatalf("Parse(%s) returned %v, want nil", name, err)
			}
			if parsed.Kind() != testCase.kind {
				t.Fatalf("Kind() = %q, want %q", parsed.Kind(), testCase.kind)
			}
		})
	}
}

func TestParseRejects(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		kind document.Kind
		raw  string
	}{
		"empty":             {document.KindCPF, ""},
		"cpf wrong check":   {document.KindCPF, "52998224724"},
		"cpf repeated":      {document.KindCPF, "11111111111"},
		"cpf zeros":         {document.KindCPF, "00000000000"},
		"cpf too short":     {document.KindCPF, "5299822472"},
		"cpf too long":      {document.KindCPF, "529982247250"},
		"cpf letters only":  {document.KindCPF, "abcdefghijk"},
		"cnpj wrong check":  {document.KindCNPJ, "11222333000182"},
		"cnpj repeated":     {document.KindCNPJ, "11111111111111"},
		"cnpj too short":    {document.KindCNPJ, "1122233300018"},
		"cnpj value as cpf": {document.KindCPF, validCNPJ},
		"cpf value as cnpj": {document.KindCNPJ, validCPF},
		"unknown kind":      {document.Kind("rg"), validCPF},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := document.Parse(testCase.kind, testCase.raw); err == nil {
				t.Fatalf("Parse(%s) returned nil error, want rejection", name)
			}
		})
	}
}

// Formatting must not change the stored value, otherwise the same person typing
// dots one day and none the next would defeat the uniqueness index.
func TestHMACIgnoresFormatting(t *testing.T) {
	t.Parallel()
	codec, err := document.NewCodec(testKey())
	if err != nil {
		t.Fatalf("NewCodec returned %v, want nil", err)
	}
	bare, err := document.Parse(document.KindCPF, validCPF)
	if err != nil {
		t.Fatalf("Parse bare returned %v, want nil", err)
	}
	formatted, err := document.Parse(document.KindCPF, "529.982.247-25")
	if err != nil {
		t.Fatalf("Parse formatted returned %v, want nil", err)
	}
	if !bytes.Equal(codec.HMAC(bare), codec.HMAC(formatted)) {
		t.Fatal("HMAC differs between bare and formatted input, want equal")
	}
}

func TestHMACSeparatesKinds(t *testing.T) {
	t.Parallel()
	codec, err := document.NewCodec(testKey())
	if err != nil {
		t.Fatalf("NewCodec returned %v, want nil", err)
	}
	cpf, err := document.Parse(document.KindCPF, validCPF)
	if err != nil {
		t.Fatalf("Parse cpf returned %v, want nil", err)
	}
	cnpj, err := document.Parse(document.KindCNPJ, validCNPJ)
	if err != nil {
		t.Fatalf("Parse cnpj returned %v, want nil", err)
	}
	if bytes.Equal(codec.HMAC(cpf), codec.HMAC(cnpj)) {
		t.Fatal("HMAC collides across kinds, want distinct values")
	}
}

func TestHMACDependsOnKey(t *testing.T) {
	t.Parallel()
	first, err := document.NewCodec(testKey())
	if err != nil {
		t.Fatalf("NewCodec first returned %v, want nil", err)
	}
	second, err := document.NewCodec(bytes.Repeat([]byte("z"), 32))
	if err != nil {
		t.Fatalf("NewCodec second returned %v, want nil", err)
	}
	parsed, err := document.Parse(document.KindCPF, validCPF)
	if err != nil {
		t.Fatalf("Parse returned %v, want nil", err)
	}
	if bytes.Equal(first.HMAC(parsed), second.HMAC(parsed)) {
		t.Fatal("HMAC ignores the key, want key-dependent output")
	}
}

// The stored value must not leak the digits it was derived from.
func TestHMACDoesNotContainDigits(t *testing.T) {
	t.Parallel()
	codec, err := document.NewCodec(testKey())
	if err != nil {
		t.Fatalf("NewCodec returned %v, want nil", err)
	}
	parsed, err := document.Parse(document.KindCPF, validCPF)
	if err != nil {
		t.Fatalf("Parse returned %v, want nil", err)
	}
	if strings.Contains(string(codec.HMAC(parsed)), validCPF) {
		t.Fatal("HMAC embeds the plaintext document, want opaque output")
	}
}

func TestNewCodecRejectsShortKey(t *testing.T) {
	t.Parallel()
	if _, err := document.NewCodec(bytes.Repeat([]byte("k"), 31)); err == nil {
		t.Fatal("NewCodec accepted a 31-byte key, want rejection")
	}
}
