package localdemo

import (
	"context"
	"errors"
	"testing"

	"github.com/Pantani/cumuru/apps/api/internal/analytics"
	"github.com/Pantani/cumuru/apps/api/internal/stay"
)

type analyticsPublisherStub struct {
	calls        []string
	reconcileErr error
	publishErr   error
}

func (s *analyticsPublisherStub) Reconcile(
	context.Context,
	analytics.ReconciliationKind,
	stay.CivilDate,
) (bool, error) {
	s.calls = append(s.calls, "reconcile")
	return true, s.reconcileErr
}

func (s *analyticsPublisherStub) BuildAndPublish(
	context.Context,
	stay.CivilDate,
) (int64, bool, error) {
	s.calls = append(s.calls, "publish")
	return 1, false, s.publishErr
}

func TestReconcileAndPublishRunsInWorkerOrder(t *testing.T) {
	t.Parallel()

	stub := &analyticsPublisherStub{}
	asOf, err := stay.ParseCivilDate("2026-07-29")
	if err != nil {
		t.Fatal(err)
	}
	if err := reconcileAndPublish(context.Background(), stub, asOf); err != nil {
		t.Fatalf("reconcileAndPublish() error = %v", err)
	}
	if len(stub.calls) != 2 ||
		stub.calls[0] != "reconcile" ||
		stub.calls[1] != "publish" {
		t.Fatalf("calls = %v, want reconcile then publish", stub.calls)
	}
}

func TestReconcileAndPublishFailsClosedBeforePublication(t *testing.T) {
	t.Parallel()

	stub := &analyticsPublisherStub{reconcileErr: errors.New("reconcile")}
	asOf, err := stay.ParseCivilDate("2026-07-29")
	if err != nil {
		t.Fatal(err)
	}
	if err := reconcileAndPublish(context.Background(), stub, asOf); err == nil {
		t.Fatal("reconcileAndPublish() error = nil, want reconciliation failure")
	}
	if len(stub.calls) != 1 || stub.calls[0] != "reconcile" {
		t.Fatalf("calls = %v, publication must not run", stub.calls)
	}
}
