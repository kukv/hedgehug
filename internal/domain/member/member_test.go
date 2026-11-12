package member_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/member"
)

func TestValidLogin(t *testing.T) {
	for s, want := range map[string]bool{
		"taro": true, "Taro_2": true, "a-b": true, "": false, "hedgehug[bot]": false, "ta ro": false,
		"abcdefghijabcdefghijabcdefghijabcdefghi": true, "abcdefghijabcdefghijabcdefghijabcdefghij": false,
	} {
		if member.ValidLogin(s) != want {
			t.Errorf("ValidLogin(%q) != %v", s, want)
		}
	}
}

func TestFromLog(t *testing.T) {
	ts := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	var log []change.Record
	n := 0
	add := func(d change.Draft) {
		n++
		meta := change.Meta{Op: "o", At: ts.Add(time.Duration(n) * time.Millisecond), Actor: change.User("x")}
		id := change.ID(fmt.Sprintf("r%02d", n))
		log = append(log, change.Stamp(log, []change.Draft{d}, meta, func() change.ID { return id })...)
	}
	add(member.Init())
	add(member.Join("Taro"))
	add(member.Join("hanako"))
	add(member.Rename("taro", "Taro", "太郎"))
	got := member.FromLog(log)
	want := []member.Member{{Login: "hanako", Name: "hanako"}, {Login: "Taro", Name: "太郎"}}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("FromLog = %+v, want %+v", got, want)
	}
}

func TestFromLogLoginIsCaseInsensitive(t *testing.T) {
	ts := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	var log []change.Record
	n := 0
	add := func(d change.Draft) {
		n++
		meta := change.Meta{Op: "o", At: ts.Add(time.Duration(n) * time.Millisecond), Actor: change.User("x")}
		id := change.ID(fmt.Sprintf("r%02d", n))
		log = append(log, change.Stamp(log, []change.Draft{d}, meta, func() change.ID { return id })...)
	}
	add(member.Init())
	add(member.Join("Taro"))
	add(member.Join("taro"))
	got := member.FromLog(log)
	if len(got) != 1 || got[0] != (member.Member{Login: "Taro", Name: "Taro"}) {
		t.Errorf("FromLog = %+v, want one member Taro", got)
	}
}
