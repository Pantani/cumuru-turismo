// Package document validates the Brazilian registry number that identifies the
// person responsible for an organization, and reduces it to a blind HMAC.
//
// The plaintext never leaves this package: ADR-038 stores only the HMAC, so the
// application can answer "this document is already registered" and nothing else.
//
// No caller has been wired yet — the keyring that feeds Codec is already loaded
// as CoreConfig.DocumentKeys, but the onboarding surface that will collect the
// document is still pending. Reachability tooling will report this package as
// unreachable until then; that is expected, not leftover code.
package document

import (
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"strings"
)

// Kind selects which registry a document belongs to. Exactly one is accepted
// per organization: a company uses KindCNPJ, an individual host uses KindCPF.
type Kind string

const (
	KindCPF  Kind = "cpf"
	KindCNPJ Kind = "cnpj"
)

const (
	cpfDigits  = 11
	cnpjDigits = 14
)

// ErrInvalid rejects a malformed document without echoing the value, so the
// error never becomes an oracle for probing which numbers exist.
var ErrInvalid = errors.New("document: invalid")

var (
	cpfFirstWeights   = []int{10, 9, 8, 7, 6, 5, 4, 3, 2}
	cpfSecondWeights  = []int{11, 10, 9, 8, 7, 6, 5, 4, 3, 2}
	cnpjFirstWeights  = []int{5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}
	cnpjSecondWeights = []int{6, 5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}
)

// Document is a registry number already validated and reduced to its digits.
type Document struct {
	kind   Kind
	digits string
}

// Kind reports which registry validated this document.
func (d Document) Kind() Kind { return d.kind }

// String redacts the digits. Without it, any %v or %+v reaching a log line, a
// trace attribute or an error wrapper would print the plaintext that ADR-038
// promises is never recorded.
func (d Document) String() string { return string(d.kind) + ":[redacted]" }

// Parse accepts a raw document as typed by a human — with dots, slashes or
// dashes — and returns it validated. Formatting is discarded before validation
// so the same number always produces the same HMAC.
func Parse(kind Kind, raw string) (Document, error) {
	digits := onlyDigits(raw)
	if !validFor(kind, digits) {
		return Document{}, ErrInvalid
	}
	return Document{kind: kind, digits: digits}, nil
}

// rule describes one registry: how many digits it carries and the two weight
// vectors its check digits are derived from.
type rule struct {
	length        int
	firstWeights  []int
	secondWeights []int
}

var rules = map[Kind]rule{
	KindCPF:  {cpfDigits, cpfFirstWeights, cpfSecondWeights},
	KindCNPJ: {cnpjDigits, cnpjFirstWeights, cnpjSecondWeights},
}

// The length check runs before anything indexes the digits, so an unknown kind
// or a truncated value can never reach the arithmetic below.
func validFor(kind Kind, digits string) bool {
	applicable, known := rules[kind]
	if !known || len(digits) != applicable.length {
		return false
	}
	return !allSameDigit(digits) &&
		checkDigitsMatch(digits, applicable.firstWeights, applicable.secondWeights)
}

func onlyDigits(raw string) string {
	var builder strings.Builder
	for _, symbol := range raw {
		if symbol >= '0' && symbol <= '9' {
			builder.WriteRune(symbol)
		}
	}
	return builder.String()
}

// A repeated sequence such as 111.111.111-11 satisfies the check digits by
// accident, so it is rejected before they are computed.
func allSameDigit(digits string) bool {
	return strings.Count(digits, digits[:1]) == len(digits)
}

func checkDigitsMatch(digits string, first, second []int) bool {
	return digits[len(first)] == verifier(digits, first) &&
		digits[len(second)] == verifier(digits, second)
}

func verifier(digits string, weights []int) byte {
	sum := 0
	for index, weight := range weights {
		sum += int(digits[index]-'0') * weight
	}
	if remainder := sum % 11; remainder >= 2 {
		return byte('0' + 11 - remainder)
	}
	return '0'
}

// Codec turns a validated document into the blind value persisted in
// core.organizations.document_hmac.
type Codec struct {
	key []byte
}

// NewCodec rejects a short key: a CPF has only eleven digits, so an
// unkeyed or weakly keyed digest would fall to brute force in seconds.
func NewCodec(key []byte) (*Codec, error) {
	if len(key) < sha256.Size {
		return nil, ErrInvalid
	}
	return &Codec{key: append([]byte(nil), key...)}, nil
}

// HMAC derives the stored value. It is one-way by construction: nothing in the
// system can recover the digits from it.
func (c *Codec) HMAC(value Document) []byte {
	mac := hmac.New(sha256.New, c.key)
	mac.Write([]byte(value.kind))
	mac.Write([]byte(":"))
	mac.Write([]byte(value.digits))
	return mac.Sum(nil)
}
