package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Pantani/cumuru/apps/api/internal/accessrequest"
	"github.com/Pantani/cumuru/apps/api/internal/accommodation"
	"github.com/google/uuid"
)

type accessRequestPageResponse struct {
	Items      []accessrequest.Request `json:"items"`
	NextCursor *string                 `json:"next_cursor"`
}

type createAccessRequestBody struct {
	ClientSubmissionID   uuid.UUID              `json:"client_submission_id"`
	AccommodationName    string                 `json:"accommodation_name"`
	Category             accommodation.Category `json:"category"`
	Capacity             int32                  `json:"capacity"`
	ContactName          string                 `json:"contact_name"`
	ContactEmail         string                 `json:"contact_email"`
	ContactPhone         *string                `json:"contact_phone"`
	CityLabel            string                 `json:"city_label"`
	StateCode            string                 `json:"state_code"`
	PrivacyNoticeVersion string                 `json:"privacy_notice_version"`
	ProofOfWork          proofOfWorkRequest     `json:"proof_of_work"`
}

type rejectAccessRequestBody struct {
	ReasonCode accessrequest.RejectionReason `json:"reason_code"`
}

// The two open routes go through the same CORS wrapper as every other public
// surface. It gains no new header: unlike the poster and the activation, there
// is no capability token to present here — whoever arrives does not have an
// account yet — so Content-Type, Idempotency-Key and X-Request-ID already cover
// what the form sends.
func (d Dependencies) registerAccessRequestRoutes(mux *http.ServeMux, metrics *httpMetrics) {
	d.handleInviteRoute(
		mux, metrics, "GET /api/v1/accommodation-access-requests/context",
		d.getAccessRequestContext,
	)
	d.handleInviteRoute(
		mux, metrics, "POST /api/v1/accommodation-access-requests",
		d.createAccessRequest,
	)
	d.handleInviteRoute(
		mux, metrics, "OPTIONS /api/v1/accommodation-access-requests", emptyHandler,
	)
	d.handleInviteRoute(
		mux, metrics, "OPTIONS /api/v1/accommodation-access-requests/context", emptyHandler,
	)
	d.registerAccessRequestDecisionRoutes(mux, metrics)
}

// Approving a request produces the same effect as creating the record by hand,
// so it must not cost less permission than creating it by hand: the scope is the
// same accommodations:onboard of POST /accommodations.
func (d Dependencies) registerAccessRequestDecisionRoutes(
	mux *http.ServeMux,
	metrics *httpMetrics,
) {
	d.handleRoute(
		mux, metrics, "GET /api/v1/accommodation-access-requests",
		"accommodations:onboard", d.listAccessRequests,
	)
	d.handleRoute(
		mux, metrics, "POST /api/v1/accommodation-access-requests/{request_id}/approve",
		"accommodations:onboard", d.approveAccessRequest,
	)
	d.handleRoute(
		mux, metrics, "POST /api/v1/accommodation-access-requests/{request_id}/reject",
		"accommodations:onboard", d.rejectAccessRequest,
	)
}

