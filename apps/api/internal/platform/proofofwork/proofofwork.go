// Package proofofwork issues and verifies the second proof of the open
// self-registration channel.
//
// It is a toll, not a bot defence, and the difference matters for how much
// weight the rest of the feature may put on it. A low-cost Android phone running
// JavaScript is three to eight orders of magnitude slower than a GPU, so the
// asymmetry runs against the defender. What it does remove is the cost-free
// abuse: the ten-line script and the bare curl loop. What actually protects
// data subjects here is, in order, human approval, the purge on rejection and
// expiry, and the presence filter.
//
// No CAPTCHA and no third party: the public form must not depend on an external
// network, and shipping every guest's IP and user agent to a foreign provider
// would be an international transfer of personal data by a municipal system.
// No cookie either, which is what keeps the channel free of a persistent
// identifier on the guest's device.
package proofofwork

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"
)

// ErrRejected is the single answer for a forged MAC, an expired challenge, a
// challenge minted for another accommodation, an insufficient solution and a
// replay. A caller that distinguished them would hand a probe a map of the
// verifier.
var ErrRejected = errors.New("proof of work rejected")

const (
	// Algorithm is the value the contract pins, and the client mirrors it.
	Algorithm = "sha256-leading-zero-bits"

	MinDifficultyBits = 1
	MaxDifficultyBits = 32

	nonceSize     = 16
	issuedAtSize  = 8
	difficultSize = 1
	inviteIDSize  = 16
	macSize       = sha256.Size
	payloadSize   = nonceSize + issuedAtSize + difficultSize + inviteIDSize

	macDomain   = "cumuru-pow-v1"
	spendDomain = "cumuru-pow-spend-v1"

	maxSolutionLength = 64
)

// Keyring holds every key a challenge may have been minted under. Verification
// walks the whole ring so a key rotation does not refuse the guest who fetched
// the form a minute earlier; the TTL bounds the window anyway.
type Keyring struct {
	CurrentVersion string
	Keys           map[string][]byte
}

type Issuer struct {
	currentVersion string
	keys           map[string][]byte
	ttl            time.Duration
}

func NewIssuer(keyring Keyring, ttl time.Duration) (*Issuer, error) {
	if ttl <= 0 {
		return nil, ErrRejected
	}
	keys, err := cloneKeys(keyring.Keys)
	if err != nil {
		return nil, err
	}
	if _, ok := keys[keyring.CurrentVersion]; !ok {
		return nil, ErrRejected
	}
	return &Issuer{currentVersion: keyring.CurrentVersion, keys: keys, ttl: ttl}, nil
}

func cloneKeys(source map[string][]byte) (map[string][]byte, error) {
	keys := make(map[string][]byte, len(source))
	for version, material := range source {
		if len(material) < 32 {
			return nil, ErrRejected
		}
		keys[version] = append([]byte(nil), material...)
	}
	return keys, nil
}

// Challenge is what the client receives. It carries no IP, no prefix, no user
// agent and no data about the subject: binding the challenge to a network
// prefix would break the guest who switches from Wi-Fi to mobile data halfway
// through the form, and would buy nothing, because the attacker controls both
// ends anyway.
type Challenge struct {
	Algorithm      string
	Value          string
	DifficultyBits uint8
	ExpiresAt      time.Time
}

// Issue mints a stateless challenge. The difficulty travels inside the MAC, so
// the client cannot lower it; the invite identifier travels inside it too, so a
// farm built for one poster cannot serve another.
func (i *Issuer) Issue(
	inviteID uuid.UUID,
	difficultyBits uint8,
	now time.Time,
) (Challenge, error) {
	if inviteID == uuid.Nil || !validDifficulty(difficultyBits) {
		return Challenge{}, ErrRejected
	}
	nonce := make([]byte, nonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return Challenge{}, ErrRejected
	}
	payload := encodePayload(nonce, now, difficultyBits, inviteID)
	key := i.keys[i.currentVersion]
	value := append(payload, challengeMAC(key, payload)...)
	return Challenge{
		Algorithm:      Algorithm,
		Value:          base64.RawURLEncoding.EncodeToString(value),
		DifficultyBits: difficultyBits,
		ExpiresAt:      now.Add(i.ttl).UTC(),
	}, nil
}

// Spend is what the caller must record before the submission is allowed to have
// any effect. Without the nonce book the same solution is replayed for the
// whole TTL and the control is worth zero.
type Spend struct {
	ChallengeHMAC []byte
	KeyVersion    string
	ExpiresAt     time.Time
}

