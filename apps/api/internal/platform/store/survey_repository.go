package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/audit"
	"github.com/Pantani/cumuru/apps/api/internal/platform/idempotency"
	"github.com/Pantani/cumuru/apps/api/internal/platform/outbox"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store/generated"
	"github.com/Pantani/cumuru/apps/api/internal/questionnaire"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func (r *QuestionnaireRepository) Submit(
	ctx context.Context,
	command questionnaire.SubmissionCommand,
) (result questionnaire.SubmissionAccepted, replayed bool, err error) {
	capability, err := r.store.surveyCodec.Resolve(command.Capability)
	if err != nil {
		return result, false, questionnaire.ErrCapabilityInvalid
	}
	rateConnection, err := r.acquireSurveyRateConnection(ctx)
	if err != nil {
		return result, false, questionnaire.ErrUnavailable
	}
	defer rateConnection.Close()
	now := r.store.currentTime()
	err = r.store.inTransaction(ctx, func(q generated.Querier) error {
		rateConnection.PairAcquired()
		spec := idempotencySpec{
			actorValue: "survey-capability\x00" + capability.ID.String(),
			operation:  idempotency.OperationSubmitSurveyResponse,
			resourceID: command.VersionID, key: command.IdempotencyKey,
			request: surveyIdempotencyRequest(command), now: now,
		}
		value, runErr := r.store.runIdempotent(ctx, q, spec, func() (storedMutation, error) {
			return r.submitSurvey(
				ctx, q, rateConnection.Queries(), command, capability, now,
			)
		})
		if runErr != nil {
			return questionnaireStoreError(runErr)
		}
		if decodeErr := json.Unmarshal(value.response.body, &result); decodeErr != nil {
			return questionnaire.ErrUnavailable
		}
		replayed = value.replayed
		return nil
	})
	return result, replayed, err
}

func (r *QuestionnaireRepository) submitSurvey(
	ctx context.Context,
	q generated.Querier,
	rateQueries generated.Querier,
	command questionnaire.SubmissionCommand,
	capability questionnaire.Capability,
	now time.Time,
) (storedMutation, error) {
	if err := r.applySurveyRateLimit(
		ctx, rateQueries, capability.ID, command.RateSubject, now,
	); err != nil {
		return storedMutation{}, err
	}
	row, version, err := r.resolveSurveySubmission(ctx, q, capability, command, now)
	if err != nil {
		return storedMutation{}, err
	}
	return r.persistSurveySubmission(ctx, q, row, version, capability, command, now)
}

func (r *QuestionnaireRepository) resolveSurveySubmission(
	ctx context.Context,
	q generated.Querier,
	capability questionnaire.Capability,
	command questionnaire.SubmissionCommand,
	now time.Time,
) (generated.SurveyCapability, questionnaire.Version, error) {
	row, err := q.LockSurveyCapability(ctx, capability.LookupHMAC)
	if err != nil {
		return row, questionnaire.Version{}, surveyCapabilityError(err)
	}
	if err := validateCapabilityRow(row, capability, command.VersionID, now); err != nil {
		return row, questionnaire.Version{}, err
	}
	version, err := r.publishedVersion(ctx, q, command)
	if err != nil {
		return row, questionnaire.Version{}, err
	}
	return row, version, nil
}

// A response may only be recorded against the published version the capability
// was issued for, and must satisfy that version's own definition.
func (r *QuestionnaireRepository) publishedVersion(
	ctx context.Context,
	q generated.Querier,
	command questionnaire.SubmissionCommand,
) (questionnaire.Version, error) {
	version, err := r.readVersion(ctx, q, command.VersionID)
	if err != nil || version.Status != questionnaire.StatusPublished {
		return questionnaire.Version{}, questionnaire.ErrCapabilityInvalid
	}
	if err := questionnaire.ValidateResponse(version.Definition(), command); err != nil {
		return questionnaire.Version{}, err
	}
	return version, nil
}

