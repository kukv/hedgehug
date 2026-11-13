package relations

import (
	"errors"
	"fmt"
	"slices"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/relation"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// snapshot は、Store のすべてのアイテムと、表示に要るワークスペースの情報(A4a-11)。
// ワークスペース → プロジェクト → アイテムの順に、既存の口で読む。削除済みのアイテム・プロジェクトは入れない
// (一覧・候補・循環の検査・追加と解除の相手から外れる。関連の行そのものは消さない。A4b-5)。
type snapshot struct {
	order    []workspace.ID // ワークスペースの並び
	names    map[workspace.ID]string
	prefixes map[workspace.ID]string
	settings map[workspace.ID]workspace.Settings
	all      []item.Item
	byID     map[item.ID]item.Item
}

func readSnapshot(r port.Reader) (snapshot, error) {
	wss, err := r.Workspaces().List()
	if err != nil {
		return snapshot{}, err
	}
	s := snapshot{
		names: map[workspace.ID]string{}, prefixes: map[workspace.ID]string{},
		settings: map[workspace.ID]workspace.Settings{}, byID: map[item.ID]item.Item{},
	}
	for _, w := range wss {
		st, err := r.Workspaces().Settings(w.ID)
		if err != nil && !errors.Is(err, port.ErrNotFound) {
			return snapshot{}, err
		}
		s.order = append(s.order, w.ID)
		s.names[w.ID], s.prefixes[w.ID], s.settings[w.ID] = w.Name, w.Prefix, st
		ps, err := port.LiveProjects(r, w.ID)
		if err != nil {
			return snapshot{}, err
		}
		for _, p := range ps {
			its, err := port.LiveItems(r, p.ID)
			if err != nil {
				return snapshot{}, err
			}
			for _, it := range its {
				s.byID[it.ID] = it
			}
			s.all = append(s.all, its...)
		}
	}
	return s, nil
}

// display は、画面の ID(HH-12)。
func (s snapshot) display(it item.Item) string { return it.DisplayID(s.prefixes[it.Place.Workspace]) }

// workspaceNameFor は、other が self と別のワークスペースにあるときだけ、そのワークスペース名を返す。
func (s snapshot) workspaceNameFor(self, other item.Item) string {
	if other.Place.Workspace == self.Place.Workspace {
		return ""
	}
	return s.names[other.Place.Workspace]
}

// status は、アイテムのステータスの値(名前と色)。値がなければ ok=false。
func (s snapshot) status(it item.Item) (workspace.Value, bool) {
	for _, v := range s.settings[it.Place.Workspace].Statuses {
		if v.ID == it.Status {
			return v, true
		}
	}
	return workspace.Value{}, false
}

// reason は、self が other と label の関連を足せない理由。足せるなら空(02「関連」の「追加」)。
// 同じ種類の関連は、向きが逆でも 1 つまで。blocks は、足すと循環になるものも断る。
func (s snapshot) reason(self, other item.ID, label relation.Label, typ string, reverse bool) string {
	if relation.Has(self, other, typ, s.all) {
		return fmt.Sprintf("すでに %s の関連があります", label)
	}
	if typ == relation.TypeBlocks {
		from, to := self, other
		if reverse {
			from, to = other, self
		}
		if relation.WouldCycle(from, to, s.all) {
			return "循環になります"
		}
	}
	return ""
}

// holder は、self から見た label の関連を、実際に持っているアイテムと、その値。持っていなければ ok=false。
// blocked by などの逆向きは相手が持ち、relates to だけは、どちらが持っていてもよい。
func (s snapshot) holder(self, other item.ID, typ string, reverse bool) (item.ID, item.Relation, bool) {
	type pair struct{ owner, to item.ID }
	cands := []pair{{self, other}}
	switch {
	case reverse:
		cands = []pair{{other, self}}
	case typ == "relates":
		cands = append(cands, pair{other, self})
	}
	for _, c := range cands {
		rel := item.Relation{Type: typ, To: c.to}
		if it, ok := s.byID[c.owner]; ok && slices.Contains(it.Relations, rel) {
			return c.owner, rel, true
		}
	}
	return "", item.Relation{}, false
}