func (d Dependencies) getAccessRequestContext(
	writer http.ResponseWriter,
	request *http.Request,
) {
	subject, ok := d.requestRateSubject(writer, request)
	if !ok {
		return
	}
	result, err := d.AccessRequests.Context(
		request.Context(), accessrequest.ContextRequest{RateSubject: subject},
	)
	if err != nil {
		d.writeAccessRequestError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

// The response returns only id and created_at, and never echoes what was sent:
// the route is open, and the echo would turn creation into a lookup of somebody
// else's contact details.
func (d Dependencies) createAccessRequest(
	writer http.ResponseWriter,
	request *http.Request,
) {
	subject, ok := d.requestRateSubject(writer, request)
	if !ok {
		return
	}
	var body createAccessRequestBody
	if err := decodeStrict(request, "application/json", &body); err != nil {
		writeBadRequest(writer, request)
		return
	}
	result, replayed, err := d.AccessRequests.Create(
		request.Context(), accessRequestCommand(request, body, subject),
	)
	if err != nil {
		d.writeAccessRequestError(writer, request, err)
		return
	}
	writer.Header().Set("Idempotency-Replayed", strconv.FormatBool(replayed))
	writeJSON(writer, http.StatusCreated, result)
}

func accessRequestCommand(
	request *http.Request,
	body createAccessRequestBody,
	subject string,
) accessrequest.CreateCommand {
	return accessrequest.CreateCommand{
		RateSubject: subject, ClientSubmissionID: body.ClientSubmissionID,
		AccommodationName: body.AccommodationName, Category: body.Category,
		Capacity: body.Capacity, ContactName: body.ContactName,
		ContactEmail: body.ContactEmail,
		ContactPhone: valueOrZero(body.ContactPhone),
		CityLabel:    body.CityLabel, StateCode: body.StateCode,
		PrivacyNoticeVersion: body.PrivacyNoticeVersion,
		ProofOfWork: accessrequest.ProofOfWorkAnswer{
			Challenge: body.ProofOfWork.Challenge,
			Solution:  body.ProofOfWork.Solution,
		},
		IdempotencyKey: request.Header.Get("Idempotency-Key"),
		RequestID:      requestID(request),
	}
}

func (d Dependencies) listAccessRequests(
	writer http.ResponseWriter,
	request *http.Request,
) {
	query := request.URL.Query()
	limit, cursor, err := parsePage(d.cursor, query.Get("limit"), query.Get("cursor"))
	if err != nil {
		writeBadRequest(writer, request)
		return
	}
	page, err := d.AccessRequests.List(request.Context(), accessrequest.PageRequest{
		CursorCreatedAt: cursor.CreatedAt, CursorID: cursor.ID, Limit: limit,
		State: accessrequest.ApprovalState(query.Get("approval_state")),
	})
	if err != nil {
		d.writeAccessRequestError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, accessRequestPageResponse{
		Items: page.Items, NextCursor: d.accessRequestCursor(page.NextCursor),
	})
}

func (d Dependencies) accessRequestCursor(cursor *accessrequest.PageCursor) *string {
	if cursor == nil {
		return nil
	}
	return d.cursor.encode(pageCursor{CreatedAt: cursor.CreatedAt, ID: cursor.ID})
}

func (d Dependencies) approveAccessRequest(
	writer http.ResponseWriter,
	request *http.Request,
) {
	id, version, ok := mutationTarget(writer, request, "request_id")
	if !ok {
		return
	}
	if err := decodeStrict(request, "application/json", &struct{}{}); err != nil {
		writeBadRequest(writer, request)
		return
	}
	result, replayed, err := d.AccessRequests.Approve(
		request.Context(), accessrequest.ApprovalCommand{
			Actor: requestPrincipal(request), AccessRequestID: id,
			ExpectedVersion: version,
			IdempotencyKey:  request.Header.Get("Idempotency-Key"),
			RequestID:       requestID(request),
		},
	)
	if err != nil {
		d.writeAccessRequestError(writer, request, err)
		return
	}
	writeMutationSuccess(writer, http.StatusOK, result.Version, replayed, result)
}

func (d Dependencies) rejectAccessRequest(
	writer http.ResponseWriter,
	request *http.Request,
) {
	id, version, ok := mutationTarget(writer, request, "request_id")
	if !ok {
		return
	}
	var body rejectAccessRequestBody
	if err := decodeStrict(request, "application/json", &body); err != nil {
		writeBadRequest(writer, request)
		return
	}
	result, replayed, err := d.AccessRequests.Reject(
		request.Context(), accessrequest.RejectionCommand{
			Actor: requestPrincipal(request), AccessRequestID: id,
			ExpectedVersion: version, ReasonCode: body.ReasonCode,
			IdempotencyKey: request.Header.Get("Idempotency-Key"),
			RequestID:      requestID(request),
		},
	)
	if err != nil {
		d.writeAccessRequestError(writer, request, err)
		return
	}
	writeMutationSuccess(writer, http.StatusOK, result.Version, replayed, result)
}

// The contract of this surface has no 422 on any of the five routes: a
// malformed field is a 400, from a refused proof of work to a reason outside the
// closed list. Mapping to 422 here would produce an answer the generated client
// cannot read. Everything else goes to the shared table, including the 409 of
// the duplicate pending e-mail, which carries no Retry-After on purpose:
// re-sending does not resolve it, the request is already in the queue and only a
// decision takes it out.
func (d Dependencies) writeAccessRequestError(
	writer http.ResponseWriter,
	request *http.Request,
	err error,
) {
	if errors.Is(err, accessrequest.ErrInvalidInput) {
		writeBadRequest(writer, request)
		return
	}
	d.writeServiceError(writer, request, err)
}
