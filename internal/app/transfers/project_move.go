package transfers

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/numbering"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
	"github.com/kukv/hedgehug/internal/domain/rank"
	"github.com/kukv/hedgehug/internal/domain/refusal"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// ProjectMoved は、プロジェクトを移した結果。Project は内部 ID(変わらない)、Display は新しい ID、Destination は
// {移動先} の文(A5b-10)。
type ProjectMoved struct {
	ToStore     port.StoreID
	Project     project.ID
	Display     string
	Destination string
}

// MoveProject は、プロジェクト id を、すべてのアイテム(削除済みを含む)ごと別のワークスペースへ移す(02「名前の変更・移動・削除」
// の **移動**)。同じ Store の中は 1 回の保存で、プロジェクトとアイテムの番号を振り直し、値を引き直し、プロジェクト別のルールと
// 専用ルールを移動先へ引き継ぎ、転送記録を残す(A5b-11)。自動化は動かさない(03「自動化」)。移動元の Store が閲覧のみなら
// 「閲覧のみの Store です」、移動先に同じ名前のプロジェクト・ルールがあれば断る(A5b-9)。移動先の Store が表示中でないなら
// port.ErrStoreNotFound、閲覧のみ・自分のワークスペース・削除済みのワークスペースなら port.ErrNotFound。
func (s Service) MoveProject(ctx context.Context, store port.StoreID, id project.ID, in ProjectMoveInput) (ProjectMoved, error) {
	info, err := s.storeInfo(ctx, store)
	if err != nil {
		return ProjectMoved{}, err
	}
	if info.ReadOnly {
		return ProjectMoved{}, refusal.New("閲覧のみの Store です")
	}
	if err := s.checkDestination(ctx, in.ToStore); err != nil {
		return ProjectMoved{}, err
	}
	if in.ToStore != store {
		return ProjectMoved{}, errors.ErrUnsupported // 別の Store への移動は Task 6 で足す
	}
	// 断るものはここでも断る(断る移動のアクティビティの文を作らない)。保存の中でも読み直して断る。
	var summary string
	err = s.Stores.Read(ctx, store, func(r port.Reader) error {
		p, err := planProjectWithin(r, id, in, port.EditableProject, noID)
		summary = p.summary()
		return err
	})
	if err != nil {
		return ProjectMoved{}, err
	}
	me, err := s.Identity.Me(ctx, store)
	if err != nil {
		return ProjectMoved{}, err
	}
	var moved ProjectMoved
	op := port.Operation{Actor: change.User(me), Summary: summary}
	err = s.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		// 別の画面で削除されていたら、削除を取り消してから移す(06「他の書き手との重なり」、A5b-15)。
		if err := port.ReviveProject(ctx, tx, id); err != nil {
			return err
		}
		p, err := planProjectWithin(tx, id, in, port.LiveProject, s.IDs.New)
		if err != nil {
			return err
		}
		moved = ProjectMoved{ToStore: store, Project: id, Display: p.display(), Destination: p.dstName}
		return writeProjectWithin(tx, p, store)
	})
	if err != nil {
		return ProjectMoved{}, err
	}
	return moved, nil
}

// noID は、専用ルールの内部 ID を使わない引き継ぎ(断りとアクティビティの文だけを決める読み取り)に渡す newID。
func noID() change.ID { return "" }

// planProjectWithin は、同じ Store の r から、移すプロジェクト(get で読む)と移動先を読んで、移動の材料を作る。移動先が
// 移動元と同じワークスペースなら port.ErrNotFound。
func planProjectWithin(r port.Reader, id project.ID, in ProjectMoveInput, get func(port.Reader, project.ID) (project.Project, error), newID func() change.ID) (projectPlan, error) {
	src, err := readProjectSource(r, id, get)
	if err != nil {
		return projectPlan{}, err
	}
	dst, err := readProjectTarget(r, in.ToWorkspace)
	if err != nil {
		return projectPlan{}, err
	}
	if dst.ws.ID == src.ws.ID {
		return projectPlan{}, port.ErrNotFound
	}
	// 同じ Store の {移動元}・{移動先} はワークスペース名(A5b-10)。人の対応はない(nil)。
	return newProjectPlan(src, dst, src.ws.Name, dst.ws.Name, in.StatusChoices, nil, newID)
}

