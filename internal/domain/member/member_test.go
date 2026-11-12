package member_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/member"
)

var base = time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)

// stampAt は、ドラフトを時刻 at・ID id の 1 行にして log に足す。
func stampAt(log []change.Record, d change.Draft, at time.Time, id string) []change.Record {
	meta := change.Meta{Op: "o", At: at, Actor: change.User("x")}
	return append(log, change.Stamp(log, []change.Draft{d}, meta, func() change.ID { return change.ID(id) })...)
}

// build は、ドラフトを 1 ミリ秒ずつずらして順に足す。
func build(drafts ...change.Draft) []change.Record {
	var log []change.Record
	for i, d := range drafts {
		log = stampAt(log, d, base.Add(time.Duration(i+1)*time.Millisecond), fmt.Sprintf("r%02d", i+1))
	}
	return log
}

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
	log := build(member.Init(), member.Join("Taro"), member.Join("hanako"), member.Rename("taro", "Taro", "太郎"))
	got := member.FromLog(log)
	want := []member.Member{{Login: "hanako", Name: "hanako"}, {Login: "Taro", Name: "太郎"}}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("FromLog = %+v, want %+v", got, want)
	}
}

func TestFromLogLoginIsCaseInsensitive(t *testing.T) {
	for name, tc := range map[string]struct{ first, second string }{
		"upper first": {"Taro", "taro"},
		"lower first": {"taro", "Taro"},
	} {
		t.Run(name, func(t *testing.T) {
			got := member.FromLog(build(member.Init(), member.Join(tc.first), member.Join(tc.second)))
			if len(got) != 1 || got[0] != (member.Member{Login: tc.first, Name: tc.first}) {
				t.Errorf("FromLog = %+v, want one member %s", got, tc.first)
			}
		})
	}
}

func TestFromLogEqualAtUsesSmallerID(t *testing.T) {
	log := stampAt(nil, member.Init(), base, "r00")
	log = stampAt(log, member.Join("Taro"), base.Add(time.Second), "b")
	log = stampAt(log, member.Join("taro"), base.Add(time.Second), "a")
	got := member.FromLog(log)
	if len(got) != 1 || got[0].Login != "taro" {
		t.Errorf("FromLog = %+v, want taro (smaller id)", got)
	}
}

func TestFromLogIgnoresInvalidLogin(t *testing.T) {
	got := member.FromLog(build(member.Init(), member.Join("hedgehug[bot]"), member.Join("taro")))
	if len(got) != 1 || got[0].Login != "taro" {
		t.Errorf("FromLog = %+v, want only taro", got)
	}
}
