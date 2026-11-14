package items

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/numbering"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/relation"
)

// Duplicate は、同じ親の子の末尾にアイテムをもう 1 つ作り、その内部 ID を返す(02「複製」)。
// タイトルは末尾に「 のコピー」を付け、種類・親・説明・プロパティは複製元と同じにする。ステータスは
// ワークスペースの初期ステータス(ワークフローの制約は検査しない)、ウォッチ中の人はあなただけ。
// 複製元の関連・子・コメントなどは写さず、新しいアイテムから複製元へ clones の関連を 1 つ書く。
// 作成と同じ 1 回の保存で書く。アクティビティは「{複製元の ID} を複製」。ないか削除済みなら port.ErrNotFound。
// スプリントは、使えて計画中か実行中のときだけ、リリースは、使えてアーカイブでないときだけ写す(F1-22)。
func (s Service) Duplicate(ctx context.Context, store port.StoreID, id item.ID) (item.ID, error) {
	_, display, err := s.withID(ctx, store, id)
	if err != nil {
		return "", err
	}
	me, err := s.Identity.Me(ctx, store)
	if err != nil {
		return "", err
	}
	op := port.Operation{Actor: change.User(me), Summary: fmt.Sprintf("%s を複製", display)}
	var created item.ID
	err = s.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		src, err := port.LiveItem(tx, id)
		if err != nil {
			return err
		}
		dup, err := s.copyOf(tx, src, me)
		if err != nil {
			return err
		}
		text, err := tx.Items().Description(src.ID)
		if err != nil {
			return err
		}
		texts := descTexts{}
		texts.set(dup.ID, text)
		if err := writeCreated(tx, []item.Item{dup}, texts); err != nil {
			return err
		}
		created = dup.ID
		return tx.WriteItems().Append(dup.ID, change.NewAdd("relations", item.Relation{Type: cloneType(), To: src.ID}))
	})
	return created, err
}

// cloneType は、複製元への関連の種類(保存する名前。relation の clones と同じ)。
func cloneType() string {
	typ, _, _ := relation.Parse(relation.Clones)
	return typ
}

// copyOf は、src の複製を作る。番号は次の番号、並びは同じ親の子の末尾(削除済みの順位は比べない)。
func (s Service) copyOf(tx port.Tx, src item.Item, me string) (item.Item, error) {
	live, err := port.LiveItems(tx, src.Place.Project)
	if err != nil {
		return item.Item{}, err
	}
	st, err := tx.Workspaces().Settings(src.Place.Workspace)
	if err != nil {
		return item.Item{}, err
	}
	wsLog, err := tx.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: src.Place.Workspace})
	if err != nil {
		return item.Item{}, err
	}
	wsItems, err := workspaceItems(tx, src.Place.Workspace)
	if err != nil {
		return item.Item{}, err
	}
	dup := src
	dup.ID = s.IDs.New()
	dup.Place.Number = numbering.Next(wsLog, numbering.Items)
	dup.Rank = planning.EndRank(lastRank(live, src.Place.Parent), wsItems, src.Place.Workspace, src.Kind)
	dup.Title = src.Title + " のコピー"
	dup.Watchers = []string{me}
	dup.Status = st.InitialStatus
	dup.Relations, dup.Blocked, dup.Deleted = nil, false, nil
	dup.Fields = fieldsOf(src)
	plan := planning.FromLog(wsLog)
	dup.Sprint, dup.Release = "", ""
	if sp, ok := plan.UsableSprint(src.Sprint); ok && sp.State != planning.Closed {
		dup.Sprint = sp.ID
	}
	if r, ok := plan.UsableRelease(src.Release); ok && r.State != planning.Archived {
		dup.Release = r.ID
	}
	return dup, nil
}

// fieldsOf は、作成の行に書く追加プロパティの値。複数選択を add の行で持つ値は Fields に入らないので、
// 選択肢の一覧(Choices)から配列で補う(作成の値が配列なら複数選択として読まれる)。
func fieldsOf(src item.Item) map[change.ID]json.RawMessage {
	out := make(map[change.ID]json.RawMessage, len(src.Fields)+len(src.Choices))
	for p, v := range src.Fields {
		out[p] = v
	}
	for p, vs := range src.Choices {
		if _, ok := out[p]; !ok {
			out[p] = change.Value(vs)
		}
	}
	return out
}
