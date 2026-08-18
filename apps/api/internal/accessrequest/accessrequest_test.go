package accessrequest_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/access"
	"github.com/Pantani/cumuru/apps/api/internal/accessrequest"
	"github.com/Pantani/cumuru/apps/api/internal/accommodation"
	"github.com/google/uuid"
)

const (
	submissionID = "019f0000-0000-7000-8000-000000000101"
	requestID    = "019f0000-0000-7000-8000-000000000102"
	powChallenge = "01234567890123456789012345678901234567890123456789"
)

func validCreateCommand() accessrequest.CreateCommand {
	return accessrequest.CreateCommand{
		RateSubject:          "203.0.113.0/24",
		ClientSubmissionID:   uuid.MustParse(submissionID),
		AccommodationName:    "Pousada do Descobrimento",
		Category:             accommodation.CategoryFormalLodging,
		Capacity:             12,
		ContactName:          "Responsavel",
		ContactEmail:         "contato@pousada.invalid",
		ContactPhone:         "+55 73 90000-0000",
		CityLabel:            "Prado",
		StateCode:            "BA",
		PrivacyNoticeVersion: accessrequest.PrivacyNoticeVersion,
		ProofOfWork: accessrequest.ProofOfWorkAnswer{
			Challenge: powChallenge, Solution: "abc",
		},
		IdempotencyKey: "access-request-key-0001",
		RequestID:      "019f0000-0000-7000-8000-0000000001ff",
	}
}

func TestCreateAcceptsTheDeclaredFormOfTheOpenChannel(t *testing.T) {
	t.Parallel()

	if err := accessrequest.ValidateCreate(validCreateCommand()); err != nil {
		t.Fatalf("ValidateCreate() error = %v", err)
	}
}

// The phone is the only optional field. Without this distinction, a form with
// no phone would become a blank string, which the database CHECK refuses as a
// technical failure instead of the server refusing it as invalid data.
func TestCreateAcceptsAnAbsentPhoneAndRefusesABlankOne(t *testing.T) {
	t.Parallel()

	command := validCreateCommand()
	command.ContactPhone = ""
	if err := accessrequest.ValidateCreate(command); err != nil {
		t.Fatalf("absent phone rejected: %v", err)
	}
	command.ContactPhone = "   "
	if !errors.Is(accessrequest.ValidateCreate(command), accessrequest.ErrInvalidInput) {
		t.Fatal("a blank phone was accepted")
	}
}

// Every bound mirrors a constraint of the migration. The server refuses by shape
// what the database refuses by CHECK, and neither barrier may depend on the
// other having worked.
func TestCreateMirrorsEveryConstraintOfTheTable(t *testing.T) {
	t.Parallel()

	tests := map[string]func(*accessrequest.CreateCommand){
		"blank name":         func(c *accessrequest.CreateCommand) { c.AccommodationName = "  " },
		"name over 200":      func(c *accessrequest.CreateCommand) { c.AccommodationName = strings.Repeat("a", 201) },
		"unclassified":       func(c *accessrequest.CreateCommand) { c.Category = accommodation.CategoryUnclassified },
		"unknown category":   func(c *accessrequest.CreateCommand) { c.Category = "hotel" },
		"capacity zero":      func(c *accessrequest.CreateCommand) { c.Capacity = 0 },
		"capacity over max":  func(c *accessrequest.CreateCommand) { c.Capacity = 10001 },
		"contact over 120":   func(c *accessrequest.CreateCommand) { c.ContactName = strings.Repeat("n", 121) },
		"email without at":   func(c *accessrequest.CreateCommand) { c.ContactEmail = "contato.pousada.invalid" },
		"email over 254":     func(c *accessrequest.CreateCommand) { c.ContactEmail = strings.Repeat("a", 250) + "@b.co" },
		"phone over 40":      func(c *accessrequest.CreateCommand) { c.ContactPhone = strings.Repeat("9", 41) },
		"city over 120":      func(c *accessrequest.CreateCommand) { c.CityLabel = strings.Repeat("c", 121) },
		"lowercase state":    func(c *accessrequest.CreateCommand) { c.StateCode = "ba" },
		"three letter state": func(c *accessrequest.CreateCommand) { c.StateCode = "BAH" },
		"submission not v7":  func(c *accessrequest.CreateCommand) { c.ClientSubmissionID = uuid.Nil },
		"short idempotency":  func(c *accessrequest.CreateCommand) { c.IdempotencyKey = "short" },
		"no request id":      func(c *accessrequest.CreateCommand) { c.RequestID = "" },
		"no rate subject":    func(c *accessrequest.CreateCommand) { c.RateSubject = "" },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			command := validCreateCommand()
			mutate(&command)
			if !errors.Is(accessrequest.ValidateCreate(command), accessrequest.ErrInvalidInput) {
				t.Fatal("an invalid submission was accepted")
			}
		})
	}
}

