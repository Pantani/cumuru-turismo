package calendarfeed_test

import (
	"strings"
	"testing"

	"github.com/Pantani/cumuru/apps/api/internal/calendarfeed"
)

// bookingCalendar is shaped like what the Booking.com extranet exports: all-day
// events, CRLF line endings, and a summary that says the room is closed rather
// than who is in it.
const bookingCalendar = "BEGIN:VCALENDAR\r\n" +
	"VERSION:2.0\r\n" +
	"PRODID:-//Booking.com//Calendar//EN\r\n" +
	"BEGIN:VEVENT\r\n" +
	"DTSTART;VALUE=DATE:20260815\r\n" +
	"DTEND;VALUE=DATE:20260818\r\n" +
	"UID:1f6d5a2e-booking-0001\r\n" +
	"SUMMARY:CLOSED - Not available\r\n" +
	"END:VEVENT\r\n" +
	"BEGIN:VEVENT\r\n" +
	"DTSTART;VALUE=DATE:20260901\r\n" +
	"DTEND;VALUE=DATE:20260903\r\n" +
	"UID:1f6d5a2e-booking-0002\r\n" +
	"SUMMARY:Reserved\r\n" +
	"END:VEVENT\r\n" +
	"END:VCALENDAR\r\n"

func TestParseCalendarReadsDatesAndClassifiesTheSummary(t *testing.T) {
	t.Parallel()

	events, err := calendarfeed.ParseCalendar(bookingCalendar)
	if err != nil {
		t.Fatalf("ParseCalendar() error = %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("ParseCalendar() returned %d events", len(events))
	}
	if events[0].Arrival.String() != "2026-08-15" || events[0].Departure.String() != "2026-08-18" {
		t.Fatalf("first event = %s..%s", events[0].Arrival, events[0].Departure)
	}
	if events[0].Kind != calendarfeed.KindBlocked {
		t.Fatalf("first event kind = %s, want blocked", events[0].Kind)
	}
	if events[1].Kind != calendarfeed.KindReserved {
		t.Fatalf("second event kind = %s, want reserved", events[1].Kind)
	}
}

// A summary nobody wrote for us must not be guessed into occupancy: unknown is
// what reaches the queue, and the lodging decides (ADR-043).
func TestParseCalendarLeavesAnUnrecognizedSummaryUnknown(t *testing.T) {
	t.Parallel()

	events, err := calendarfeed.ParseCalendar(calendarWithSummary("Chalé 3"))
	if err != nil {
		t.Fatalf("ParseCalendar() error = %v", err)
	}
	if len(events) != 1 || events[0].Kind != calendarfeed.KindUnknown {
		t.Fatalf("ParseCalendar() kind = %v", events)
	}
}

// A folded UID that is not rejoined becomes a different identifier on every
// synchronization, which would re-import the same reservation forever.
func TestParseCalendarRejoinsFoldedLines(t *testing.T) {
	t.Parallel()

	folded := "BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\n" +
		"DTSTART;VALUE=DATE:20260815\r\nDTEND;VALUE=DATE:20260818\r\n" +
		"UID:1f6d5a2e-booking-\r\n 0001-continued\r\n" +
		"SUMMARY:Reserved\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	events, err := calendarfeed.ParseCalendar(folded)
	if err != nil {
		t.Fatalf("ParseCalendar() error = %v", err)
	}
	if len(events) != 1 || events[0].UID != "1f6d5a2e-booking-0001-continued" {
		t.Fatalf("ParseCalendar() uid = %v", events)
	}
}

func TestParseCalendarAcceptsTimestampedDates(t *testing.T) {
	t.Parallel()

	stamped := strings.ReplaceAll(
		calendarWithSummary("Reserved"),
		"DTSTART;VALUE=DATE:20260815",
		"DTSTART:20260815T140000Z",
	)
	events, err := calendarfeed.ParseCalendar(stamped)
	if err != nil {
		t.Fatalf("ParseCalendar() error = %v", err)
	}
	if events[0].Arrival.String() != "2026-08-15" {
		t.Fatalf("ParseCalendar() arrival = %s", events[0].Arrival)
	}
}

// An expired feed URL redirects to a login page, so "not a calendar" is the
// common failure and has to be distinguishable from a broken one.
func TestParseCalendarRejectsAResponseThatIsNotACalendar(t *testing.T) {
	t.Parallel()

	_, err := calendarfeed.ParseCalendar("<html><body>Sign in</body></html>")
	if err != calendarfeed.ErrNotCalendar {
		t.Fatalf("ParseCalendar(html) error = %v, want ErrNotCalendar", err)
	}
}

func TestParseCalendarRejectsMalformedEvents(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"missing uid":       "UID:1f6d5a2e-booking-0001\r\n",
		"departure before":  "DTEND;VALUE=DATE:20260818\r\n",
		"unparseable start": "DTSTART;VALUE=DATE:20260815\r\n",
	}
	replacements := map[string]string{
		"missing uid":       "",
		"departure before":  "DTEND;VALUE=DATE:20260814\r\n",
		"unparseable start": "DTSTART;VALUE=DATE:2026August\r\n",
	}
	for name, original := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			broken := strings.Replace(
				calendarWithSummary("Reserved"), original, replacements[name], 1,
			)
			if _, err := calendarfeed.ParseCalendar(broken); err != calendarfeed.ErrMalformed {
				t.Fatalf("ParseCalendar(%s) error = %v, want ErrMalformed", name, err)
			}
		})
	}
}

func calendarWithSummary(summary string) string {
	return "BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\n" +
		"DTSTART;VALUE=DATE:20260815\r\n" +
		"DTEND;VALUE=DATE:20260818\r\n" +
		"UID:1f6d5a2e-booking-0001\r\n" +
		"SUMMARY:" + summary + "\r\n" +
		"END:VEVENT\r\nEND:VCALENDAR\r\n"
}
