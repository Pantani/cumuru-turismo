package application

import (
	"fmt"
	"regexp"
	"time"
)

const reproducibleBuildTimeLayout = "2006-01-02T15:04:05Z"

var (
	buildVersionPattern  = regexp.MustCompile(`^[0-9A-Za-z][0-9A-Za-z.+_-]{0,63}$`)
	buildRevisionPattern = regexp.MustCompile(`^[0-9A-Za-z][0-9A-Za-z._-]{0,127}$`)
)

func ParseBuild(version, revision, builtAt string) (Build, error) {
	buildTime, err := time.Parse(reproducibleBuildTimeLayout, builtAt)
	if err != nil || buildTime.Format(reproducibleBuildTimeLayout) != builtAt {
		return Build{}, invalidBuildMetadata("BUILT_AT")
	}
	build := Build{
		Version:  version,
		Revision: revision,
		BuiltAt:  buildTime,
	}
	if err := build.validate(); err != nil {
		return Build{}, err
	}
	return build, nil
}

// "unknown" is the linker default; seeing it means the build wrapper did not
// inject provenance, which must fail rather than ship unlabelled.
func injectedMetadata(value string, pattern *regexp.Regexp) bool {
	return value != "unknown" && pattern.MatchString(value)
}

func (b Build) validate() error {
	if !injectedMetadata(b.Version, buildVersionPattern) {
		return invalidBuildMetadata("VERSION")
	}
	if !injectedMetadata(b.Revision, buildRevisionPattern) {
		return invalidBuildMetadata("REVISION")
	}
	if !reproducibleTimestamp(b.BuiltAt) {
		return invalidBuildMetadata("BUILT_AT")
	}
	return nil
}

// A reproducible build stamps a whole-second UTC timestamp; a zero, epoch or
// zoned value means the metadata was not injected by the build wrapper.
func placeholderTimestamp(value time.Time) bool {
	return value.IsZero() || value.Equal(time.Unix(0, 0))
}

func reproducibleTimestamp(value time.Time) bool {
	if placeholderTimestamp(value) {
		return false
	}
	_, offset := value.Zone()
	return offset == 0 && value.Nanosecond() == 0
}

func invalidBuildMetadata(field string) error {
	return fmt.Errorf("invalid build metadata field %s", field)
}