// projectTarget は、プロジェクトの移動先のワークスペースと、断り・番号・順位に使うもの。live は削除されていないプロジェクト、
// rules は削除されていない自動化のルール、nextProject・nextItem は次の番号。
type projectTarget struct {
	projectDestination
	live                  []project.Project
	rules                 []automation.Rule
	nextProject, nextItem int
}

// readProjectTarget は、移動先のワークスペース ws を読む。ないか削除済みなら port.ErrNotFound。
func readProjectTarget(r port.Reader, ws workspace.ID) (projectTarget, error) {
	d, err := readProjectDestination(r, ws)
	if err != nil {
		return projectTarget{}, err
	}
	live, err := port.LiveProjects(r, d.ws.ID)
	if err != nil {
		return projectTarget{}, err
	}
	log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: d.ws.ID})
	if err != nil {
		return projectTarget{}, err
	}
	return projectTarget{
		projectDestination: d, live: live, rules: automation.FromLog(log),
		nextProject: numbering.Next(log, numbering.Projects), nextItem: numbering.Next(log, numbering.Items),
	}, nil
}

// projectPlan は、プロジェクトの移動の材料(同じ Store と別の Store で共有)。srcName・dstName は {移動元}・{移動先} の文、
// number・rank は移動先でのプロジェクトの番号と順位、numbers はアイテムの番号、remaps はアイテムの引き直し、carried は
// ルールの引き継ぎ。
type projectPlan struct {
	src              projectSource
	dst              projectTarget
	srcName, dstName string
	number           int
	rank             string
	numbers          map[item.ID]int
	remaps           map[item.ID]transfer.RemapResult
	carried          transfer.Carried
}

// newProjectPlan は、読んだ移動元 src と移動先 dst から移動の材料を作る。移動先に同じ名前のプロジェクト・ルールがあれば断る
// (A5b-9)。choices のうち、移動先で選べない置き換え先(一緒に移る使わないステータスと、移動先にないもの)は捨てて、同じ分類の
// 既定にする(A5b-5)。people は別の Store の人の対応(同じ Store なら nil)、newID は専用ルールの新しい内部 ID。
func newProjectPlan(src projectSource, dst projectTarget, srcName, dstName string, choices map[change.ID]change.ID, people transfer.PeopleMap, newID func() change.ID) (projectPlan, error) {
	if err := refuseProjectMove(src, dst, dstName); err != nil {
		return projectPlan{}, err
	}
	hidden := src.carry(dst.settings, nil, people, noID).Hidden
	choice := usableChoices(choices, transfer.UsableStatuses(dst.settings, projectrule.Rules{Hidden: hidden}))
	p := projectPlan{
		src: src, dst: dst, srcName: srcName, dstName: dstName,
		number: dst.nextProject, rank: rank.After(lastProjectRank(dst.live)),
		numbers: transfer.NumberProject(src.set, dst.nextItem), remaps: map[item.ID]transfer.RemapResult{},
		carried: src.carry(dst.settings, choice, people, newID),
	}
	for _, it := range src.set.Items() {
		// ルールは一緒に移るので、移動先のワークスペースのすべてのステータスが使える(A5b-5 の toRules は空)。
		p.remaps[it.ID] = transfer.Remap(it, src.settings, dst.settings, projectrule.Rules{}, people, choice)
	}
	return p, nil
}

// refuseProjectMove は、移動先 dst に、移すプロジェクトと同じ名前の削除されていないプロジェクト、移す専用ルールと同じ名前の
// 削除されていないルールがあれば断る(A5b-9。順はプロジェクト → ルール、ルールは移動元の並びで最初の 1 つ)。
func refuseProjectMove(src projectSource, dst projectTarget, dstName string) error {
	if slices.ContainsFunc(dst.live, func(p project.Project) bool { return p.ID != src.project.ID && p.Title == src.project.Title }) {
		return refusal.New(dstName + " には同じ名前のプロジェクトがあります")
	}
	for _, r := range src.own {
		if slices.ContainsFunc(dst.rules, func(o automation.Rule) bool { return o.Name == r.Name }) {
			return refusal.New(fmt.Sprintf("%s には同じ名前のルールがあります: %s。先にルールの名前を変えてください", dstName, r.Name))
		}
	}
	return nil
}

