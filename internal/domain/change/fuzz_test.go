package change_test

import (
	"fmt"
	"math/rand/v2"
	"reflect"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// randomLog は、1 つのエンティティのログを乱数で作る。prev は、書いた時点の先端の一部だけを見たことにして、
// 同時の変更(先端が複数)を作る。時刻と by は重なりやすくして、比べ方の全段を通す。
func randomLog(seed uint64) []change.Record {
	rng := rand.New(rand.NewPCG(seed, 0))
	log := []change.Record{create(map[string]any{"a": 0, "b": 0, "systems": []string{"Y1"}})}
	bys := []string{"Ann", "ann", "bob"}
	for i := range rng.IntN(25) {
		var d change.Draft
		switch rng.IntN(4) {
		case 0, 1:
			f := []string{"a", "b"}[rng.IntN(2)]
			d = change.NewSet(f, nil, i)
		case 2:
			d = change.NewAdd("systems", fmt.Sprintf("Y%d", rng.IntN(3)))
		default:
			d = change.NewRemove("systems", fmt.Sprintf("Y%d", rng.IntN(3)))
		}
		r := change.Record{
			ID: change.ID(fmt.Sprintf("r%03d", i)), At: time.Date(2026, 1, 1, 0, rng.IntN(3), 0, 0, time.UTC),
			By: bys[rng.IntN(len(bys))], Actor: change.User("x"), T: d.T, F: d.F, From: d.From, To: d.To,
		}
		tips := change.Tips(log, r)
		for _, p := range tips {
			if rng.IntN(3) > 0 || len(r.Prev) == 0 {
				r.Prev = append(r.Prev, p)
			}
		}
		log = append(log, r)
	}
	return log
}

// どの順で適用しても、同じ値になる(保存形式「テスト」)。
func FuzzResolveOrderIndependent(f *testing.F) {
	for i := range uint64(20) {
		f.Add(i, i*7+1)
	}
	f.Fuzz(func(t *testing.T, seed, perm uint64) {
		log := randomLog(seed)
		want := change.Resolve(log, opts)
		shuffled := append([]change.Record(nil), log...)
		rand.New(rand.NewPCG(perm, 1)).Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		if got := change.Resolve(shuffled, opts); !reflect.DeepEqual(got, want) {
			t.Fatalf("order changed the result:\n%+v\n%+v", got, want)
		}
	})
}
