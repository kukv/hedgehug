package noticing

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// unknownUser は、{変更した人} を決められないときの名前(C4b2-9)。
const unknownUser = "不明なユーザー"

// repairs は、同期の修復の保存(Repair が真のもの。C4b2-13)と合わせからの「同期の修復」の通知(06「同期の修復」)。
// 修復の種類は、保存の行で見分ける(C4b2-8)。差出人は空(「hedgehug」)。読み取りの失敗(ないもの以外)はエラー。
func repairs(r port.Reader, b batch, store port.StoreID, me string) ([]port.Notice, error) {
	var out []port.Notice
	for i, s := range b.saves {
		if !s.Repair && !s.Merge {
			continue
		}
		kinds := []func(port.Reader, batch, int, string) ([]port.Notice, error){
			restores, cascades, concurrentCascades, places, purgedUnder, purgedRelations, droppedEntities,
			activeSprints, planRefs, droppedPlans, orphanRules, droppedRules, movedRules, valueRefs, finishedInitial, duplicates,
		}
		for _, kind := range kinds {
			ns, err := kind(r, b, i, me)
			if err != nil {
				return nil, err
			}
			for _, n := range ns {
				n.Kind, n.Store, n.Sender = port.NoticeRepair, store, ""
				out = append(out, n)
			}
		}
	}
	return out, nil
}

// beforeSave は、i 番目の保存の直前の ref のログ(いまのログから、saves[i:] の行を除いたもの。C4b2-7)。
func (b batch) beforeSave(ref change.Ref, i int) []change.Record { return b.without(b.logs[ref], i) }

// beforeRead は、判定の中にないものも読む beforeSave。ないものは nil。
func beforeRead(r port.Reader, b batch, ref change.Ref, i int) ([]change.Record, error) {
	log, err := logOf(r, b, ref)
	return b.without(log, i), err
}

// without は、log から saves[i:] の行を除いたもの。
func (b batch) without(log []change.Record, i int) []change.Record {
	later := map[change.ID]bool{}
	for _, s := range b.saves[i:] {
		for _, e := range s.Entities {
			for _, row := range e.Rows {
				later[row.ID] = true
			}
		}
	}
	var out []change.Record
	for _, row := range log {
		if !later[row.ID] {
			out = append(out, row)
		}
	}
	return out
}

// lastSaver は、log で最後に保存した人(lastSaved の行の by。C4b2-7)。なければ空。
func lastSaver(log []change.Record, field string) string {
	if w := lastSaved(log, field); w != nil {
		return w.By
	}
	return ""
}

// lastSaved は、log で最後に保存した行(実行者が人の行のうち、at が最もあとの行。同じなら ID の大きい行)。
// field が空ならすべての項目、ほかは f が field の行と、作成の行(すべての項目を書く)だけを見る。なければ nil。
func lastSaved(log []change.Record, field string) *change.Record {
	var w *change.Record
	for i := range log {
		c := &log[i]
		if c.Actor.Kind != change.ActorUser || (field != "" && c.F != field && c.T != change.Create) {
			continue
		}
		if w == nil || c.At.After(w.At) || (c.At.Equal(w.At) && c.ID > w.ID) {
			w = c
		}
	}
	return w
}

// nameOf は、login の Store のメンバーの表示名。メンバーにいないか、メンバーの一覧がない(ErrNotFound)なら
// ログイン名。ほかの読み取りの失敗はエラー(C4b1-5)。
func nameOf(r port.Reader, login string) (string, error) {
	ms, err := r.Members().List()
	if err != nil {
		return login, notFoundIsSkip(err)
	}
	for _, m := range ms {
		if same(m.Login, login) {
			return oneLine(m.Name), nil
		}
	}
	return login, nil
}

// memberName は、{変更した人} の名前。空なら「不明なユーザー」。
func memberName(r port.Reader, login string) (string, error) {
	if login == "" {
		return unknownUser, nil
	}
	return nameOf(r, login)
}

// oneLine は、改行と続いた空白を 1 つの空白にする。
func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

// withParticle は、呼び方 text に助詞 p を付ける。ID のように英数字で終わる呼び方は間に空白を置き(「HH-1 は」)、
// かぎかっこや日本語で終わる呼び方は置かない(「「Review」は」「HH-1 のコメントは」)。
func withParticle(text, p string) string {
	if r, _ := utf8.DecodeLastRuneInString(text); r < utf8.RuneSelf && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
		return text + " " + p
	}
	return text + p
}

// logOf は、ref のいまのログ。判定の中のものは覚えたログ、ほかは読む。ないものは nil。
func logOf(r port.Reader, b batch, ref change.Ref) ([]change.Record, error) {
	if log, ok := b.logs[ref]; ok {
		return log, nil
	}
	log, err := r.Changes().Of(ref)
	if err != nil {
		return nil, notFoundIsSkip(err)
	}
	return log, nil
}

// workspaceOf は、ワークスペース id(削除済みを含む)。なければ false。
func workspaceOf(r port.Reader, b batch, id workspace.ID) (workspace.Workspace, bool, error) {
	log, err := logOf(r, b, change.Ref{Kind: change.KindWorkspace, ID: id})
	if err != nil {
		return workspace.Workspace{}, false, err
	}
	w, ok := workspace.FromLog(log)
	return w, ok, nil
}

// entity は、通知に書くエンティティ: 本文での呼び方と、それを指す通知の下地(C4b2-9・C4b2-10)。
type entity struct {
	text string
	base port.Notice
}

// entityOf は、アイテム・プロジェクト・ワークスペース ref の呼び方と下地(削除済みを含む)。
// ref か、その置き場のワークスペースがなければ false。
func entityOf(r port.Reader, b batch, ref change.Ref) (entity, bool, error) {
	log, err := logOf(r, b, ref)
	if err != nil {
		return entity{}, false, err
	}
	switch ref.Kind {
	case change.KindItem:
		it, ok := item.FromLog(log)
		if !ok {
			return entity{}, false, nil
		}
		w, ok, err := workspaceOf(r, b, it.Place.Workspace)
		if !ok {
			return entity{}, false, err
		}
		d := it.DisplayID(w.Prefix)
		return entity{d, port.Notice{Workspace: w.ID, Item: it.ID, Display: d}}, true, nil
	case change.KindProject:
		p, ok := project.FromLog(log)
		if !ok {
			return entity{}, false, nil
		}
		w, ok, err := workspaceOf(r, b, p.Place.Workspace)
		if !ok {
			return entity{}, false, err
		}
		d := p.DisplayID(w.Prefix)
		return entity{d, port.Notice{Workspace: w.ID, Project: p.ID, Display: d}}, true, nil
	case change.KindWorkspace:
		w, ok := workspace.FromLog(log)
		if !ok {
			return entity{}, false, nil
		}
		return entity{"ワークスペース「" + oneLine(w.Name) + "」", port.Notice{Workspace: w.ID, Section: "general"}}, true, nil
	}
	return entity{}, false, nil
}