// usableChoices は、choices のうち、置き換え先が usable にあるものだけを返す。
func usableChoices(choices map[change.ID]change.ID, usable []change.ID) map[change.ID]change.ID {
	out := map[change.ID]change.ID{}
	for from, to := range choices {
		if slices.Contains(usable, to) {
			out[from] = to
		}
	}
	return out
}

// lastProjectRank は、ps の最後の順位。なければ空。
func lastProjectRank(ps []project.Project) string {
	last := ""
	for _, p := range ps {
		last = max(last, p.Rank)
	}
	return last
}

// summary は、アクティビティの 1 行「プロジェクト「{タイトル}」を {移動元} から {移動先} へ移動」(A5b-10)。
func (p projectPlan) summary() string {
	return fmt.Sprintf("プロジェクト「%s」を %s から %s へ移動", p.src.project.Title, p.srcName, p.dstName)
}

// display は、移したあとのプロジェクトの ID。
func (p projectPlan) display() string {
	return project.Project{Place: project.Place{Number: p.number}}.DisplayID(p.dst.ws.Prefix)
}

// projectLines は、プロジェクトを移動先の番号と順位へ置く行。順位が変わらなければ rank は書かない。
func (p projectPlan) projectLines() []change.Draft {
	pr := p.src.project
	drafts := []change.Draft{change.NewSet("place", pr.Place, project.Place{Workspace: p.dst.ws.ID, Number: p.number})}
	if pr.Rank != p.rank {
		drafts = append(drafts, change.NewSet("rank", pr.Rank, p.rank))
	}
	return drafts
}

// itemLines は、アイテム it を移動先のワークスペースと番号へ置く行と引き直しの行。プロジェクト・親・順位は変えない。
func (p projectPlan) itemLines(it item.Item) []change.Draft {
	place := it.Place
	place.Workspace, place.Number = p.dst.ws.ID, p.numbers[it.ID]
	return append([]change.Draft{change.NewSet("place", it.Place, place)}, p.remaps[it.ID].Drafts...)
}

// records は、プロジェクトとアイテムごとの転送記録。toStore は移動先の Store。
func (p projectPlan) records(toStore port.StoreID) []transfer.Record {
	pr := p.src.project
	out := []transfer.Record{{
		Entity: transfer.EntityProject, Target: pr.ID, Old: pr.DisplayID(p.src.ws.Prefix), New: p.display(), ToStore: string(toStore),
	}}
	for _, it := range p.src.set.Items() {
		out = append(out, transfer.Record{
			Entity: transfer.EntityItem, Target: it.ID, Old: it.DisplayID(p.src.ws.Prefix),
			New: displayID(p.dst.ws.Prefix, p.numbers[it.ID]), ToStore: string(toStore),
		})
	}
	return out
}

// writeProjectWithin は、同じ Store の中のプロジェクトの移動の行を書く(A5b-11): プロジェクトとアイテムの行、移動先の
// ワークスペースに番号の行とルールの行、移動元のワークスペースにルールを消す行(A5b-7)、転送記録。
func writeProjectWithin(tx port.Tx, p projectPlan, store port.StoreID) error {
	errs := []error{tx.WriteProjects().Append(p.src.project.ID, p.projectLines()...)}
	for _, it := range p.src.set.Items() {
		errs = append(errs, tx.WriteItems().Append(it.ID, p.itemLines(it)...))
	}
	dst := append([]change.Draft{numbering.Draft(numbering.Projects, p.number)}, numberLines(p.numbers)...)
	errs = append(errs, tx.WriteWorkspaces().Append(p.dst.ws.ID, append(dst, p.carried.Destination...)...))
	if len(p.carried.Source) > 0 {
		errs = append(errs, tx.WriteWorkspaces().Append(p.src.ws.ID, p.carried.Source...))
	}
	for _, r := range p.records(store) {
		errs = append(errs, tx.WriteTransfers().Append(r))
	}
	return errors.Join(errs...)
}
