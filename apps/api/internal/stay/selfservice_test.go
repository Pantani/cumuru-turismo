package stay_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/Pantani/cumuru/apps/api/internal/access"
	"github.com/Pantani/cumuru/apps/api/internal/stay"
	"github.com/google/uuid"
)

func selfRegistrationFixture() stay.SelfRegistrationCommand {
	return stay.SelfRegistrationCommand{
		Token:                strings.Repeat("a", 96),
		RateSubject:          "203.0.113.0/24",
		ClientSubmissionID:   uuid.MustParse("019f0000-0000-7000-8000-000000000001"),
		PrivacyNoticeVersion: "2026-08",
		PlannedArrivalOn:     "2026-12-10",
		PlannedDepartureOn:   "2026-12-12",
		Visitors: []stay.SelfServiceVisitor{{
			ClientID:          "019f0000-0000-7000-8000-000000000002",
			Role:              stay.VisitorResponsible,
			AgeBand:           stay.Age25To34,
			ResidenceCountry:  "BR",
			ResidenceState:    "BA",
			ResidenceCityCode: "2925303",
		}},
		ProofOfWork: stay.ProofOfWorkAnswer{
			Challenge: strings.Repeat("c", 98),
			Solution:  "AAAAAAAAAAA",
		},
		IdempotencyKey: "self-registration-key-0001",
		RequestID:      "request-0001",
	}
}

func TestSelfRegistrationAcceptsGeneralizedData(t *testing.T) {
	t.Parallel()

	if err := stay.ValidateSelfRegistration(selfRegistrationFixture()); err != nil {
		t.Fatalf("ValidateSelfRegistration() error = %v", err)
	}
}

// The open channel is unauthenticated and nobody vouches for the submitter, so
// a submission about a child is refused outright (ADR-040).
func TestSelfRegistrationRefusesMinors(t *testing.T) {
	t.Parallel()

	byRole := selfRegistrationFixture()
	byRole.Visitors[0].Role = stay.VisitorMinor
	if err := stay.ValidateSelfRegistration(byRole); !errors.Is(err, stay.ErrMinorNotAccepted) {
		t.Fatalf("ValidateSelfRegistration(minor role) error = %v", err)
	}

	// The age band is the second door. Refusing only the role would let the
	// same submission through under role=companion.
	for _, band := range []stay.AgeBand{stay.Age0To5, stay.Age6To11, stay.Age12To17} {
		byAge := selfRegistrationFixture()
		byAge.Visitors[0].AgeBand = band
		if err := stay.ValidateSelfRegistration(byAge); !errors.Is(err, stay.ErrMinorNotAccepted) {
			t.Fatalf("ValidateSelfRegistration(%s) error = %v", band, err)
		}
	}
}

// A companion above the minor bands is fine; the guard must not refuse the
// whole group.
func TestSelfRegistrationAcceptsAnAdultCompanion(t *testing.T) {
	t.Parallel()

	command := selfRegistrationFixture()
	command.Visitors = append(command.Visitors, stay.SelfServiceVisitor{
		ClientID:         "019f0000-0000-7000-8000-000000000003",
		Role:             stay.VisitorCompanion,
		AgeBand:          stay.Age60Plus,
		ResidenceCountry: "PT",
	})
	if err := stay.ValidateSelfRegistration(command); err != nil {
		t.Fatalf("ValidateSelfRegistration() error = %v", err)
	}
}

