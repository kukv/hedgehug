// Package numbering は、ワークスペースの連番(アイテム用・プロジェクト用)を持つ。
// 番号を振るたびに、ワークスペースのログに number の行を書き、使った最大番号はその行の最大値で決める
// (保存形式「移動・番号・旧 ID」)。同時に振っても小さくならず、番号は使い回さない。
package numbering

import (
	"encoding/json"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// Type は、番号を使った行の種類。
const Type change.Type = "number"

// Kind は、どの連番か。行の f に書く。
type Kind string

const (
	Items    Kind = "items"
	Projects Kind = "projects"
)

// Draft は、番号 n を使った行。
func Draft(k Kind, n int) change.Draft {
	return change.Draft{T: Type, F: string(k), To: change.Value(n)}
}

// Next は、ワークスペースのログから、次に振る番号を返す。
func Next(wsLog []change.Record, k Kind) int {
	highest := 0
	for _, r := range wsLog {
		if r.T != Type || r.F != string(k) {
			continue
		}
		var n int
		if json.Unmarshal(r.To, &n) == nil && n > highest {
			highest = n
		}
	}
	return highest + 1
}
