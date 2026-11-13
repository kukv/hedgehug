package history

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
)

// linkText は、実行者「GitHub」の links の行の履歴の文(D3-12。06 の文から「{ID} の」を除いたもの)。
// 手での追加・解除(実行者が人)は呼ばれない。初めの値(from が null)と、値の変わらない set は出さない。
func linkText(log []change.Record, i int) (string, bool) {
	r := log[i]
	switch r.T {
	case change.Set:
		return linkSetText(r)
	case change.Add:
		var to itemlink.Ref
		if r.F != itemlink.LinksField || json.Unmarshal(r.To, &to) != nil {
			return "", false
		}
		if from, ok := removedInOp(log, r.Op); ok {
			return fmt.Sprintf("リンクを %s から %s に更新", from.Display(), to.Display()), true
		}
		return to.Display() + " を自動でリンク", true
	}
	return "", false
}

func linkSetText(r change.Record) (string, bool) {
	if string(r.From) == "null" || string(r.From) == string(r.To) {
		return "", false
	}
	rest := strings.TrimPrefix(r.F, itemlink.LinksField+"/")
	cut := strings.LastIndex(rest, "/")
	colon := strings.Index(rest, ":")
	if cut < 0 || colon < 0 || colon > cut {
		return "", false
	}
	target, name := rest[colon+1:cut], rest[cut+1:]
	var to string
	if json.Unmarshal(r.To, &to) != nil {
		return "", false
	}
	switch name {
	case "state":
		return fmt.Sprintf("%s の状態を %s に変更", target, itemlink.StateText(itemlink.State(to))), true
	case "review":
		return fmt.Sprintf("%s のレビューを %s に変更", target, itemlink.ReviewText(itemlink.Review(to))), true
	case "title":
		return fmt.Sprintf("%s のタイトルを「%s」に変更", target, strings.Join(strings.Fields(to), " ")), true
	}
	return "", false
}

// removedInOp は、同じ保存(op)で外したリンク(改名・移管の追従。D3-11)。
func removedInOp(log []change.Record, op change.ID) (itemlink.Ref, bool) {
	for _, r := range log {
		var ref itemlink.Ref
		if r.Op == op && r.T == change.Remove && r.F == itemlink.LinksField && json.Unmarshal(r.From, &ref) == nil {
			return ref, true
		}
	}
	return itemlink.Ref{}, false
}
