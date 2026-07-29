package store

import (
	"context"
	"errors"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/platform/store/generated"
	"github.com/Pantani/cumuru/apps/api/internal/questionnaire"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type surveyGrant struct {
	CapabilityID uuid.UUID
	KeyVersion   string
	VersionID    uuid.UUID
	ExpiresAt    time.Time
	Token        string
}

func (g surveyGrant) Available() bool {
	return g.CapabilityID != uuid.Nil && g.Token != ""
}

func (s *Store) issueSurveyCapability(
	ctx context.Context,
	q generated.Querier,
	stayID uuid.UUID,
	now time.Time,
) (surveyGrant, error) {
	if !s.phase3.Enabled || s.surveyCodec == nil {
		return surveyGrant{}, nil
	}
	version, err := q.GetPublishedTourismProfileVersion(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return surveyGrant{}, nil
	}
	if err != nil {
		return surveyGrant{}, questionnaire.ErrUnavailable
	}
	capabilityID, err := uuid.NewV7()
	if err != nil {
		return surveyGrant{}, questionnaire.ErrUnavailable
	}
	capability, err := s.surveyCodec.Issue(capabilityID)
	if err != nil {
		return surveyGrant{}, questionnaire.ErrUnavailable
	}
	expiresAt := now.Add(s.phase3.SurveyTTL)
	_, err = q.CreateSurveyCapability(ctx, generated.CreateSurveyCapabilityParams{
		ID: pgUUID(capability.ID), TokenHmac: capability.LookupHMAC,
		TokenKeyVersion: capability.KeyVersion, StayID: pgUUID(stayID),
		QuestionnaireVersionID: version.ID, ExpiresAt: pgTime(expiresAt),
		CreatedAt: pgTime(now),
	})
	if err != nil {
		return surveyGrant{}, questionnaireStoreError(err)
	}
	return surveyGrant{
		CapabilityID: capability.ID, KeyVersion: capability.KeyVersion,
		VersionID: uuid.UUID(version.ID.Bytes), ExpiresAt: expiresAt,
		Token: capability.Token,
	}, nil
}

func (s *Store) reconstructSurveyGrant(
	id uuid.UUID,
	keyVersion string,
	versionID uuid.UUID,
	expiresAt time.Time,
) (surveyGrant, error) {
	if id == uuid.Nil {
		return surveyGrant{}, nil
	}
	if s.surveyCodec == nil {
		return surveyGrant{}, questionnaire.ErrUnavailable
	}
	capability, err := s.surveyCodec.Reconstruct(id, keyVersion)
	if err != nil {
		return surveyGrant{}, questionnaire.ErrUnavailable
	}
	return surveyGrant{
		CapabilityID: id, KeyVersion: keyVersion, VersionID: versionID,
		ExpiresAt: expiresAt.UTC(), Token: capability.Token,
	}, nil
}
