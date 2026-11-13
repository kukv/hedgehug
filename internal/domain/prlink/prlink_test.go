package prlink_test

import (
	"reflect"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/prlink"
)

const ulid = "01JF0000000000000000000001"

func TestRead(t *testing.T) {
	lower := "01jf0000000000000000000001"
	tests := []struct {
		name  string
		in    prlink.Text
		ids   []string
		marks []string
	}{
		{"ブランチ hh/", prlink.Text{Branch: "hh/WEB-1-login"}, []string{"WEB-1"}, nil},
		{"ブランチ hedgehug/ は小文字でも読む", prlink.Text{Branch: "hedgehug/web-12"}, []string{"WEB-12"}, nil},
		{"ブランチの hh は大文字でも読む", prlink.Text{Branch: "HH/WEB-1"}, []string{"WEB-1"}, nil},
		{"ブランチの番号のあとが英数字なら読まない", prlink.Text{Branch: "hh/WEB-1x"}, nil, nil},
		{"ブランチの接頭辞が違えば読まない", prlink.Text{Branch: "feature/WEB-1"}, nil, nil},
		{"タイトルの ID", prlink.Text{Title: "WEB-3 を直す"}, []string{"WEB-3"}, nil},
		{"前がハイフン", prlink.Text{Title: "PRE-WEB-7"}, nil, nil},
		{"後ろがハイフン", prlink.Text{Title: "WEB-7-2"}, nil, nil},
		{"小文字は読まない", prlink.Text{Title: "web-3"}, nil, nil},
		{"プロジェクトの ID は読まない", prlink.Text{Title: "WEB-P1"}, nil, nil},
		{"番号の先頭の 0", prlink.Text{Title: "WEB-01"}, nil, nil},
		{"句読点の隣", prlink.Text{Title: "(WEB-4)、WEB-5。"}, []string{"WEB-4", "WEB-5"}, nil},
		{"本文の印は小文字でも大文字にする", prlink.Text{Body: "hedgehug:" + lower}, nil, []string{ulid}},
		{"本文に印があるとタイトルと本文の ID は読まない", prlink.Text{Title: "WEB-8", Body: "hh:" + ulid + "\nWEB-9"}, nil, []string{ulid}},
		{"タイトルの印は本文の ID を止めない", prlink.Text{Title: "hh:" + ulid, Body: "WEB-9"}, []string{"WEB-9"}, []string{ulid}},
		{"前が英数字の印", prlink.Text{Body: "xhh:" + ulid}, nil, nil},
		{"後ろが英数字の印", prlink.Text{Body: "hh:" + ulid + "x"}, nil, nil},
		{"コメントは ID と印を読む", prlink.Text{Body: "WEB-2", Comments: []string{"WEB-3", "hh:" + ulid}}, []string{"WEB-2", "WEB-3"}, []string{ulid}},
		{"本文の印はコメントの ID を止めない", prlink.Text{Body: "hh:" + ulid, Comments: []string{"WEB-3"}}, []string{"WEB-3"}, []string{ulid}},
		{"ID の重なりなし", prlink.Text{Branch: "hh/WEB-1", Title: "WEB-1"}, []string{"WEB-1"}, nil},
		{"印の重なりなし", prlink.Text{Title: "hh:" + ulid, Comments: []string{"hedgehug:" + lower}}, nil, []string{ulid}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := prlink.Read(tc.in)
			if !reflect.DeepEqual(got.IDs, tc.ids) || !reflect.DeepEqual(got.Marks, tc.marks) {
				t.Fatalf("Read = IDs %v Marks %v, want IDs %v Marks %v", got.IDs, got.Marks, tc.ids, tc.marks)
			}
		})
	}
}

func TestSplitID(t *testing.T) {
	tests := []struct {
		in     string
		prefix string
		number int
		ok     bool
	}{
		{"WEB-12", "WEB", 12, true},
		{"WEB", "", 0, false},
		{"WEB-0", "", 0, false},
		{"A1-3", "A1", 3, true},
		{"WEB-99999999999999999999", "", 0, false},
	}
	for _, tc := range tests {
		prefix, number, ok := prlink.SplitID(tc.in)
		if prefix != tc.prefix || number != tc.number || ok != tc.ok {
			t.Errorf("SplitID(%q) = %q, %d, %v", tc.in, prefix, number, ok)
		}
	}
}
