-- name: ListQuestionnaires :many
SELECT
  id,
  stable_key,
  name,
  created_at
FROM survey.questionnaires
WHERE (
  sqlc.narg(cursor_created_at)::timestamptz IS NULL
  OR (created_at, id) < (
    sqlc.narg(cursor_created_at)::timestamptz,
    sqlc.narg(cursor_id)::uuid
  )
)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_limit);

-- name: CreateQuestionnaire :one
INSERT INTO survey.questionnaires (
  id,
  stable_key,
  name,
  created_at
) VALUES (
  sqlc.arg(id),
  sqlc.arg(stable_key),
  sqlc.arg(name),
  sqlc.arg(created_at)
)
RETURNING id, stable_key, name, created_at;

-- name: GetQuestionnaire :one
SELECT id, stable_key, name, created_at
FROM survey.questionnaires
WHERE id = sqlc.arg(id);

-- name: LockQuestionnaire :one
SELECT id, stable_key, name, created_at
FROM survey.questionnaires
WHERE id = sqlc.arg(id)
FOR UPDATE;

-- name: ListQuestionnaireVersions :many
SELECT
  id,
  questionnaire_id,
  version_number,
  revision,
  status,
  title,
  privacy_notice_version,
  created_at,
  updated_at
FROM survey.questionnaire_versions
WHERE questionnaire_id = sqlc.arg(questionnaire_id)
  AND (
    sqlc.narg(cursor_version_number)::integer IS NULL
    OR (version_number, id) < (
      sqlc.narg(cursor_version_number)::integer,
      sqlc.narg(cursor_id)::uuid
    )
  )
ORDER BY version_number DESC, id DESC
LIMIT sqlc.arg(page_limit);

-- name: GetNextQuestionnaireVersionNumber :one
SELECT COALESCE(max(version_number), 0)::integer + 1
FROM survey.questionnaire_versions
WHERE questionnaire_id = sqlc.arg(questionnaire_id);

-- name: CreateQuestionnaireVersion :one
INSERT INTO survey.questionnaire_versions (
  id,
  questionnaire_id,
  version_number,
  title,
  introduction,
  privacy_notice_version,
  last_editor_hmac,
  last_editor_key_version,
  created_at,
  updated_at
) VALUES (
  sqlc.arg(id),
  sqlc.arg(questionnaire_id),
  sqlc.arg(version_number),
  sqlc.arg(title),
  sqlc.narg(introduction),
  sqlc.arg(privacy_notice_version),
  sqlc.arg(last_editor_hmac),
  sqlc.arg(last_editor_key_version),
  sqlc.arg(created_at),
  sqlc.arg(created_at)
)
RETURNING
  id,
  questionnaire_id,
  version_number,
  revision,
  status,
  title,
  privacy_notice_version,
  created_at,
  updated_at;

-- name: GetQuestionnaireVersion :one
SELECT *
FROM survey.questionnaire_versions
WHERE id = sqlc.arg(id);

-- name: LockQuestionnaireVersion :one
SELECT *
FROM survey.questionnaire_versions
WHERE id = sqlc.arg(id)
FOR UPDATE;

-- name: UpdateDraftQuestionnaireVersion :one
UPDATE survey.questionnaire_versions
SET
  title = sqlc.arg(title),
  introduction = sqlc.narg(introduction),
  privacy_notice_version = sqlc.arg(privacy_notice_version),
  last_editor_hmac = sqlc.arg(last_editor_hmac),
  last_editor_key_version = sqlc.arg(last_editor_key_version),
  revision = revision + 1,
  updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id)
  AND status = 'draft'
  AND revision = sqlc.arg(expected_revision)
RETURNING *;

-- name: DeleteDraftQuestionnaireContent :exec
DELETE FROM survey.questions
WHERE questionnaire_version_id = sqlc.arg(questionnaire_version_id)
  AND EXISTS (
    SELECT 1
    FROM survey.questionnaire_versions
    WHERE id = sqlc.arg(questionnaire_version_id)
      AND status = 'draft'
  );

-- name: DeleteDraftConsentRequirements :exec
DELETE FROM survey.consent_requirements
WHERE questionnaire_version_id = sqlc.arg(questionnaire_version_id)
  AND EXISTS (
    SELECT 1
    FROM survey.questionnaire_versions
    WHERE id = sqlc.arg(questionnaire_version_id)
      AND status = 'draft'
  );

