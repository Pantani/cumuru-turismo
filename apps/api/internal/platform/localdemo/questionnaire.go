package localdemo

import (
	"context"
	"errors"
	"reflect"

	"github.com/Pantani/cumuru/apps/api/internal/access"
	"github.com/Pantani/cumuru/apps/api/internal/questionnaire"
)

var errFixtureConflict = errors.New("local demo fixture conflicts with existing data")

func ensureQuestionnaire(
	ctx context.Context,
	service *questionnaire.Service,
) error {
	editor := access.NewPrincipal(
		issuer,
		"fixture-questionnaire-editor",
		[]string{"questionnaires:manage"},
	)
	reviewer := access.NewPrincipal(
		issuer,
		"fixture-questionnaire-reviewer",
		[]string{"questionnaires:approve"},
	)
	current, err := service.GetVersion(ctx, editor, versionID)
	if errors.Is(err, questionnaire.ErrNotFound) {
		current, err = createQuestionnaire(ctx, service, editor)
	}
	if err != nil {
		return err
	}
	if current.Status == questionnaire.StatusPublished {
		if !definitionsEqual(current.Definition(), questionnaireDefinition()) {
			return errFixtureConflict
		}
		return nil
	}
	if current.Status == questionnaire.StatusDraft {
		current, err = updateQuestionnaire(ctx, service, editor, current)
	}
	if err != nil {
		return err
	}
	current, err = transitionQuestionnaire(
		ctx,
		service,
		editor,
		reviewer,
		current,
	)
	if err != nil {
		return err
	}
	if current.Status != questionnaire.StatusPublished {
		return errFixtureConflict
	}
	return nil
}

func createQuestionnaire(
	ctx context.Context,
	service *questionnaire.Service,
	editor access.Principal,
) (questionnaire.Version, error) {
	_, _, err := service.Create(ctx, questionnaire.CreateCommand{
		Actor:                editor,
		ID:                   questionnaireID,
		VersionID:            versionID,
		StableKey:            "tourism_profile",
		Name:                 "Pesquisa turística fictícia",
		Title:                questionnaireDefinition().Title,
		PrivacyNoticeVersion: privacyNoticeVersion,
		IdempotencyKey:       fixtureKey("questionnaire-create"),
		RequestID:            fixtureRequestID("questionnaire-create"),
	})
	if err != nil {
		return questionnaire.Version{}, err
	}
	return service.GetVersion(ctx, editor, versionID)
}

func updateQuestionnaire(
	ctx context.Context,
	service *questionnaire.Service,
	editor access.Principal,
	current questionnaire.Version,
) (questionnaire.Version, error) {
	if definitionsEqual(current.Definition(), questionnaireDefinition()) {
		return current, nil
	}
	return service.UpdateVersion(ctx, questionnaire.UpdateCommand{
		Actor:           editor,
		VersionID:       versionID,
		ExpectedVersion: current.Revision,
		Definition:      questionnaireDefinition(),
		RequestID:       fixtureRequestID("questionnaire-update"),
	})
}

func definitionsEqual(
	left questionnaire.Definition,
	right questionnaire.Definition,
) bool {
	return reflect.DeepEqual(
		normalizedDefinition(left),
		normalizedDefinition(right),
	)
}

func normalizedDefinition(
	definition questionnaire.Definition,
) questionnaire.Definition {
	definition.Questions = append(
		[]questionnaire.Question(nil),
		definition.Questions...,
	)
	zeroValidation := questionnaire.ValidationDefinition{}
	for index := range definition.Questions {
		if definition.Questions[index].RetentionPolicyCode ==
			"prototype-aggregate-only" {
			definition.Questions[index].RetentionPolicyCode =
				"prototype_aggregate_only"
		}
		validation := definition.Questions[index].Validation
		if validation != nil && reflect.DeepEqual(*validation, zeroValidation) {
			definition.Questions[index].Validation = nil
		}
	}
	return definition
}

func transitionQuestionnaire(
	ctx context.Context,
	service *questionnaire.Service,
	editor access.Principal,
	reviewer access.Principal,
	current questionnaire.Version,
) (questionnaire.Version, error) {
	steps := []struct {
		from       questionnaire.VersionStatus
		transition questionnaire.Transition
		actor      access.Principal
	}{
		{
			from:       questionnaire.StatusDraft,
			transition: questionnaire.TransitionSubmitReview,
			actor:      editor,
		},
		{
			from:       questionnaire.StatusPrivacyReview,
			transition: questionnaire.TransitionApprove,
			actor:      reviewer,
		},
		{
			from:       questionnaire.StatusApproved,
			transition: questionnaire.TransitionPublish,
			actor:      reviewer,
		},
	}
	for _, step := range steps {
		if current.Status != step.from {
			continue
		}
		mutation, _, err := service.Transition(ctx, questionnaire.TransitionCommand{
			Actor:           step.actor,
			VersionID:       versionID,
			ExpectedVersion: current.Revision,
			Transition:      step.transition,
			IdempotencyKey:  fixtureKey("questionnaire-" + string(step.transition)),
			RequestID:       fixtureRequestID("questionnaire-" + string(step.transition)),
		})
		if err != nil {
			return questionnaire.Version{}, err
		}
		current, err = service.GetVersion(ctx, step.actor, mutation.ID)
		if err != nil {
			return questionnaire.Version{}, err
		}
	}
	return current, nil
}
