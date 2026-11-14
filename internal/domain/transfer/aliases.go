package transfer

import (
	"sort"
	"strconv"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// Alias は、アイテム・プロジェクトの旧 ID。At は移動した日時で、旧い接頭辞から作ったものは nil。
type Alias struct {
	ID string
	At *time.Time
}

// Aliases は、entity の target の旧 ID を、(1) 転送記録の Old を At の古い順、(2) 旧い接頭辞ごとの
// 「{接頭辞}{sep}{number}」の並びで返す。重複は At の古い方(記録があるものを接頭辞から作ったものより先)を残す。
// いまの ID と同じ文字列を除くのは呼ぶ側の仕事。
func Aliases(records []Record, entity string, target change.ID, oldPrefixes []string, number int, sep string) []Alias {
	var moves []Record
	for _, r := range records {
		if r.Entity == entity && r.Target == target {
			moves = append(moves, r)
		}
	}
	sort.SliceStable(moves, func(i, j int) bool { return moves[i].At.Before(moves[j].At) })

	var out []Alias
	seen := map[string]bool{}
	for _, r := range moves {
		if seen[r.Old] {
			continue
		}
		seen[r.Old] = true
		at := r.At
		out = append(out, Alias{ID: r.Old, At: &at})
	}
	for _, p := range oldPrefixes {
		id := p + sep + strconv.Itoa(number)
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, Alias{ID: id})
	}
	return out
}