func (r *QuestionnaireRepository) persistSurveySubmission(
	ctx context.Context,
	q generated.Querier,
	row generated.SurveyCapability,
	version questionnaire.Version,
	capability questionnaire.Capability,
	command questionnaire.SubmissionCommand,
	now time.Time,
) (storedMutation, error) {
	responseID, err := uuid.NewV7()
	if err != nil {
		return storedMutation{}, questionnaire.ErrUnavailable
	}
	if err := insertSurveyResponse(ctx, q, responseID, row, command, now); err != nil {
		return storedMutation{}, questionnaireStoreError(err)
	}
	if err := r.insertSurveyAnswers(ctx, q, responseID, version, command.Answers, now); err != nil {
		return storedMutation{}, err
	}
	if err := insertSurveyConsents(ctx, q, responseID, command, now); err != nil {
		return storedMutation{}, err
	}
	return r.completeSurveySubmission(ctx, q, row, capability.ID, responseID, command, now)
}

func (r *QuestionnaireRepository) completeSurveySubmission(
	ctx context.Context,
	q generated.Querier,
	row generated.SurveyCapability,
	capabilityID uuid.UUID,
	responseID uuid.UUID,
	command questionnaire.SubmissionCommand,
	now time.Time,
) (storedMutation, error) {
	if _, err := q.ConsumeSurveyCapability(ctx, generated.ConsumeSurveyCapabilityParams{
		ConsumedAt: pgTime(now), ID: row.ID,
	}); err != nil {
		return storedMutation{}, questionnaireStoreError(err)
	}
	result := questionnaire.SubmissionAccepted{
		ResponseID: responseID, Participation: command.Participation, Status: "accepted",
	}
	if err := r.store.recordSurveyResponse(ctx, q, capabilityID, result, command.RequestID, now); err != nil {
		return storedMutation{}, err
	}
	return jsonMutation(200, responseID, result, nil)
}

func surveyIdempotencyRequest(command questionnaire.SubmissionCommand) any {
	return struct {
		VersionID        uuid.UUID                            `json:"questionnaire_version_id"`
		ClientSubmission uuid.UUID                            `json:"client_submission_id"`
		Participation    questionnaire.Participation          `json:"participation"`
		Answers          []questionnaire.AnswerInput          `json:"answers"`
		Consents         []questionnaire.ConsentDecisionInput `json:"consent_decisions"`
	}{
		VersionID: command.VersionID, ClientSubmission: command.ClientSubmission,
		Participation: command.Participation, Answers: command.Answers, Consents: command.Consents,
	}
}

func validateCapabilityRow(
	row generated.SurveyCapability,
	capability questionnaire.Capability,
	versionID uuid.UUID,
	now time.Time,
) error {
	if !capabilityMatches(row, capability, versionID) || !capabilityLive(row, now) {
		return questionnaire.ErrCapabilityInvalid
	}
	if row.ConsumedAt.Valid {
		return questionnaire.ErrConflict
	}
	return nil
}

func capabilityMatches(
	row generated.SurveyCapability,
	capability questionnaire.Capability,
	versionID uuid.UUID,
) bool {
	return uuid.UUID(row.ID.Bytes) == capability.ID &&
		row.TokenKeyVersion == capability.KeyVersion &&
		uuid.UUID(row.QuestionnaireVersionID.Bytes) == versionID
}

func capabilityLive(row generated.SurveyCapability, now time.Time) bool {
	return !row.RevokedAt.Valid &&
		row.ExpiresAt.Valid &&
		row.ExpiresAt.Time.After(now)
}

func surveyCapabilityError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return questionnaire.ErrCapabilityInvalid
	}
	return questionnaire.ErrUnavailable
}

func insertSurveyResponse(
	ctx context.Context,
	q generated.Querier,
	responseID uuid.UUID,
	capability generated.SurveyCapability,
	command questionnaire.SubmissionCommand,
	now time.Time,
) error {
	return q.InsertSurveyResponse(ctx, generated.InsertSurveyResponseParams{
		ID: pgUUID(responseID), StayID: capability.StayID,
		QuestionnaireVersionID: capability.QuestionnaireVersionID,
		CapabilityID:           capability.ID, ClientSubmissionID: pgUUID(command.ClientSubmission),
		Participation: generated.SurveyParticipation(command.Participation),
		SubmittedAt:   pgTime(now),
	})
}

func (r *QuestionnaireRepository) insertSurveyAnswers(
	ctx context.Context,
	q generated.Querier,
	responseID uuid.UUID,
	version questionnaire.Version,
	answers []questionnaire.AnswerInput,
	now time.Time,
) error {
	questions := make(map[uuid.UUID]questionnaire.Question, len(version.Questions))
	for _, question := range version.Questions {
		questions[question.ID] = question
	}
	for _, answer := range answers {
		if err := r.insertSurveyAnswer(
			ctx, q, responseID, version.ID, questions[answer.QuestionID], answer, now,
		); err != nil {
			return err
		}
	}
	return nil
}

