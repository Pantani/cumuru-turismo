package stay

import (
	"errors"
	"regexp"

	"github.com/Pantani/cumuru/apps/api/internal/access"
	"github.com/google/uuid"
)

// ErrMinorNotAccepted refuses a submission about a child made by a stranger.
// There is no way to verify a guardian in an anonymous form, and the assisted
// channel of the core exists precisely for that case.
var ErrMinorNotAccepted = errors.New("minor is not accepted in the open channel")

// SelfServiceVisitor carries the generalized fields and nothing else.
//
// There is deliberately no field for a name, a document, an e-mail or a phone
// number. Refusing identity by validation would mean the value existed in
// memory, in a decoder error and possibly in a log line before being refused;
// refusing it by shape means it never has anywhere to land. The transport
// completes the guard with additionalProperties: false, so an identity field in
// the body is a 400 at decode rather than a silently dropped value.
type SelfServiceVisitor struct {
	ClientID          string
	Role              VisitorRole
	AgeBand           AgeBand
	ResidenceCountry  string
	ResidenceState    string
	ResidenceCityCode string
}

// generalized reuses the core group rules verbatim. The conversion is only
// legal while the two structs stay field-for-field identical, which is the
// point: the day somebody adds an identity field to Visitor, this stops
// compiling instead of quietly carrying it into the open channel.
func (v SelfServiceVisitor) generalized() Visitor {
	return Visitor(v)
}

// ProofOfWorkAnswer is the second proof. It is not a proof of a human and not a
// proof of legitimacy: it measures electricity, not entitlement.
type ProofOfWorkAnswer struct {
	Challenge string
	Solution  string
}

func (p ProofOfWorkAnswer) valid() bool {
	return validOpaqueToken(p.Challenge, 32, 256) && validOpaqueToken(p.Solution, 1, 64)
}

// SelfRegistrationCommand is the whole open-channel submission. There is no
// expected_guest_count: it is len(Visitors), which removes any divergence
// between what was declared and what was submitted.
type SelfRegistrationCommand struct {
	Token                string
	RateSubject          string
	ClientSubmissionID   uuid.UUID
	PrivacyNoticeVersion string
	PlannedArrivalOn     string
	PlannedDepartureOn   string
	Visitors             []SelfServiceVisitor
	ProofOfWork          ProofOfWorkAnswer
	IdempotencyKey       string
	RequestID            string
}

// ValidateSelfRegistration answers with ErrMinorNotAccepted for the one case
// the operator has to be told about, and with ErrInvalidInput for everything
// else, so the response never varies with anything about a pre-existing data
// subject (T-02).
func ValidateSelfRegistration(command SelfRegistrationCommand) error {
	if !validSelfRegistrationEnvelope(command) {
		return ErrInvalidInput
	}
	if err := refuseMinors(command.Visitors); err != nil {
		return err
	}
	return ValidateGroup(generalizedGroup(command.Visitors))
}

func validSelfRegistrationEnvelope(command SelfRegistrationCommand) bool {
	return validSelfRegistrationCapability(command) &&
		validSelfRegistrationBody(command)
}

func validSelfRegistrationCapability(command SelfRegistrationCommand) bool {
	return validCapabilityRequest(command.Token, command.RateSubject) &&
		command.ProofOfWork.valid() &&
		validMutationMeta(command.IdempotencyKey, command.RequestID)
}

func validSelfRegistrationBody(command SelfRegistrationCommand) bool {
	return command.ClientSubmissionID.Version() == 7 &&
		validNoticeVersion(command.PrivacyNoticeVersion) &&
		validDateRange(command.PlannedArrivalOn, command.PlannedDepartureOn)
}

// The generated type already narrows role to [responsible, companion], so a
// minor is not even representable on the wire. The server check stays anyway:
// server-side validation must not depend on a client-side type.
func refuseMinors(visitors []SelfServiceVisitor) error {
	for _, visitor := range visitors {
		if visitor.Role == VisitorMinor || minorAgeBand(visitor.AgeBand) {
			return ErrMinorNotAccepted
		}
	}
	return nil
}

func minorAgeBand(band AgeBand) bool {
	return band == Age0To5 || band == Age6To11 || band == Age12To17
}

func generalizedGroup(visitors []SelfServiceVisitor) []Visitor {
	group := make([]Visitor, 0, len(visitors))
	for _, visitor := range visitors {
		group = append(group, visitor.generalized())
	}
	return group
}

// The alphabet is the contract's, so a value carrying anything else never
// reaches the verifier, the digest or a log line.
var base64URLPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func validOpaqueToken(value string, minimum, maximum int) bool {
	if len(value) < minimum || len(value) > maximum {
		return false
	}
	return base64URLPattern.MatchString(value)
}

// ApprovalCommand and RejectionCommand share the shape of every other stay
// command: an expected version for If-Match, an idempotency key, and a request
// identifier for the audit trail. The approval is a privacy control and not
// only a data-quality one, so the decision always has a named holder.
type ApprovalCommand struct {
	Actor           access.Principal
	StayID          uuid.UUID
	ExpectedVersion int64
	IdempotencyKey  string
	RequestID       string
}

type RejectionCommand struct {
	Actor           access.Principal
	StayID          uuid.UUID
	ExpectedVersion int64
	ReasonCode      RejectionReason
	IdempotencyKey  string
	RequestID       string
}

func ValidateApproval(command ApprovalCommand) error {
	if !validActor(command.Actor) || !validDecisionIdentity(
		command.StayID, command.ExpectedVersion, command.IdempotencyKey, command.RequestID,
	) {
		return ErrInvalidInput
	}
	return nil
}

// A reason outside the closed list is refused here and again by the CHECK in
// the database. Free text in platform.audit_events, which is append-only with
// no UPDATE and no DELETE, would become permanent personal data on the very
// path designed to erase it.
func ValidateRejection(command RejectionCommand) error {
	if !validActor(command.Actor) || !command.ReasonCode.Valid() {
		return ErrInvalidInput
	}
	if !validDecisionIdentity(
		command.StayID, command.ExpectedVersion, command.IdempotencyKey, command.RequestID,
	) {
		return ErrInvalidInput
	}
	return nil
}

func validDecisionIdentity(
	stayID uuid.UUID,
	expectedVersion int64,
	idempotencyKey string,
	requestID string,
) bool {
	return stayID != uuid.Nil &&
		expectedVersion > 0 &&
		validMutationMeta(idempotencyKey, requestID)
}
