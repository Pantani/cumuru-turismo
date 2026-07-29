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

func (b Build) validate() error {
	if b.Version == "unknown" || !buildVersionPattern.MatchString(b.Version) {
		return invalidBuildMetadata("VERSION")
	}
	if b.Revision == "unknown" || !buildRevisionPattern.MatchString(b.Revision) {
		return invalidBuildMetadata("REVISION")
	}
	_, offset := b.BuiltAt.Zone()
	if b.BuiltAt.IsZero() ||
		b.BuiltAt.Equal(time.Unix(0, 0)) ||
		offset != 0 ||
		b.BuiltAt.Nanosecond() != 0 {
		return invalidBuildMetadata("BUILT_AT")
	}
	return nil
}

func invalidBuildMetadata(field string) error {
	return fmt.Errorf("invalid build metadata field %s", field)
}
