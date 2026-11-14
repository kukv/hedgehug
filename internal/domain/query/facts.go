package query

import (
	"time"

	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// Ref は、相手(プロジェクト・親・関連の相手)のいまの ID と旧 ID。
type Ref struct {
	Display string
	Aliases []string
}

// Facts は、アイテム 1 件の照合と並べ替えの材料。
type Facts struct {
	StoreName      string
	StoreOrder     int
	WorkspaceOrder int
	Workspace      workspace.Workspace
	Settings       workspace.Settings
	Item           item.Item // Blocked・Deleted を含む
	Display        string
	Aliases        []string
	Description    string
	Comments       []string
	Project        Ref
	ProjectTitle   string
	Parent         *Ref
	ParentTitle    string
	Ancestors      []Ref
	// Relations は、関連のキー(blocks・blocked_by・relates_to など)→ 相手。削除済みの相手を除く。
	// relates_to は向きがないので、作る側が両方向を入れる。
	Relations     map[string][]Ref
	SprintName    string
	SprintCurrent bool
	SprintNext    bool
	ReleaseName   string
	Me            string
	Names         map[string]string // ログイン名を小文字にしたもの → 表示名
	HasPR         bool
	Overwritten   bool
	Invalid       bool
	Created       time.Time
	Updated       time.Time
}

// Hit は、照合の結果。OldID は、旧 ID だけで一致した自由な語があること(E1-6)。
type Hit struct {
	Match bool
	OldID bool
}
