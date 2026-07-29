package httpapi

import (
	"net/http"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/stay"
	"github.com/google/uuid"
)

type createStayRequest struct {
	AccommodationID    uuid.UUID `json:"accommodation_id"`
	PlannedArrivalOn   string    `json:"planned_arrival_on"`
	PlannedDepartureOn string    `json:"planned_departure_on"`
	ExpectedGuestCount int32     `json:"expected_guest_count"`
	ClientSubmissionID uuid.UUID `json:"client_submission_id"`
}

type updateStayRequest struct {
	PlannedArrivalOn   requiredPatch[string] `json:"planned_arrival_on"`
	PlannedDepartureOn requiredPatch[string] `json:"planned_departure_on"`
	ExpectedGuestCount requiredPatch[int32]  `json:"expected_guest_count"`
}

type visitorRequest struct {
	ClientID          string           `json:"client_id"`
	Role              stay.VisitorRole `json:"role"`
	AgeBand           stay.AgeBand     `json:"age_band"`
	ResidenceCountry  string           `json:"residence_country"`
	ResidenceState    *string          `json:"residence_state"`
	ResidenceCityCode *string          `json:"residence_city_code"`
}

type submitGroupRequest struct {
	ClientSubmissionID   uuid.UUID        `json:"client_submission_id"`
	PrivacyNoticeVersion string           `json:"privacy_notice_version"`
	Visitors             []visitorRequest `json:"visitors"`
}

type createInviteRequest struct {
	PrivacyNoticeVersion string `json:"privacy_notice_version"`
}

type occurrenceRequest struct {
	OccurredAt *time.Time `json:"occurred_at"`
}

type cancelRequest struct {
	OccurredAt *time.Time `json:"occurred_at"`
	ReasonCode string     `json:"reason_code"`
	Correction bool       `json:"correction"`
}

type noShowRequest struct {
	OccurredAt *time.Time `json:"occurred_at"`
	ReasonCode string     `json:"reason_code"`
}

type stayPageResponse struct {
	Items      []stay.Record `json:"items"`
	NextCursor *string       `json:"next_cursor"`
}

func (d Dependencies) registerStayRoutes(mux *http.ServeMux, metrics *httpMetrics) {
	d.handleRoute(mux, metrics, "POST /api/v1/stays", "stays:write", d.createStay)
	d.handleRoute(mux, metrics, "GET /api/v1/stays", "stays:read:own", d.listStays)
	d.handleRoute(mux, metrics, "GET /api/v1/stays/{stay_id}", "stays:read:own", d.getStay)
	d.handleRoute(mux, metrics, "PATCH /api/v1/stays/{stay_id}", "stays:write", d.updateStay)
	d.handleRoute(mux, metrics, "GET /api/v1/stays/{stay_id}/group", "stays:read:own", d.getStayGroup)
	d.handleRoute(mux, metrics, "POST /api/v1/stays/{stay_id}/group", "stays:write", d.submitAssistedGroup)
	d.handleRoute(mux, metrics, "POST /api/v1/stays/{stay_id}/invite", "stays:write", d.createStayInvite)
	d.handleRoute(mux, metrics, "POST /api/v1/stays/{stay_id}/check-in", "stays:write", d.checkInStay)
	d.handleRoute(mux, metrics, "POST /api/v1/stays/{stay_id}/check-out", "stays:write", d.checkOutStay)
	d.handleRoute(mux, metrics, "POST /api/v1/stays/{stay_id}/cancel", "stays:write", d.cancelStay)
	d.handleRoute(mux, metrics, "POST /api/v1/stays/{stay_id}/no-show", "stays:write", d.noShowStay)
	d.handleInviteRoute(mux, metrics, "GET /api/v1/invites/{token}", d.getInvite)
	d.handleInviteRoute(mux, metrics, "POST /api/v1/invites/{token}/submit", d.submitInviteGroup)
	d.handleInviteRoute(
		mux, metrics, "OPTIONS /api/v1/invites/{token}", emptyHandler,
	)
	d.handleInviteRoute(
		mux, metrics, "OPTIONS /api/v1/invites/{token}/submit", emptyHandler,
	)
}

