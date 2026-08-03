package localdemo

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/access"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store"
	"github.com/Pantani/cumuru/apps/api/internal/questionnaire"
	"github.com/Pantani/cumuru/apps/api/internal/stay"
	"github.com/google/uuid"
)

type fixtureServices struct {
	stays          *stay.Service
	questionnaires *questionnaire.Service
}

func loadStayFixture(
	ctx context.Context,
	services fixtureServices,
	provisioner *store.LocalDemoRepository,
	fixture stayFixture,
) error {
	operator := access.NewPrincipal(
		issuer,
		operatorSubject,
		[]string{"stays:write"},
	)
	current, err := ensureStayFixture(
		ctx,
		services.stays,
		provisioner,
		operator,
		fixture,
	)
	if err != nil {
		return fmt.Errorf("stay create: %w", err)
	}
	group, groupExists, err := ensureGroupFixture(
		ctx,
		services.stays,
		provisioner,
		operator,
		fixture,
		current,
	)
	if err != nil {
		return err
	}
	if err := ensureSurveyFixture(
		ctx,
		services,
		provisioner,
		operator,
		fixture,
		current,
		group,
		groupExists,
	); err != nil {
		return err
	}
	current, err = services.stays.Get(ctx, operator, current.ID)
	if err != nil {
		return fmt.Errorf("stay inspect: %w", err)
	}
	return transitionStayFixture(
		ctx,
		services.stays,
		operator,
		fixture,
		current,
	)
}

func ensureGroupFixture(
	ctx context.Context,
	service *stay.Service,
	provisioner *store.LocalDemoRepository,
	operator access.Principal,
	fixture stayFixture,
	current stay.Record,
) (stay.SubmissionAccepted, bool, error) {
	exists, err := provisioner.HasGroupSubmission(ctx, current.ID)
	if err != nil {
		return stay.SubmissionAccepted{}, false, fmt.Errorf("group inspect: %w", err)
	}
	if exists {
		return stay.SubmissionAccepted{}, true, nil
	}
	group, err := submitGroupFixture(
		ctx,
		service,
		operator,
		fixture,
		current,
	)
	if err != nil {
		return stay.SubmissionAccepted{}, false, fmt.Errorf("group submit: %w", err)
	}
	return group, false, nil
}

func ensureSurveyFixture(
	ctx context.Context,
	services fixtureServices,
	provisioner *store.LocalDemoRepository,
	operator access.Principal,
	fixture stayFixture,
	current stay.Record,
	group stay.SubmissionAccepted,
	groupExists bool,
) error {
	if fixture.responseCategory == "" {
		return nil
	}
	responseExists, err := provisioner.HasSurveyResponse(
		ctx,
		current.ID,
		versionID,
		surveyClientSubmissionID(fixture),
	)
	if err != nil {
		return fmt.Errorf("survey inspect: %w", err)
	}
	if responseExists {
		return nil
	}
	if groupExists {
		group, err = submitGroupFixture(
			ctx,
			services.stays,
			operator,
			fixture,
			current,
		)
		if err != nil {
			return fmt.Errorf("group replay: %w", err)
		}
	}
	if err := submitSurveyFixture(
		ctx,
		services.questionnaires,
		fixture,
		group,
	); err != nil {
		return fmt.Errorf("survey submit: %w", err)
	}
	return nil
}

func ensureStayFixture(
	ctx context.Context,
	service *stay.Service,
	provisioner *store.LocalDemoRepository,
	operator access.Principal,
	fixture stayFixture,
) (stay.Record, error) {
	clientSubmissionID := stayClientSubmissionID(fixture)
	stayID, found, err := provisioner.FindStay(
		ctx,
		fixture.accommodationID,
		clientSubmissionID,
	)
	if err != nil {
		return stay.Record{}, err
	}
	if !found {
		created, _, createErr := service.Create(ctx, stay.CreateCommand{
			Actor:              operator,
			AccommodationID:    fixture.accommodationID,
			ClientSubmissionID: clientSubmissionID,
			PlannedArrivalOn:   civilDate(fixture.arrival),
			PlannedDepartureOn: civilDate(fixture.departure),
			ExpectedGuestCount: 4,
			IdempotencyKey:     fixtureKey("stay-create-" + fixture.key),
			RequestID:          fixtureRequestID("stay-create-" + fixture.key),
		})
		if createErr != nil {
			return stay.Record{}, createErr
		}
		stayID = created.ID
	}
	current, err := service.Get(ctx, operator, stayID)
	if err != nil {
		return stay.Record{}, err
	}
	return reconcileStayDates(ctx, service, operator, fixture, current)
}

func reconcileStayDates(
	ctx context.Context,
	service *stay.Service,
	operator access.Principal,
	fixture stayFixture,
	current stay.Record,
) (stay.Record, error) {
	arrival := civilDate(fixture.arrival)
	departure := civilDate(fixture.departure)
	if current.PlannedArrivalOn == arrival &&
		current.PlannedDepartureOn == departure {
		return current, nil
	}
	if current.Status == stay.StatusCheckedOut ||
		current.Status == stay.StatusCancelled ||
		current.Status == stay.StatusNoShow {
		return stay.Record{}, errFixtureConflict
	}
	return service.Update(ctx, stay.UpdateCommand{
		Actor:           operator,
		StayID:          current.ID,
		ExpectedVersion: current.Version,
		Patch: stay.UpdatePatch{
			SetPlannedArrival:   true,
			PlannedArrivalOn:    arrival,
			SetPlannedDeparture: true,
			PlannedDepartureOn:  departure,
		},
		RequestID: fixtureRequestID("stay-update-" + fixture.key),
	})
}

