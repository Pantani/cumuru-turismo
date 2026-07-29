package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/questionnaire"
	"github.com/google/uuid"
)

var questionnaireReadScopes = []string{"questionnaires:manage", "questionnaires:approve"}

type createQuestionnaireRequest struct {
	ID                   uuid.UUID `json:"id"`
	VersionID            uuid.UUID `json:"version_id"`
	StableKey            string    `json:"stable_key"`
	Name                 string    `json:"name"`
	Title                string    `json:"title"`
	PrivacyNoticeVersion string    `json:"privacy_notice_version"`
}

type cloneQuestionnaireRequest struct {
	SourceVersionID uuid.UUID `json:"source_version_id"`
	NewVersionID    uuid.UUID `json:"new_version_id"`
}

type requestChangesRequest struct {
	ReasonCode string `json:"reason_code"`
}

type surveySubmissionRequest struct {
	VersionID        uuid.UUID                            `json:"questionnaire_version_id"`
	ClientSubmission uuid.UUID                            `json:"client_submission_id"`
	Participation    questionnaire.Participation          `json:"participation"`
	Answers          []questionnaire.AnswerInput          `json:"answers"`
	Consents         []questionnaire.ConsentDecisionInput `json:"consent_decisions"`
}

type questionnairePageResponse struct {
	Items      []questionnaire.Questionnaire `json:"items"`
	NextCursor *string                       `json:"next_cursor"`
}

type versionPageResponse struct {
	Items      []questionnaire.VersionSummary `json:"items"`
	NextCursor *string                        `json:"next_cursor"`
}

func (d Dependencies) registerQuestionnaireRoutes(mux *http.ServeMux, metrics *httpMetrics) {
	d.handleRoute(mux, metrics, "GET /api/v1/questionnaires", "questionnaires:manage", d.listQuestionnaires)
	d.handleRoute(mux, metrics, "POST /api/v1/questionnaires", "questionnaires:manage", d.createQuestionnaire)
	d.handleRoute(mux, metrics, "GET /api/v1/questionnaires/{questionnaire_id}", "questionnaires:manage", d.getQuestionnaire)
	d.handleAnyScopeRoute(mux, metrics, "GET /api/v1/questionnaires/{questionnaire_id}/versions", questionnaireReadScopes, d.listQuestionnaireVersions)
	d.handleRoute(mux, metrics, "POST /api/v1/questionnaires/{questionnaire_id}/versions", "questionnaires:manage", d.cloneQuestionnaireVersion)
	d.handleAnyScopeRoute(mux, metrics, "GET /api/v1/questionnaire-versions/{version_id}", questionnaireReadScopes, d.getQuestionnaireVersion)
	d.handleRoute(mux, metrics, "PUT /api/v1/questionnaire-versions/{version_id}", "questionnaires:manage", d.updateQuestionnaireVersion)
	d.registerQuestionnaireTransitions(mux, metrics)
	d.handlePublicQuestionnaireRoute(mux, metrics, "GET /api/v1/questionnaires/{stable_key}/active", d.getActiveQuestionnaire)
	d.handlePublicQuestionnaireRoute(mux, metrics, "POST /api/v1/survey-responses", d.submitSurveyResponse)
	d.handlePublicQuestionnaireRoute(
		mux, metrics,
		"OPTIONS /api/v1/questionnaires/{stable_key}/active",
		emptyHandler,
	)
	d.handlePublicQuestionnaireRoute(
		mux, metrics, "OPTIONS /api/v1/survey-responses", emptyHandler,
	)
}

func (d Dependencies) registerQuestionnaireTransitions(mux *http.ServeMux, metrics *httpMetrics) {
	d.handleRoute(mux, metrics, "POST /api/v1/questionnaire-versions/{version_id}/submit-review", "questionnaires:manage", d.submitQuestionnaireReview)
	d.handleRoute(mux, metrics, "POST /api/v1/questionnaire-versions/{version_id}/request-changes", "questionnaires:approve", d.requestQuestionnaireChanges)
	d.handleRoute(mux, metrics, "POST /api/v1/questionnaire-versions/{version_id}/approve", "questionnaires:approve", d.approveQuestionnaireVersion)
	d.handleRoute(mux, metrics, "POST /api/v1/questionnaire-versions/{version_id}/publish", "questionnaires:approve", d.publishQuestionnaireVersion)
	d.handleRoute(mux, metrics, "POST /api/v1/questionnaire-versions/{version_id}/retire", "questionnaires:approve", d.retireQuestionnaireVersion)
}

func emptyHandler(http.ResponseWriter, *http.Request) {}

func (d Dependencies) handlePublicQuestionnaireRoute(
	mux *http.ServeMux,
	metrics *httpMetrics,
	pattern string,
	handler func(http.ResponseWriter, *http.Request),
) {
	_, route, _ := stringsCut(pattern)
	wrapped := d.inviteCORS(http.HandlerFunc(handler))
	mux.Handle(pattern, d.routeHandler(route, metrics, wrapped))
}

