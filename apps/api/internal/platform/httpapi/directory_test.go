package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/accommodation"
	"github.com/Pantani/cumuru/apps/api/internal/directory"
	"github.com/google/uuid"
)

type directoryReaderStub struct {
	err     error
	listing directory.Listing
}

func (f directoryReaderStub) List(context.Context) (directory.Listing, error) {
	return f.listing, f.err
}

func publishedListing() directory.Listing {
	listing, err := directory.NewListing(
		[]directory.Entry{{
			ID:       uuid.MustParse("0198f000-0000-7000-8000-000000000001"),
			Name:     "Pousada da Vila",
			Category: accommodation.CategoryFormalLodging,
			Phone:    "+5573999990001",
			WhatsApp: true,
		}},
		time.Date(2026, 8, 18, 9, 0, 0, 0, time.UTC),
	)
	if err != nil {
		panic(err)
	}
	return listing
}

func directoryRequest() *http.Request {
	return httptest.NewRequest(http.MethodGet, "/api/v1/public/accommodations", nil)
}

func TestPublicDirectoryServesTheListingWithStrongETagAndCache(t *testing.T) {
	t.Parallel()

	handler, _ := mustNew(t, Dependencies{
		PublicDirectory: directoryReaderStub{listing: publishedListing()},
	})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, directoryRequest())

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body)
	}
	if got := recorder.Header().Get("Cache-Control"); got != publicDirectoryCache {
		t.Fatalf("cache-control = %q", got)
	}
	tag := recorder.Header().Get("ETag")
	if !strongDocumentETag.MatchString(tag) {
		t.Fatalf("etag = %q", tag)
	}
	var payload directory.Listing
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode = %v", err)
	}
	if payload.Count != 1 || payload.Entries[0].Phone != "+5573999990001" {
		t.Fatalf("payload = %+v", payload)
	}

	revalidation := httptest.NewRecorder()
	request := directoryRequest()
	request.Header.Set("If-None-Match", tag)
	handler.ServeHTTP(revalidation, request)
	if revalidation.Code != http.StatusNotModified {
		t.Fatalf("revalidation status = %d", revalidation.Code)
	}
}

// A rota é aberta e sem seletor: parâmetro inesperado é recusado em vez de
// ignorado, para o cache não guardar duas cópias do mesmo documento.
func TestPublicDirectoryRejectsAnyQueryParameter(t *testing.T) {
	t.Parallel()

	handler, _ := mustNew(t, Dependencies{
		PublicDirectory: directoryReaderStub{listing: publishedListing()},
	})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(
		http.MethodGet, "/api/v1/public/accommodations?category=camping", nil,
	))

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestPublicDirectoryAnswersUnavailableWhenTheListingCannotBeRead(t *testing.T) {
	t.Parallel()

	handler, _ := mustNew(t, Dependencies{
		PublicDirectory: directoryReaderStub{err: directory.ErrUnavailable},
	})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, directoryRequest())

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestDirectoryRouteIsAbsentWithoutTheReader(t *testing.T) {
	t.Parallel()

	handler, _ := mustNew(t, Dependencies{})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, directoryRequest())

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d", recorder.Code)
	}
}
