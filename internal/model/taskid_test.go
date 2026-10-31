package model

import (
	"slices"
	"testing"
)

func TestValidIDPrefix(t *testing.T) {
	for _, p := range []string{"WEB", "A1", "ABCDEFGHIJ"} {
		if err := ValidIDPrefix(p); err != nil {
			t.Errorf("%q: %v", p, err)
		}
	}
	for _, p := range []string{"", "W", "web", "1AB", "ABCDEFGHIJK", "WE-B"} {
		if ValidIDPrefix(p) == nil {
			t.Errorf("%q is accepted", p)
		}
	}
}

func TestParseTaskID(t *testing.T) {
	if p, n, ok := ParseTaskID("WEB-42"); !ok || p != "WEB" || n != 42 {
		t.Errorf("WEB-42 = %q %d %v", p, n, ok)
	}
	for _, k := range []string{"", "WEB", "WEB-0", "WEB-042", "web-1", "W-1", "WEB-1x"} {
		if _, _, ok := ParseTaskID(k); ok {
			t.Errorf("%q parses", k)
		}
	}
	if FormatTaskID("API", 7) != "API-7" {
		t.Error(FormatTaskID("API", 7))
	}
}

func TestIDNumbersIn(t *testing.T) {
	got := IDNumbersIn([]byte("id: WEB-7\nformer_ids: [WEB-12, API-3]\nsee WEB-2, xWEB-99, WEB-0, WEB-5x and api-4\n"))
	if len(got) != 2 || got["WEB"] != 12 || got["API"] != 3 {
		t.Errorf("IDNumbersIn = %v", got)
	}
}

func TestCompareIDs(t *testing.T) {
	ids := []string{"WEB-10", "API-3", "WEB-9", "", "WEB-1"}
	slices.SortFunc(ids, CompareIDs)
	if want := []string{"API-3", "WEB-1", "WEB-9", "WEB-10", ""}; !slices.Equal(ids, want) {
		t.Errorf("sorted = %v", ids)
	}
}
