//go:build integration

package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/platform/config"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store"
	"github.com/Pantani/cumuru/apps/api/internal/questionnaire"
	"github.com/Pantani/cumuru/apps/api/internal/stay"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestQuestionnairePostgreSQLSurveySecurityAndAtomicity(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	admin := openIntegrationPool(t, ctx, "CUMURU_TEST_ADMIN_DATABASE_URL")
	runtime := openIntegrationPool(t, ctx, "CUMURU_TEST_DATABASE_URL")
	requireRuntimeRole(t, ctx, runtime)
	fixture := seedQuestionnaireFixture(t, ctx, admin)
	settings := integrationQuestionnaireConfig()
	subject, err := store.NewQuestionnaire(
		runtime,
		5*time.Second,
		integrationCoreConfig(t),
		settings,
	)
	if err != nil {
		t.Fatalf("questionnaire store: %v", err)
	}
	repository := store.NewQuestionnaireRepository(subject)
	service := questionnaire.NewService(repository)
	codec := integrationCapabilityCodec(t, settings)

	first := assertSurveyReplayCipherAndSinks(
		t, ctx, admin, service, codec, fixture,
	)
	assertSurveyPoolSaturationDoesNotStarve(
		t, ctx, admin, settings, first,
	)
	assertSurveySingleConnectionPoolFailsClosed(
		t, ctx, settings, first,
	)
	assertSurveyReplayBypassesConsumedRateBudget(
		t, ctx, admin, service, codec, fixture,
	)
	assertConsumedCapabilityRejectsNewKey(
		t, ctx, admin, service, fixture, first,
	)
	assertSurveyCapabilityVersionBinding(
		t, ctx, admin, service, codec, fixture,
	)
	assertSurveyAnswerVersionBinding(
		t, ctx, admin, codec, fixture,
	)
	assertConcurrentSurveyConsumption(
		t, ctx, admin, service, codec, fixture,
	)
	assertSurveyRollbackAfterAnswers(
		t, ctx, admin, service, codec, fixture,
	)
	assertDeclineDoesNotBlockCheckIn(
		t, ctx, admin, subject, service, codec, fixture,
	)
	assertRetiredVersionAllowsReplay(
		t, ctx, admin, service, fixture, first,
	)
}

func assertSurveySingleConnectionPoolFailsClosed(
	t *testing.T,
	ctx context.Context,
	settings config.QuestionnaireConfig,
	accepted acceptedSurvey,
) {
	t.Helper()
	runtime := openLimitedIntegrationPool(t, ctx, 1)
	subject, err := store.NewQuestionnaire(
		runtime, time.Second, integrationCoreConfig(t), settings,
	)
	if err != nil {
		t.Fatalf("single-connection questionnaire store: %v", err)
	}
	service := questionnaire.NewService(store.NewQuestionnaireRepository(subject))
	if _, _, err := service.Submit(
		ctx, accepted.command,
	); !errors.Is(err, questionnaire.ErrUnavailable) {
		t.Fatalf("single-connection pool error=%v, want unavailable", err)
	}
}

func assertSurveyPoolSaturationDoesNotStarve(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
	settings config.QuestionnaireConfig,
	accepted acceptedSurvey,
) {
	t.Helper()
	runtime := openLimitedIntegrationPool(t, ctx, 2)
	subject, err := store.NewQuestionnaire(
		runtime, 2*time.Second, integrationCoreConfig(t), settings,
	)
	if err != nil {
		t.Fatalf("limited questionnaire store: %v", err)
	}
	services := []*questionnaire.Service{
		questionnaire.NewService(store.NewQuestionnaireRepository(subject)),
		questionnaire.NewService(store.NewQuestionnaireRepository(subject)),
	}
	before := surveyRateLimitTotal(t, ctx, admin)
	commands := make([]questionnaire.SubmissionCommand, 4)
	for index := range commands {
		commands[index] = accepted.command
		commands[index].IdempotencyKey = fmt.Sprintf(
			"questionnaire-pool-saturation-key-%04d", index,
		)
		commands[index].RequestID = fmt.Sprintf(
			"questionnaire-pool-saturation-request-%04d", index,
		)
		commands[index].ClientSubmission = mustV7(t)
	}
	successes, conflicts := runConcurrentSurveySubmissionsAcrossServices(
		ctx, services, commands,
	)
	if successes != 0 || conflicts != len(commands) {
		t.Fatalf(
			"pool saturation successes=%d conflicts=%d, want 0/%d",
			successes, conflicts, len(commands),
		)
	}
	after := surveyRateLimitTotal(t, ctx, admin)
	if after != before+len(commands) {
		t.Fatalf("pool saturation rate total=%d, want %d", after, before+len(commands))
	}
	replay := accepted.command
	replay.RequestID = "questionnaire-pool-saturation-replay"
	assertSurveyReplayAfterLimit(t, ctx, services[0], replay, accepted.result)
	afterReplay := surveyRateLimitTotal(t, ctx, admin)
	if afterReplay != after {
		t.Fatalf(
			"pool saturation replay incremented rate total: before=%d after=%d",
			after, afterReplay,
		)
	}
}