func (d Dependencies) handleInviteRoute(
	mux *http.ServeMux,
	metrics *httpMetrics,
	pattern string,
	handler func(http.ResponseWriter, *http.Request),
) {
	_, route, _ := stringsCut(pattern)
	wrapped := d.inviteCORS(http.HandlerFunc(handler))
	mux.Handle(pattern, d.routeHandler(route, metrics, wrapped))
}

func stringsCut(value string) (string, string, bool) {
	for index, character := range value {
		if character == ' ' {
			return value[:index], value[index+1:], true
		}
	}
	return value, "", false
}

func (d Dependencies) createStay(writer http.ResponseWriter, request *http.Request) {
	var body createStayRequest
	if err := decodeStrict(request, "application/json", &body); err != nil {
		writeBadRequest(writer, request)
		return
	}
	result, replayed, err := d.Stays.Create(request.Context(), stay.CreateCommand{
		Actor: requestPrincipal(request), AccommodationID: body.AccommodationID,
		ClientSubmissionID: body.ClientSubmissionID,
		PlannedArrivalOn:   body.PlannedArrivalOn, PlannedDepartureOn: body.PlannedDepartureOn,
		ExpectedGuestCount: body.ExpectedGuestCount,
		IdempotencyKey:     request.Header.Get("Idempotency-Key"), RequestID: requestID(request),
	})
	if err != nil {
		d.writeServiceError(writer, request, err)
		return
	}
	writer.Header().Set("Location", "/api/v1/stays/"+result.ID.String())
	writeMutationSuccess(writer, http.StatusCreated, result.Version, replayed, result)
}

func (d Dependencies) listStays(writer http.ResponseWriter, request *http.Request) {
	page, err := d.stayPageRequest(request)
	if err != nil {
		writeBadRequest(writer, request)
		return
	}
	result, err := d.Stays.List(request.Context(), requestPrincipal(request), page)
	if err != nil {
		d.writeServiceError(writer, request, err)
		return
	}
	var next *string
	if result.NextCursor != nil {
		next = d.cursor.encode(pageCursor{CreatedAt: result.NextCursor.CreatedAt, ID: result.NextCursor.ID})
	}
	writeJSON(writer, http.StatusOK, stayPageResponse{Items: result.Items, NextCursor: next})
}

func (d Dependencies) getStay(writer http.ResponseWriter, request *http.Request) {
	id, ok := pathUUID(writer, request, "stay_id")
	if !ok {
		return
	}
	result, err := d.Stays.Get(request.Context(), requestPrincipal(request), id)
	if err != nil {
		d.writeServiceError(writer, request, err)
		return
	}
	writer.Header().Set("ETag", etag(result.Version))
	writeJSON(writer, http.StatusOK, result)
}

func (d Dependencies) updateStay(writer http.ResponseWriter, request *http.Request) {
	id, version, ok := mutationTarget(writer, request, "stay_id")
	if !ok {
		return
	}
	var body updateStayRequest
	if err := decodeStrict(request, "application/merge-patch+json", &body); err != nil {
		writeBadRequest(writer, request)
		return
	}
	result, err := d.Stays.Update(request.Context(), stay.UpdateCommand{
		Actor: requestPrincipal(request), StayID: id, ExpectedVersion: version,
		Patch: stay.UpdatePatch{
			SetPlannedArrival:     body.PlannedArrivalOn.Set,
			PlannedArrivalOn:      body.PlannedArrivalOn.Value,
			SetPlannedDeparture:   body.PlannedDepartureOn.Set,
			PlannedDepartureOn:    body.PlannedDepartureOn.Value,
			SetExpectedGuestCount: body.ExpectedGuestCount.Set,
			ExpectedGuestCount:    body.ExpectedGuestCount.Value,
		},
		RequestID: requestID(request),
	})
	if err != nil {
		d.writeServiceError(writer, request, err)
		return
	}
	writer.Header().Set("ETag", etag(result.Version))
	writeJSON(writer, http.StatusOK, result)
}

func (d Dependencies) getStayGroup(writer http.ResponseWriter, request *http.Request) {
	id, ok := pathUUID(writer, request, "stay_id")
	if !ok {
		return
	}
	result, version, err := d.Stays.GetGroup(request.Context(), requestPrincipal(request), id)
	if err != nil {
		d.writeServiceError(writer, request, err)
		return
	}
	writer.Header().Set("ETag", etag(version))
	writeJSON(writer, http.StatusOK, result)
}