func TestSelfRegistrationRefusesAnIncompleteEnvelope(t *testing.T) {
	t.Parallel()

	tests := map[string]func(*stay.SelfRegistrationCommand){
		"absent token": func(c *stay.SelfRegistrationCommand) { c.Token = "" },
		"short token":  func(c *stay.SelfRegistrationCommand) { c.Token = "short" },
		"absent rate subject": func(c *stay.SelfRegistrationCommand) {
			c.RateSubject = "  "
		},
		"absent challenge": func(c *stay.SelfRegistrationCommand) {
			c.ProofOfWork.Challenge = ""
		},
		"absent solution": func(c *stay.SelfRegistrationCommand) {
			c.ProofOfWork.Solution = ""
		},
		"challenge outside the alphabet": func(c *stay.SelfRegistrationCommand) {
			c.ProofOfWork.Challenge = strings.Repeat("c", 97) + "+"
		},
		"oversized solution": func(c *stay.SelfRegistrationCommand) {
			c.ProofOfWork.Solution = strings.Repeat("s", 65)
		},
		"non v7 submission id": func(c *stay.SelfRegistrationCommand) {
			c.ClientSubmissionID = uuid.MustParse("00000000-0000-4000-8000-000000000001")
		},
		"absent notice version": func(c *stay.SelfRegistrationCommand) {
			c.PrivacyNoticeVersion = " "
		},
		"departure before arrival": func(c *stay.SelfRegistrationCommand) {
			c.PlannedDepartureOn = "2026-12-09"
		},
		"absent idempotency key": func(c *stay.SelfRegistrationCommand) {
			c.IdempotencyKey = ""
		},
		"absent request id": func(c *stay.SelfRegistrationCommand) { c.RequestID = "" },
		"no visitors": func(c *stay.SelfRegistrationCommand) {
			c.Visitors = nil
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			command := selfRegistrationFixture()
			mutate(&command)
			if err := stay.ValidateSelfRegistration(command); err == nil {
				t.Fatal("ValidateSelfRegistration() accepted an invalid command")
			}
		})
	}
}

// N-28: a rejection without a reason, or with anything outside the closed list,
// is refused before it can reach an append-only audit table.
func TestRejectionRequiresAClosedListReason(t *testing.T) {
	t.Parallel()

	base := stay.RejectionCommand{
		Actor:           access.Principal{Issuer: "local", Subject: "operator"},
		StayID:          uuid.MustParse("019f0000-0000-7000-8000-000000000001"),
		ExpectedVersion: 3,
		ReasonCode:      stay.RejectionNotAGuest,
		IdempotencyKey:  "rejection-key-000000001",
		RequestID:       "request-0001",
	}
	if err := stay.ValidateRejection(base); err != nil {
		t.Fatalf("ValidateRejection() error = %v", err)
	}
	for _, reason := range []stay.RejectionReason{"", "CPF falso, não é hóspede", "unknown"} {
		refused := base
		refused.ReasonCode = reason
		if err := stay.ValidateRejection(refused); err == nil {
			t.Fatalf("ValidateRejection(%q) accepted free text", reason)
		}
	}
}

func TestApprovalRequiresAnIdentifiedActorAndAVersion(t *testing.T) {
	t.Parallel()

	base := stay.ApprovalCommand{
		Actor:           access.Principal{Issuer: "local", Subject: "operator"},
		StayID:          uuid.MustParse("019f0000-0000-7000-8000-000000000001"),
		ExpectedVersion: 3,
		IdempotencyKey:  "approval-key-0000000001",
		RequestID:       "request-0001",
	}
	if err := stay.ValidateApproval(base); err != nil {
		t.Fatalf("ValidateApproval() error = %v", err)
	}
	tests := map[string]func(*stay.ApprovalCommand){
		"anonymous actor": func(c *stay.ApprovalCommand) { c.Actor = access.Principal{} },
		"absent stay":     func(c *stay.ApprovalCommand) { c.StayID = uuid.Nil },
		"absent version":  func(c *stay.ApprovalCommand) { c.ExpectedVersion = 0 },
		"absent key":      func(c *stay.ApprovalCommand) { c.IdempotencyKey = "" },
		"absent request":  func(c *stay.ApprovalCommand) { c.RequestID = "" },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			command := base
			mutate(&command)
			if err := stay.ValidateApproval(command); err == nil {
				t.Fatal("ValidateApproval() accepted an invalid command")
			}
		})
	}
}
