package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Pantani/cumuru/apps/api/internal/calendarfeed"
	"github.com/google/uuid"
)

var errInvalidLimit = errors.New("invalid limit")

type calendarFeedListResponse struct {
	Items []calendarfeed.Feed `json:"items"`
}

type calendarReservationListResponse struct {
	Items []calendarfeed.Reservation `json:"items"`
}

// createCalendarFeedBody carries the address on the way in and never on the way
// out. It is the only request body in the API that holds a bearer secret, which
// is why the response schema has no field for it (ADR-043).
type createCalendarFeedBody struct {
	Provider calendarfeed.Provider `json:"provider"`
	Label    string                `json:"label"`
	URL      string                `json:"url"`
}

type confirmCalendarReservationBody struct {
	ExpectedGuestCount int32     `json:"expected_guest_count"`
	ClientSubmissionID uuid.UUID `json:"client_submission_id"`
}

// Registering a feed declares where an accommodation's dates come from, which is
// the same act as registering a stay by hand, so it costs the same scope. The
// two listings are reads of the lodging's own data and take stays:read:own.
func (d Dependencies) registerCalendarFeedRoutes(mux *http.ServeMux, metrics *httpMetrics) {
	if d.CalendarFeeds == nil {
		return
	}
	d.handleRoute(
		mux, metrics, "GET /api/v1/accommodations/{accommodation_id}/calendar-feeds",
		"stays:read:own", d.listCalendarFeeds,
	)
	d.handleRoute(
		mux, metrics, "POST /api/v1/accommodations/{accommodation_id}/calendar-feeds",
		"stays:write", d.createCalendarFeed,
	)
	d.handleRoute(
		mux, metrics, "POST /api/v1/calendar-feeds/{feed_id}/remove",
		"stays:write", d.removeCalendarFeed,
	)
	d.registerCalendarReservationRoutes(mux, metrics)
}

func (d Dependencies) registerCalendarReservationRoutes(
	mux *http.ServeMux,
	metrics *httpMetrics,
) {
	d.handleRoute(
		mux, metrics,
		"GET /api/v1/accommodations/{accommodation_id}/calendar-reservations",
		"stays:read:own", d.listCalendarReservations,
	)
	d.handleRoute(
		mux, metrics, "POST /api/v1/calendar-reservations/{reservation_id}/confirm",
		"stays:write", d.confirmCalendarReservation,
	)
	d.handleRoute(
		mux, metrics, "POST /api/v1/calendar-reservations/{reservation_id}/dismiss",
		"stays:write", d.dismissCalendarReservation,
	)
}

func (d Dependencies) listCalendarFeeds(writer http.ResponseWriter, request *http.Request) {
	id, ok := pathUUID(writer, request, "accommodation_id")
	if !ok {
		return
	}
	feeds, err := d.CalendarFeeds.ListFeeds(request.Context(), calendarfeed.ListFeedsRequest{
		Actor: requestPrincipal(request), AccommodationID: id,
	})
	if err != nil {
		d.writeServiceError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, calendarFeedListResponse{Items: feeds})
}

func (d Dependencies) createCalendarFeed(writer http.ResponseWriter, request *http.Request) {
	id, ok := pathUUID(writer, request, "accommodation_id")
	if !ok {
		return
	}
	var body createCalendarFeedBody
	if err := decodeStrict(request, "application/json", &body); err != nil {
		writeBadRequest(writer, request)
		return
	}
	feed, replayed, err := d.CalendarFeeds.CreateFeed(
		request.Context(), createCalendarFeedCommand(request, body, id),
	)
	if err != nil {
		d.writeServiceError(writer, request, err)
		return
	}
	writeMutationSuccess(writer, http.StatusCreated, feed.Version, replayed, feed)
}

func createCalendarFeedCommand(
	request *http.Request,
	body createCalendarFeedBody,
	accommodationID uuid.UUID,
) calendarfeed.CreateFeedCommand {
	return calendarfeed.CreateFeedCommand{
		Actor: requestPrincipal(request), AccommodationID: accommodationID,
		Provider: body.Provider, Label: body.Label, URL: body.URL,
		IdempotencyKey: request.Header.Get("Idempotency-Key"),
		RequestID:      requestID(request),
	}
}

