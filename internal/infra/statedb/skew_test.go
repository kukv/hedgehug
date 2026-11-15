package statedb

import "testing"

func TestClockSkewsRoundTrip(t *testing.T) {
	ctx := t.Context()
	s := openTemp(t).ClockSkews()

	if got, err := s.Skew(ctx); err != nil || got != 0 {
		t.Fatalf("empty: %d err=%v", got, err)
	}
	for _, want := range []int{7, -6, 0} {
		if err := s.SetSkew(ctx, want); err != nil {
			t.Fatal(err)
		}
		if got, err := s.Skew(ctx); err != nil || got != want {
			t.Fatalf("after SetSkew(%d): %d err=%v", want, got, err)
		}
	}
}
