package questionnaire

import (
	"context"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/access"
	"github.com/google/uuid"
)

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) List(ctx context.Context, actor access.Principal, page PageRequest) (Page, error) {
	if !validActor(actor.Issuer, actor.Subject) ||
		!validPage(!page.CursorCreatedAt.IsZero(), page.CursorID, page.Limit) {
		return Page{}, ErrInvalidInput
	}
	return s.repository.List(ctx, actor, page)
}

func (s *Service) Create(ctx context.Context, command CreateCommand) (VersionMutation, bool, error) {
	if !validCreate(command) {
		return VersionMutation{}, false, ErrInvalidInput
	}
	return s.repository.Create(ctx, command)
}

func (s *Service) Get(ctx context.Context, actor access.Principal, id uuid.UUID) (Questionnaire, error) {
	if !validActor(actor.Issuer, actor.Subject) || id == uuid.Nil {
		return Questionnaire{}, ErrInvalidInput
	}
	return s.repository.Get(ctx, actor, id)
}

func (s *Service) ListVersions(
	ctx context.Context,
	actor access.Principal,
	id uuid.UUID,
	page VersionPageRequest,
) (VersionPage, error) {
	cursorSet := page.CursorVersionNumber > 0
	if !validActor(actor.Issuer, actor.Subject) ||
		id == uuid.Nil ||
		!validPage(cursorSet, page.CursorID, page.Limit) {
		return VersionPage{}, ErrInvalidInput
	}
	return s.repository.ListVersions(ctx, actor, id, page)
}

func (s *Service) Clone(ctx context.Context, command CloneCommand) (VersionMutation, bool, error) {
	if !validClone(command) {
		return VersionMutation{}, false, ErrInvalidInput
	}
	return s.repository.Clone(ctx, command)
}

func (s *Service) GetVersion(ctx context.Context, actor access.Principal, id uuid.UUID) (Version, error) {
	if !validActor(actor.Issuer, actor.Subject) || id == uuid.Nil {
		return Version{}, ErrInvalidInput
	}
	return s.repository.GetVersion(ctx, actor, id)
}

func (s *Service) UpdateVersion(ctx context.Context, command UpdateCommand) (Version, error) {
	if !validUpdate(command) {
		return Version{}, ErrInvalidInput
	}
	return s.repository.UpdateVersion(ctx, command)
}

func (s *Service) Transition(
	ctx context.Context,
	command TransitionCommand,
) (VersionMutation, bool, error) {
	if !validTransition(command) {
		return VersionMutation{}, false, ErrInvalidInput
	}
	return s.repository.Transition(ctx, command)
}

func (s *Service) GetPublished(ctx context.Context, stableKey string) (Published, error) {
	if !validCatalogKey(stableKey) {
		return Published{}, ErrInvalidInput
	}
	return s.repository.GetPublished(ctx, stableKey)
}

func (s *Service) Submit(
	ctx context.Context,
	command SubmissionCommand,
) (SubmissionAccepted, bool, error) {
	if !validSubmission(command) {
		return SubmissionAccepted{}, false, ErrInvalidInput
	}
	return s.repository.Submit(ctx, command)
}

func (s *Service) EraseExpiredFreeText(ctx context.Context, cutoff time.Time) (int32, error) {
	if cutoff.IsZero() {
		return 0, ErrInvalidInput
	}
	return s.repository.EraseExpiredFreeText(ctx, cutoff.UTC())
}
