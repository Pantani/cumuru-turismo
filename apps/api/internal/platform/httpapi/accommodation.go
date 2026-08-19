package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/Pantani/cumuru/apps/api/internal/accommodation"
	"github.com/google/uuid"
)

type accommodationPageResponse struct {
	Items      []accommodation.Accommodation `json:"items"`
	NextCursor *string                       `json:"next_cursor"`
}

type membershipPageResponse struct {
	Items      []accommodation.Membership `json:"items"`
	NextCursor *string                    `json:"next_cursor"`
}

type createAccommodationRequest struct {
	Name               string                 `json:"name"`
	Category           accommodation.Category `json:"category"`
	Capacity           int32                  `json:"capacity"`
	ClientSubmissionID uuid.UUID              `json:"client_submission_id"`
}

type updateAccommodationRequest struct {
	Name                  requiredPatch[string]                 `json:"name"`
	Category              requiredPatch[accommodation.Category] `json:"category"`
	Capacity              nullableInt32                         `json:"capacity"`
	PublicAreaCode        nullableString                        `json:"public_area_code"`
	PublicListingEnabled  requiredPatch[bool]                   `json:"public_listing_enabled"`
	PublicContactPhone    nullableString                        `json:"public_contact_phone"`
	PublicContactWhatsApp requiredPatch[bool]                   `json:"public_contact_whatsapp"`
	PublicWebsiteURL      nullableString                        `json:"public_website_url"`
}

type createMembershipRequest struct {
	OIDCIssuer  string             `json:"oidc_issuer"`
	OIDCSubject string             `json:"oidc_subject"`
	Role        accommodation.Role `json:"role"`
}

type updateMembershipRequest struct {
	Role   requiredPatch[accommodation.Role] `json:"role"`
	Active requiredPatch[bool]               `json:"active"`
}

type requiredPatch[T any] struct {
	Set   bool
	Value T
}

func (p *requiredPatch[T]) UnmarshalJSON(content []byte) error {
	p.Set = true
	if bytes.Equal(bytes.TrimSpace(content), []byte("null")) {
		return errInvalidJSON
	}
	return json.Unmarshal(content, &p.Value)
}

type nullableString struct {
	Set   bool
	Value *string
}

func (n *nullableString) UnmarshalJSON(content []byte) error {
	n.Set = true
	if bytes.Equal(content, []byte("null")) {
		n.Value = nil
		return nil
	}
	return json.Unmarshal(content, &n.Value)
}

type nullableInt32 struct {
	Set   bool
	Value *int32
}

func (n *nullableInt32) UnmarshalJSON(content []byte) error {
	n.Set = true
	if bytes.Equal(content, []byte("null")) {
		n.Value = nil
		return nil
	}
	return json.Unmarshal(content, &n.Value)
}

func (d Dependencies) registerAccommodationRoutes(mux *http.ServeMux, metrics *httpMetrics) {
	d.handleRoute(mux, metrics, "GET /api/v1/accommodations", "stays:read:own", d.listAccommodations)
	d.handleRoute(mux, metrics, "POST /api/v1/accommodations", "accommodations:onboard", d.createAccommodation)
	d.handleRoute(mux, metrics, "GET /api/v1/accommodations/{accommodation_id}", "stays:read:own", d.getAccommodation)
	d.handleRoute(mux, metrics, "PATCH /api/v1/accommodations/{accommodation_id}", "accommodations:manage", d.updateAccommodation)
	d.handleRoute(mux, metrics, "GET /api/v1/accommodations/{accommodation_id}/memberships", "accommodations:manage", d.listMemberships)
	d.handleRoute(mux, metrics, "POST /api/v1/accommodations/{accommodation_id}/memberships", "accommodations:manage", d.createMembership)
	d.handleRoute(mux, metrics, "PATCH /api/v1/accommodations/{accommodation_id}/memberships/{membership_id}", "accommodations:manage", d.updateMembership)
}

