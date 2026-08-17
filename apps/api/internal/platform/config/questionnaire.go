package config

import (
	"time"
)

const (
	maximumSurveyTTL   = 24 * time.Hour
	aesGCM256KeyLength = 32
)

type QuestionnaireConfig struct {
	Enabled                bool
	PrimaryStableKey       string
	SurveyTTL              time.Duration
	FreeTextTTL            time.Duration
	SurveySubmitRateLimit  int
	FreeTextCleanupEnabled bool
	SurveyKeys             KeyringConfig
	FreeTextKeys           KeyringConfig
}

func loadQuestionnaire(
	environment Environment,
	core CoreConfig,
	lookup LookupEnv,
) (QuestionnaireConfig, error) {
	enabled, err := parseBoolean(lookup, "QUESTIONNAIRE_ENABLED", false)
	if err != nil {
		return QuestionnaireConfig{}, err
	}
	config := QuestionnaireConfig{
		Enabled:          enabled,
		PrimaryStableKey: "tourism_profile",
	}
	if !enabled {
		return config, nil
	}
	if !localOrTest(environment) {
		return QuestionnaireConfig{}, invalid("QUESTIONNAIRE_ENABLED")
	}

	if err := applyQuestionnaireSettings(&config, core, lookup); err != nil {
		return QuestionnaireConfig{}, err
	}
	return config, nil
}

// A survey keyring must never share a key with a core keyring: one leaked
// key would otherwise span two capability domains.
func (k questionnaireKeyrings) distinctFrom(core CoreConfig) error {
	if keyringsOverlap(append(core.keyrings(), k.survey, k.freeText)) {
		return invalid("QUESTIONNAIRE_KEYRINGS")
	}
	return nil
}

func applyQuestionnaireSettings(
	config *QuestionnaireConfig,
	core CoreConfig,
	lookup LookupEnv,
) error {
	keys, err := loadQuestionnaireKeyrings(lookup)
	if err != nil {
		return err
	}
	reader := newEnvReader(lookup)
	config.SurveyTTL = reader.duration("SURVEY_CAPABILITY_TTL", maximumSurveyTTL)
	config.FreeTextTTL = reader.duration("SURVEY_FREE_TEXT_TTL", maximumSurveyTTL)
	config.SurveySubmitRateLimit = reader.integer("SURVEY_SUBMIT_RATE_LIMIT", 10)
	config.FreeTextCleanupEnabled = reader.boolean(
		"SURVEY_FREE_TEXT_CLEANUP_ENABLED",
		false,
	)
	if err := reader.Err(); err != nil {
		return err
	}
	config.SurveyKeys = keys.survey
	config.FreeTextKeys = keys.freeText
	return keys.distinctFrom(core)
}

type questionnaireKeyrings struct {
	survey   KeyringConfig
	freeText KeyringConfig
}

func loadQuestionnaireKeyrings(lookup LookupEnv) (questionnaireKeyrings, error) {
	surveyKeys, err := parseKeyring(
		"SURVEY_HMAC_CURRENT_VERSION",
		required(lookup, "SURVEY_HMAC_CURRENT_VERSION"),
		"SURVEY_HMAC_KEYS",
		required(lookup, "SURVEY_HMAC_KEYS"),
	)
	if err != nil {
		return questionnaireKeyrings{}, err
	}
	freeTextKeys, err := parseKeyring(
		"SURVEY_FREE_TEXT_CURRENT_VERSION",
		required(lookup, "SURVEY_FREE_TEXT_CURRENT_VERSION"),
		"SURVEY_FREE_TEXT_KEYS",
		required(lookup, "SURVEY_FREE_TEXT_KEYS"),
	)
	if err != nil {
		return questionnaireKeyrings{}, err
	}
	return questionnaireKeyrings{survey: surveyKeys, freeText: freeTextKeys}, nil
}

func (c QuestionnaireConfig) validate() error {
	if !c.Enabled {
		return nil
	}
	if c.PrimaryStableKey != "tourism_profile" {
		return invalid("QUESTIONNAIRE_PRIMARY_STABLE_KEY")
	}
	if err := c.validateDurations(); err != nil {
		return err
	}
	return c.validateSurveyPolicy()
}

// Free text is only accepted when its erase pipeline is enabled, so plaintext
// always has a deletion path.
func (c QuestionnaireConfig) validateSurveyPolicy() error {
	if c.SurveySubmitRateLimit <= 0 {
		return invalid("SURVEY_SUBMIT_RATE_LIMIT")
	}
	if !c.FreeTextCleanupEnabled {
		return invalid("SURVEY_FREE_TEXT_CLEANUP_ENABLED")
	}
	return c.validateFreeTextKeys()
}

func (c QuestionnaireConfig) validateDurations() error {
	if c.SurveyTTL <= 0 || c.SurveyTTL > maximumSurveyTTL {
		return invalid("SURVEY_CAPABILITY_TTL")
	}
	if c.FreeTextTTL <= 0 || c.FreeTextTTL > maximumSurveyTTL {
		return invalid("SURVEY_FREE_TEXT_TTL")
	}
	return nil
}

func (c QuestionnaireConfig) validateFreeTextKeys() error {
	for _, key := range c.FreeTextKeys.Keys {
		if len(key) != aesGCM256KeyLength {
			return invalid("SURVEY_FREE_TEXT_KEYS")
		}
	}
	return nil
}
