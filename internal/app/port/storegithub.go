package port

import (
	"context"

	"github.com/kukv/hedgehug/internal/domain/account"
)

// StoreGitHubInfo は、Store の GitHub に関わる値(08「GitHub に使うアカウント」)。
type StoreGitHubInfo struct {
	ID       StoreID
	Name     string
	Account  account.ID // 割り当てたアカウント。「なし」なら空
	Remote   bool       // remote を持つか
	ReadOnly bool       // 閲覧のみの Store か(01。リンクの取り直しをしない。D3-14)
}

// StoreGitHub は、Store の一覧のうち GitHub に関わる値と、マシンの設定「ログイン名」の口(トラック D の決定 1)。
type StoreGitHub interface {
	// List は、このマシンのすべての Store(非表示・閲覧のみを含む)を返す。
	List(ctx context.Context) ([]StoreGitHubInfo, error)
	// Unassign は、id を割り当てた Store の割り当てを「なし」にする(07「削除の影響」)。
	Unassign(ctx context.Context, id account.ID) error
	// MachineLogin は、このマシンの設定「ログイン名」(01「「あなた」の決まり方」)。
	MachineLogin(ctx context.Context) (string, error)
}
