// Package transfer は、アイテムの別のワークスペース・別の Store への移動の規則を持つ。入出力はしない。
package transfer

import (
	"bytes"
	"encoding/json"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// 転送記録の entity。
const (
	EntityItem      = "item"
	EntityProject   = "project"
	EntityWorkspace = "workspace"
)

// Record は、転送記録(transfers.jsonl)の 1 行。
// 同じ Store の別のワークスペースへの移動は、ToStore に自分の Store ID だけを書く。
type Record struct {
	ID, Op                         change.ID
	At                             time.Time
	By                             string
	Entity                         string
	Target                         change.ID
	Old, New                       string
	ToStore, ToStoreName, ToRemote string
}

// line は transfers.jsonl に書く形。キーの並びは保存形式の例に entity を足したもの。
type line struct {
	ID          change.ID `json:"id"`
	Op          change.ID `json:"op"`
	At          string    `json:"at"`
	By          string    `json:"by"`
	Entity      string    `json:"entity"`
	Target      change.ID `json:"target"`
	Old         string    `json:"old"`
	New         string    `json:"new"`
	ToStore     string    `json:"to_store"`
	ToStoreName string    `json:"to_store_name,omitempty"`
	ToRemote    string    `json:"to_remote,omitempty"`
}

// JSON は、transfers.jsonl の 1 行にする(末尾の改行なし)。
func (r Record) JSON() []byte {
	b, err := json.Marshal(line{
		ID: r.ID, Op: r.Op, At: r.At.Format(change.TimeLayout), By: r.By, Entity: r.Entity, Target: r.Target,
		Old: r.Old, New: r.New, ToStore: r.ToStore, ToStoreName: r.ToStoreName, ToRemote: r.ToRemote,
	})
	if err != nil {
		panic(err)
	}
	return b
}

// ParseRecords は、transfers.jsonl を読む。空の行・JSON として読めない行・id が空の行・at が RFC 3339 でない行は飛ばす
// (手で書き換えた行は仕様どおり直さない)。entity がない行は item として読む。
func ParseRecords(b []byte) []Record {
	var out []Record
	for _, raw := range bytes.Split(b, []byte("\n")) {
		if len(bytes.TrimSpace(raw)) == 0 {
			continue
		}
		var l line
		if json.Unmarshal(raw, &l) != nil || l.ID == "" {
			continue
		}
		at, err := time.Parse(time.RFC3339, l.At)
		if err != nil {
			continue
		}
		if l.Entity == "" {
			l.Entity = EntityItem
		}
		out = append(out, Record{
			ID: l.ID, Op: l.Op, At: at, By: l.By, Entity: l.Entity, Target: l.Target, Old: l.Old, New: l.New,
			ToStore: l.ToStore, ToStoreName: l.ToStoreName, ToRemote: l.ToRemote,
		})
	}
	return out
}
