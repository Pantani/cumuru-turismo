package httpapi

import (
	"errors"
	"regexp"
	"strconv"
)

var (
	errIfMatchRequired = errors.New("If-Match required")
	errInvalidIfMatch  = errors.New("invalid If-Match")
	strongETagPattern  = regexp.MustCompile(`^"([1-9][0-9]*)"$`)
)

func parseIfMatch(value string) (int64, error) {
	if value == "" {
		return 0, errIfMatchRequired
	}
	matches := strongETagPattern.FindStringSubmatch(value)
	if len(matches) != 2 {
		return 0, errInvalidIfMatch
	}
	version, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		return 0, errInvalidIfMatch
	}
	return version, nil
}

func etag(version int64) string {
	return `"` + strconv.FormatInt(version, 10) + `"`
}
