package localdemo

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/access"
	"github.com/Pantani/cumuru/apps/api/internal/stay"
	"github.com/google/uuid"
)

type stayTransitionRepositoryStub struct {
	stay.Repository
	commands []stay.TransitionCommand
	err      error
}

func (s *stayTransitionRepositoryStub) Transition(
	_ context.Context,
	command stay.TransitionCommand,
) (stay.MutationResult, bool, error) {
	s.commands = append(s.commands, command)
	if s.err != nil {
		return stay.MutationResult{}, false, s.err
	}
	return stay.MutationResult{
		ID: command.StayID, Status: stay.StatusCheckedOut, Version: command.ExpectedVersion + 1,
	}, false, nil
}

func TestTransitionStayFixtureResumesHistoricalCheckedInStay(t *testing.T) {
	t.Parallel()

	repository := &stayTransitionRepositoryStub{}
	service := stay.NewService(repository)
	fixture, current := checkedInHistoricalFixture()
	err := transitionStayFixture(
		context.Background(),
		service,
		access.NewPrincipal(issuer, operatorSubject, []string{"stays:write"}),
		fixture,
		current,
	)
	if err != nil {
		t.Fatalf("transitionStayFixture() error = %v", err)
	}
	if len(repository.commands) != 1 || repository.commands[0].Kind != stay.TransitionCheckOut {
		t.Fatalf("transition commands = %+v, want one check-out", repository.commands)
	}
}

func TestTransitionStayFixturePreservesCheckOutFailure(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("database unavailable")
	repository := &stayTransitionRepositoryStub{err: wantErr}
	fixture, current := checkedInHistoricalFixture()
	err := transitionStayFixture(
		context.Background(),
		stay.NewService(repository),
		access.NewPrincipal(issuer, operatorSubject, []string{"stays:write"}),
		fixture,
		current,
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("transitionStayFixture() error = %v, want check-out cause", err)
	}
}

func checkedInHistoricalFixture() (stayFixture, stay.Record) {
	id := uuid.MustParse("019f0000-0000-7000-8000-000000000041")
	arrival := time.Date(2026, time.July, 1, 12, 0, 0, 0, time.UTC)
	departure := arrival.AddDate(0, 0, 1)
	return stayFixture{
		key: "history-2026-07-01", arrival: arrival, departure: departure,
	}, stay.Record{ID: id, Status: stay.StatusCheckedIn, Version: 2}
}
