package githublink

import (
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/prlink"
)

// storeItems は、Store の削除済みでないアイテムを、ID(いまの接頭辞と番号)と内部の ID から引く表(D4b-2・6)。
type storeItems struct {
	byID map[string]item.ID // 「WEB-1」→ 内部の ID
	live map[item.ID]string // 内部の ID → 表示の ID
}

// readStoreItems は、ワークスペースのいまの接頭辞と、削除済みでないプロジェクトの削除済みでないアイテムから表を作る。
func readStoreItems(r port.Reader) (storeItems, error) {
	s := storeItems{byID: map[string]item.ID{}, live: map[item.ID]string{}}
	wss, err := r.Workspaces().List()
	if err != nil {
		return s, err
	}
	for _, ws := range wss {
		projects, err := port.LiveProjects(r, ws.ID)
		if err != nil {
			return s, err
		}
		for _, p := range projects {
			items, err := port.LiveItems(r, p.ID)
			if err != nil {
				return s, err
			}
			for _, it := range items {
				id := strings.ToUpper(it.DisplayID(ws.Prefix))
				s.byID[id] = it.ID
				s.live[it.ID] = id
			}
		}
	}
	return s, nil
}

// targets は、PR p が指すアイテム(重なりなし。見つけた順)。
func (s storeItems) targets(p port.Pull) []item.ID {
	m := prlink.Read(p.Text)
	var out []item.ID
	add := func(id item.ID, ok bool) {
		if !ok {
			return
		}
		for _, o := range out {
			if o == id {
				return
			}
		}
		out = append(out, id)
	}
	for _, d := range m.IDs {
		id, ok := s.byID[d]
		add(id, ok)
	}
	for _, mark := range m.Marks {
		id := item.ID(strings.ToUpper(mark))
		_, ok := s.live[id]
		add(id, ok)
	}
	return out
}