func (d Dependencies) submitAssistedGroup(writer http.ResponseWriter, request *http.Request) {
	id, version, ok := mutationTarget(writer, request, "stay_id")
	if !ok {
		return
	}
	var body submitGroupRequest
	if err := decodeStrict(request, "application/json", &body); err != nil {
		writeBadRequest(writer, request)
		return
	}
	result, replayed, err := d.Stays.SubmitAssistedGroup(request.Context(), stay.GroupCommand{
		Actor: requestPrincipal(request), StayID: id,
		ClientSubmissionID:   body.ClientSubmissionID,
		PrivacyNoticeVersion: body.PrivacyNoticeVersion, Visitors: visitorsFromRequest(body.Visitors),
		ExpectedVersion: version, IdempotencyKey: request.Header.Get("Idempotency-Key"),
		RequestID: requestID(request),
	})
	if err != nil {
		d.writeServiceError(writer, request, err)
		return
	}
	setSurveyCapability(writer, result.SurveyCapability)
	writeMutationSuccess(writer, http.StatusOK, result.Version, replayed, result)
}

func (d Dependencies) createStayInvite(writer http.ResponseWriter, request *http.Request) {
	id, version, ok := mutationTarget(writer, request, "stay_id")
	if !ok {
		return
	}
	var body createInviteRequest
	if err := decodeStrict(request, "application/json", &body); err != nil {
		writeBadRequest(writer, request)
		return
	}
	result, replayed, err := d.Stays.CreateInvite(request.Context(), stay.InviteCommand{
		Actor: requestPrincipal(request), StayID: id,
		PrivacyNoticeVersion: body.PrivacyNoticeVersion, ExpectedVersion: version,
		IdempotencyKey: request.Header.Get("Idempotency-Key"), RequestID: requestID(request),
	})
	if err != nil {
		d.writeServiceError(writer, request, err)
		return
	}
	writer.Header().Set("Location", "/api/v1/stays/"+id.String()+"/invite")
	writeMutationSuccess(writer, http.StatusCreated, result.Version, replayed, result)
}

func (d Dependencies) checkInStay(writer http.ResponseWriter, request *http.Request) {
	d.handleOccurrence(writer, request, stay.TransitionCheckIn)
}

func (d Dependencies) checkOutStay(writer http.ResponseWriter, request *http.Request) {
	d.handleOccurrence(writer, request, stay.TransitionCheckOut)
}

func (d Dependencies) handleOccurrence(
	writer http.ResponseWriter,
	request *http.Request,
	kind stay.TransitionKind,
) {
	id, version, ok := mutationTarget(writer, request, "stay_id")
	if !ok {
		return
	}
	var body occurrenceRequest
	if err := decodeStrict(request, "application/json", &body); err != nil {
		writeBadRequest(writer, request)
		return
	}
	d.runTransition(writer, request, stay.TransitionCommand{
		Actor: requestPrincipal(request), StayID: id, ExpectedVersion: version,
		Kind: kind, OccurredAt: valueOrZero(body.OccurredAt),
		IdempotencyKey: request.Header.Get("Idempotency-Key"), RequestID: requestID(request),
	})
}

func (d Dependencies) cancelStay(writer http.ResponseWriter, request *http.Request) {
	id, version, ok := mutationTarget(writer, request, "stay_id")
	if !ok {
		return
	}
	var body cancelRequest
	if err := decodeStrict(request, "application/json", &body); err != nil {
		writeBadRequest(writer, request)
		return
	}
	d.runTransition(writer, request, stay.TransitionCommand{
		Actor: requestPrincipal(request), StayID: id, ExpectedVersion: version,
		Kind: stay.TransitionCancel, OccurredAt: valueOrZero(body.OccurredAt),
		ReasonCode: body.ReasonCode, Correction: body.Correction,
		IdempotencyKey: request.Header.Get("Idempotency-Key"), RequestID: requestID(request),
	})
}

