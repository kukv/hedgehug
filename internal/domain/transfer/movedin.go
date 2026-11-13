package transfer

import (
	"encoding/json"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// MovedInInfo は、moved_in の行(別の Store から移ってきた印)の中身。Names は、写した行が指す、移動元の Store の
// ワークスペース(移動元のワークスペースと、その前に同じ Store の中でいたワークスペース)の値・プロパティの内部 ID と名前。
// Moves は、移動元の Store の中で、この移動より前に別のワークスペースへ移したときの記録(移動元の転送記録から)。
type MovedInInfo struct {
	Old, FromStore, FromStoreName string
	FromWorkspace                 change.ID
	FromWorkspaceName             string
	FromProject                   change.ID
	FromProjectTitle              string
	At                            time.Time
	Names                         map[change.ID]string
	Moves                         []Move
}

// Move は、同じ Store の中で別のワークスペースへ移した 1 回(履歴の「{旧 ID} から移動」に使う)。Op はその操作(写した行の
// 場所の set の op)、Old は旧 ID、FromWorkspaceName・FromProjectTitle は移動元の名前(引けなければ空)。
type Move struct {
	Op                change.ID `json:"op"`
	Old               string    `json:"old"`
	FromWorkspaceName string    `json:"from_workspace_name"`
	FromProjectTitle  string    `json:"from_project_title"`
}

// movedInTo は、moved_in の to。キーの並びは保存形式のとおり。
type movedInTo struct {
	Old               string               `json:"old"`
	FromStore         string               `json:"from_store"`
	FromStoreName     string               `json:"from_store_name"`
	FromWorkspace     change.ID            `json:"from_workspace"`
	FromWorkspaceName string               `json:"from_workspace_name"`
	FromProject       change.ID            `json:"from_project"`
	FromProjectTitle  string               `json:"from_project_title"`
	At                string               `json:"at"`
	Names             map[change.ID]string `json:"names"`
	Moves             []Move               `json:"moves,omitempty"`
}

// NewMovedIn は、moved_in の行にする変更。f は持たない。
func NewMovedIn(m MovedInInfo) change.Draft {
	names := m.Names
	if names == nil {
		names = map[change.ID]string{}
	}
	return change.Draft{T: change.MovedIn, To: change.Value(movedInTo{
		Old: m.Old, FromStore: m.FromStore, FromStoreName: m.FromStoreName,
		FromWorkspace: m.FromWorkspace, FromWorkspaceName: m.FromWorkspaceName,
		FromProject: m.FromProject, FromProjectTitle: m.FromProjectTitle,
		At: m.At.Format(change.TimeLayout), Names: names, Moves: m.Moves,
	})}
}

// MovedInOf は、ログの moved_in の行を書いた順に返す。読めない行は飛ばす。
func MovedInOf(log []change.Record) []MovedInInfo {
	var out []MovedInInfo
	for _, r := range log {
		if r.T != change.MovedIn {
			continue
		}
		var to movedInTo
		if json.Unmarshal(r.To, &to) != nil {
			continue
		}
		at, err := time.Parse(time.RFC3339, to.At)
		if err != nil {
			continue
		}
		out = append(out, MovedInInfo{
			Old: to.Old, FromStore: to.FromStore, FromStoreName: to.FromStoreName,
			FromWorkspace: to.FromWorkspace, FromWorkspaceName: to.FromWorkspaceName,
			FromProject: to.FromProject, FromProjectTitle: to.FromProjectTitle, At: at, Names: to.Names, Moves: to.Moves,
		})
	}
	return out
}