func (d Dependencies) removeCalendarFeed(writer http.ResponseWriter, request *http.Request) {
	id, version, ok := mutationTarget(writer, request, "feed_id")
	if !ok {
		return
	}
	if err := decodeStrict(request, "application/json", &struct{}{}); err != nil {
		writeBadRequest(writer, request)
		return
	}
	feed, replayed, err := d.CalendarFeeds.RemoveFeed(
		request.Context(), calendarfeed.RemoveFeedCommand{
			Actor: requestPrincipal(request), FeedID: id, ExpectedVersion: version,
			IdempotencyKey: request.Header.Get("Idempotency-Key"),
			RequestID:      requestID(request),
		},
	)
	if err != nil {
		d.writeServiceError(writer, request, err)
		return
	}
	writeMutationSuccess(writer, http.StatusOK, feed.Version, replayed, feed)
}

func (d Dependencies) listCalendarReservations(
	writer http.ResponseWriter,
	request *http.Request,
) {
	id, ok := pathUUID(writer, request, "accommodation_id")
	if !ok {
		return
	}
	limit, err := parseCalendarLimit(request.URL.Query().Get("limit"))
	if err != nil {
		writeBadRequest(writer, request)
		return
	}
	items, err := d.CalendarFeeds.ListReservations(
		request.Context(), calendarfeed.ListReservationsRequest{
			Actor: requestPrincipal(request), AccommodationID: id, Limit: limit,
			State: calendarfeed.ReservationState(request.URL.Query().Get("state")),
		},
	)
	if err != nil {
		d.writeServiceError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, calendarReservationListResponse{Items: items})
}

// The queue is not paginated by cursor: it is bounded by the calendar of one
// accommodation, and a cursor would add a signed token to a list that fits on a
// screen.
func parseCalendarLimit(raw string) (int32, error) {
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.ParseInt(raw, 10, 32)
	if err != nil || value < 1 || value > 200 {
		return 0, errInvalidLimit
	}
	return int32(value), nil
}

func (d Dependencies) confirmCalendarReservation(
	writer http.ResponseWriter,
	request *http.Request,
) {
	id, version, ok := mutationTarget(writer, request, "reservation_id")
	if !ok {
		return
	}
	var body confirmCalendarReservationBody
	if err := decodeStrict(request, "application/json", &body); err != nil {
		writeBadRequest(writer, request)
		return
	}
	result, replayed, err := d.CalendarFeeds.Confirm(
		request.Context(), confirmCalendarReservationCommand(request, body, id, version),
	)
	if err != nil {
		d.writeServiceError(writer, request, err)
		return
	}
	writeMutationSuccess(writer, http.StatusOK, result.Version, replayed, result)
}

func confirmCalendarReservationCommand(
	request *http.Request,
	body confirmCalendarReservationBody,
	reservationID uuid.UUID,
	version int64,
) calendarfeed.ConfirmCommand {
	return calendarfeed.ConfirmCommand{
		Actor: requestPrincipal(request), ReservationID: reservationID,
		ExpectedVersion:    version,
		ExpectedGuestCount: body.ExpectedGuestCount,
		ClientSubmissionID: body.ClientSubmissionID,
		IdempotencyKey:     request.Header.Get("Idempotency-Key"),
		RequestID:          requestID(request),
	}
}

func (d Dependencies) dismissCalendarReservation(
	writer http.ResponseWriter,
	request *http.Request,
) {
	id, version, ok := mutationTarget(writer, request, "reservation_id")
	if !ok {
		return
	}
	if err := decodeStrict(request, "application/json", &struct{}{}); err != nil {
		writeBadRequest(writer, request)
		return
	}
	result, replayed, err := d.CalendarFeeds.Dismiss(
		request.Context(), calendarfeed.DismissCommand{
			Actor: requestPrincipal(request), ReservationID: id,
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
