package filemerge

import "testing"

func TestMergeLogUnionsAndSorts(t *testing.T) {
	a := []byte(`{"id":"A1","at":"2026-11-12T10:00:00.000+09:00","t":"create"}
{"id":"A2","at":"2026-11-12T10:05:00.000+09:00","t":"set"}
`)
	b := []byte(`{"id":"A1","at":"2026-11-12T10:00:00.000+09:00","t":"create"}
{"id":"B1","at":"2026-11-12T01:03:00.000Z","t":"set"}
`)
	// B1 は UTC で 01:03 = JST 10:03 なので、A1 と A2 のあいだに入る。
	want := `{"id":"A1","at":"2026-11-12T10:00:00.000+09:00","t":"create"}
{"id":"B1","at":"2026-11-12T01:03:00.000Z","t":"set"}
{"id":"A2","at":"2026-11-12T10:05:00.000+09:00","t":"set"}
`
	if got := string(MergeLog(a, b)); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
	if got := string(MergeLog(b, a)); got != want {
		t.Errorf("not symmetric:\n%s", got)
	}
}

func TestMergeLogSameAtSortsByID(t *testing.T) {
	a := []byte(`{"id":"B","at":"2026-11-12T10:00:00.000Z"}` + "\n")
	b := []byte(`{"id":"A","at":"2026-11-12T10:00:00.000Z"}` + "\n")
	want := `{"id":"A","at":"2026-11-12T10:00:00.000Z"}` + "\n" + `{"id":"B","at":"2026-11-12T10:00:00.000Z"}` + "\n"
	if got := string(MergeLog(a, b)); got != want {
		t.Errorf("got\n%s", got)
	}
}

func TestMergeLogDropsMalformedLines(t *testing.T) {
	a := []byte("not json\n" + `{"id":"","at":"2026-11-12T10:00:00.000Z"}` + "\n" + `{"id":"X","at":"yesterday"}` + "\n" + `{"id":"OK","at":"2026-11-12T10:00:00.000Z"}` + "\n")
	want := `{"id":"OK","at":"2026-11-12T10:00:00.000Z"}` + "\n"
	if got := string(MergeLog(a, nil)); got != want {
		t.Errorf("got\n%s", got)
	}
}

func TestMergeLogSameIDDifferentBytesIsDeterministic(t *testing.T) {
	a := []byte(`{"id":"X","at":"2026-11-12T10:00:00.000Z","v":2}` + "\n")
	b := []byte(`{"id":"X","at":"2026-11-12T10:00:00.000Z","v":1}` + "\n")
	if string(MergeLog(a, b)) != string(MergeLog(b, a)) {
		t.Error("the result depends on the order")
	}
}

func TestMergeLogEmpty(t *testing.T) {
	if got := MergeLog(nil, nil); len(got) != 0 {
		t.Errorf("got %q", got)
	}
}
