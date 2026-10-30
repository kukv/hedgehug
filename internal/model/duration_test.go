package model

import (
	"testing"
	"time"

	"go.yaml.in/yaml/v4"
)

func TestParseDuration(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want time.Duration
		str  string
	}{
		{"6h", 6 * time.Hour, "6h"},
		{"1h30m", 90 * time.Minute, "1h30m"},
		{"45m", 45 * time.Minute, "45m"},
		{"90m", 90 * time.Minute, "1h30m"},
		{"0m", 0, "0m"},
	} {
		got, err := ParseDuration(tc.in)
		if err != nil || time.Duration(got) != tc.want || got.String() != tc.str {
			t.Errorf("ParseDuration(%q) = %v (%s), %v; want %v (%s)", tc.in, time.Duration(got), got, err, tc.want, tc.str)
		}
	}
	for _, bad := range []string{"", "1d", "1w", "1.5h", "h", "30m1h", "-1h", "1h 30m", "99999999999999999999h", "3000000h", "9999999999m"} {
		if _, err := ParseDuration(bad); err == nil {
			t.Errorf("ParseDuration(%q) succeeded", bad)
		}
	}
}

func TestDurationYAML(t *testing.T) {
	var v struct {
		D Duration `yaml:"d"`
	}
	if err := yaml.Load([]byte("d: 90m\n"), &v); err != nil {
		t.Fatal(err)
	}
	out, err := yaml.Dump(v)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(out), "d: 1h30m\n"; got != want {
		t.Errorf("Dump = %q, want %q", got, want)
	}
}
