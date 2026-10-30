package model

import "testing"

func TestSprintRoundTrip(t *testing.T) {
	src := readFile(t, "testdata/sprint/basic.md")
	s, err := DecodeSprint(src)
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "Sprint 24" || s.Start.String() != "2026-11-02" || s.End.String() != "2026-11-13" ||
		s.State != SprintActive || s.Goal != "ログイン改修を出す" || s.Body != "## 振り返り\n" {
		t.Errorf("decoded = %+v", s)
	}
	out, err := s.Encode()
	if err != nil || string(out) != string(src) {
		t.Errorf("round trip = %q, %v", out, err)
	}
}

func TestReleaseRoundTrip(t *testing.T) {
	src := readFile(t, "testdata/release/basic.md")
	r, err := DecodeRelease(src)
	if err != nil {
		t.Fatal(err)
	}
	if r.Name != "v2.4" || r.Date.String() != "2026-11-20" || r.State != ReleaseUnreleased || r.Body != "" {
		t.Errorf("decoded = %+v", r)
	}
	out, err := r.Encode()
	if err != nil || string(out) != string(src) {
		t.Errorf("round trip = %q, %v", out, err)
	}
	r.State = ReleaseReleased
	out, _ = r.Encode()
	if !contains(out, "state: released\n") {
		t.Errorf("state not written:\n%s", out)
	}
}

func TestSprintAndReleaseNeedULID(t *testing.T) {
	if _, err := DecodeSprint([]byte("---\nname: x\n---\n")); err == nil {
		t.Error("DecodeSprint without ulid succeeded")
	}
	if _, err := DecodeRelease([]byte("---\nname: x\n---\n")); err == nil {
		t.Error("DecodeRelease without ulid succeeded")
	}
}
