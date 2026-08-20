package analytics

import (
	"context"
	"errors"
)

var ErrInvalidFunnelWindow = errors.New("invalid funnel window")

// O funil não instrumenta ninguém. Ele conta estados que o registro já guarda —
// convite emitido e usado, capability de pesquisa emitida e consumida,
// autocadastro pendente e decidido — e por isso não abre canal de telemetria,
// não grava evento por pessoa e não precisa de base legal nova. É a diferença
// entre medir o funil e vigiar quem passa por ele.
const (
	// A mediana só sai com amostra a partir deste piso. Abaixo dele ela deixa
	// de descrever um comportamento e passa a descrever uma pessoa: com uma
	// única submissão na janela, a "mediana" é o tempo exato daquele hóspede.
	FunnelLatencyMinimum = 10
	funnelWindowCode     = "last_30_days"
)

type InviteFunnel struct {
	Issued              int32  `json:"issued"`
	Submitted           int32  `json:"submitted"`
	ExpiredUnused       int32  `json:"expired_unused"`
	Revoked             int32  `json:"revoked"`
	MedianHoursToSubmit *int32 `json:"median_hours_to_submit,omitempty"`
}

// SurveyFunnel não separa resposta de recusa explícita, e a razão é de
// privilégio: `app_runtime` grava em `survey.responses` e não lê de volta.
// Concluída é a capability consumida, seja qual for a participação.
type SurveyFunnel struct {
	Issued              int32  `json:"issued"`
	Completed           int32  `json:"completed"`
	ExpiredUnanswered   int32  `json:"expired_unanswered"`
	Revoked             int32  `json:"revoked"`
	MedianHoursToAnswer *int32 `json:"median_hours_to_answer,omitempty"`
}

type SelfRegistrationFunnel struct {
	Started  int32 `json:"started"`
	Pending  int32 `json:"pending"`
	Approved int32 `json:"approved"`
	Rejected int32 `json:"rejected"`
	Expired  int32 `json:"expired"`
}

type Funnel struct {
	Window           string                 `json:"window"`
	AsOf             string                 `json:"as_of"`
	Invite           InviteFunnel           `json:"invite"`
	Survey           SurveyFunnel           `json:"survey"`
	SelfRegistration SelfRegistrationFunnel `json:"self_registration"`
}

type FunnelReader interface {
	Funnel(context.Context, string) (Funnel, error)
}

// ValidFunnelWindow mantém o seletor fechado, como no resto de analytics: uma
// janela desconhecida é requisição inválida, nunca um default silencioso.
func ValidFunnelWindow(window string) bool {
	return window == funnelWindowCode
}

// LatencyMedian aplica o piso de amostra. Devolver a mediana crua e deixar a
// tela decidir espalharia a regra por dois lugares, e o lugar errado ganharia
// em algum refactor.
func LatencyMedian(medianHours float64, sample int32) *int32 {
	if sample < FunnelLatencyMinimum || medianHours < 0 {
		return nil
	}
	rounded := int32(roundHalfUp(medianHours, 1))
	return &rounded
}