// Verify recomputes the MAC, checks the lifetime, checks the binding to this
// poster and checks the work, in that order, and every failure answers with the
// same ErrRejected.
func (i *Issuer) Verify(
	challenge string,
	solution string,
	inviteID uuid.UUID,
	now time.Time,
) (Spend, error) {
	decoded, version, err := i.authenticate(challenge)
	if err != nil {
		return Spend{}, err
	}
	if err := decoded.usable(inviteID, now, i.ttl); err != nil {
		return Spend{}, err
	}
	if !solved(challenge, solution, decoded.difficultyBits) {
		return Spend{}, ErrRejected
	}
	return Spend{
		ChallengeHMAC: spendDigest(i.keys[version], decoded.nonce),
		KeyVersion:    version,
		ExpiresAt:     decoded.issuedAt.Add(i.ttl).UTC(),
	}, nil
}

type decodedChallenge struct {
	nonce          []byte
	issuedAt       time.Time
	difficultyBits uint8
	inviteID       uuid.UUID
}

func (d decodedChallenge) usable(inviteID uuid.UUID, now time.Time, ttl time.Duration) error {
	if d.inviteID != inviteID || !validDifficulty(d.difficultyBits) {
		return ErrRejected
	}
	if now.Before(d.issuedAt) || !now.Before(d.issuedAt.Add(ttl)) {
		return ErrRejected
	}
	return nil
}

// authenticate walks the ring in a deterministic order so two processes with
// the same keyring agree on which version signed a challenge.
func (i *Issuer) authenticate(challenge string) (decodedChallenge, string, error) {
	value, err := base64.RawURLEncoding.DecodeString(challenge)
	if err != nil || len(value) != payloadSize+macSize {
		return decodedChallenge{}, "", ErrRejected
	}
	payload, mac := value[:payloadSize], value[payloadSize:]
	for _, version := range i.versions() {
		if hmac.Equal(mac, challengeMAC(i.keys[version], payload)) {
			return decodePayload(payload), version, nil
		}
	}
	return decodedChallenge{}, "", ErrRejected
}

func (i *Issuer) versions() []string {
	versions := make([]string, 0, len(i.keys))
	for version := range i.keys {
		versions = append(versions, version)
	}
	sort.Strings(versions)
	return versions
}

func encodePayload(
	nonce []byte,
	issuedAt time.Time,
	difficultyBits uint8,
	inviteID uuid.UUID,
) []byte {
	payload := make([]byte, 0, payloadSize)
	payload = append(payload, nonce...)
	payload = binary.BigEndian.AppendUint64(payload, uint64(issuedAt.UTC().Unix()))
	payload = append(payload, difficultyBits)
	return append(payload, inviteID[:]...)
}

func decodePayload(payload []byte) decodedChallenge {
	seconds := binary.BigEndian.Uint64(payload[nonceSize : nonceSize+issuedAtSize])
	offset := nonceSize + issuedAtSize
	return decodedChallenge{
		nonce:          append([]byte(nil), payload[:nonceSize]...),
		issuedAt:       time.Unix(int64(seconds), 0).UTC(),
		difficultyBits: payload[offset],
		inviteID:       uuid.UUID(payload[offset+difficultSize:]),
	}
}

func challengeMAC(key, payload []byte) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(macDomain))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write(payload)
	return mac.Sum(nil)
}

// spendDigest keys the nonce so the book carries no reversible value. The table
// holds nothing else: no subject, no IP, no token, no data about the guest.
func spendDigest(key, nonce []byte) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(spendDomain))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write(nonce)
	return mac.Sum(nil)
}

// solved mirrors the browser exactly: SHA-256 over the concatenated UTF-8 of
// challenge and solution, difficulty counted in leading zero bits from the most
// significant bit. This is a producer and consumer boundary, and a disagreement
// of one bit would make every legitimate submission fail.
func solved(challenge, solution string, difficultyBits uint8) bool {
	if solution == "" || len(solution) > maxSolutionLength {
		return false
	}
	digest := sha256.Sum256([]byte(challenge + solution))
	return leadingZeroBits(digest[:]) >= int(difficultyBits)
}

func leadingZeroBits(digest []byte) int {
	bits := 0
	for _, value := range digest {
		if value != 0 {
			return bits + leadingZeroBitsInByte(value)
		}
		bits += 8
	}
	return bits
}

func leadingZeroBitsInByte(value byte) int {
	bits := 0
	for mask := byte(0x80); mask != 0 && value&mask == 0; mask >>= 1 {
		bits++
	}
	return bits
}

func validDifficulty(bits uint8) bool {
	return bits >= MinDifficultyBits && bits <= MaxDifficultyBits
}