func runConcurrentSurveySubmissionsAcrossServices(
	ctx context.Context,
	services []*questionnaire.Service,
	commands []questionnaire.SubmissionCommand,
) (successes int, conflicts int) {
	start := make(chan struct{})
	results := make(chan error, len(commands))
	var group sync.WaitGroup
	for index, command := range commands {
		group.Add(1)
		go func(service *questionnaire.Service, command questionnaire.SubmissionCommand) {
			defer group.Done()
			<-start
			_, _, err := service.Submit(ctx, command)
			results <- err
		}(services[index%len(services)], command)
	}
	close(start)
	group.Wait()
	close(results)
	return countSurveySubmissionResults(results)
}

func openLimitedIntegrationPool(
	t *testing.T,
	ctx context.Context,
	maxConnections int32,
) *pgxpool.Pool {
	t.Helper()
	config, err := pgxpool.ParseConfig(os.Getenv("CUMURU_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatalf("parse limited runtime pool: %v", err)
	}
	config.MaxConns = maxConnections
	config.MinConns = 0
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatalf("open limited runtime pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

type questionnaireFixture struct {
	organizationID    uuid.UUID
	accommodationID   uuid.UUID
	membershipID      uuid.UUID
	versionID         uuid.UUID
	otherVersionID    uuid.UUID
	booleanQuestionID uuid.UUID
	textQuestionID    uuid.UUID
	otherQuestionID   uuid.UUID
	stays             []uuid.UUID
}

type issuedSurveyCapability struct {
	questionnaire.Capability
	stayID uuid.UUID
}

type acceptedSurvey struct {
	command    questionnaire.SubmissionCommand
	capability issuedSurveyCapability
	result     questionnaire.SubmissionAccepted
}

func seedQuestionnaireFixture(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
) questionnaireFixture {
	t.Helper()
	fixture := questionnaireFixture{
		organizationID: mustV7(t), accommodationID: mustV7(t),
		membershipID: mustV7(t), versionID: mustV7(t),
		otherVersionID: mustV7(t), booleanQuestionID: mustV7(t),
		textQuestionID: mustV7(t), otherQuestionID: mustV7(t),
	}
	for range 8 {
		fixture.stays = append(fixture.stays, mustV7(t))
	}
	seedQuestionnaireTenant(t, ctx, admin, fixture)
	seedQuestionnaireStays(t, ctx, admin, fixture)
	seedPublishedQuestionnaire(t, ctx, admin, fixture)
	seedOtherQuestionnaireVersion(t, ctx, admin, fixture)
	return fixture
}

func seedQuestionnaireTenant(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
	fixture questionnaireFixture,
) {
	t.Helper()
	_, err := admin.Exec(ctx,
		`INSERT INTO core.organizations (id, name)
		 VALUES ($1, 'Organização F3 repository')`,
		fixture.organizationID,
	)
	if err != nil {
		t.Fatalf("seed questionnaire organization: %v", err)
	}
	_, err = admin.Exec(ctx,
		`INSERT INTO core.accommodations
		   (id, organization_id, name, category, status)
		 VALUES ($1, $2, 'Hospedagem F3 repository', 'other', 'active')`,
		fixture.accommodationID,
		fixture.organizationID,
	)
	if err != nil {
		t.Fatalf("seed questionnaire accommodation: %v", err)
	}
	_, err = admin.Exec(ctx,
		`INSERT INTO core.memberships
		   (id, accommodation_id, oidc_issuer, oidc_subject, role)
		 VALUES ($1, $2, 'https://issuer.invalid', 'questionnaire-manager', 'manager')`,
		fixture.membershipID,
		fixture.accommodationID,
	)
	if err != nil {
		t.Fatalf("seed questionnaire membership: %v", err)
	}
}

func seedQuestionnaireStays(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
	fixture questionnaireFixture,
) {
	t.Helper()
	for _, stayID := range fixture.stays {
		_, err := admin.Exec(ctx,
			`INSERT INTO core.stays
			   (id, accommodation_id, created_by_membership_id, status,
			    client_submission_id, planned_arrival_on, planned_departure_on,
			    expected_guest_count)
			 VALUES ($1, $2, $3, 'pre_registered', $4,
			         DATE '2026-08-01', DATE '2026-08-03', 1)`,
			stayID,
			fixture.accommodationID,
			fixture.membershipID,
			mustV7(t),
		)
		if err != nil {
			t.Fatalf("seed questionnaire stay: %v", err)
		}
	}
	_, err := admin.Exec(ctx,
		`INSERT INTO core.visitors
		   (id, stay_id, client_id, role, age_band, residence_country)
		 VALUES ($1, $2, $3, 'responsible', '25_34', 'BR')`,
		mustV7(t),
		fixture.stays[5],
		mustV7(t),
	)
	if err != nil {
		t.Fatalf("seed questionnaire visitor: %v", err)
	}
}

func seedPublishedQuestionnaire(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
	fixture questionnaireFixture,
) {
	t.Helper()
	questionnaireID := mustV7(t)
	_, err := admin.Exec(ctx,
		`INSERT INTO survey.questionnaires (id, stable_key, name)
		 VALUES ($1, 'questionnaire_repository_profile', 'Pesquisa F3 repository')`,
		questionnaireID,
	)
	if err != nil {
		t.Fatalf("seed questionnaire questionnaire: %v", err)
	}
	_, err = admin.Exec(ctx,
		`INSERT INTO survey.questionnaire_versions
		   (id, questionnaire_id, version_number, title, introduction,
		    privacy_notice_version, last_editor_hmac, last_editor_key_version)
		 VALUES ($1, $2, 1, 'Pesquisa F3 repository',
		         'Introdução F3 repository', 'notice-v1',
		         decode('0101', 'hex'), 'v1')`,
		fixture.versionID,
		questionnaireID,
	)
	if err != nil {
		t.Fatalf("seed questionnaire questionnaire version: %v", err)
	}
	seedQuestionnaireQuestions(t, ctx, admin, fixture)
	transitionSeededQuestionnaire(t, ctx, admin, fixture.versionID)
}

func seedQuestionnaireQuestions(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
	fixture questionnaireFixture,
) {
	t.Helper()
	_, err := admin.Exec(ctx,
		`INSERT INTO survey.questions
		   (id, questionnaire_version_id, stable_key, prompt, answer_type,
		    required, data_classification, purpose_code,
		    retention_policy_code, analytics_key, public_aggregation_allowed,
		    minimum_public_cell, display_order)
		 VALUES
		   ($1, $3, 'first_visit', 'Primeira visita?', 'boolean',
		    true, 'personal', 'tourism_planning', 'survey_prototype_v1',
		    'first_visit', true, 10, 1),
		   ($2, $3, 'expectation', 'Expectativa curta', 'short_text',
		    false, 'personal', 'tourism_planning', 'survey_prototype_v1',
		    NULL, false, NULL, 2)`,
		fixture.booleanQuestionID,
		fixture.textQuestionID,
		fixture.versionID,
	)
	if err != nil {
		t.Fatalf("seed questionnaire questions: %v", err)
	}
	_, err = admin.Exec(ctx,
		`INSERT INTO survey.consent_requirements
		   (questionnaire_version_id, purpose_code, notice_version, prompt,
		    required_for_answers, display_order)
		 VALUES
		   ($1, 'tourism_planning', 'notice-v1', 'Aceito participar', true, 1)`,
		fixture.versionID,
	)
	if err != nil {
		t.Fatalf("seed questionnaire consent requirement: %v", err)
	}
}

func transitionSeededQuestionnaire(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
	versionID uuid.UUID,
) {
	t.Helper()
	statements := []string{
		`UPDATE survey.questionnaire_versions
		 SET status='privacy_review', submitted_by_hmac=decode('0101', 'hex'),
		     submitted_by_key_version='v1', submitted_for_review_at=now(),
		     revision=revision+1, updated_at=now()
		 WHERE id=$1`,
		`UPDATE survey.questionnaire_versions
		 SET status='approved', reviewed_by_hmac=decode('0202', 'hex'),
		     reviewed_by_key_version='v1', privacy_reviewed_at=now(),
		     revision=revision+1, updated_at=now()
		 WHERE id=$1`,
		`UPDATE survey.questionnaire_versions
		 SET status='published', published_at=now(),
		     revision=revision+1, updated_at=now()
		 WHERE id=$1`,
	}
	for _, statement := range statements {
		if _, err := admin.Exec(ctx, statement, versionID); err != nil {
			t.Fatalf("transition seeded questionnaire: %v", err)
		}
	}
}

func seedOtherQuestionnaireVersion(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
	fixture questionnaireFixture,
) {
	t.Helper()
	questionnaireID := mustV7(t)
	_, err := admin.Exec(ctx,
		`INSERT INTO survey.questionnaires (id, stable_key, name)
		 VALUES ($1, 'questionnaire_cross_version', 'Questionário de controle')`,
		questionnaireID,
	)
	if err != nil {
		t.Fatalf("seed other questionnaire: %v", err)
	}
	_, err = admin.Exec(ctx,
		`INSERT INTO survey.questionnaire_versions
		   (id, questionnaire_id, version_number, title,
		    privacy_notice_version, last_editor_hmac, last_editor_key_version)
		 VALUES ($1, $2, 1, 'Controle', 'notice-v1',
		         decode('0303', 'hex'), 'v1')`,
		fixture.otherVersionID,
		questionnaireID,
	)
	if err != nil {
		t.Fatalf("seed other questionnaire version: %v", err)
	}
	_, err = admin.Exec(ctx,
		`INSERT INTO survey.questions
		   (id, questionnaire_version_id, stable_key, prompt, answer_type,
		    required, data_classification, purpose_code,
		    retention_policy_code, public_aggregation_allowed, display_order)
		 VALUES ($1, $2, 'control_question', 'Controle?', 'boolean',
		         false, 'personal', 'tourism_planning',
		         'survey_prototype_v1', false, 1)`,
		fixture.otherQuestionID,
		fixture.otherVersionID,
	)
	if err != nil {
		t.Fatalf("seed other question: %v", err)
	}
}

func integrationQuestionnaireConfig() config.QuestionnaireConfig {
	return config.QuestionnaireConfig{
		Enabled: true, PrimaryStableKey: "tourism_profile",
		SurveyTTL: 24 * time.Hour, FreeTextTTL: 24 * time.Hour,
		SurveySubmitRateLimit: 10, FreeTextCleanupEnabled: true,
		SurveyKeys: testKeyring('s'), FreeTextKeys: testKeyring('f'),
	}
}

func integrationCapabilityCodec(
	t *testing.T,
	settings config.QuestionnaireConfig,
) *questionnaire.CapabilityCodec {
	t.Helper()
	codec, err := questionnaire.NewCapabilityCodec(questionnaire.Keyring{
		CurrentVersion: settings.SurveyKeys.CurrentVersion,
		Keys:           settings.SurveyKeys.Keys,
	})
	if err != nil {
		t.Fatalf("survey capability codec: %v", err)
	}
	return codec
}

func issueSurveyCapability(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
	codec *questionnaire.CapabilityCodec,
	stayID uuid.UUID,
	versionID uuid.UUID,
) issuedSurveyCapability {
	t.Helper()
	value, err := codec.Issue(mustV7(t))
	if err != nil {
		t.Fatalf("issue survey capability: %v", err)
	}
	_, err = admin.Exec(ctx,
		`INSERT INTO survey.capabilities
		   (id, token_hmac, token_key_version, purpose, stay_id,
		    questionnaire_version_id, expires_at)
		 VALUES ($1, $2, $3, 'survey_response', $4, $5, now() + interval '1 hour')`,
		value.ID,
		value.LookupHMAC,
		value.KeyVersion,
		stayID,
		versionID,
	)
	if err != nil {
		t.Fatalf("persist survey capability: %v", err)
	}
	return issuedSurveyCapability{Capability: value, stayID: stayID}
}

func submittedSurveyCommand(
	t *testing.T,
	fixture questionnaireFixture,
	capability issuedSurveyCapability,
	key string,
	requestID string,
	freeText string,
) questionnaire.SubmissionCommand {
	t.Helper()
	text, err := json.Marshal(freeText)
	if err != nil {
		t.Fatalf("encode free text: %v", err)
	}
	return questionnaire.SubmissionCommand{
		Capability: capability.Token, RateSubject: "203.0.113.0/24",
		VersionID: fixture.versionID, ClientSubmission: mustV7(t),
		Participation: questionnaire.ParticipationSubmitted,
		Answers: []questionnaire.AnswerInput{
			{QuestionID: fixture.booleanQuestionID, Value: json.RawMessage(`true`)},
			{QuestionID: fixture.textQuestionID, Value: text},
		},
		Consents: []questionnaire.ConsentDecisionInput{{
			PurposeCode: "tourism_planning", NoticeVersion: "notice-v1", Granted: true,
		}},
		IdempotencyKey: key, RequestID: requestID,
	}
}

func assertSurveyReplayCipherAndSinks(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
	service *questionnaire.Service,
	codec *questionnaire.CapabilityCodec,
	fixture questionnaireFixture,
) acceptedSurvey {
	t.Helper()
	const canary = "free-text-canary-questionnaire-never-plaintext"
	capability := issueSurveyCapability(
		t, ctx, admin, codec, fixture.stays[0], fixture.versionID,
	)
	command := submittedSurveyCommand(
		t, fixture, capability,
		"questionnaire-submit-replay-key-0001",
		"questionnaire-submit-request-0001",
		canary,
	)
	result, replayed, err := service.Submit(ctx, command)
	if err != nil || replayed {
		t.Fatalf("first survey submission = %#v, replay=%v, err=%v", result, replayed, err)
	}
	replay := command
	replay.RequestID = "questionnaire-submit-request-0002"
	second, replayed, err := service.Submit(ctx, replay)
	if err != nil || !replayed || second.ResponseID != result.ResponseID {
		t.Fatalf("survey replay = %#v, replay=%v, err=%v", second, replayed, err)
	}
	assertSurveyRows(t, ctx, admin, result.ResponseID, capability.stayID)
	assertEncryptedFreeText(t, ctx, admin, result.ResponseID, fixture, canary)
	assertCanaryAbsentFromSurveySinks(t, ctx, admin, capability.Token)
	assertCanaryAbsentFromSurveySinks(t, ctx, admin, canary)
	return acceptedSurvey{command: command, capability: capability, result: result}
}

func assertSurveyReplayBypassesConsumedRateBudget(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
	service *questionnaire.Service,
	codec *questionnaire.CapabilityCodec,
	fixture questionnaireFixture,
) {
	t.Helper()
	capability := issueSurveyCapability(
		t, ctx, admin, codec, fixture.stays[7], fixture.versionID,
	)
	command := submittedSurveyCommand(
		t, fixture, capability,
		"questionnaire-rate-replay-key-0001",
		"questionnaire-rate-replay-request-0001",
		"rate-replay-canary",
	)
	first := submitSurveyRateBaseline(t, ctx, service, command)
	consumeSurveyRateBudget(t, ctx, service, command)
	beforeReplay := surveyRateLimitMaximum(t, ctx, admin)
	if beforeReplay != 11 {
		t.Fatalf("rate bucket count = %d, want 11", beforeReplay)
	}
	assertSurveyReplayAfterLimit(t, ctx, service, command, first)
	afterReplay := surveyRateLimitMaximum(t, ctx, admin)
	if afterReplay != beforeReplay {
		t.Fatalf("exact replay incremented bucket: before=%d after=%d", beforeReplay, afterReplay)
	}
}

func submitSurveyRateBaseline(
	t *testing.T,
	ctx context.Context,
	service *questionnaire.Service,
	command questionnaire.SubmissionCommand,
) questionnaire.SubmissionAccepted {
	t.Helper()
	first, replayed, err := service.Submit(ctx, command)
	if err != nil || replayed {
		t.Fatalf("rate-limit baseline=%#v replay=%v err=%v", first, replayed, err)
	}
	return first
}

func consumeSurveyRateBudget(
	t *testing.T,
	ctx context.Context,
	service *questionnaire.Service,
	command questionnaire.SubmissionCommand,
) {
	t.Helper()
	for attempt := 2; attempt <= 10; attempt++ {
		probe := command
		probe.IdempotencyKey = fmt.Sprintf("questionnaire-rate-new-key-%04d", attempt)
		probe.RequestID = fmt.Sprintf("questionnaire-rate-new-request-%04d", attempt)
		probe.ClientSubmission = mustV7(t)
		if _, _, err := service.Submit(ctx, probe); !errors.Is(err, questionnaire.ErrConflict) {
			t.Fatalf("rate attempt %d error=%v, want consumed conflict", attempt, err)
		}
	}
	limited := command
	limited.IdempotencyKey = "questionnaire-rate-over-limit-key-0011"
	limited.RequestID = "questionnaire-rate-over-limit-request-0011"
	limited.ClientSubmission = mustV7(t)
	if _, _, err := service.Submit(ctx, limited); !errors.Is(err, questionnaire.ErrRateLimited) {
		t.Fatalf("N+1 rate attempt error=%v, want rate limited", err)
	}
}

func assertSurveyReplayAfterLimit(
	t *testing.T,
	ctx context.Context,
	service *questionnaire.Service,
	command questionnaire.SubmissionCommand,
	first questionnaire.SubmissionAccepted,
) {
	t.Helper()
	replay := command
	replay.RequestID = "questionnaire-rate-replay-after-limit"
	got, replayed, err := service.Submit(ctx, replay)
	if err != nil || !replayed || got.ResponseID != first.ResponseID {
		t.Fatalf("replay after consumed budget=%#v replay=%v err=%v", got, replayed, err)
	}
}

func surveyRateLimitMaximum(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
) int {
	t.Helper()
	var count int
	if err := admin.QueryRow(ctx,
		`SELECT COALESCE(max(request_count), 0)
		 FROM platform.rate_limit_buckets
		 WHERE scope='survey_submit'`,
	).Scan(&count); err != nil {
		t.Fatalf("read survey rate limit maximum: %v", err)
	}
	return count
}

func assertSurveyRows(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
	responseID uuid.UUID,
	stayID uuid.UUID,
) {
	t.Helper()
	var storedStay uuid.UUID
	var answers, decisions, audits, outbox int
	err := admin.QueryRow(ctx,
		`SELECT response.stay_id,
		        (SELECT count(*) FROM survey.answers WHERE response_id=response.id),
		        (SELECT count(*) FROM survey.consent_decisions WHERE response_id=response.id),
		        (SELECT count(*) FROM platform.audit_events
		         WHERE entity_id=response.id AND action='survey_response.recorded'),
		        (SELECT count(*) FROM platform.outbox_events
		         WHERE aggregate_id=response.id AND event_type='survey_response.recorded')
		 FROM survey.responses AS response
		 WHERE response.id=$1`,
		responseID,
	).Scan(&storedStay, &answers, &decisions, &audits, &outbox)
	if err != nil {
		t.Fatalf("read persisted survey: %v", err)
	}
	if storedStay != stayID || answers != 2 || decisions != 1 || audits != 1 || outbox != 1 {
		t.Fatalf(
			"survey rows stay=%s answers=%d decisions=%d audits=%d outbox=%d",
			storedStay, answers, decisions, audits, outbox,
		)
	}
}

func assertEncryptedFreeText(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
	responseID uuid.UUID,
	fixture questionnaireFixture,
	canary string,
) {
	t.Helper()
	var content, nonce []byte
	var keyVersion string
	err := admin.QueryRow(ctx,
		`SELECT encrypted_free_text, free_text_nonce, encryption_key_version
		 FROM survey.answers
		 WHERE response_id=$1 AND question_id=$2`,
		responseID,
		fixture.textQuestionID,
	).Scan(&content, &nonce, &keyVersion)
	if err != nil {
		t.Fatalf("read encrypted answer: %v", err)
	}
	cipher, err := questionnaire.NewTextCipher(questionnaire.Keyring{
		CurrentVersion: "v1", Keys: testKeyring('f').Keys,
	})
	if err != nil {
		t.Fatalf("create integration text cipher: %v", err)
	}
	value := questionnaire.Ciphertext{
		Content: content, Nonce: nonce, KeyVersion: keyVersion,
	}
	if _, err := cipher.Decrypt(value, []byte("missing-key-version")); err == nil {
		t.Fatal("ciphertext decrypted with incomplete associated data")
	}
	aad := strings.Join([]string{
		responseID.String(),
		fixture.versionID.String(),
		fixture.textQuestionID.String(),
		keyVersion,
	}, "\x00")
	plaintext, err := cipher.Decrypt(value, []byte(aad))
	if err != nil || string(plaintext) != canary {
		t.Fatalf("decrypt repository ciphertext = %q, %v", plaintext, err)
	}
}

func assertCanaryAbsentFromSurveySinks(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
	canary string,
) {
	t.Helper()
	var matches int
	err := admin.QueryRow(ctx,
		`WITH sinks(value) AS (
		   SELECT to_jsonb(value)::text FROM survey.capabilities AS value
		   UNION ALL SELECT to_jsonb(value)::text FROM survey.responses AS value
		   UNION ALL SELECT to_jsonb(value)::text FROM survey.answers AS value
		   UNION ALL SELECT to_jsonb(value)::text FROM survey.consent_decisions AS value
		   UNION ALL SELECT to_jsonb(value)::text FROM platform.idempotency_records AS value
		   UNION ALL SELECT to_jsonb(value)::text FROM platform.audit_events AS value
		   UNION ALL SELECT to_jsonb(value)::text FROM platform.outbox_events AS value
		 )
		 SELECT count(*) FROM sinks WHERE position($1 in value) > 0`,
		canary,
	).Scan(&matches)
	if err != nil {
		t.Fatalf("scan survey sinks: %v", err)
	}
	if matches != 0 {
		t.Fatalf("survey canary appeared in %d persisted sinks", matches)
	}
}

func assertConsumedCapabilityRejectsNewKey(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
	service *questionnaire.Service,
	fixture questionnaireFixture,
	accepted acceptedSurvey,
) {
	t.Helper()
	command := accepted.command
	command.IdempotencyKey = "questionnaire-consumed-new-key-0001"
	command.RequestID = "questionnaire-consumed-request-0001"
	command.ClientSubmission = mustV7(t)
	if _, _, err := service.Submit(ctx, command); !errors.Is(err, questionnaire.ErrConflict) {
		t.Fatalf("consumed capability error = %v, want conflict", err)
	}
	assertCapabilityResponseCount(t, ctx, admin, accepted.capability.ID, 1)
	assertVersionResponseCount(t, ctx, admin, fixture.versionID, 1)
}

func assertSurveyCapabilityVersionBinding(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
	service *questionnaire.Service,
	codec *questionnaire.CapabilityCodec,
	fixture questionnaireFixture,
) {
	t.Helper()
	capability := issueSurveyCapability(
		t, ctx, admin, codec, fixture.stays[1], fixture.versionID,
	)
	command := submittedSurveyCommand(
		t, fixture, capability,
		"questionnaire-wrong-version-key-0001",
		"questionnaire-wrong-version-request-0001",
		"wrong-version-canary",
	)
	command.VersionID = fixture.otherVersionID
	if _, _, err := service.Submit(ctx, command); !errors.Is(err, questionnaire.ErrCapabilityInvalid) {
		t.Fatalf("wrong version capability error = %v", err)
	}
	assertCapabilityResponseCount(t, ctx, admin, capability.ID, 0)
}

func assertSurveyAnswerVersionBinding(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
	codec *questionnaire.CapabilityCodec,
	fixture questionnaireFixture,
) {
	t.Helper()
	capability := issueSurveyCapability(
		t, ctx, admin, codec, fixture.stays[2], fixture.versionID,
	)
	tx, err := admin.Begin(ctx)
	if err != nil {
		t.Fatalf("begin cross-version answer: %v", err)
	}
	responseID := mustV7(t)
	_, err = tx.Exec(ctx,
		`INSERT INTO survey.responses
		   (id, stay_id, questionnaire_version_id, capability_id,
		    client_submission_id, participation)
		 VALUES ($1, $2, $3, $4, $5, 'submitted')`,
		responseID,
		capability.stayID,
		fixture.versionID,
		capability.ID,
		mustV7(t),
	)
	if err == nil {
		_, err = tx.Exec(ctx,
			`INSERT INTO survey.answers
			   (id, response_id, questionnaire_version_id, question_id,
			    structured_value)
			 VALUES ($1, $2, $3, $4, 'true'::jsonb)`,
			mustV7(t),
			responseID,
			fixture.versionID,
			fixture.otherQuestionID,
		)
	}
	_ = tx.Rollback(ctx)
	if err == nil {
		t.Fatal("answer from another questionnaire version was accepted")
	}
	assertCapabilityResponseCount(t, ctx, admin, capability.ID, 0)
}

func assertConcurrentSurveyConsumption(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
	service *questionnaire.Service,
	codec *questionnaire.CapabilityCodec,
	fixture questionnaireFixture,
) {
	t.Helper()
	capability := issueSurveyCapability(
		t, ctx, admin, codec, fixture.stays[3], fixture.versionID,
	)
	commands := []questionnaire.SubmissionCommand{
		submittedSurveyCommand(
			t, fixture, capability,
			"questionnaire-concurrent-key-0001",
			"questionnaire-concurrent-request-0001",
			"concurrent-canary-a",
		),
		submittedSurveyCommand(
			t, fixture, capability,
			"questionnaire-concurrent-key-0002",
			"questionnaire-concurrent-request-0002",
			"concurrent-canary-b",
		),
	}
	successes, conflicts := runConcurrentSurveySubmissions(ctx, service, commands)
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent submissions successes=%d conflicts=%d", successes, conflicts)
	}
	assertCapabilityResponseCount(t, ctx, admin, capability.ID, 1)
}

func runConcurrentSurveySubmissions(
	ctx context.Context,
	service *questionnaire.Service,
	commands []questionnaire.SubmissionCommand,
) (successes int, conflicts int) {
	start := make(chan struct{})
	results := make(chan error, len(commands))
	var group sync.WaitGroup
	for _, command := range commands {
		group.Add(1)
		go func(command questionnaire.SubmissionCommand) {
			defer group.Done()
			<-start
			_, _, err := service.Submit(ctx, command)
			results <- err
		}(command)
	}
	close(start)
	group.Wait()
	close(results)
	return countSurveySubmissionResults(results)
}

func countSurveySubmissionResults(
	results <-chan error,
) (successes int, conflicts int) {
	for err := range results {
		if err == nil {
			successes++
		}
		if errors.Is(err, questionnaire.ErrConflict) {
			conflicts++
		}
	}
	return successes, conflicts
}

func assertSurveyRollbackAfterAnswers(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
	service *questionnaire.Service,
	codec *questionnaire.CapabilityCodec,
	fixture questionnaireFixture,
) {
	t.Helper()
	capability := issueSurveyCapability(
		t, ctx, admin, codec, fixture.stays[4], fixture.versionID,
	)
	installAuditFailureTrigger(t, ctx, admin)
	before := versionIdempotencyCount(t, ctx, admin, fixture.versionID)
	rateBefore := surveyRateLimitTotal(t, ctx, admin)
	command := submittedSurveyCommand(
		t, fixture, capability,
		"questionnaire-rollback-key-0001",
		"questionnaire-rollback-request",
		"rollback-canary",
	)
	_, _, submitErr := service.Submit(ctx, command)
	removeAuditFailureTrigger(t, ctx, admin)
	if !errors.Is(submitErr, questionnaire.ErrUnavailable) {
		t.Fatalf("injected survey failure = %v, want unavailable", submitErr)
	}
	assertCapabilityResponseCount(t, ctx, admin, capability.ID, 0)
	after := versionIdempotencyCount(t, ctx, admin, fixture.versionID)
	if after != before {
		t.Fatalf("idempotency rows after rollback = %d, want %d", after, before)
	}
	rateAfter := surveyRateLimitTotal(t, ctx, admin)
	if rateAfter != rateBefore+1 {
		t.Fatalf("rate budget after rollback = %d, want %d", rateAfter, rateBefore+1)
	}
	var consumed bool
	err := admin.QueryRow(ctx,
		`SELECT consumed_at IS NOT NULL FROM survey.capabilities WHERE id=$1`,
		capability.ID,
	).Scan(&consumed)
	if err != nil || consumed {
		t.Fatalf("capability consumed after rollback = %v, %v", consumed, err)
	}
}

func surveyRateLimitTotal(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
) int {
	t.Helper()
	var count int
	if err := admin.QueryRow(ctx,
		`SELECT COALESCE(sum(request_count), 0)
		 FROM platform.rate_limit_buckets
		 WHERE scope='survey_submit'`,
	).Scan(&count); err != nil {
		t.Fatalf("read survey rate limit total: %v", err)
	}
	return count
}

func installAuditFailureTrigger(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
) {
	t.Helper()
	_, err := admin.Exec(ctx,
		`CREATE OR REPLACE FUNCTION platform.questionnaire_test_fail_audit()
		 RETURNS trigger LANGUAGE plpgsql AS $$
		 BEGIN
		   IF NEW.request_id = 'questionnaire-rollback-request' THEN
		     RAISE EXCEPTION 'questionnaire injected audit failure';
		   END IF;
		   RETURN NEW;
		 END;
		 $$;
		 CREATE TRIGGER questionnaire_test_fail_audit
		 BEFORE INSERT ON platform.audit_events
		 FOR EACH ROW EXECUTE FUNCTION platform.questionnaire_test_fail_audit()`,
	)
	if err != nil {
		t.Fatalf("install audit failure trigger: %v", err)
	}
}

func removeAuditFailureTrigger(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
) {
	t.Helper()
	_, err := admin.Exec(ctx,
		`DROP TRIGGER questionnaire_test_fail_audit ON platform.audit_events;
		 DROP FUNCTION platform.questionnaire_test_fail_audit()`,
	)
	if err != nil {
		t.Fatalf("remove audit failure trigger: %v", err)
	}
}

func assertDeclineDoesNotBlockCheckIn(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
	subject *store.Store,
	service *questionnaire.Service,
	codec *questionnaire.CapabilityCodec,
	fixture questionnaireFixture,
) {
	t.Helper()
	capability := issueSurveyCapability(
		t, ctx, admin, codec, fixture.stays[5], fixture.versionID,
	)
	command := questionnaire.SubmissionCommand{
		Capability: capability.Token, RateSubject: "198.51.100.0/24",
		VersionID: fixture.versionID, ClientSubmission: mustV7(t),
		Participation:  questionnaire.ParticipationDeclined,
		IdempotencyKey: "questionnaire-decline-key-0001",
		RequestID:      "questionnaire-decline-request-0001",
	}
	result, replayed, err := service.Submit(ctx, command)
	if err != nil || replayed || result.Participation != questionnaire.ParticipationDeclined {
		t.Fatalf("decline result=%#v replay=%v err=%v", result, replayed, err)
	}
	assertDeclinedRows(t, ctx, admin, result.ResponseID)
	stayRepository, err := store.NewStayRepository(subject)
	if err != nil {
		t.Fatalf("stay repository for check-in: %v", err)
	}
	stayService := stay.NewService(stayRepository)
	transition, replayed, err := stayService.Transition(ctx, stay.TransitionCommand{
		Actor: principal("questionnaire-manager"), StayID: capability.stayID,
		ExpectedVersion: 1, Kind: stay.TransitionCheckIn,
		OccurredAt:     time.Now().UTC(),
		IdempotencyKey: "questionnaire-check-in-key-0001",
		RequestID:      "questionnaire-check-in-request-0001",
	})
	if err != nil || replayed || transition.Status != stay.StatusCheckedIn {
		t.Fatalf("check-in after decline=%#v replay=%v err=%v", transition, replayed, err)
	}
}

func assertDeclinedRows(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
	responseID uuid.UUID,
) {
	t.Helper()
	var answers, consents int
	err := admin.QueryRow(ctx,
		`SELECT
		   (SELECT count(*) FROM survey.answers WHERE response_id=$1),
		   (SELECT count(*) FROM survey.consent_decisions WHERE response_id=$1)`,
		responseID,
	).Scan(&answers, &consents)
	if err != nil || answers != 0 || consents != 0 {
		t.Fatalf("decline rows answers=%d consents=%d err=%v", answers, consents, err)
	}
}

func assertRetiredVersionAllowsReplay(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
	service *questionnaire.Service,
	fixture questionnaireFixture,
	accepted acceptedSurvey,
) {
	t.Helper()
	_, err := admin.Exec(ctx,
		`UPDATE survey.questionnaire_versions
		 SET status='retired', retired_at=now(),
		     revision=revision+1, updated_at=now()
		 WHERE id=$1`,
		fixture.versionID,
	)
	if err != nil {
		t.Fatalf("retire questionnaire version: %v", err)
	}
	replay := accepted.command
	replay.RequestID = "questionnaire-retired-replay-request"
	result, replayed, err := service.Submit(ctx, replay)
	if err != nil || !replayed || result.ResponseID != accepted.result.ResponseID {
		t.Fatalf("retired replay=%#v replayed=%v err=%v", result, replayed, err)
	}
	other := accepted.command
	other.Capability = issueSurveyCapability(
		t, ctx, admin, integrationCapabilityCodec(t, integrationQuestionnaireConfig()),
		fixture.stays[6], fixture.versionID,
	).Token
	other.IdempotencyKey = "questionnaire-retired-new-key-0001"
	other.RequestID = "questionnaire-retired-new-request-0001"
	other.ClientSubmission = mustV7(t)
	if _, _, err := service.Submit(ctx, other); !errors.Is(err, questionnaire.ErrCapabilityInvalid) {
		t.Fatalf("new collection on retired version error = %v", err)
	}
}

func assertCapabilityResponseCount(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
	capabilityID uuid.UUID,
	want int,
) {
	t.Helper()
	var got int
	if err := admin.QueryRow(ctx,
		`SELECT count(*) FROM survey.responses WHERE capability_id=$1`,
		capabilityID,
	).Scan(&got); err != nil {
		t.Fatalf("count capability responses: %v", err)
	}
	if got != want {
		t.Fatalf("capability responses=%d, want %d", got, want)
	}
}

func assertVersionResponseCount(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
	versionID uuid.UUID,
	minimum int,
) {
	t.Helper()
	var got int
	if err := admin.QueryRow(ctx,
		`SELECT count(*) FROM survey.responses WHERE questionnaire_version_id=$1`,
		versionID,
	).Scan(&got); err != nil {
		t.Fatalf("count version responses: %v", err)
	}
	if got < minimum {
		t.Fatalf("version responses=%d, want at least %d", got, minimum)
	}
}

func versionIdempotencyCount(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
	versionID uuid.UUID,
) int {
	t.Helper()
	var count int
	if err := admin.QueryRow(ctx,
		`SELECT count(*) FROM platform.idempotency_records WHERE resource_id=$1`,
		versionID,
	).Scan(&count); err != nil {
		t.Fatalf("count version idempotency rows: %v", err)
	}
	return count
}
