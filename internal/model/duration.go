package model

import (
	"fmt"
	"regexp"
	"strconv"
	"time"

	"go.yaml.in/yaml/v4"
)

// Duration is a length of work, written with hours and minutes only: "6h",
// "1h30m", "45m". Days and weeks are left out because a day could mean 8 or
// 24 hours.
type Duration time.Duration

var durationRE = regexp.MustCompile(`^(?:(\d+)h)?(?:(\d+)m)?$`)

// ParseDuration parses a duration such as "1h30m".
func ParseDuration(s string) (Duration, error) {
	m := durationRE.FindStringSubmatch(s)
	if s == "" || m == nil {
		return 0, fmt.Errorf("invalid duration %q: use hours and minutes, like 1h30m", s)
	}
	var total time.Duration
	for i, unit := range []time.Duration{time.Hour, time.Minute} {
		if m[i+1] == "" {
			continue
		}
		v, err := strconv.ParseInt(m[i+1], 10, 32)
		if err != nil {
			return 0, fmt.Errorf("invalid duration %q: %w", s, err)
		}
		total += time.Duration(v) * unit
	}
	return Duration(total), nil
}

// String writes d as "1h30m", "6h" or "45m".
func (d Duration) String() string {
	total := int64(time.Duration(d) / time.Minute)
	h, m := total/60, total%60
	switch {
	case h == 0:
		return fmt.Sprintf("%dm", m)
	case m == 0:
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dh%dm", h, m)
}

func (d Duration) MarshalYAML() (any, error) { return d.String(), nil }

func (d *Duration) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind != yaml.ScalarNode {
		return fmt.Errorf("line %d: want a duration", n.Line)
	}
	p, err := ParseDuration(n.Value)
	if err != nil {
		return err
	}
	*d = p
	return nil
}