func (d Dependencies) listQuestionnaires(writer http.ResponseWriter, request *http.Request) {
	limit, cursor, err := parsePage(d.cursor, request.URL.Query().Get("limit"), request.URL.Query().Get("cursor"))
	if err != nil {
		writeBadRequest(writer, request)
		return
	}
	page, err := d.Questionnaires.List(request.Context(), requestPrincipal(request), questionnaire.PageRequest{
		CursorCreatedAt: cursor.CreatedAt, CursorID: cursor.ID, Limit: limit,
	})
	if err != nil {
		d.writeServiceError(writer, request, err)
		return
	}
	var next *string
	if page.NextCursor != nil {
		next = d.cursor.encode(pageCursor{CreatedAt: page.NextCursor.CreatedAt, ID: page.NextCursor.ID})
	}
	writeJSON(writer, http.StatusOK, questionnairePageResponse{Items: page.Items, NextCursor: next})
}

func (d Dependencies) createQuestionnaire(writer http.ResponseWriter, request *http.Request) {
	var body createQuestionnaireRequest
	if decodeStrict(request, "application/json", &body) != nil {
		writeBadRequest(writer, request)
		return
	}
	result, replayed, err := d.Questionnaires.Create(request.Context(), questionnaire.CreateCommand{
		Actor: requestPrincipal(request), ID: body.ID, VersionID: body.VersionID,
		StableKey: body.StableKey, Name: body.Name, Title: body.Title,
		PrivacyNoticeVersion: body.PrivacyNoticeVersion,
		IdempotencyKey:       request.Header.Get("Idempotency-Key"), RequestID: requestID(request),
	})
	if err != nil {
		d.writeServiceError(writer, request, err)
		return
	}
	writer.Header().Set("Location", "/api/v1/questionnaire-versions/"+result.ID.String())
	writeMutationSuccess(writer, http.StatusCreated, result.Revision, replayed, result)
}

