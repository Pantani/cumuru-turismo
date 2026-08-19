package localdemo

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/access"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store"
	"github.com/Pantani/cumuru/apps/api/internal/questionnaire"
	"github.com/google/uuid"
)

const (
	issuer               = "https://oidc.invalid/local"
	privacyNoticeVersion = "prototype-v1"
	firstVisitMetricCode = "first_visit_share"

	// DemoAccountEmail identifies the fixture operator of the local prototype.
	// The matching secret is never compiled in: it comes from
	// LOCAL_DEMO_ACCOUNT_PASSWORD so the binary carries no credential.
	DemoAccountEmail = "operador@cumuru.local"

	demoPasswordVariable = "LOCAL_DEMO_ACCOUNT_PASSWORD"
)

var (
	demoAccountID      = uuid.MustParse("019fae14-0000-7000-8000-000000000001")
	organizationID     = uuid.MustParse("019fae10-0000-7000-8000-000000000001")
	questionnaireID    = uuid.MustParse("019fae13-0000-7000-8000-000000000001")
	versionID          = uuid.MustParse("019fae13-0000-7000-8000-000000000002")
	questionID         = uuid.MustParse("019fae13-0000-7000-8000-000000000003")
	firstVisitOptionID = uuid.MustParse("019fae13-0000-7000-8000-000000000004")
	returningOptionID  = uuid.MustParse("019fae13-0000-7000-8000-000000000005")
)

type stayFixture struct {
	key              string
	accommodationID  uuid.UUID
	arrival          time.Time
	departure        time.Time
	clock            time.Time
	guestCount       int32
	responseCategory string
	keepCheckedIn    bool
}

func foundationFixture() store.LocalDemoFoundation {
	fakeCadastur := "CADASTUR-FICTICIO-NAO-VALIDO"
	return store.LocalDemoFoundation{
		OrganizationID:   organizationID,
		OrganizationName: "Organização fictícia Cumuru",
		OIDCIssuer:       issuer,
		// The fake OIDC track is reached only by the static development token,
		// so its memberships belong to that probe subject. The human demo
		// operator authenticates through the local session issuer and gets its
		// own memberships in EnsureAccount; keeping the two subjects apart is
		// what lets the audit trail tell the probe from the operator.
		OIDCSubject:    access.DevelopmentPlatformSubject,
		Accommodations: localAccommodations(&fakeCadastur),
	}
}

// accountFixture hashes the environment-supplied demo secret. It fails closed:
// without LOCAL_DEMO_ACCOUNT_PASSWORD the seeder stops instead of creating an
// account nobody can reach or, worse, one with a guessable default.
func accountFixture(lookup func(string) (string, bool)) (store.LocalDemoAccount, error) {
	secret, ok := lookup(demoPasswordVariable)
	if !ok || strings.TrimSpace(secret) == "" {
		return store.LocalDemoAccount{}, fmt.Errorf(
			"%s is required to seed the local demo account", demoPasswordVariable,
		)
	}
	hash, err := access.NewPasswordHasher().Hash(secret)
	if err != nil {
		return store.LocalDemoAccount{}, fmt.Errorf(
			"local demo account secret is unusable: %w", err,
		)
	}
	return store.LocalDemoAccount{
		ID:           demoAccountID,
		Email:        DemoAccountEmail,
		DisplayName:  "Operadora fictícia da hospedagem",
		PasswordHash: hash,
		Scopes: []string{
			"platform:read",
			// accommodations:onboard is absent on purpose, and its absence is what
			// separates this fixture from the seeded administrator. The scope gates
			// both POST /accommodations and the three decision routes of the invite
			// queue (ADR-042), and ListAccommodationAccessRequests joins nothing
			// against core.memberships: whoever holds it decides every request of
			// the platform, not the ones of a single establishment. Admitting an
			// establishment is an act of the platform administrator, so it does not
			// belong to an account that merely operates one lodging.
			"accommodations:manage",
			"stays:read:own",
			"stays:write",
			// The approval queue and the poster panel are gated on this scope by
			// the client. Without it the local demo renders neither, and
			// local-demo-e2e would pass by never opening the self-service screens —
			// the same green-for-the-wrong-reason failure as D-01, one layer up.
			//
			// Safe here and only here: compose.local.yaml pins
			// SELF_SERVICE_ENABLED: "true", so the single runtime that loads this
			// fixture always serves the routes the scope unlocks.
			"stays:approve",
			"questionnaires:manage",
			"questionnaires:approve",
			"analytics:read:internal",
		},
	}, nil
}

