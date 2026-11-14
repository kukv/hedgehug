package search

import (
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/comment"
	"github.com/kukv/hedgehug/internal/domain/history"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/query"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// facts は、アイテム it の照合の材料。records は、すべての表示中の Store の転送記録(旧 ID)。now は検索の時刻。
func (s *snapshot) facts(it item.Item, records []transfer.Record, q query.Query, now time.Time) query.Facts {
	sp := s.spaceOf(it.Place.Workspace)
	p := s.projects[it.Place.Project]
	log := s.logs[it.ID]
	it.Blocked = s.blocked[it.ID]
	f := query.Facts{
		StoreName: s.info.Name, StoreOrder: s.order, WorkspaceOrder: sp.order, Workspace: sp.ws, Settings: sp.settings,
		Item: it, Display: s.display(it), Aliases: s.itemRef(it, records).Aliases,
		Project: s.projectRef(p, records), ProjectTitle: p.Title,
		Relations: s.relations(it, records),
		Me:        s.me, Names: s.names, HasPR: hasPR(log),
		Overwritten: s.overwritten[it.ID], Invalid: s.invalid[it.ID],
		Created: it.Created.At,
		Updated: history.Updated(log, history.Context{
			Settings: sp.settings, Names: s.whoNames, Hidden: sp.hidden, Today: now, Origin: s.origin(it.ID),
		}),
	}
	f.Parent, f.Ancestors = s.ancestors(it, records)
	planFacts(&f, sp.plan, it)
	if q.NeedsText {
		f.Description = s.descs[it.ID]
		for _, c := range comment.FromLog(log) {
			f.Comments = append(f.Comments, c.Body)
		}
	}
	return f
}

// origin は、同じ Store の中の移動の行の材料(app/history の readOrigin と同じ): すべてのワークスペースの設定と名前、
// すべてのプロジェクト(削除済みを含む)の名前、この Store の転送記録のうちアイテム id のもの。
func (s *snapshot) origin(id item.ID) history.Origin {
	var mine []transfer.Record
	for _, rec := range s.records {
		if rec.Entity == transfer.EntityItem && rec.Target == id {
			mine = append(mine, rec)
		}
	}
	return history.Origin{
		Settings: func(w workspace.ID) (workspace.Settings, bool) {
			sp, ok := s.spaces[w]
			if !ok || !sp.hasSettings {
				return workspace.Settings{}, false
			}
			return sp.settings, true
		},
		WorkspaceName: func(w workspace.ID) (string, bool) {
			sp, ok := s.spaces[w]
			if !ok {
				return "", false
			}
			return sp.ws.Name, true
		},
		ProjectTitle: func(p project.ID) (string, bool) { pr, ok := s.projects[p]; return pr.Title, ok },
		StoreName:    s.info.Name,
		Transfers:    mine,
	}
}

// planFacts は、スプリント・リリースの名前と、実行中・次のスプリントか。削除済みのスプリント・リリースは名前を出さない。
func planFacts(f *query.Facts, plan planning.Plan, it item.Item) {
	if it.Sprint != "" {
		if sp, ok := plan.UsableSprint(it.Sprint); ok {
			f.SprintName = sp.Name
		}
		live := plan.LiveSprints()
		if a := planning.ActiveSprint(live); a != nil {
			f.SprintCurrent = a.ID == it.Sprint
		}
		if n := planning.NextSprint(live); n != nil {
			f.SprintNext = n.ID == it.Sprint
		}
	}
	if it.Release != "" {
		if r, ok := plan.UsableRelease(it.Release); ok {
			f.ReleaseName = r.Name
		}
	}
}

// hasPR は、アイテムのログに PR のリンクがあるか(Issue のリンクは数えない)。
func hasPR(log []change.Record) bool {
	for _, l := range itemlink.FromLog(log).Links {
		if l.Kind == itemlink.PR {
			return true
		}
	}
	return false
}
