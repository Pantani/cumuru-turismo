package analytics

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"
)

var ErrInvalidPublication = errors.New("invalid publication")

type PublicationCell struct {
	CellKey            string
	MetricCode         string
	PeriodSelector     string
	PeriodStart        string
	PeriodEnd          string
	Unit               string
	DimensionCode      string
	CategoryCode       string
	Kind               string
	Status             CellStatus
	ExactValue         *float64
	ExactLower         *float64
	ExactCentral       *float64
	ExactUpper         *float64
	SampleSize         int32
	AccommodationCount int32
	PublishedValue     *int32
	PublishedLower     *int32
	PublishedCentral   *int32
	PublishedUpper     *int32
}

type Publication struct {
	RunID                uuid.UUID
	AsOfOn               string
	DataMode             string
	PrivacyPolicyVersion string
	MethodologyVersion   string
	CoverageStatus       string
	CoverageRatioPercent *int32
	PublishedAt          time.Time
	Cells                []PublicationCell
}

func Fingerprint(publication Publication) (string, error) {
	canonical := publication
	canonical.RunID = uuid.Nil
	canonical.PublishedAt = time.Time{}
	canonical.Cells = append([]PublicationCell(nil), publication.Cells...)
	sort.Slice(canonical.Cells, func(first, second int) bool {
		return canonical.Cells[first].CellKey < canonical.Cells[second].CellKey
	})
	if !validPublication(canonical) {
		return "", ErrInvalidPublication
	}
	payload, err := json.Marshal(canonical)
	if err != nil {
		return "", ErrInvalidPublication
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func validPublication(publication Publication) bool {
	return validPublicationHeader(publication) &&
		validPublicationCells(publication.Cells)
}

func validPublicationVersions(publication Publication) bool {
	return publication.PrivacyPolicyVersion != "" &&
		publication.MethodologyVersion != ""
}

func validPublicationHeader(publication Publication) bool {
	if publication.AsOfOn == "" || publication.DataMode == "" {
		return false
	}
	return validPublicationVersions(publication) &&
		publication.CoverageStatus != "" &&
		len(publication.Cells) > 0
}

func validPublicationCells(cells []PublicationCell) bool {
	previousKey := ""
	for _, cell := range cells {
		if !validPublicationCell(cell) || cell.CellKey == previousKey {
			return false
		}
		previousKey = cell.CellKey
	}
	return true
}

func validPublicationCell(cell PublicationCell) bool {
	return cell.CellKey != "" &&
		cell.MetricCode != "" &&
		cell.PeriodSelector != ""
}
