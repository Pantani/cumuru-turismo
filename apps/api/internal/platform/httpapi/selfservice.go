package httpapi

import (
	"net/http"

	"github.com/Pantani/cumuru/apps/api/internal/activation"
	"github.com/Pantani/cumuru/apps/api/internal/stay"
	"github.com/google/uuid"
)

const (
	inviteTokenHeader     = "X-Cumuru-Invite-Token"
	activationTokenHeader = "X-Cumuru-Activation-Token"
)

type createAccommodationInviteRequest struct {
	PrivacyNoticeVersion string `json:"privacy_notice_version"`
	MaxUses              *int32 `json:"max_uses"`
}

type selfServiceVisitorRequest struct {
	ClientID          string           `json:"client_id"`
	Role              stay.VisitorRole `json:"role"`
	AgeBand           stay.AgeBand     `json:"age_band"`
	ResidenceCountry  string           `json:"residence_country"`
	ResidenceState    *string          `json:"residence_state"`
	ResidenceCityCode *string          `json:"residence_city_code"`
}

type proofOfWorkRequest struct {
	Challenge string `json:"challenge"`
	Solution  string `json:"solution"`
}

// There is no expected_guest_count and there is no identity field. The wire
// type has nowhere to put a name, a document, an e-mail or a phone number, so
// one cannot arrive and be dropped silently: with additionalProperties false in
// the contract it is a 400 at decode.
type selfRegistrationRequest struct {
	ClientSubmissionID   uuid.UUID                   `json:"client_submission_id"`
	PrivacyNoticeVersion string                      `json:"privacy_notice_version"`
	PlannedArrivalOn     string                      `json:"planned_arrival_on"`
	PlannedDepartureOn   string                      `json:"planned_departure_on"`
	Visitors             []selfServiceVisitorRequest `json:"visitors"`
	ProofOfWork          proofOfWorkRequest          `json:"proof_of_work"`
}

type rejectStayRequest struct {
	ReasonCode stay.RejectionReason `json:"reason_code"`
}