-- name: InsertQuestion :exec
INSERT INTO survey.questions (
  id,
  questionnaire_version_id,
  stable_key,
  prompt,
  help_text,
  answer_type,
  required,
  data_classification,
  purpose_code,
  retention_policy_code,
  analytics_key,
  public_aggregation_allowed,
  minimum_public_cell,
  validation,
  visibility_rule,
  display_order
) VALUES (
  sqlc.arg(id),
  sqlc.arg(questionnaire_version_id),
  sqlc.arg(stable_key),
  sqlc.arg(prompt),
  sqlc.narg(help_text),
  sqlc.arg(answer_type),
  sqlc.arg(required),
  sqlc.arg(data_classification),
  sqlc.arg(purpose_code),
  sqlc.arg(retention_policy_code),
  sqlc.narg(analytics_key),
  sqlc.arg(public_aggregation_allowed),
  sqlc.narg(minimum_public_cell),
  sqlc.arg(validation),
  sqlc.narg(visibility_rule),
  sqlc.arg(display_order)
);

-- name: InsertQuestionOption :exec
INSERT INTO survey.question_options (
  id,
  question_id,
  value,
  label,
  display_order
) VALUES (
  sqlc.arg(id),
  sqlc.arg(question_id),
  sqlc.arg(value),
  sqlc.arg(label),
  sqlc.arg(display_order)
);

-- name: InsertConsentRequirement :exec
INSERT INTO survey.consent_requirements (
  questionnaire_version_id,
  purpose_code,
  notice_version,
  prompt,
  required_for_answers,
  display_order
) VALUES (
  sqlc.arg(questionnaire_version_id),
  sqlc.arg(purpose_code),
  sqlc.arg(notice_version),
  sqlc.arg(prompt),
  sqlc.arg(required_for_answers),
  sqlc.arg(display_order)
);

-- name: ListQuestionsForVersion :many
SELECT *
FROM survey.questions
WHERE questionnaire_version_id = sqlc.arg(questionnaire_version_id)
ORDER BY display_order;

-- name: ListQuestionOptionsForVersion :many
SELECT option.*
FROM survey.question_options AS option
JOIN survey.questions AS question
  ON question.id = option.question_id
WHERE question.questionnaire_version_id = sqlc.arg(questionnaire_version_id)
ORDER BY question.display_order, option.display_order;

-- name: ListConsentRequirementsForVersion :many
SELECT *
FROM survey.consent_requirements
WHERE questionnaire_version_id = sqlc.arg(questionnaire_version_id)
ORDER BY display_order;

-- name: SubmitQuestionnaireVersionReview :one
UPDATE survey.questionnaire_versions
SET
  status = 'privacy_review',
  submitted_by_hmac = sqlc.arg(submitted_by_hmac),
  submitted_by_key_version = sqlc.arg(submitted_by_key_version),
  submitted_for_review_at = sqlc.arg(transitioned_at),
  reviewed_by_hmac = NULL,
  reviewed_by_key_version = NULL,
  privacy_reviewed_at = NULL,
  change_reason_code = NULL,
  revision = revision + 1,
  updated_at = sqlc.arg(transitioned_at)
WHERE id = sqlc.arg(id)
  AND status = 'draft'
  AND revision = sqlc.arg(expected_revision)
RETURNING *;

-- name: RequestQuestionnaireVersionChanges :one
UPDATE survey.questionnaire_versions
SET
  status = 'draft',
  reviewed_by_hmac = sqlc.arg(reviewed_by_hmac),
  reviewed_by_key_version = sqlc.arg(reviewed_by_key_version),
  privacy_reviewed_at = sqlc.arg(transitioned_at),
  change_reason_code = sqlc.arg(change_reason_code),
  revision = revision + 1,
  updated_at = sqlc.arg(transitioned_at)
WHERE id = sqlc.arg(id)
  AND status = 'privacy_review'
  AND revision = sqlc.arg(expected_revision)
RETURNING *;

-- name: ApproveQuestionnaireVersion :one
UPDATE survey.questionnaire_versions
SET
  status = 'approved',
  reviewed_by_hmac = sqlc.arg(reviewed_by_hmac),
  reviewed_by_key_version = sqlc.arg(reviewed_by_key_version),
  privacy_reviewed_at = sqlc.arg(transitioned_at),
  change_reason_code = NULL,
  revision = revision + 1,
  updated_at = sqlc.arg(transitioned_at)
WHERE id = sqlc.arg(id)
  AND status = 'privacy_review'
  AND revision = sqlc.arg(expected_revision)
RETURNING *;

-- name: RetireCurrentPublishedVersion :exec
UPDATE survey.questionnaire_versions
SET
  status = 'retired',
  retired_at = sqlc.arg(transitioned_at),
  revision = revision + 1,
  updated_at = sqlc.arg(transitioned_at)
WHERE questionnaire_id = sqlc.arg(questionnaire_id)
  AND status = 'published';

