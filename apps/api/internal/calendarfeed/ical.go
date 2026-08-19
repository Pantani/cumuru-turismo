package calendarfeed

import (
	"strings"

	"github.com/Pantani/cumuru/apps/api/internal/stay"
)

// MaxEvents bounds a single calendar because the file comes from a host the
// operator named, and an unbounded loop over somebody else's response is the
// cheapest denial of service there is. A pousada's yearly calendar is in the
// hundreds; five thousand is generous and still finite.
const MaxEvents = 5000

// blockedMarkers comes first in classification on purpose: Booking.com writes
// "CLOSED - Not available" and Airbnb writes "Airbnb (Not available)", so a
// summary can carry the platform name and the block in the same line.
var blockedMarkers = []string{
	"not available",
	"unavailable",
	"blocked",
	"closed",
	"indispon",
	"bloquead",
}

var reservedMarkers = []string{
	"reserved",
	"reserva",
	"booked",
	"busy",
	"ocupad",
}

// Event is one VEVENT reduced to what this system is allowed to keep: an
// identifier, two civil dates and whether the origin said it was a booking.
// Whatever else the file carries — and Booking.com carries almost nothing —
// is dropped here, at the border, rather than stored and filtered later.
type Event struct {
	UID       string
	Arrival   stay.CivilDate
	Departure stay.CivilDate
	Kind      ReservationKind
}

// ParseCalendar reads an iCalendar document. It answers ErrNotCalendar when the
// response was something else entirely — an HTML login page is the usual case,
// because an expired feed URL redirects instead of failing — and ErrMalformed
// when the document is a calendar whose events do not hold up.
func ParseCalendar(raw string) ([]Event, error) {
	lines := unfold(raw)
	if !hasCalendarHeader(lines) {
		return nil, ErrNotCalendar
	}
	scanner := &calendarScanner{}
	for _, line := range lines {
		if err := scanner.consume(line); err != nil {
			return nil, err
		}
	}
	return scanner.events, nil
}

type calendarScanner struct {
	inEvent bool
	props   map[string]string
	events  []Event
}

func (s *calendarScanner) consume(line string) error {
	switch strings.ToUpper(strings.TrimSpace(line)) {
	case "BEGIN:VEVENT":
		s.inEvent = true
		s.props = make(map[string]string, 8)
	case "END:VEVENT":
		return s.closeEvent()
	default:
		s.property(line)
	}
	return nil
}

func (s *calendarScanner) closeEvent() error {
	if !s.inEvent {
		return nil
	}
	event, err := buildEvent(s.props)
	s.inEvent = false
	s.props = nil
	if err != nil {
		return err
	}
	if len(s.events) >= MaxEvents {
		return ErrMalformed
	}
	s.events = append(s.events, event)
	return nil
}

func (s *calendarScanner) property(line string) {
	if !s.inEvent {
		return
	}
	head, value, found := strings.Cut(line, ":")
	if !found {
		return
	}
	name, _, _ := strings.Cut(head, ";")
	s.props[strings.ToUpper(strings.TrimSpace(name))] = value
}

func buildEvent(props map[string]string) (Event, error) {
	uid := strings.TrimSpace(props["UID"])
	if uid == "" || len(uid) > 512 {
		return Event{}, ErrMalformed
	}
	return buildEventDates(uid, props)
}

func buildEventDates(uid string, props map[string]string) (Event, error) {
	arrival, err := parseCalendarDate(props["DTSTART"])
	if err != nil {
		return Event{}, err
	}
	departure, err := parseCalendarDate(props["DTEND"])
	if err != nil {
		return Event{}, err
	}
	if !arrival.Before(departure) {
		return Event{}, ErrMalformed
	}
	return Event{
		UID:       uid,
		Arrival:   arrival,
		Departure: departure,
		Kind:      classify(props["SUMMARY"]),
	}, nil
}

// parseCalendarDate accepts the all-day form the lodging platforms emit and the
// timestamped form the specification also allows, and keeps only the date. A
// stay is a civil interval in America/Bahia; the hour a booking engine happened
// to stamp is not a fact about presence.
func parseCalendarDate(value string) (stay.CivilDate, error) {
	trimmed := strings.TrimSpace(value)
	if date, _, found := strings.Cut(trimmed, "T"); found {
		trimmed = date
	}
	if len(trimmed) != 8 {
		return stay.CivilDate{}, ErrMalformed
	}
	parsed, err := stay.ParseCivilDate(trimmed[0:4] + "-" + trimmed[4:6] + "-" + trimmed[6:8])
	if err != nil {
		return stay.CivilDate{}, ErrMalformed
	}
	return parsed, nil
}

// classify never guesses beyond the two vocabularies it knows. KindUnknown is
// the honest answer for a summary nobody wrote for us, and the queue asks the
// lodging instead of inventing an occupancy that would reach the public
// indicator (ADR-044).
func classify(summary string) ReservationKind {
	normalized := strings.ToLower(strings.TrimSpace(summary))
	if containsAny(normalized, blockedMarkers) {
		return KindBlocked
	}
	if containsAny(normalized, reservedMarkers) {
		return KindReserved
	}
	return KindUnknown
}

func containsAny(value string, markers []string) bool {
	for _, marker := range markers {
		if strings.Contains(value, marker) {
			return true
		}
	}
	return false
}

func hasCalendarHeader(lines []string) bool {
	for _, line := range lines {
		if strings.EqualFold(strings.TrimSpace(line), "BEGIN:VCALENDAR") {
			return true
		}
	}
	return false
}

// unfold restores the continuation lines the specification allows a producer to
// split. Without it a long UID broken across two lines becomes two properties,
// and the same reservation would enter the queue again on every synchronization.
func unfold(raw string) []string {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	lines := make([]string, 0, 64)
	for _, line := range strings.Split(normalized, "\n") {
		lines = appendFolded(lines, strings.TrimSuffix(line, "\r"))
	}
	return lines
}

func appendFolded(lines []string, line string) []string {
	if len(lines) == 0 || !isContinuation(line) {
		return append(lines, line)
	}
	lines[len(lines)-1] += line[1:]
	return lines
}

func isContinuation(line string) bool {
	return strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")
}
