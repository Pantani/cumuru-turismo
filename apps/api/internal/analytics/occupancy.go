package analytics

// A ocupação da vila fica atrás de autenticação, e não vira célula publicada,
// por uma razão que só apareceu com a janela de 730 dias: presença já é pública
// em múltiplos de 10 e a cobertura em múltiplos de 5, então uma ocupação
// publicada tornaria `capacidade ≈ presença / ocupação` derivável. O
// arredondamento embaralha um dia, mas a capacidade é praticamente constante, e
// a média de centenas de dias converge para o valor exato — daí a diferença
// entre duas publicações denunciaria a capacidade do estabelecimento que
// entrou, que é dado individualizado de estabelecimento (docs/06, Portaria
// 41/2025). Um número por janela, para um leitor identificado, é outra escala de
// exposição: são unidades de observação, não centenas.
const (
	// O lado da vila sai na mesma malha da cobertura publicada. Bandas de cinco
	// pontos não impedem a média de convergir, mas encarecem a estimativa e
	// mantêm a leitura no nível em que ela é útil.
	VillageOccupancyRoundingBase = 5
	occupancyPercent             = 100
)

type Occupancy struct {
	// Own é exato: é dado da própria hospedagem.
	Own *int32 `json:"own_percent,omitempty"`
	// Village só existe quando o comparativo está liberado, e sai em banda.
	Village *int32 `json:"village_percent,omitempty"`
}

// OccupancyInput reúne o que as duas taxas precisam. Dias é o número de dias da
// janela publicada, para que numerador e denominador falem do mesmo período.
type OccupancyInput struct {
	Days              int
	OwnCapacity       int32
	OwnPersonDays     int64
	VillageCapacity   int64
	VillagePersonDays int64
}

// ComputeOccupancy devolve a ocupação própria sempre que houver capacidade
// declarada, e a da vila apenas com o comparativo aberto. Uma taxa acima de 100
// não é corrigida: significa capacidade declarada menor que a operação real, e
// esconder isso trocaria um problema de cadastro por um número plausível.
func ComputeOccupancy(input OccupancyInput, comparable bool) Occupancy {
	occupancy := Occupancy{
		Own: exactOccupancy(input.OwnPersonDays, int64(input.OwnCapacity), input.Days),
	}
	if !comparable {
		return occupancy
	}
	occupancy.Village = bandedOccupancy(
		input.VillagePersonDays, input.VillageCapacity, input.Days,
	)
	return occupancy
}

func occupancyRatio(personDays, capacity int64, days int) (float64, bool) {
	if capacity <= 0 || days <= 0 || personDays < 0 {
		return 0, false
	}
	available := capacity * int64(days)
	return float64(personDays) * occupancyPercent / float64(available), true
}

func exactOccupancy(personDays, capacity int64, days int) *int32 {
	ratio, ok := occupancyRatio(personDays, capacity, days)
	if !ok {
		return nil
	}
	value := int32(roundHalfUp(ratio, 1))
	return &value
}

func bandedOccupancy(personDays, capacity int64, days int) *int32 {
	ratio, ok := occupancyRatio(personDays, capacity, days)
	if !ok {
		return nil
	}
	value := int32(roundHalfUp(ratio, VillageOccupancyRoundingBase))
	return &value
}
