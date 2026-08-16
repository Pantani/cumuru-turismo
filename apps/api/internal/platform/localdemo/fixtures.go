package localdemo

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/access"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store"
	"github.com/Pantani/cumuru/apps/api/internal/questionnaire"
	"github.com/Pantani/cumuru/apps/api/internal/stay"
	"github.com/google/uuid"
)

const (
	issuer               = "https://oidc.invalid/local"
	operatorSubject      = "fixture-operator"
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
	responseCategory string
	keepCheckedIn    bool
}

func foundationFixture() store.LocalDemoFoundation {
	fakeCadastur := "CADASTUR-FICTICIO-NAO-VALIDO"
	return store.LocalDemoFoundation{
		OrganizationID:   organizationID,
		OrganizationName: "Organização fictícia Cumuru",
		OIDCIssuer:       issuer,
		OIDCSubject:      operatorSubject,
		Accommodations: []store.LocalDemoAccommodation{
			localAccommodation(1, "Pousada Farol Fictícia", "formal_lodging", 24, &fakeCadastur),
			localAccommodation(2, "Hospedaria Rio Fictícia", "formal_lodging", 18, nil),
			localAccommodation(3, "Chalés Areia Fictícios", "seasonal_rental", 16, nil),
			localAccommodation(4, "Casa Silenciosa Fictícia", "family_hosting", 8, nil),
		},
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
			"accommodations:onboard",
			"accommodations:manage",
			"stays:read:own",
			"stays:write",
			"questionnaires:manage",
			"questionnaires:approve",
			"analytics:read:internal",
		},
	}, nil
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

func stayFixtures(now time.Time, location *time.Location) []stayFixture {
	today := civilDay(now, location)
	previousMonth := today.AddDate(0, -1, -(today.Day() - 1))
	result := make([]stayFixture, 0, 27)
	result = append(
		result,
		historicalStayFixtures(now, today, previousMonth, location)...,
	)
	return append(result, currentStayFixtures(now, today)...)
}

func historicalStayFixtures(
	now time.Time,
	today time.Time,
	previousMonth time.Time,
	location *time.Location,
) []stayFixture {
	result := make([]stayFixture, 0, 24)
	currentMonth := today.AddDate(0, 0, -(today.Day() - 1))
	for week := 1; week <= 8; week++ {
		for accommodation := 1; accommodation <= 3; accommodation++ {
			index := (week-1)*3 + accommodation
			arrival := currentMonth.AddDate(0, 0, -week*7)
			clock, category := responseFixtureDetails(
				index,
				now,
				previousMonth,
				location,
			)
			result = append(result, stayFixture{
				key: fmt.Sprintf(
					"history-%s-%02d",
					previousMonth.Format("2006-01"),
					index,
				),
				accommodationID:  localAccommodationID(accommodation),
				arrival:          arrival,
				departure:        arrival.AddDate(0, 0, 1),
				clock:            clock,
				responseCategory: category,
			})
		}
	}
	return result
}

func currentStayFixtures(now, today time.Time) []stayFixture {
	result := make([]stayFixture, 0, 3)
	for accommodation := 1; accommodation <= 3; accommodation++ {
		result = append(result, stayFixture{
			key:             fmt.Sprintf("current-%02d", accommodation),
			accommodationID: localAccommodationID(accommodation),
			arrival:         today.AddDate(0, 0, -2),
			departure:       today.AddDate(0, 0, 35),
			clock:           now,
			keepCheckedIn:   true,
		})
	}
	return result
}

func responseFixtureDetails(
	index int,
	now time.Time,
	previousMonth time.Time,
	location *time.Location,
) (time.Time, string) {
	if index > 20 {
		return now, ""
	}
	clock := civilNoon(previousMonth.AddDate(0, 0, index-1), location)
	if index <= 10 {
		return clock, "first_visit"
	}
	return clock, "returning"
}

func localAccommodationID(index int) uuid.UUID {
	return uuid.MustParse(fmt.Sprintf(
		"019fae11-0000-7000-8000-%012x",
		index,
	))
}

func fixtureVisitors(key string) []stay.Visitor {
	result := make([]stay.Visitor, 0, 4)
	for index := 1; index <= 4; index++ {
		role := stay.VisitorCompanion
		if index == 1 {
			role = stay.VisitorResponsible
		}
		age := stay.Age25To34
		if index%2 == 0 {
			age = stay.Age35To44
		}
		result = append(result, stay.Visitor{
			ClientID:          deterministicUUID("visitor-"+key, index).String(),
			Role:              role,
			AgeBand:           age,
			ResidenceCountry:  "BR",
			ResidenceState:    "BA",
			ResidenceCityCode: "2925509",
		})
	}
	return result
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
