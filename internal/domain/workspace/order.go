package workspace

import (
	"encoding/json"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// RankOf は、ワークスペースのログの rank のいまの値(作成の行の初期値と、あとの set を合わせた値)。
// なければ空(B2-7)。
func RankOf(log []change.Record) string {
	var r string
	_ = json.Unmarshal(change.Resolve(log, opts).Values["rank"], &r)
	return r
}

// CreateWithRank は、CreateWithSettings の作成の行に、並びの rank を足したもの。
func CreateWithRank(w Workspace, s Settings, rank string) (change.Draft, error) {
	d, err := CreateWithSettings(w, s)
	if err != nil {
		return d, err
	}
	var m map[string]any
	if err := json.Unmarshal(d.To, &m); err != nil {
		return d, err
	}
	m["rank"] = rank
	return change.NewCreate(m), nil
}