func (r *QuestionnaireRepository) insertSurveyAnswer(
	ctx context.Context,
	q generated.Querier,
	responseID uuid.UUID,
	versionID uuid.UUID,
	question questionnaire.Question,
	answer questionnaire.AnswerInput,
	now time.Time,
) error {
	answerID, err := uuid.NewV7()
	if err != nil {
		return questionnaire.ErrUnavailable
	}
	params := generated.InsertSurveyAnswerParams{
		ID: pgUUID(answerID), ResponseID: pgUUID(responseID),
		QuestionnaireVersionID: pgUUID(versionID), QuestionID: pgUUID(answer.QuestionID),
		CreatedAt: pgTime(now),
	}
	if err := r.fillAnswerValue(&params, question, responseID, versionID, answer, now); err != nil {
		return err
	}
	if err := q.InsertSurveyAnswer(ctx, params); err != nil {
		return questionnaireStoreError(err)
	}
	return nil
}

// Free text is encrypted at rest with an erase deadline; every other answer
// type is structured and stored in the clear for aggregation.
func (r *QuestionnaireRepository) fillAnswerValue(
	params *generated.InsertSurveyAnswerParams,
	question questionnaire.Question,
	responseID uuid.UUID,
	versionID uuid.UUID,
	answer questionnaire.AnswerInput,
	now time.Time,
) error {
	freeText := question.AnswerType == questionnaire.AnswerShortText ||
		question.AnswerType == questionnaire.AnswerLongText
	if !freeText {
		params.StructuredValue = append([]byte(nil), answer.Value...)
		return nil
	}
	return r.encryptFreeText(params, responseID, versionID, answer, now)
}

func (r *QuestionnaireRepository) encryptFreeText(
	params *generated.InsertSurveyAnswerParams,
	responseID uuid.UUID,
	versionID uuid.UUID,
	answer questionnaire.AnswerInput,
	now time.Time,
) error {
	keyVersion, err := r.freeTextKeyVersion()
	if err != nil {
		return err
	}
	var plaintext string
	if json.Unmarshal(answer.Value, &plaintext) != nil {
		return questionnaire.ErrInvalidInput
	}
	aad := surveyAAD(responseID, versionID, answer.QuestionID, keyVersion)
	value, err := r.store.textCipher.Encrypt([]byte(plaintext), aad)
	if err != nil {
		return err
	}
	params.EncryptedFreeText = value.Content
	params.FreeTextNonce = value.Nonce
	params.EncryptionKeyVersion = &value.KeyVersion
	params.EraseAfter = pgTime(now.Add(r.store.phase3.FreeTextTTL))
	return nil
}

// Free text is refused unless the cipher and the erase pipeline are both
// configured, so plaintext can never be stored without a deletion path.
func (r *QuestionnaireRepository) freeTextKeyVersion() (string, error) {
	if r.store.textCipher == nil || !r.store.phase3.FreeTextCleanupEnabled {
		return "", questionnaire.ErrInvalidInput
	}
	keyVersion := r.store.textCipher.CurrentVersion()
	if keyVersion == "" {
		return "", questionnaire.ErrInvalidInput
	}
	return keyVersion, nil
}

func insertSurveyConsents(
	ctx context.Context,
	q generated.Querier,
	responseID uuid.UUID,
	command questionnaire.SubmissionCommand,
	now time.Time,
) error {
	for _, consent := range command.Consents {
		id, err := uuid.NewV7()
		if err != nil {
			return questionnaire.ErrUnavailable
		}
		err = q.InsertConsentDecision(ctx, generated.InsertConsentDecisionParams{
			ID: pgUUID(id), ResponseID: pgUUID(responseID),
			QuestionnaireVersionID: pgUUID(command.VersionID),
			PurposeCode:            consent.PurposeCode, NoticeVersion: consent.NoticeVersion,
			Granted: consent.Granted, RecordedAt: pgTime(now),
		})
		if err != nil {
			return questionnaireStoreError(err)
		}
	}
	return nil
}

func surveyAAD(responseID, versionID, questionID uuid.UUID, keyVersion string) []byte {
	return []byte(strings.Join([]string{
		responseID.String(), versionID.String(), questionID.String(), keyVersion,
	}, "\x00"))
}

