package model

import (
	"testing"

	"go.yaml.in/yaml/v4"
)

func TestParseDate(t *testing.T) {
	t.Parallel()
	d, err := ParseDate("2026-11-03")
	if err != nil || d.String() != "2026-11-03" || d.IsZero() {
		t.Errorf("ParseDate = %v, %v", d, err)
	}
	for _, bad := range []string{"", "2026-13-01", "2026/11/03", "2026-11-03T00:00:00Z"} {
		if _, err := ParseDate(bad); err == nil {
			t.Errorf("ParseDate(%q) succeeded", bad)
		}
	}
	if !(Date{}).IsZero() {
		t.Error("zero Date is not zero")
	}
}

func TestDateYAML(t *testing.T) {
	t.Parallel()
	var v struct {
		D Date `yaml:"d"`
	}
	if err := yaml.Load([]byte("d: 2026-11-03\n"), &v); err != nil {
		t.Fatal(err)
	}
	out, err := yaml.Dump(v)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(out), "d: 2026-11-03\n"; got != want {
		t.Errorf("Dump = %q, want %q (no quotes, no time)", got, want)
	}
}
