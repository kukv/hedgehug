package query

import "testing"

func TestUsesKey(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{"customer:A", true},
		{"-Customer:A status:Todo", true},
		{"status:Todo customer", false},
		{"status:Todo", false},
		{"status:Todo customer>3", true},
		{"status:customer", false},
	}
	for _, c := range cases {
		if got := UsesKey(c.text, "customer"); got != c.want {
			t.Errorf("UsesKey(%q) = %v, want %v", c.text, got, c.want)
		}
	}
	if UsesKey("status:Todo", "status") {
		t.Error("組み込みのキーは使うことにしない")
	}
}