func (r *QuestionnaireRepository) applySurveyRateLimit(
	ctx context.Context,
	queries generated.Querier,
	capabilityID uuid.UUID,
	subject string,
	now time.Time,
) error {
	key, ok := r.store.phase2.RateLimitKeys.Key(r.store.phase2.RateLimitKeys.CurrentVersion)
	if !ok {
		return questionnaire.ErrUnavailable
	}
	const scope = "survey_submit"
	window := now.Truncate(r.store.phase2.RateLimitWindow)
	rateCtx, cancel := context.WithTimeout(ctx, r.store.timeout)
	defer cancel()
	row, err := queries.IncrementRateLimit(rateCtx, generated.IncrementRateLimitParams{
		Scope:             scope,
		SubjectHmac:       keyedDigest(key, "rate-limit:"+scope, capabilityID.String()+"\x00"+subject),
		SubjectKeyVersion: r.store.phase2.RateLimitKeys.CurrentVersion,
		WindowStartedAt:   timeToPG(window),
		ExpiresAt:         timeToPG(window.Add(2 * r.store.phase2.RateLimitWindow)),
	})
	if err != nil {
		return questionnaire.ErrUnavailable
	}
	if row.RequestCount > int32(r.store.phase3.SurveySubmitRateLimit) {
		return questionnaire.ErrRateLimited
	}
	return nil
}

type surveyRateConnection struct {
	queries    generated.Querier
	connection *pgxpool.Conn
	permit     chan struct{}
	permitHeld bool
}

func (r *QuestionnaireRepository) acquireSurveyRateConnection(
	ctx context.Context,
) (*surveyRateConnection, error) {
	if r.store.pool == nil {
		return &surveyRateConnection{queries: r.store.queries}, nil
	}
	if r.store.pool.Config().MaxConns < 2 || r.store.surveyPairPermit == nil {
		return nil, questionnaire.ErrUnavailable
	}
	acquireCtx, cancel := context.WithTimeout(ctx, r.store.timeout)
	defer cancel()
	return r.acquirePairedConnection(acquireCtx)
}

// The rate-limit counter runs on a second connection, so a permit is held to
// guarantee the pool can always serve both halves of the pair.
func (r *QuestionnaireRepository) acquirePairedConnection(
	ctx context.Context,
) (*surveyRateConnection, error) {
	if err := acquireSurveyPairPermit(ctx, r.store.surveyPairPermit); err != nil {
		return nil, err
	}
	connection, err := r.store.pool.Acquire(ctx)
	if err != nil {
		<-r.store.surveyPairPermit
		return nil, questionnaire.ErrUnavailable
	}
	return &surveyRateConnection{
		queries: generated.New(connection), connection: connection,
		permit: r.store.surveyPairPermit, permitHeld: true,
	}, nil
}

func acquireSurveyPairPermit(ctx context.Context, permit chan struct{}) error {
	select {
	case permit <- struct{}{}:
		return nil
	case <-ctx.Done():
		return questionnaire.ErrUnavailable
	}
}

func (c *surveyRateConnection) Queries() generated.Querier {
	return c.queries
}

func (c *surveyRateConnection) PairAcquired() {
	if c.permitHeld {
		<-c.permit
		c.permitHeld = false
	}
}

func (c *surveyRateConnection) Close() {
	c.PairAcquired()
	if c.connection != nil {
		c.connection.Release()
		c.connection = nil
	}
}

func (s *Store) recordSurveyResponse(
	ctx context.Context,
	q generated.Querier,
	capabilityID uuid.UUID,
	result questionnaire.SubmissionAccepted,
	requestID string,
	now time.Time,
) error {
	return s.recordEvents(ctx, q, eventSpec{
		actorType:   audit.ActorSurveyCapability,
		actorIssuer: "urn:cumuru:survey-capability", actorSubject: capabilityID.String(),
		action: audit.ActionSurveyRecorded, entityType: audit.EntitySurveyResponse,
		entityID: result.ResponseID, requestID: requestID,
		changedFields: []audit.ChangedField{audit.FieldParticipation},
		version:       1, aggregateType: outbox.AggregateSurveyResponse,
		eventType: outbox.EventSurveyResponseRecorded,
		purpose:   audit.PurposeTourismSurvey, now: now,
	})
}
