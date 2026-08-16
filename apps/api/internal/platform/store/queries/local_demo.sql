-- name: AcquireLocalDemoRunLock :exec
SELECT pg_catalog.pg_advisory_lock(4852042520260729);

-- name: ReleaseLocalDemoRunLock :one
SELECT pg_catalog.pg_advisory_unlock(4852042520260729);

-- name: GetLocalDemoOrganization :one
SELECT name
FROM core.organizations
WHERE id = sqlc.arg(id);

-- name: InsertLocalDemoOrganization :exec
INSERT INTO core.organizations (id, name)
VALUES (sqlc.arg(id), sqlc.arg(name))
ON CONFLICT DO NOTHING;

-- name: GetLocalDemoAccommodation :one
SELECT
  organization_id,
  name,
  category,
  status,
  cadastur_id,
  capacity,
  public_area_code
FROM core.accommodations
WHERE id = sqlc.arg(id);

-- name: InsertLocalDemoAccommodation :exec
INSERT INTO core.accommodations (
  id,
  organization_id,
  name,
  category,
  status,
  cadastur_id,
  capacity,
  public_area_code
)
VALUES (
  sqlc.arg(id),
  sqlc.arg(organization_id),
  sqlc.arg(name),
  sqlc.arg(category),
  'active',
  sqlc.narg(cadastur_id),
  sqlc.arg(capacity),
  sqlc.arg(public_area_code)
)
ON CONFLICT DO NOTHING;

-- name: GetLocalDemoMembership :one
SELECT
  accommodation_id,
  oidc_issuer,
  oidc_subject,
  role,
  active
FROM core.memberships
WHERE id = sqlc.arg(id);

-- name: InsertLocalDemoMembership :exec
INSERT INTO core.memberships (
  id,
  accommodation_id,
  oidc_issuer,
  oidc_subject,
  role
)
VALUES (
  sqlc.arg(id),
  sqlc.arg(accommodation_id),
  sqlc.arg(oidc_issuer),
  sqlc.arg(oidc_subject),
  'manager'
)
ON CONFLICT DO NOTHING;

-- name: GetLocalDemoAccount :one
SELECT
  email,
  display_name,
  scopes,
  status
FROM auth.accounts
WHERE id = sqlc.arg(id);

-- name: InsertLocalDemoAccount :exec
INSERT INTO auth.accounts (
  id,
  email,
  display_name,
  password_hash,
  scopes
)
VALUES (
  sqlc.arg(id),
  sqlc.arg(email),
  sqlc.arg(display_name),
  sqlc.arg(password_hash),
  sqlc.arg(scopes)
)
ON CONFLICT DO NOTHING;

-- name: GetLocalDemoMetricMapping :one
SELECT category_code
FROM analytics.metric_mappings
WHERE privacy_policy_version = sqlc.arg(privacy_policy_version)
  AND metric_code = sqlc.arg(metric_code)
  AND questionnaire_version_id = sqlc.arg(questionnaire_version_id)
  AND question_id = sqlc.arg(question_id)
  AND source_value = sqlc.arg(source_value);

-- name: InsertLocalDemoMetricMapping :exec
INSERT INTO analytics.metric_mappings (
  privacy_policy_version,
  metric_code,
  questionnaire_version_id,
  question_id,
  source_value,
  category_code
)
VALUES (
  sqlc.arg(privacy_policy_version),
  sqlc.arg(metric_code),
  sqlc.arg(questionnaire_version_id),
  sqlc.arg(question_id),
  sqlc.arg(source_value),
  sqlc.arg(category_code)
)
ON CONFLICT DO NOTHING;

-- name: FindLocalDemoStay :one
SELECT id
FROM core.stays
WHERE accommodation_id = sqlc.arg(accommodation_id)
  AND client_submission_id = sqlc.arg(client_submission_id);

-- name: HasLocalDemoGroupSubmission :one
SELECT EXISTS (
  SELECT 1
  FROM core.group_submissions
  WHERE stay_id = sqlc.arg(stay_id)
);

-- name: HasLocalDemoSurveyResponse :one
SELECT EXISTS (
  SELECT 1
  FROM survey.responses
  WHERE stay_id = sqlc.arg(stay_id)
    AND questionnaire_version_id = sqlc.arg(questionnaire_version_id)
    AND client_submission_id = sqlc.arg(client_submission_id)
);