// The challenge alphabet is the contract's: a value outside it never reaches the
// verifier, the digest or a log line.
func TestCreateRefusesAProofOfWorkOutsideTheContractAlphabet(t *testing.T) {
	t.Parallel()

	tests := map[string]accessrequest.ProofOfWorkAnswer{
		"short challenge":     {Challenge: "abc", Solution: "abc"},
		"empty solution":      {Challenge: powChallenge, Solution: ""},
		"challenge with pad":  {Challenge: powChallenge + "==", Solution: "abc"},
		"solution with slash": {Challenge: powChallenge, Solution: "a/b"},
	}
	for name, answer := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			command := validCreateCommand()
			command.ProofOfWork = answer
			if !errors.Is(accessrequest.ValidateCreate(command), accessrequest.ErrInvalidInput) {
				t.Fatal("a malformed proof of work was accepted")
			}
		})
	}
}

// The partial unique index is on the normalized address. Without normalizing,
// the same e-mail in a different case would become a second row in the queue
// instead of a 409.
func TestNormalizeEmailMatchesTheColumnConstraint(t *testing.T) {
	t.Parallel()

	got := accessrequest.NormalizeEmail("  Contato@Pousada.INVALID ")
	if got != "contato@pousada.invalid" {
		t.Fatalf("NormalizeEmail() = %q", got)
	}
}

func TestApprovalStatesAndRejectionReasonsAreClosedLists(t *testing.T) {
	t.Parallel()

	for _, state := range []accessrequest.ApprovalState{
		accessrequest.StatePending, accessrequest.StateApproved,
		accessrequest.StateRejected, accessrequest.StateExpired,
	} {
		if !state.Valid() {
			t.Fatalf("state %q was refused", state)
		}
	}
	if accessrequest.ApprovalState("cancelled").Valid() {
		t.Fatal("an unknown state was accepted")
	}
	for _, reason := range []accessrequest.RejectionReason{
		accessrequest.ReasonDuplicateRequest, accessrequest.ReasonNotALodging,
		accessrequest.ReasonInsufficientInformation, accessrequest.ReasonAbuse,
	} {
		if !reason.Valid() {
			t.Fatalf("reason %q was refused", reason)
		}
	}
	if accessrequest.RejectionReason("nao gostei").Valid() {
		t.Fatal("free text passed as a rejection reason")
	}
}

func decisionActor() access.Principal {
	return access.NewPrincipal(
		"https://issuer.invalid", "administrador", []string{"accommodations:onboard"},
	)
}

func validApproval() accessrequest.ApprovalCommand {
	return accessrequest.ApprovalCommand{
		Actor: decisionActor(), AccessRequestID: uuid.MustParse(requestID),
		ExpectedVersion: 1, IdempotencyKey: "decision-key-000000001",
		RequestID: "019f0000-0000-7000-8000-0000000001ff",
	}
}

func TestDecisionRefusesAnIncompleteCommand(t *testing.T) {
	t.Parallel()

	tests := map[string]func(*accessrequest.ApprovalCommand){
		"no actor":       func(c *accessrequest.ApprovalCommand) { c.Actor = access.Principal{} },
		"no request":     func(c *accessrequest.ApprovalCommand) { c.AccessRequestID = uuid.Nil },
		"no version":     func(c *accessrequest.ApprovalCommand) { c.ExpectedVersion = 0 },
		"no idempotency": func(c *accessrequest.ApprovalCommand) { c.IdempotencyKey = "" },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			command := validApproval()
			mutate(&command)
			if !errors.Is(accessrequest.ValidateApproval(command), accessrequest.ErrInvalidInput) {
				t.Fatal("an incomplete approval was accepted")
			}
		})
	}
}

