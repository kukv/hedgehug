package transfers

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// StatusRow は、ダイアログの「ステータスの置き換え」の 1 行。Count はそのステータスの移すアイテム(下のもの・
// 削除済みを含む)の数、Choices は移動先のプロジェクトが使うステータス、Default は同じ分類の移動先の既定。
type StatusRow struct {
	Status  change.ID
	Name    string
	Count   int
	Choices []workspace.Value
	Default change.ID
}

// Preview は、移動のダイアログの予告(A5a-14)。Below・Deleted は下の削除されていないもの・削除済みのものの数、
// NextDisplay は移動先の次の番号の ID。RemoteNotice は、別の Store で、移動先に remote がなく移動元にあるとき。
// DestinationHasRemote は、移動先の Store に remote があるか(同じ Store でも、その Store にあれば真)。
type Preview struct {
	Below, Deleted                                 int
	NextDisplay                                    string
	CrossStore, RemoteNotice, DestinationHasRemote bool
	Replacements                                   []StatusRow
}

// Preview は、アイテム id を in の移動先へ移すときの予告を返す(02「移す先の選び方」)。in の ToParent・
// StatusChoices は使わない。移すアイテム・移動先が選べないものなら、Move と同じエラー(port.ErrNotFound・
// port.ErrStoreNotFound)。移動元と移動先は 1 つずつ順に読む(読み取りを入れ子にしない)。
func (s Service) Preview(ctx context.Context, store port.StoreID, id item.ID, in MoveInput) (Preview, error) {
	if err := s.checkDestination(ctx, in.ToStore); err != nil {
		return Preview{}, err
	}
	var src source
	if err := s.Stores.Read(ctx, store, func(r port.Reader) (err error) { src, err = readSource(r, id); return }); err != nil {
		return Preview{}, err
	}
	var dst destination
	if err := s.Stores.Read(ctx, in.ToStore, func(r port.Reader) (err error) { dst, err = readDestination(r, in); return }); err != nil {
		return Preview{}, err
	}
	cross := in.ToStore != store
	if !cross && dst.ws.ID == src.ws.ID {
		return Preview{}, port.ErrNotFound
	}
	srcRemote, err := s.hasRemote(ctx, store)
	if err != nil {
		return Preview{}, err
	}
	dstRemote, err := s.hasRemote(ctx, in.ToStore)
	if err != nil {
		return Preview{}, err
	}
	p := Preview{
		NextDisplay: displayID(dst.ws.Prefix, dst.next), CrossStore: cross,
		RemoteNotice: cross && srcRemote && !dstRemote, DestinationHasRemote: dstRemote,
		Replacements: replacements(src, dst),
	}
	for _, it := range src.set.Below {
		if it.Deleted != nil {
			p.Deleted++
		} else {
			p.Below++
		}
	}
	return p, nil
}

// replacements は、移すアイテムのステータスのうち、移動先で使えないもの(transfer.Unmatched。Remap と同じ規則)の
// 行を、移動元のステータスの定義の順で返す。
func replacements(src source, dst destination) []StatusRow {
	counts := map[change.ID]int{}
	for _, it := range src.items() {
		counts[it.Status]++
	}
	var present []change.ID
	for _, v := range src.settings.Statuses {
		if counts[v.ID] > 0 {
			present = append(present, v.ID)
		}
	}
	return statusRows(present, counts, src.settings, dst.settings, dst.rules, dst.rules)
}

// statusRows は、present(移動元のステータス。行の並び)のうち、移動先で matchRules のもとで使えないもの
// (transfer.Unmatched)の行を返す。選択肢は移動先で choiceRules のもとで使うステータス、件数は counts。
func statusRows(present []change.ID, counts map[change.ID]int, from, to workspace.Settings, matchRules, choiceRules projectrule.Rules) []StatusRow {
	var choices []workspace.Value
	for _, id := range transfer.UsableStatuses(to, choiceRules) {
		if v, ok := valueOf(to.Statuses, id); ok {
			choices = append(choices, v)
		}
	}
	var rows []StatusRow
	for _, id := range transfer.Unmatched(present, from, to, matchRules) {
		v, _ := valueOf(from.Statuses, id)
		def := to.CategoryDefaults[v.Category]
		if def == "" {
			def = to.InitialStatus // Remap と同じく、分類の既定がなければ初期ステータス
		}
		rows = append(rows, StatusRow{Status: id, Name: v.Name, Count: counts[id], Choices: choices, Default: def})
	}
	return rows
}

// valueOf は、vs のうち内部 ID が id の値。
func valueOf(vs []workspace.Value, id change.ID) (workspace.Value, bool) {
	for _, v := range vs {
		if v.ID == id {
			return v, true
		}
	}
	return workspace.Value{}, false
}