type createActivationRequest struct {
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

type activateAccountRequest struct {
	Password string `json:"password"`
}

func (d Dependencies) registerSelfServiceRoutes(mux *http.ServeMux, metrics *httpMetrics) {
	d.handleRoute(
		mux, metrics, "POST /api/v1/accommodations/{accommodation_id}/invite",
		"accommodations:manage", d.createAccommodationInvite,
	)
	d.handleRoute(
		mux, metrics, "GET /api/v1/accommodations/{accommodation_id}/invite",
		"accommodations:manage", d.getAccommodationInvite,
	)
	d.handleRoute(
		mux, metrics, "POST /api/v1/accommodations/{accommodation_id}/invite/revoke",
		"accommodations:manage", d.revokeAccommodationInvite,
	)
	d.handleRoute(
		mux, metrics, "POST /api/v1/stays/{stay_id}/approve",
		"stays:approve", d.approveStay,
	)
	d.handleRoute(
		mux, metrics, "POST /api/v1/stays/{stay_id}/reject",
		"stays:approve", d.rejectStay,
	)
	d.registerPosterRoutes(mux, metrics)
}

// The open surfaces reuse the invite CORS wrapper, which is what makes the
// custom capability header force a preflight instead of allowing a simple form
// POST from any origin.
func (d Dependencies) registerPosterRoutes(mux *http.ServeMux, metrics *httpMetrics) {
	d.handleInviteRoute(
		mux, metrics, "GET /api/v1/accommodation-invite", d.getAccommodationInviteContext,
	)
	d.handleInviteRoute(
		mux, metrics, "POST /api/v1/accommodation-invite/submit", d.submitSelfRegistration,
	)
	d.handleInviteRoute(
		mux, metrics, "OPTIONS /api/v1/accommodation-invite", emptyHandler,
	)
	d.handleInviteRoute(
		mux, metrics, "OPTIONS /api/v1/accommodation-invite/submit", emptyHandler,
	)
}

func (d Dependencies) registerActivationRoutes(mux *http.ServeMux, metrics *httpMetrics) {
	d.handleRoute(
		mux, metrics, "POST /api/v1/accommodations/{accommodation_id}/activation",
		"accommodations:manage", d.createAccommodationActivation,
	)
	d.handleInviteRoute(mux, metrics, "GET /api/v1/activation", d.getActivationContext)
	d.handleInviteRoute(
		mux, metrics, "POST /api/v1/activation/complete", d.completeActivation,
	)
	d.handleInviteRoute(mux, metrics, "OPTIONS /api/v1/activation", emptyHandler)
	d.handleInviteRoute(mux, metrics, "OPTIONS /api/v1/activation/complete", emptyHandler)
}

func (d Dependencies) createAccommodationInvite(
	writer http.ResponseWriter,
	request *http.Request,
) {
	id, version, ok := mutationTarget(writer, request, "accommodation_id")
	if !ok {
		return
	}
	var body createAccommodationInviteRequest
	if err := decodeStrict(request, "application/json", &body); err != nil {
		writeBadRequest(writer, request)
		return
	}
	result, replayed, err := d.Stays.CreateAccommodationInvite(
		request.Context(), stay.AccommodationInviteCommand{
			Actor: requestPrincipal(request), AccommodationID: id,
			PrivacyNoticeVersion: body.PrivacyNoticeVersion, MaxUses: body.MaxUses,
			ExpectedVersion: version,
			IdempotencyKey:  request.Header.Get("Idempotency-Key"),
			RequestID:       requestID(request),
		},
	)
	if err != nil {
		d.writeServiceError(writer, request, err)
		return
	}
	writer.Header().Set("Location", accommodationInvitePath(id))
	writeMutationSuccess(writer, http.StatusCreated, result.Version, replayed, result)
}

func accommodationInvitePath(id uuid.UUID) string {
	return "/api/v1/accommodations/" + id.String() + "/invite"
}

func (d Dependencies) getAccommodationInvite(
	writer http.ResponseWriter,
	request *http.Request,
) {
	id, ok := pathUUID(writer, request, "accommodation_id")
	if !ok {
		return
	}
	result, err := d.Stays.GetAccommodationInvite(
		request.Context(), requestPrincipal(request), id,
	)
	if err != nil {
		d.writeServiceError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

func (d Dependencies) revokeAccommodationInvite(
	writer http.ResponseWriter,
	request *http.Request,
) {
	id, version, ok := mutationTarget(writer, request, "accommodation_id")
	if !ok {
		return
	}
	if err := decodeStrict(request, "application/json", &struct{}{}); err != nil {
		writeBadRequest(writer, request)
		return
	}
	result, replayed, err := d.Stays.RevokeAccommodationInvite(
		request.Context(), stay.AccommodationInviteRevokeCommand{
			Actor: requestPrincipal(request), AccommodationID: id,
			ExpectedVersion: version,
			IdempotencyKey:  request.Header.Get("Idempotency-Key"),
			RequestID:       requestID(request),
		},
	)
	if err != nil {
		d.writeServiceError(writer, request, err)
		return
	}
	writeMutationSuccess(writer, http.StatusOK, result.Version, replayed, result)
}

// The token arrives in a header, read by the client from the URL fragment. It
// never appears in the request line, so it reaches no access log, WAF or CDN.
func (d Dependencies) getAccommodationInviteContext(
	writer http.ResponseWriter,
	request *http.Request,
) {
	subject, ok := d.requestRateSubject(writer, request)
	if !ok {
		return
	}
	result, err := d.Stays.GetAccommodationInviteContext(
		request.Context(), stay.InviteRequest{
			Token: request.Header.Get(inviteTokenHeader), RateSubject: subject,
		},
	)
	if err != nil {
		d.writeServiceError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

func (d Dependencies) submitSelfRegistration(
	writer http.ResponseWriter,
	request *http.Request,
) {
	subject, ok := d.requestRateSubject(writer, request)
	if !ok {
		return
	}
	var body selfRegistrationRequest
	if err := decodeStrict(request, "application/json", &body); err != nil {
		writeBadRequest(writer, request)
		return
	}
	result, replayed, err := d.Stays.SubmitSelfRegistration(
		request.Context(), selfRegistrationCommand(request, body, subject),
	)
	if err != nil {
		d.writeServiceError(writer, request, err)
		return
	}
	writeMutationSuccess(writer, http.StatusOK, result.Version, replayed, result)
}

func selfRegistrationCommand(
	request *http.Request,
	body selfRegistrationRequest,
	subject string,
) stay.SelfRegistrationCommand {
	return stay.SelfRegistrationCommand{
		Token:       request.Header.Get(inviteTokenHeader),
		RateSubject: subject, ClientSubmissionID: body.ClientSubmissionID,
		PrivacyNoticeVersion: body.PrivacyNoticeVersion,
		PlannedArrivalOn:     body.PlannedArrivalOn,
		PlannedDepartureOn:   body.PlannedDepartureOn,
		Visitors:             selfServiceVisitors(body.Visitors),
		ProofOfWork: stay.ProofOfWorkAnswer{
			Challenge: body.ProofOfWork.Challenge, Solution: body.ProofOfWork.Solution,
		},
		IdempotencyKey: request.Header.Get("Idempotency-Key"),
		RequestID:      requestID(request),
	}
}

func selfServiceVisitors(values []selfServiceVisitorRequest) []stay.SelfServiceVisitor {
	result := make([]stay.SelfServiceVisitor, 0, len(values))
	for _, value := range values {
		result = append(result, stay.SelfServiceVisitor{
			ClientID: value.ClientID, Role: value.Role, AgeBand: value.AgeBand,
			ResidenceCountry:  value.ResidenceCountry,
			ResidenceState:    valueOrZero(value.ResidenceState),
			ResidenceCityCode: valueOrZero(value.ResidenceCityCode),
		})
	}
	return result
}

// The scope is stays:approve, not stays:write. An operator holding only the
// edit permission is refused before the handler even runs (N-25).
func (d Dependencies) approveStay(writer http.ResponseWriter, request *http.Request) {
	id, version, ok := mutationTarget(writer, request, "stay_id")
	if !ok {
		return
	}
	if err := decodeStrict(request, "application/json", &struct{}{}); err != nil {
		writeBadRequest(writer, request)
		return
	}
	result, replayed, err := d.Stays.Approve(request.Context(), stay.ApprovalCommand{
		Actor: requestPrincipal(request), StayID: id, ExpectedVersion: version,
		IdempotencyKey: request.Header.Get("Idempotency-Key"),
		RequestID:      requestID(request),
	})
	if err != nil {
		d.writeServiceError(writer, request, err)
		return
	}
	writeMutationSuccess(writer, http.StatusOK, result.Version, replayed, result)
}

func (d Dependencies) rejectStay(writer http.ResponseWriter, request *http.Request) {
	id, version, ok := mutationTarget(writer, request, "stay_id")
	if !ok {
		return
	}
	var body rejectStayRequest
	if err := decodeStrict(request, "application/json", &body); err != nil {
		writeBadRequest(writer, request)
		return
	}
	result, replayed, err := d.Stays.Reject(request.Context(), stay.RejectionCommand{
		Actor: requestPrincipal(request), StayID: id, ExpectedVersion: version,
		ReasonCode:     body.ReasonCode,
		IdempotencyKey: request.Header.Get("Idempotency-Key"),
		RequestID:      requestID(request),
	})
	if err != nil {
		d.writeServiceError(writer, request, err)
		return
	}
	writeMutationSuccess(writer, http.StatusOK, result.Version, replayed, result)
}

func (d Dependencies) createAccommodationActivation(
	writer http.ResponseWriter,
	request *http.Request,
) {
	id, version, ok := mutationTarget(writer, request, "accommodation_id")
	if !ok {
		return
	}
	var body createActivationRequest
	if err := decodeStrict(request, "application/json", &body); err != nil {
		writeBadRequest(writer, request)
		return
	}
	result, replayed, err := d.Activation.Create(
		request.Context(), activation.CreateCommand{
			Actor: requestPrincipal(request), AccommodationID: id,
			Email: body.Email, DisplayName: body.DisplayName,
			ExpectedVersion: version,
			IdempotencyKey:  request.Header.Get("Idempotency-Key"),
			RequestID:       requestID(request),
		},
	)
	if err != nil {
		d.writeServiceError(writer, request, err)
		return
	}
	writer.Header().Set(
		"Location", "/api/v1/accommodations/"+id.String()+"/activation",
	)
	writeMutationSuccess(writer, http.StatusCreated, result.Version, replayed, result)
}

func (d Dependencies) getActivationContext(
	writer http.ResponseWriter,
	request *http.Request,
) {
	subject, ok := d.requestRateSubject(writer, request)
	if !ok {
		return
	}
	result, err := d.Activation.Context(request.Context(), activation.Request{
		Token: request.Header.Get(activationTokenHeader), RateSubject: subject,
	})
	if err != nil {
		d.writeServiceError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

// The answer is 204 with no body and no session, for the same reason
// POST /auth/password is: no response may echo the secret, and the capability
// must not become a long-lived credential. The operator logs in afterwards.
func (d Dependencies) completeActivation(
	writer http.ResponseWriter,
	request *http.Request,
) {
	subject, ok := d.requestRateSubject(writer, request)
	if !ok {
		return
	}
	var body activateAccountRequest
	if err := decodeStrict(request, "application/json", &body); err != nil {
		writeBadRequest(writer, request)
		return
	}
	err := d.Activation.Complete(request.Context(), activation.CompleteCommand{
		Token: request.Header.Get(activationTokenHeader), RateSubject: subject,
		Password: body.Password, RequestID: requestID(request),
	})
	if err != nil {
		d.writeServiceError(writer, request, err)
		return
	}
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(http.StatusNoContent)
}