// A reason outside the closed list is refused before it reaches the append-only
// trail.
func TestRejectionDemandsAReasonFromTheClosedList(t *testing.T) {
	t.Parallel()

	approval := validApproval()
	command := accessrequest.RejectionCommand{
		Actor: approval.Actor, AccessRequestID: approval.AccessRequestID,
		ExpectedVersion: approval.ExpectedVersion,
		IdempotencyKey:  approval.IdempotencyKey, RequestID: approval.RequestID,
	}
	if !errors.Is(accessrequest.ValidateRejection(command), accessrequest.ErrInvalidInput) {
		t.Fatal("a rejection without a reason was accepted")
	}
	command.ReasonCode = accessrequest.ReasonAbuse
	if err := accessrequest.ValidateRejection(command); err != nil {
		t.Fatalf("ValidateRejection() error = %v", err)
	}
}

// The stub answers nothing and only records that it was reached: what the two
// paging tests below assert is which side of the validation the call landed on,
// not what the queue holds.
type listedRepository struct{ reached bool }

func (r *listedRepository) Context(
	context.Context, accessrequest.ContextRequest,
) (accessrequest.Context, error) {
	return accessrequest.Context{}, nil
}

func (r *listedRepository) Create(
	context.Context, accessrequest.CreateCommand,
) (accessrequest.Created, bool, error) {
	return accessrequest.Created{}, false, nil
}

func (r *listedRepository) List(
	context.Context, accessrequest.PageRequest,
) (accessrequest.Page, error) {
	r.reached = true
	return accessrequest.Page{}, nil
}

func (r *listedRepository) Approve(
	context.Context, accessrequest.ApprovalCommand,
) (accessrequest.Request, bool, error) {
	return accessrequest.Request{}, false, nil
}

func (r *listedRepository) Reject(
	context.Context, accessrequest.RejectionCommand,
) (accessrequest.Request, bool, error) {
	return accessrequest.Request{}, false, nil
}

// The half-set cursor is the rule a caller breaks silently: the other two
// refusals surface as an odd page, but a cursor with an id and no instant — or
// the reverse — would reach the query and answer a page nobody asked for.
func TestListRefusesAPageTheQueueCannotAnswer(t *testing.T) {
	t.Parallel()

	valid := accessrequest.PageRequest{Limit: 25}
	cases := map[string]func(*accessrequest.PageRequest){
		"limit zero":     func(p *accessrequest.PageRequest) { p.Limit = 0 },
		"limit negative": func(p *accessrequest.PageRequest) { p.Limit = -1 },
		"limit over 100": func(p *accessrequest.PageRequest) { p.Limit = 101 },
		"unknown state": func(p *accessrequest.PageRequest) {
			p.State = accessrequest.ApprovalState("cancelled")
		},
		"cursor id only": func(p *accessrequest.PageRequest) {
			p.CursorID = uuid.MustParse(requestID)
		},
		"cursor time only": func(p *accessrequest.PageRequest) {
			p.CursorCreatedAt = time.Unix(1, 0).UTC()
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			page := valid
			mutate(&page)
			repository := &listedRepository{}
			_, err := accessrequest.NewService(repository).
				List(context.Background(), page)
			if !errors.Is(err, accessrequest.ErrInvalidInput) {
				t.Fatalf("an invalid page was accepted: %v", err)
			}
			if repository.reached {
				t.Fatal("an invalid page reached the query")
			}
		})
	}
}

// The empty state means "every state" and the paired cursor is legitimate, so
// neither may be refused: a nil repository proves validation let them through.
func TestListAcceptsAnEmptyStateAndAPairedCursor(t *testing.T) {
	t.Parallel()

	page := accessrequest.PageRequest{
		Limit:           25,
		CursorCreatedAt: time.Unix(1, 0).UTC(),
		CursorID:        uuid.MustParse(requestID),
	}
	repository := &listedRepository{}
	if _, err := accessrequest.NewService(repository).
		List(context.Background(), page); err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if !repository.reached {
		t.Fatal("a legitimate page never reached the query")
	}
}

func TestPendingDeadlineIsThirtyDays(t *testing.T) {
	t.Parallel()

	if hours := accessrequest.PendingTTL.Hours(); hours != 30*24 {
		t.Fatalf("PendingTTL = %v hours, want 720", hours)
	}
}