// The catalogue spans every category the observatory publishes, with at least
// three houses in each: the coverage panel only reports a category that reaches
// the minimum number of reporting accommodations, so a category with one house
// would render as unavailable no matter how much history it had.
func localAccommodations(fakeCadastur *string) []store.LocalDemoAccommodation {
	return []store.LocalDemoAccommodation{
		published(localAccommodation(1, "Pousada Farol Fictícia", "formal_lodging", 24, fakeCadastur), 1, true, "https://pousada-farol.invalid/"),
		published(localAccommodation(2, "Hospedaria Rio Fictícia", "formal_lodging", 18, nil), 2, true, ""),
		published(localAccommodation(3, "Chalés Areia Fictícios", "seasonal_rental", 16, nil), 3, true, "https://chales-areia.invalid/"),
		published(localAccommodation(4, "Casa Silenciosa Fictícia", "family_hosting", 8, nil), 4, false, ""),
		published(localAccommodation(5, "Pousada Vento Sul Fictícia", "formal_lodging", 30, nil), 5, true, ""),
		published(localAccommodation(6, "Camping Ondas Fictício", "camping", 40, nil), 6, true, "https://camping-ondas.invalid/"),
		published(localAccommodation(7, "Kitnets Maré Fictícias", "seasonal_rental", 12, nil), 7, false, ""),
		// Cadastradas e não publicadas de propósito: a lista aberta mostra quem
		// consentiu, não quem existe.
		localAccommodation(8, "Quintal da Vovó Fictício", "family_hosting", 6, nil),
		published(localAccommodation(9, "Pousada Mirante Fictícia", "formal_lodging", 22, nil), 9, true, ""),
		localAccommodation(10, "Recanto Regularizando Fictício", "regularizing", 10, nil),
		published(localAccommodation(11, "Camping Rio das Ostras Fictício", "camping", 28, nil), 11, true, ""),
		published(localAccommodation(12, "Camping Barra Fictício", "camping", 34, nil), 12, false, ""),
		published(localAccommodation(13, "Flats Coqueiral Fictícios", "seasonal_rental", 14, nil), 13, true, ""),
		published(localAccommodation(14, "Casa da Ponte Fictícia", "family_hosting", 5, nil), 14, true, ""),
		published(localAccommodation(15, "Sítio em Regularização Fictício", "regularizing", 12, nil), 15, false, ""),
		localAccommodation(16, "Chácara Regularizando Fictícia", "regularizing", 9, nil),
	}
}

func localAccommodation(
	index int,
	name string,
	category string,
	capacity int32,
	cadasturID *string,
) store.LocalDemoAccommodation {
	return store.LocalDemoAccommodation{
		ID:             localAccommodationID(index),
		Name:           name,
		Category:       category,
		CadasturID:     cadasturID,
		Capacity:       capacity,
		PublicAreaCode: "cumuruxatiba",
	}
}

// Telefone fictício em E.164, derivado do índice do próprio fixture: nenhum
// número real da região entra aqui, porque o fixture roda na máquina de
// qualquer pessoa e o telefone publicado vira link discável.
func published(
	accommodation store.LocalDemoAccommodation,
	index int,
	whatsapp bool,
	website string,
) store.LocalDemoAccommodation {
	phone := fmt.Sprintf("+557399999%04d", index)
	accommodation.PublicPhone = &phone
	accommodation.PublicWhatsApp = whatsapp
	if website != "" {
		accommodation.PublicWebsite = &website
	}
	return accommodation
}

func questionnaireDefinition() questionnaire.Definition {
	introduction := "Participação voluntária com dados fictícios e finalidade demonstrativa."
	help := "Escolha somente uma opção. Não informe dados pessoais."
	analyticsKey := firstVisitMetricCode
	minimumCell := int32(10)
	return questionnaire.Definition{
		Title:                "Pesquisa turística de demonstração",
		Introduction:         &introduction,
		PrivacyNoticeVersion: privacyNoticeVersion,
		Questions: []questionnaire.Question{
			{
				ID:                       questionID,
				StableKey:                "visit_profile",
				Prompt:                   "Esta é sua primeira visita a Cumuruxatiba?",
				HelpText:                 &help,
				AnswerType:               questionnaire.AnswerSingleChoice,
				Required:                 false,
				DataClassification:       questionnaire.ClassificationOperational,
				PurposeCode:              "tourism_planning",
				RetentionPolicyCode:      "prototype_aggregate_only",
				AnalyticsKey:             &analyticsKey,
				PublicAggregationAllowed: true,
				MinimumPublicCell:        &minimumCell,
				DisplayOrder:             1,
				Options: []questionnaire.Option{
					{
						ID:           firstVisitOptionID,
						Value:        "first_visit",
						Label:        "Sim, é a primeira visita",
						DisplayOrder: 1,
					},
					{
						ID:           returningOptionID,
						Value:        "returning",
						Label:        "Não, já visitei antes",
						DisplayOrder: 2,
					},
				},
			},
		},
		ConsentRequirements: []questionnaire.ConsentRequirement{
			{
				PurposeCode:       "tourism_planning",
				NoticeVersion:     privacyNoticeVersion,
				Prompt:            "Aceito o uso agregado desta resposta fictícia para demonstrar o observatório.",
				RequiredForAnswer: true,
				DisplayOrder:      1,
			},
		},
	}
}

func mappingFixtures() []store.LocalDemoMetricMapping {
	result := make([]store.LocalDemoMetricMapping, 0, 2)
	for _, category := range []string{"first_visit", "returning"} {
		result = append(result, metricMapping(category))
	}
	return result
}

func metricMapping(category string) store.LocalDemoMetricMapping {
	return store.LocalDemoMetricMapping{
		PrivacyPolicyVersion:   privacyNoticeVersion,
		MetricCode:             firstVisitMetricCode,
		QuestionnaireVersionID: versionID,
		QuestionID:             questionID,
		SourceValue:            category,
		CategoryCode:           category,
	}
}

func localAccommodationID(index int) uuid.UUID {
	return uuid.MustParse(fmt.Sprintf(
		"019fae11-0000-7000-8000-%012x",
		index,
	))
}

func deterministicUUID(namespace string, ordinal int) uuid.UUID {
	digest := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", namespace, ordinal)))
	var result uuid.UUID
	copy(result[:], digest[:16])
	result[6] = (result[6] & 0x0f) | 0x70
	result[8] = (result[8] & 0x3f) | 0x80
	return result
}

func civilDay(value time.Time, location *time.Location) time.Time {
	local := value.In(location)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
}

func civilNoon(value time.Time, location *time.Location) time.Time {
	local := value.In(location)
	return time.Date(local.Year(), local.Month(), local.Day(), 12, 0, 0, 0, location)
}
