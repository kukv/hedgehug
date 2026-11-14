package planning

import (
	"context"
	"errors"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/refusal"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// 別の画面で削除されたスプリント・リリース・ワークスペースへの保存は、変更を残して削除を取り消す(決定 R12・R11)。

const (
	sprintNameTaken  = "同じ名前のスプリントがあります。先にそのスプリントの名前を変えてください"
	releaseNameTaken = "同じ名前のリリースがあります。先にそのリリースの名前を変えてください"
)

// errNoRows は、保存で書く行がないことを伝える。取り消すものがあれば ErrNotFound(決定 R3)、なければ何もしない。
var errNoRows = errors.New("書く行がない")

// planKind は、削除を取り消すものの種類。
type planKind int

const (
	noPlan      planKind = iota // スプリント・リリースは取り消さない
	sprintKind                  // スプリント
	releaseKind                 // リリース
)

// revival は、保存で取り消すもの。ゼロ値は何も取り消さず、削除済みのワークスペースを ErrNotFound で断る。
type revival struct {
	workspace bool      // 削除済みのワークスペースの削除を取り消す
	kind      planKind  // 削除を取り消すものの種類
	id        change.ID // 削除を取り消すスプリント・リリースの ID
	name      *string   // 保存のあとの名前(正規化済み)。nil なら、いまの名前
}

// field は、削除を取り消すものの項目名(sprints/<ID>・releases/<ID>)。取り消さないなら空。
func (rv revival) field() string {
	switch rv.kind {
	case sprintKind:
		return "sprints/" + string(rv.id)
	case releaseKind:
		return "releases/" + string(rv.id)
	}
	return ""
}

// plan は、保存の前と中で読む計画。rv.field() を覆う削除の行は、ないものとして読む。
func (rv revival) plan(r port.Reader, ws workspace.ID) (planning.Plan, error) {
	if rv.workspace {
		if _, err := port.EditableWorkspace(r, ws); err != nil {
			return planning.Plan{}, err
		}
	} else if _, err := port.LiveWorkspace(r, ws); err != nil {
		return planning.Plan{}, err
	}
	log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: ws})
	if err != nil {
		return planning.Plan{}, err
	}
	if rv.field() != "" {
		log = workspace.WithoutCovering(log, rv.field())
	}
	return planning.FromLog(log), nil
}

// pending は、保存すれば取り消すもの(削除済みのワークスペースかスプリント・リリース)があるか。
func (rv revival) pending(r port.Reader, ws workspace.ID) (bool, error) {
	if rv.workspace {
		w, err := r.Workspaces().Get(ws)
		if err != nil || w.Deleted != nil {
			return err == nil, err
		}
	}
	if rv.field() == "" {
		return false, nil
	}
	log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: ws})
	return len(workspace.Covering(log, rv.field())) > 0, err
}

// revive は、書く Tx の中で、ワークスペースとスプリント・リリースの削除を取り消す。取り消したら true。スプリント・リリースを取り消して
// 名前が削除されていないものと重なるなら、08「復元」の文言で断る(行を書く前に、保存のあとの名前で判定する)。
func (rv revival) revive(ctx context.Context, tx port.Tx, ws workspace.ID) (bool, error) {
	var wsRevived bool
	if rv.workspace {
		var err error
		if wsRevived, err = port.ReviveWorkspace(ctx, tx, ws, "", ""); err != nil {
			return false, err
		}
	}
	if rv.field() == "" {
		return wsRevived, nil
	}
	ok, err := port.ReviveSetting(ctx, tx, ws, rv.field())
	if err != nil || !ok {
		return wsRevived, err
	}
	p, err := logPlan(tx, ws)
	if err != nil {
		return false, err
	}
	return true, checkRevivedName(p, rv)
}

// write は、writePlanFrom の書く Tx の中身。最初に削除を取り消し、読み直した計画で write を呼ぶ。
// write が errNoRows を返したら、取り消したなら ErrNotFound(決定 R3)、取り消していなければ何も書かない。
func (rv revival) write(ctx context.Context, tx port.Tx, ws workspace.ID,
	plan func(r port.Reader, ws workspace.ID) (planning.Plan, error),
	write func(tx port.Tx, p planning.Plan) error,
) error {
	revived, err := rv.revive(ctx, tx, ws)
	if err != nil {
		return err
	}
	p, err := plan(tx, ws)
	if err != nil {
		return err
	}
	if err := write(tx, p); !errors.Is(err, errNoRows) {
		return err
	}
	if revived {
		return port.ErrNotFound
	}
	return nil
}

// checkRevivedName は、取り消したスプリント・リリースの保存のあとの名前が、空なら入力を求め、削除されていない
// ほかのものと重なれば 08 の文言で断る。p は取り消したあとの計画。
func checkRevivedName(p planning.Plan, rv revival) error {
	if rv.name != nil && *rv.name == "" {
		return refusal.New("名前を入力してください")
	}
	switch rv.kind {
	case sprintKind:
		sp, ok := p.UsableSprint(rv.id)
		if ok && nameTaken(rv.id, rv.nameOr(sp.Name), sprintNames(p.LiveSprints())) {
			return refusal.New(sprintNameTaken)
		}
	case releaseKind:
		r, ok := p.UsableRelease(rv.id)
		if ok && nameTaken(rv.id, rv.nameOr(r.Name), releaseNames(p.LiveReleases())) {
			return refusal.New(releaseNameTaken)
		}
	}
	return nil
}

// nameOr は、保存のあとの名前。名前を変えない保存なら current。
func (rv revival) nameOr(current string) string {
	if rv.name != nil {
		return *rv.name
	}
	return current
}

func sprintNames(live []planning.Sprint) map[change.ID]string {
	names := make(map[change.ID]string, len(live))
	for _, s := range live {
		names[s.ID] = s.Name
	}
	return names
}

func releaseNames(live []planning.Release) map[change.ID]string {
	names := make(map[change.ID]string, len(live))
	for _, r := range live {
		names[r.ID] = r.Name
	}
	return names
}

// nameTaken は、names(ID ごとの名前)のうち id でないものに、名前 name と同じものがあるか。
func nameTaken(id change.ID, name string, names map[change.ID]string) bool {
	for oid, oname := range names {
		if oid != id && planning.NormalizeName(oname) == planning.NormalizeName(name) {
			return true
		}
	}
	return false
}
