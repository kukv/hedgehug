package noticing

import (
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// noticed は、項目名の表で作らない項目(ウォッチ中の人・PR のリンクの状態)も、表に頼らずに除く(C4c1-5)。
func TestNoticed(t *testing.T) {
	user := change.Record{Actor: change.User("taro")}
	cases := map[string]struct {
		l    change.Loss
		want bool
	}{
		"title":      {change.Loss{T: change.Set, F: "title", Winner: user}, true},
		"watchers":   {change.Loss{T: change.Add, F: "watchers", Winner: user}, false},
		"link state": {change.Loss{T: change.Set, F: "links/L1/state", Winner: user}, false},
		"links list": {change.Loss{T: change.Add, F: "links", Winner: user}, true},
		"github":     {change.Loss{T: change.Set, F: "title", Winner: change.Record{Actor: change.Actor{Kind: change.ActorGitHub}}}, false},
		"sync":       {change.Loss{T: change.Set, F: "title", Winner: change.Record{Actor: change.Actor{Kind: change.ActorSync}}}, false},
	}
	for name, c := range cases {
		if got := noticed(c.l); got != c.want {
			t.Errorf("%s: noticed = %v, want %v", name, got, c.want)
		}
	}
}
