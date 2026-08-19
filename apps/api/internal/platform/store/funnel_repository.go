package store

import (
	"context"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/analytics"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store/generated"
	"github.com/jackc/pgx/v5/pgtype"
)

// A janela do funil é fixa em trinta dias, como a do painel de qualidade: são a
// mesma pergunta operacional lida lado a lado, e duas janelas diferentes na
// mesma tela só produziriam comparação errada.
const funnelWindowDays = 30

type FunnelRepository struct {
	store *Store
}

func NewFunnelRepository(store *Store) *FunnelRepository {
	return &FunnelRepository{store: store}
}

var _ analytics.FunnelReader = (*FunnelRepository)(nil)

func (r *FunnelRepository) Funnel(
	ctx context.Context,
	window string,
) (analytics.Funnel, error) {
	if !analytics.ValidFunnelWindow(window) {
		return analytics.Funnel{}, analytics.ErrInvalidFunnelWindow
	}
	ctx, cancel := context.WithTimeout(ctx, r.store.timeout)
	defer cancel()
	asOf := r.store.currentTime()
	bounds := funnelBounds(asOf)
	return r.compose(ctx, window, asOf, bounds)
}

type funnelWindow struct {
	start pgtype.Timestamptz
	end   pgtype.Timestamptz
	asOf  pgtype.Timestamptz
}

func (r *FunnelRepository) compose(
	ctx context.Context,
	window string,
	asOf time.Time,
	bounds funnelWindow,
) (analytics.Funnel, error) {
	invite, err := r.invite(ctx, bounds)
	if err != nil {
		return analytics.Funnel{}, err
	}
	survey, err := r.survey(ctx, bounds)
	if err != nil {
		return analytics.Funnel{}, err
	}
	selfRegistration, err := r.selfRegistration(ctx, bounds)
	if err != nil {
		return analytics.Funnel{}, err
	}
	return analytics.Funnel{
		Window: window, AsOf: asOf.Format(time.RFC3339),
		Invite: invite, Survey: survey, SelfRegistration: selfRegistration,
	}, nil
}

func (r *FunnelRepository) invite(
	ctx context.Context,
	bounds funnelWindow,
) (analytics.InviteFunnel, error) {
	row, err := r.store.queries.SummarizeInviteFunnel(
		ctx,
		generated.SummarizeInviteFunnelParams{
			AsOf: bounds.asOf, WindowStart: bounds.start, WindowEnd: bounds.end,
		},
	)
	if err != nil {
		return analytics.InviteFunnel{}, ErrUnavailable
	}
	return analytics.InviteFunnel{
		Issued: row.Issued, Submitted: row.Submitted,
		ExpiredUnused: row.ExpiredUnused, Revoked: row.Revoked,
		MedianHoursToSubmit: analytics.LatencyMedian(
			row.MedianHours, row.LatencySample,
		),
	}, nil
}

func (r *FunnelRepository) survey(
	ctx context.Context,
	bounds funnelWindow,
) (analytics.SurveyFunnel, error) {
	row, err := r.store.queries.SummarizeSurveyFunnel(
		ctx,
		generated.SummarizeSurveyFunnelParams{
			AsOf: bounds.asOf, WindowStart: bounds.start, WindowEnd: bounds.end,
		},
	)
	if err != nil {
		return analytics.SurveyFunnel{}, ErrUnavailable
	}
	return analytics.SurveyFunnel{
		Issued: row.Issued, Answered: row.Answered, Declined: row.Declined,
		ExpiredUnanswered: row.ExpiredUnanswered,
		MedianHoursToAnswer: analytics.LatencyMedian(
			row.MedianHours, row.LatencySample,
		),
	}, nil
}

func (r *FunnelRepository) selfRegistration(
	ctx context.Context,
	bounds funnelWindow,
) (analytics.SelfRegistrationFunnel, error) {
	row, err := r.store.queries.SummarizeSelfRegistrationFunnel(
		ctx,
		generated.SummarizeSelfRegistrationFunnelParams{
			WindowStart: bounds.start, WindowEnd: bounds.end,
		},
	)
	if err != nil {
		return analytics.SelfRegistrationFunnel{}, ErrUnavailable
	}
	return analytics.SelfRegistrationFunnel{
		Started: row.Started, Pending: row.Pending, Approved: row.Approved,
		Rejected: row.Rejected, Expired: row.Expired,
	}, nil
}

func funnelBounds(asOf time.Time) funnelWindow {
	return funnelWindow{
		start: pgTime(asOf.AddDate(0, 0, -funnelWindowDays)),
		end:   pgTime(asOf),
		asOf:  pgTime(asOf),
	}
}
