package model

import "testing"

func TestValidKeyPrefix(t *testing.T) {
	for _, p := range []string{"WEB", "A1", "ABCDEFGHIJ"} {
		if err := ValidKeyPrefix(p); err != nil {
			t.Errorf("%q: %v", p, err)
		}
	}
	for _, p := range []string{"", "W", "web", "1AB", "ABCDEFGHIJK", "WE-B"} {
		if ValidKeyPrefix(p) == nil {
			t.Errorf("%q is accepted", p)
		}
	}
}

func TestParseKey(t *testing.T) {
	if p, n, ok := ParseKey("WEB-42"); !ok || p != "WEB" || n != 42 {
		t.Errorf("WEB-42 = %q %d %v", p, n, ok)
	}
	for _, k := range []string{"", "WEB", "WEB-0", "WEB-042", "web-1", "W-1", "WEB-1x"} {
		if _, _, ok := ParseKey(k); ok {
			t.Errorf("%q parses", k)
		}
	}
	if FormatKey("API", 7) != "API-7" {
		t.Error(FormatKey("API", 7))
	}
}

func TestKeyNumbersIn(t *testing.T) {
	got := KeyNumbersIn([]byte("key: WEB-7\nformer_keys: [WEB-12, API-3]\nsee WEB-2, xWEB-99, WEB-0, WEB-5x and api-4\n"))
	if len(got) != 2 || got["WEB"] != 12 || got["API"] != 3 {
		t.Errorf("KeyNumbersIn = %v", got)
	}
}