func (d Dependencies) getQuestionnaire(writer http.ResponseWriter, request *http.Request) {
	id, ok := pathUUID(writer, request, "questionnaire_id")
	if !ok {
		return
	}
	result, err := d.Questionnaires.Get(request.Context(), requestPrincipal(request), id)
	if err != nil {
		d.writeServiceError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

func (d Dependencies) listQuestionnaireVersions(writer http.ResponseWriter, request *http.Request) {
	id, ok := pathUUID(writer, request, "questionnaire_id")
	if !ok {
		return
	}
	limit, cursor, err := parsePage(d.cursor, request.URL.Query().Get("limit"), request.URL.Query().Get("cursor"))
	if err != nil {
		writeBadRequest(writer, request)
		return
	}
	page, err := d.Questionnaires.ListVersions(
		request.Context(), requestPrincipal(request), id,
		questionnaire.VersionPageRequest{
			CursorVersionNumber: cursorVersionNumber(cursor), CursorID: cursor.ID, Limit: limit,
		},
	)
	if err != nil {
		d.writeServiceError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, versionPageResponse{
		Items: page.Items, NextCursor: d.versionCursor(page.NextCursor),
	})
}

func cursorVersionNumber(cursor pageCursor) int32 {
	if cursor.CreatedAt.IsZero() {
		return 0
	}
	return int32(cursor.CreatedAt.Unix())
}

func (d Dependencies) versionCursor(cursor *questionnaire.VersionCursor) *string {
	if cursor == nil {
		return nil
	}
	return d.cursor.encode(pageCursor{
		CreatedAt: time.Unix(int64(cursor.VersionNumber), 0).UTC(), ID: cursor.ID,
	})
}

func (d Dependencies) cloneQuestionnaireVersion(writer http.ResponseWriter, request *http.Request) {
	id, ok := pathUUID(writer, request, "questionnaire_id")
	if !ok {
		return
	}
	var body cloneQuestionnaireRequest
	if decodeStrict(request, "application/json", &body) != nil {
		writeBadRequest(writer, request)
		return
	}
	result, replayed, err := d.Questionnaires.Clone(request.Context(), questionnaire.CloneCommand{
		Actor: requestPrincipal(request), QuestionnaireID: id,
		SourceVersionID: body.SourceVersionID, NewVersionID: body.NewVersionID,
		IdempotencyKey: request.Header.Get("Idempotency-Key"), RequestID: requestID(request),
	})
	if err != nil {
		d.writeServiceError(writer, request, err)
		return
	}
	writer.Header().Set("Location", "/api/v1/questionnaire-versions/"+result.ID.String())
	writeMutationSuccess(writer, http.StatusCreated, result.Revision, replayed, result)
}

func (d Dependencies) getQuestionnaireVersion(writer http.ResponseWriter, request *http.Request) {
	id, ok := pathUUID(writer, request, "version_id")
	if !ok {
		return
	}
	result, err := d.Questionnaires.GetVersion(request.Context(), requestPrincipal(request), id)
	if err != nil {
		d.writeServiceError(writer, request, err)
		return
	}
	writer.Header().Set("ETag", etag(result.Revision))
	writeJSON(writer, http.StatusOK, result)
}

func (d Dependencies) updateQuestionnaireVersion(writer http.ResponseWriter, request *http.Request) {
	id, revision, ok := mutationTarget(writer, request, "version_id")
	if !ok {
		return
	}
	var body questionnaire.Definition
	if decodeStrict(request, "application/json", &body) != nil {
		writeBadRequest(writer, request)
		return
	}
	result, err := d.Questionnaires.UpdateVersion(request.Context(), questionnaire.UpdateCommand{
		Actor: requestPrincipal(request), VersionID: id,
		ExpectedVersion: revision, Definition: body, RequestID: requestID(request),
	})
	if err != nil {
		d.writeServiceError(writer, request, err)
		return
	}
	writer.Header().Set("ETag", etag(result.Revision))
	writeJSON(writer, http.StatusOK, result)
}

func (d Dependencies) submitQuestionnaireReview(writer http.ResponseWriter, request *http.Request) {
	d.runQuestionnaireTransition(writer, request, questionnaire.TransitionSubmitReview, "")
}

func (d Dependencies) requestQuestionnaireChanges(writer http.ResponseWriter, request *http.Request) {
	var body requestChangesRequest
	if decodeStrict(request, "application/json", &body) != nil {
		writeBadRequest(writer, request)
		return
	}
	d.runQuestionnaireTransition(writer, request, questionnaire.TransitionRequestChanges, body.ReasonCode)
}

func (d Dependencies) approveQuestionnaireVersion(writer http.ResponseWriter, request *http.Request) {
	d.runQuestionnaireTransition(writer, request, questionnaire.TransitionApprove, "")
}

func (d Dependencies) publishQuestionnaireVersion(writer http.ResponseWriter, request *http.Request) {
	d.runQuestionnaireTransition(writer, request, questionnaire.TransitionPublish, "")
}

func (d Dependencies) retireQuestionnaireVersion(writer http.ResponseWriter, request *http.Request) {
	d.runQuestionnaireTransition(writer, request, questionnaire.TransitionRetire, "")
}

func (d Dependencies) runQuestionnaireTransition(
	writer http.ResponseWriter,
	request *http.Request,
	transition questionnaire.Transition,
	reason string,
) {
	id, revision, ok := mutationTarget(writer, request, "version_id")
	if !ok {
		return
	}
	if transition != questionnaire.TransitionRequestChanges {
		var body struct{}
		if decodeStrict(request, "application/json", &body) != nil {
			writeBadRequest(writer, request)
			return
		}
	}
	result, replayed, err := d.Questionnaires.Transition(request.Context(), questionnaire.TransitionCommand{
		Actor: requestPrincipal(request), VersionID: id, ExpectedVersion: revision,
		Transition: transition, ReasonCode: reason,
		IdempotencyKey: request.Header.Get("Idempotency-Key"), RequestID: requestID(request),
	})
	if err != nil {
		d.writeServiceError(writer, request, err)
		return
	}
	writeMutationSuccess(writer, http.StatusOK, result.Revision, replayed, result)
}

func (d Dependencies) getActiveQuestionnaire(writer http.ResponseWriter, request *http.Request) {
	result, err := d.Questionnaires.GetPublished(request.Context(), request.PathValue("stable_key"))
	if err != nil {
		d.writeServiceError(writer, request, err)
		return
	}
	writer.Header().Set("ETag", etag(result.Revision))
	writeJSON(writer, http.StatusOK, result)
}

func (d Dependencies) submitSurveyResponse(writer http.ResponseWriter, request *http.Request) {
	capability := request.Header.Get("Survey-Capability")
	if !questionnaire.ValidCapabilitySyntax(capability) {
		d.writeServiceError(writer, request, questionnaire.ErrCapabilityInvalid)
		return
	}
	subject, ok := d.requestRateSubject(writer, request)
	if !ok {
		return
	}
	var body surveySubmissionRequest
	if decodeStrict(request, "application/json", &body) != nil {
		writeBadRequest(writer, request)
		return
	}
	result, replayed, err := d.Questionnaires.Submit(request.Context(), questionnaire.SubmissionCommand{
		Capability: capability, RateSubject: subject,
		VersionID: body.VersionID, ClientSubmission: body.ClientSubmission,
		Participation: body.Participation, Answers: body.Answers, Consents: body.Consents,
		IdempotencyKey: request.Header.Get("Idempotency-Key"), RequestID: requestID(request),
	})
	if err != nil {
		d.writeServiceError(writer, request, err)
		return
	}
	writer.Header().Set("Idempotency-Replayed", boolString(replayed))
	writeJSON(writer, http.StatusOK, result)
}

func boolString(value bool) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}
