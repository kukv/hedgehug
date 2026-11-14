package statedb

import "testing"

func TestScreenStatesRoundTrip(t *testing.T) {
	ctx := t.Context()
	s := openTemp(t).ScreenStates()

	if _, ok, err := s.Get(ctx, "search"); err != nil || ok {
		t.Fatalf("empty: ok=%v err=%v", ok, err)
	}
	if err := s.Put(ctx, "search", `{"query":"a"}`); err != nil {
		t.Fatal(err)
	}
	if got, ok, err := s.Get(ctx, "search"); err != nil || !ok || got != `{"query":"a"}` {
		t.Fatalf("get: %q ok=%v err=%v", got, ok, err)
	}
	// 2 回目の Put は上書き。ほかの画面には効かない。
	if err := s.Put(ctx, "search", `{"query":"b"}`); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := s.Get(ctx, "search"); got != `{"query":"b"}` {
		t.Errorf("overwrite: %q", got)
	}
	if _, ok, _ := s.Get(ctx, "other"); ok {
		t.Error("other screen")
	}
}