func stayClientSubmissionID(fixture stayFixture) uuid.UUID {
	return deterministicUUID("stay-create-"+fixture.key, 1)
}

func surveyClientSubmissionID(fixture stayFixture) uuid.UUID {
	return deterministicUUID("survey-"+fixture.key, 1)
}

func submitGroupFixture(
	ctx context.Context,
	service *stay.Service,
	operator access.Principal,
	fixture stayFixture,
	current stay.Record,
) (stay.SubmissionAccepted, error) {
	group, _, err := service.SubmitAssistedGroup(ctx, stay.GroupCommand{
		Actor:                operator,
		StayID:               current.ID,
		ClientSubmissionID:   deterministicUUID("stay-group-"+fixture.key, 1),
		PrivacyNoticeVersion: privacyNoticeVersion,
		Visitors:             fixtureVisitors(fixture.key),
		ExpectedVersion:      current.Version,
		IdempotencyKey:       fixtureKey("stay-group-" + fixture.key),
		RequestID:            fixtureRequestID("stay-group-" + fixture.key),
	})
	return group, err
}

func transitionStayFixture(
	ctx context.Context,
	service *stay.Service,
	operator access.Principal,
	fixture stayFixture,
	current stay.Record,
) error {
	if stayTransitionComplete(current.Status, fixture.keepCheckedIn) {
		return nil
	}
	if current.Status == stay.StatusCheckedIn && !fixture.keepCheckedIn {
		return checkOutStayFixture(
			ctx,
			service,
			operator,
			fixture,
			stay.MutationResult{
				ID: current.ID, Status: current.Status, Version: current.Version,
			},
		)
	}
	if current.Status != stay.StatusPreRegistered {
		return errFixtureConflict
	}
	checkedIn, _, err := service.Transition(ctx, stay.TransitionCommand{
		Actor:           operator,
		StayID:          current.ID,
		ExpectedVersion: current.Version,
		Kind:            stay.TransitionCheckIn,
		OccurredAt:      fixture.arrival,
		IdempotencyKey:  fixtureKey("stay-checkin-" + fixture.key),
		RequestID:       fixtureRequestID("stay-checkin-" + fixture.key),
	})
	if err != nil {
		return fmt.Errorf("stay check-in: %w", err)
	}
	if fixture.keepCheckedIn {
		return nil
	}
	return checkOutStayFixture(ctx, service, operator, fixture, checkedIn)
}

func stayTransitionComplete(status stay.Status, keepCheckedIn bool) bool {
	return status == stay.StatusCheckedOut && !keepCheckedIn ||
		status == stay.StatusCheckedIn && keepCheckedIn
}

func checkOutStayFixture(
	ctx context.Context,
	service *stay.Service,
	operator access.Principal,
	fixture stayFixture,
	checkedIn stay.MutationResult,
) error {
	_, _, err := service.Transition(ctx, stay.TransitionCommand{
		Actor:           operator,
		StayID:          checkedIn.ID,
		ExpectedVersion: checkedIn.Version,
		Kind:            stay.TransitionCheckOut,
		OccurredAt:      fixture.departure,
		IdempotencyKey:  fixtureKey("stay-checkout-" + fixture.key),
		RequestID:       fixtureRequestID("stay-checkout-" + fixture.key),
	})
	if err != nil {
		return fmt.Errorf("stay check-out: %w", err)
	}
	return nil
}

func submitSurveyFixture(
	ctx context.Context,
	service *questionnaire.Service,
	fixture stayFixture,
	group stay.SubmissionAccepted,
) error {
	value, err := json.Marshal(fixture.responseCategory)
	if err != nil {
		return err
	}
	_, _, err = service.Submit(ctx, questionnaire.SubmissionCommand{
		Capability:       group.SurveyCapability,
		RateSubject:      "local-demo-fixture",
		VersionID:        versionID,
		ClientSubmission: surveyClientSubmissionID(fixture),
		Participation:    questionnaire.ParticipationSubmitted,
		Answers: []questionnaire.AnswerInput{
			{QuestionID: questionID, Value: value},
		},
		Consents: []questionnaire.ConsentDecisionInput{
			{
				PurposeCode:   "tourism_planning",
				NoticeVersion: privacyNoticeVersion,
				Granted:       true,
			},
		},
		IdempotencyKey: fixtureKey("survey-" + fixture.key),
		RequestID:      fixtureRequestID("survey-" + fixture.key),
	})
	return err
}

func fixtureKey(namespace string) string {
	return fmt.Sprintf(
		"local-demo-%x",
		deterministicUUID(namespace, 1),
	)
}

func fixtureRequestID(namespace string) string {
	return "local-demo-request-" + deterministicUUID(namespace, 2).String()
}

func civilDate(value time.Time) string {
	return value.Format("2006-01-02")
}
