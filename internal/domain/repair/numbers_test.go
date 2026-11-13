package repair_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/repair"
)

var t0 = time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)

// num は、作成の行 row(時刻 t0 の mins 分後)を持つ番号 n のものを作る。
func num(id string, n int, row string, mins int) repair.Numbered {
	return repair.Numbered{ID: change.ID(id), Number: n, Create: change.Record{ID: change.ID(row), At: t0.Add(time.Duration(mins) * time.Minute)}}
}

func facts(ours, theirs []change.ID) repair.Facts {
	f := repair.Facts{Ours: map[change.ID]bool{}, Theirs: map[change.ID]bool{}}
	for _, id := range ours {
		f.Ours[id] = true
	}
	for _, id := range theirs {
		f.Theirs[id] = true
	}
	return f
}

func TestRenumbers(t *testing.T) {
	tests := []struct {
		name string
		all  []repair.Numbered
		f    repair.Facts
		next int
		want []repair.Renumber
	}{
		{"重ならなければ空", []repair.Numbered{num("a", 1, "ra", 0), num("b", 2, "rb", 1)}, facts([]change.ID{"ra", "rb"}, nil), 3, nil},
		{
			"Ours の方を振り直す",
			[]repair.Numbered{num("a", 3, "ra", 0), num("b", 3, "rb", 5)},
			facts([]change.ID{"ra"}, []change.ID{"rb"}), 7,
			[]repair.Renumber{{ID: "a", From: 3, To: 7}},
		},
		{
			"両方 remote なら作成が新しい方",
			[]repair.Numbered{num("a", 3, "ra", 5), num("b", 3, "rb", 1)},
			facts(nil, []change.ID{"ra", "rb"}), 7,
			[]repair.Renumber{{ID: "a", From: 3, To: 7}},
		},
		{
			"時刻が同じなら内部 ID が後の方",
			[]repair.Numbered{num("a", 3, "ra", 1), num("b", 3, "rb", 1)},
			facts(nil, []change.ID{"ra", "rb"}), 7,
			[]repair.Renumber{{ID: "b", From: 3, To: 7}},
		},
		{
			"3 件のうち Ours が 2 件なら Ours の 2 件を内部 ID の順に",
			[]repair.Numbered{num("c", 3, "rc", 0), num("b", 3, "rb", 9), num("a", 3, "ra", 8)},
			facts([]change.ID{"ra", "rb"}, []change.ID{"rc"}), 7,
			[]repair.Renumber{{ID: "a", From: 3, To: 7}, {ID: "b", From: 3, To: 8}},
		},
		{
			"旧い番号の小さい順",
			[]repair.Numbered{num("a", 5, "ra", 0), num("b", 5, "rb", 1), num("c", 3, "rc", 0), num("d", 3, "rd", 1)},
			facts([]change.ID{"rb", "rd"}, []change.ID{"ra", "rc"}), 7,
			[]repair.Renumber{{ID: "d", From: 3, To: 7}, {ID: "b", From: 5, To: 8}},
		},
		{
			"全部 Ours なら最初に作った 1 件は残す",
			[]repair.Numbered{num("a", 3, "ra", 0), num("b", 3, "rb", 1)},
			facts([]change.ID{"ra", "rb"}, nil), 7,
			[]repair.Renumber{{ID: "b", From: 3, To: 7}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := repair.Renumbers(tt.all, tt.f, tt.next); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Renumbers = %+v, want %+v", got, tt.want)
			}
		})
	}
}