func (d Dependencies) createAccommodation(writer http.ResponseWriter, request *http.Request) {
	if !d.AccommodationOnboardingEnabled {
		writeProblem(
			writer,
			request,
			http.StatusServiceUnavailable,
			"dependency-unavailable",
			"Serviço temporariamente indisponível",
		)
		return
	}
	var body createAccommodationRequest
	if err := decodeStrict(request, "application/json", &body); err != nil {
		writeProblem(writer, request, http.StatusBadRequest, "invalid-request", "Requisição inválida")
		return
	}
	result, replayed, err := d.Accommodations.Create(request.Context(), accommodation.CreateCommand{
		Actor: requestPrincipal(request), Name: body.Name, Category: body.Category,
		Capacity: body.Capacity, ClientSubmissionID: body.ClientSubmissionID,
		IdempotencyKey: request.Header.Get("Idempotency-Key"), RequestID: requestID(request),
	})
	if err != nil {
		d.writeServiceError(writer, request, err)
		return
	}
	writer.Header().Set("Location", "/api/v1/accommodations/"+result.ID.String())
	writeMutationSuccess(writer, http.StatusCreated, result.Version, replayed, result)
}

func (d Dependencies) listAccommodations(writer http.ResponseWriter, request *http.Request) {
	limit, cursor, err := parsePage(d.cursor, request.URL.Query().Get("limit"), request.URL.Query().Get("cursor"))
	if err != nil {
		writeProblem(writer, request, http.StatusBadRequest, "invalid-request", "Requisição inválida")
		return
	}
	page, err := d.Accommodations.List(request.Context(), requestPrincipal(request), accommodation.PageRequest{
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
	writeJSON(writer, http.StatusOK, accommodationPageResponse{Items: page.Items, NextCursor: next})
}

func (d Dependencies) getAccommodation(writer http.ResponseWriter, request *http.Request) {
	id, ok := pathUUID(writer, request, "accommodation_id")
	if !ok {
		return
	}
	result, err := d.Accommodations.Get(request.Context(), requestPrincipal(request), id)
	if err != nil {
		d.writeServiceError(writer, request, err)
		return
	}
	writer.Header().Set("ETag", etag(result.Version))
	writeJSON(writer, http.StatusOK, result)
}

func (d Dependencies) updateAccommodation(writer http.ResponseWriter, request *http.Request) {
	id, version, ok := mutationTarget(writer, request, "accommodation_id")
	if !ok {
		return
	}
	var body updateAccommodationRequest
	if err := decodeStrict(request, "application/merge-patch+json", &body); err != nil {
		writeProblem(writer, request, http.StatusBadRequest, "invalid-request", "Requisição inválida")
		return
	}
	result, err := d.Accommodations.Update(request.Context(), accommodation.UpdateCommand{
		Actor: requestPrincipal(request), AccommodationID: id, ExpectedVersion: version,
		Patch: accommodationPatch(body), RequestID: requestID(request),
	})
	if err != nil {
		d.writeServiceError(writer, request, err)
		return
	}
	writer.Header().Set("ETag", etag(result.Version))
	writeJSON(writer, http.StatusOK, result)
}

func (d Dependencies) listMemberships(writer http.ResponseWriter, request *http.Request) {
	id, ok := pathUUID(writer, request, "accommodation_id")
	if !ok {
		return
	}
	limit, cursor, err := parsePage(d.cursor, request.URL.Query().Get("limit"), request.URL.Query().Get("cursor"))
	if err != nil {
		writeProblem(writer, request, http.StatusBadRequest, "invalid-request", "Requisição inválida")
		return
	}
	page, err := d.Accommodations.ListMemberships(
		request.Context(), requestPrincipal(request), id,
		accommodation.PageRequest{CursorCreatedAt: cursor.CreatedAt, CursorID: cursor.ID, Limit: limit},
	)
	if err != nil {
		d.writeServiceError(writer, request, err)
		return
	}
	var next *string
	if page.NextCursor != nil {
		next = d.cursor.encode(pageCursor{CreatedAt: page.NextCursor.CreatedAt, ID: page.NextCursor.ID})
	}
	writeJSON(writer, http.StatusOK, membershipPageResponse{Items: page.Items, NextCursor: next})
}

func (d Dependencies) createMembership(writer http.ResponseWriter, request *http.Request) {
	id, ok := pathUUID(writer, request, "accommodation_id")
	if !ok {
		return
	}
	var body createMembershipRequest
	if err := decodeStrict(request, "application/json", &body); err != nil {
		writeProblem(writer, request, http.StatusBadRequest, "invalid-request", "Requisição inválida")
		return
	}
	result, replayed, err := d.Accommodations.CreateMembership(request.Context(), accommodation.CreateMembershipCommand{
		Actor: requestPrincipal(request), AccommodationID: id,
		TargetIssuer: body.OIDCIssuer, TargetSubject: body.OIDCSubject, Role: body.Role,
		IdempotencyKey: request.Header.Get("Idempotency-Key"), RequestID: requestID(request),
	})
	if err != nil {
		d.writeServiceError(writer, request, err)
		return
	}
	writer.Header().Set("Location", "/api/v1/accommodations/"+id.String()+"/memberships/"+result.ID.String())
	writeMutationSuccess(writer, http.StatusCreated, result.Version, replayed, result)
}

func (d Dependencies) updateMembership(writer http.ResponseWriter, request *http.Request) {
	accommodationID, version, ok := mutationTarget(writer, request, "accommodation_id")
	if !ok {
		return
	}
	membershipID, ok := pathUUID(writer, request, "membership_id")
	if !ok {
		return
	}
	var body updateMembershipRequest
	if err := decodeStrict(request, "application/merge-patch+json", &body); err != nil {
		writeProblem(writer, request, http.StatusBadRequest, "invalid-request", "Requisição inválida")
		return
	}
	result, err := d.Accommodations.UpdateMembership(request.Context(), accommodation.UpdateMembershipCommand{
		Actor: requestPrincipal(request), AccommodationID: accommodationID,
		MembershipID: membershipID, ExpectedVersion: version,
		Patch: accommodation.UpdateMembershipPatch{
			SetRole: body.Role.Set, Role: body.Role.Value,
			SetActive: body.Active.Set, Active: body.Active.Value,
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

func accommodationPatch(body updateAccommodationRequest) accommodation.UpdatePatch {
	return accommodation.UpdatePatch{
		SetName: body.Name.Set, Name: body.Name.Value,
		SetCategory: body.Category.Set, Category: body.Category.Value,
		SetCapacity: body.Capacity.Set, Capacity: body.Capacity.Value,
		SetPublicAreaCode: body.PublicAreaCode.Set, PublicAreaCode: body.PublicAreaCode.Value,
		SetPublicListingEnabled:  body.PublicListingEnabled.Set,
		PublicListingEnabled:     body.PublicListingEnabled.Value,
		SetPublicContactPhone:    body.PublicContactPhone.Set,
		PublicContactPhone:       body.PublicContactPhone.Value,
		SetPublicContactWhatsApp: body.PublicContactWhatsApp.Set,
		PublicContactWhatsApp:    body.PublicContactWhatsApp.Value,
		SetPublicWebsiteURL:      body.PublicWebsiteURL.Set,
		PublicWebsiteURL:         body.PublicWebsiteURL.Value,
	}
}

func valueOrZero[T any](value *T) T {
	if value == nil {
		var zero T
		return zero
	}
	return *value
}

func pathUUID(writer http.ResponseWriter, request *http.Request, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(request.PathValue(name))
	if err != nil {
		writeProblem(writer, request, http.StatusBadRequest, "invalid-request", "Requisição inválida")
		return uuid.Nil, false
	}
	return id, true
}