func (d Dependencies) noShowStay(writer http.ResponseWriter, request *http.Request) {
	id, version, ok := mutationTarget(writer, request, "stay_id")
	if !ok {
		return
	}
	var body noShowRequest
	if err := decodeStrict(request, "application/json", &body); err != nil {
		writeBadRequest(writer, request)
		return
	}
	d.runTransition(writer, request, stay.TransitionCommand{
		Actor: requestPrincipal(request), StayID: id, ExpectedVersion: version,
		Kind: stay.TransitionNoShow, OccurredAt: valueOrZero(body.OccurredAt),
		ReasonCode:     body.ReasonCode,
		IdempotencyKey: request.Header.Get("Idempotency-Key"), RequestID: requestID(request),
	})
}

func (d Dependencies) runTransition(
	writer http.ResponseWriter,
	request *http.Request,
	command stay.TransitionCommand,
) {
	result, replayed, err := d.Stays.Transition(request.Context(), command)
	if err != nil {
		d.writeServiceError(writer, request, err)
		return
	}
	writeMutationSuccess(writer, http.StatusOK, result.Version, replayed, result)
}

func setSurveyCapability(writer http.ResponseWriter, capability string) {
	if capability != "" {
		writer.Header().Set("Survey-Capability", capability)
		writer.Header().Add("Access-Control-Expose-Headers", "Survey-Capability")
	}
}

func (d Dependencies) getInvite(writer http.ResponseWriter, request *http.Request) {
	subject, ok := d.requestRateSubject(writer, request)
	if !ok {
		return
	}
	result, err := d.Stays.GetInvite(request.Context(), stay.InviteRequest{
		Token: request.PathValue("token"), RateSubject: subject,
	})
	if err != nil {
		d.writeServiceError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

func (d Dependencies) submitInviteGroup(writer http.ResponseWriter, request *http.Request) {
	subject, ok := d.requestRateSubject(writer, request)
	if !ok {
		return
	}
	var body submitGroupRequest
	if err := decodeStrict(request, "application/json", &body); err != nil {
		writeBadRequest(writer, request)
		return
	}
	result, replayed, err := d.Stays.SubmitInviteGroup(request.Context(), stay.InviteGroupCommand{
		Token: request.PathValue("token"), RateSubject: subject,
		ClientSubmissionID:   body.ClientSubmissionID,
		PrivacyNoticeVersion: body.PrivacyNoticeVersion, Visitors: visitorsFromRequest(body.Visitors),
		IdempotencyKey: request.Header.Get("Idempotency-Key"), RequestID: requestID(request),
	})
	if err != nil {
		d.writeServiceError(writer, request, err)
		return
	}
	setSurveyCapability(writer, result.SurveyCapability)
	writeMutationSuccess(writer, http.StatusOK, result.Version, replayed, result)
}

func (d Dependencies) stayPageRequest(request *http.Request) (stay.PageRequest, error) {
	limit, cursor, err := parsePage(
		d.cursor,
		request.URL.Query().Get("limit"),
		request.URL.Query().Get("cursor"),
	)
	if err != nil {
		return stay.PageRequest{}, err
	}
	var accommodationID uuid.UUID
	if value := request.URL.Query().Get("accommodation_id"); value != "" {
		accommodationID, err = uuid.Parse(value)
		if err != nil {
			return stay.PageRequest{}, errInvalidCursor
		}
	}
	return stay.PageRequest{
		CursorCreatedAt: cursor.CreatedAt, CursorID: cursor.ID, Limit: limit,
		AccommodationID: accommodationID, Status: stay.Status(request.URL.Query().Get("status")),
		ArrivalFrom: request.URL.Query().Get("arrival_from"),
		ArrivalTo:   request.URL.Query().Get("arrival_to"),
	}, nil
}

func visitorsFromRequest(values []visitorRequest) []stay.Visitor {
	result := make([]stay.Visitor, 0, len(values))
	for _, value := range values {
		result = append(result, stay.Visitor{
			ClientID: value.ClientID, Role: value.Role, AgeBand: value.AgeBand,
			ResidenceCountry:  value.ResidenceCountry,
			ResidenceState:    valueOrZero(value.ResidenceState),
			ResidenceCityCode: valueOrZero(value.ResidenceCityCode),
		})
	}
	return result
}

func writeBadRequest(writer http.ResponseWriter, request *http.Request) {
	writeProblem(writer, request, http.StatusBadRequest, "invalid-request", "Requisição inválida")
}
