package model

import (
	"fmt"
	"time"

	"go.yaml.in/yaml/v4"
)

// Date is a calendar date with no time or zone, written as 2006-01-02.
type Date struct{ t time.Time }

// ParseDate parses a date such as "2026-11-03".
func ParseDate(s string) (Date, error) {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return Date{}, fmt.Errorf("invalid date %q: use YYYY-MM-DD", s)
	}
	return Date{t}, nil
}

func (d Date) String() string { return d.t.Format(time.DateOnly) }
func (d Date) IsZero() bool   { return d.t.IsZero() }

// Time returns midnight UTC of d.
func (d Date) Time() time.Time { return d.t }

// MarshalYAML writes d as a plain timestamp scalar, so it is not quoted.
func (d Date) MarshalYAML() (any, error) {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!timestamp", Value: d.String()}, nil
}

func (d *Date) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind != yaml.ScalarNode {
		return fmt.Errorf("line %d: want a date", n.Line)
	}
	p, err := ParseDate(n.Value)
	if err != nil {
		return err
	}
	*d = p
	return nil
}