-- name: PublishQuestionnaireVersion :one
UPDATE survey.questionnaire_versions
SET
  status = 'published',
  published_at = sqlc.arg(transitioned_at),
  revision = revision + 1,
  updated_at = sqlc.arg(transitioned_at)
WHERE id = sqlc.arg(id)
  AND status = 'approved'
  AND revision = sqlc.arg(expected_revision)
RETURNING *;

-- name: RetireQuestionnaireVersion :one
UPDATE survey.questionnaire_versions
SET
  status = 'retired',
  retired_at = sqlc.arg(transitioned_at),
  revision = revision + 1,
  updated_at = sqlc.arg(transitioned_at)
WHERE id = sqlc.arg(id)
  AND status = 'published'
  AND revision = sqlc.arg(expected_revision)
RETURNING *;

-- name: GetPublishedQuestionnaireVersionByStableKey :one
SELECT version.*
FROM survey.questionnaire_versions AS version
JOIN survey.questionnaires AS questionnaire
  ON questionnaire.id = version.questionnaire_id
WHERE questionnaire.stable_key = sqlc.arg(stable_key)
  AND version.status = 'published';

-- name: GetPublishedTourismProfileVersion :one
SELECT version.*
FROM survey.questionnaire_versions AS version
JOIN survey.questionnaires AS questionnaire
  ON questionnaire.id = version.questionnaire_id
WHERE questionnaire.stable_key = 'tourism_profile'
  AND version.status = 'published';

-- name: CreateSurveyCapability :one
INSERT INTO survey.capabilities (
  id,
  token_hmac,
  token_key_version,
  purpose,
  stay_id,
  questionnaire_version_id,
  expires_at,
  created_at
) VALUES (
  sqlc.arg(id),
  sqlc.arg(token_hmac),
  sqlc.arg(token_key_version),
  'survey_response',
  sqlc.arg(stay_id),
  sqlc.arg(questionnaire_version_id),
  sqlc.arg(expires_at),
  sqlc.arg(created_at)
)
RETURNING *;

-- name: GetSurveyCapabilityByStayVersion :one
SELECT *
FROM survey.capabilities
WHERE stay_id = sqlc.arg(stay_id)
  AND questionnaire_version_id = sqlc.arg(questionnaire_version_id);

-- name: LockSurveyCapability :one
SELECT *
FROM survey.capabilities
WHERE token_hmac = sqlc.arg(token_hmac)
  AND purpose = 'survey_response'
FOR UPDATE;

-- name: ConsumeSurveyCapability :one
UPDATE survey.capabilities
SET consumed_at = sqlc.arg(consumed_at)
WHERE id = sqlc.arg(id)
  AND consumed_at IS NULL
  AND revoked_at IS NULL
  AND expires_at > sqlc.arg(consumed_at)
RETURNING *;

-- name: InsertSurveyResponse :exec
INSERT INTO survey.responses (
  id,
  stay_id,
  questionnaire_version_id,
  capability_id,
  client_submission_id,
  participation,
  submitted_at
) VALUES (
  sqlc.arg(id),
  sqlc.arg(stay_id),
  sqlc.arg(questionnaire_version_id),
  sqlc.arg(capability_id),
  sqlc.arg(client_submission_id),
  sqlc.arg(participation),
  sqlc.arg(submitted_at)
);

-- name: InsertSurveyAnswer :exec
INSERT INTO survey.answers (
  id,
  response_id,
  questionnaire_version_id,
  question_id,
  structured_value,
  encrypted_free_text,
  free_text_nonce,
  encryption_key_version,
  erase_after,
  created_at
) VALUES (
  sqlc.arg(id),
  sqlc.arg(response_id),
  sqlc.arg(questionnaire_version_id),
  sqlc.arg(question_id),
  sqlc.narg(structured_value),
  sqlc.narg(encrypted_free_text),
  sqlc.narg(free_text_nonce),
  sqlc.narg(encryption_key_version),
  sqlc.narg(erase_after),
  sqlc.arg(created_at)
);

-- name: InsertConsentDecision :exec
INSERT INTO survey.consent_decisions (
  id,
  response_id,
  questionnaire_version_id,
  purpose_code,
  notice_version,
  granted,
  collection_channel,
  recorded_at
) VALUES (
  sqlc.arg(id),
  sqlc.arg(response_id),
  sqlc.arg(questionnaire_version_id),
  sqlc.arg(purpose_code),
  sqlc.arg(notice_version),
  sqlc.arg(granted),
  'survey_capability',
  sqlc.arg(recorded_at)
);

-- name: EraseExpiredSurveyFreeText :one
SELECT survey.erase_expired_free_text(sqlc.arg(cutoff));
