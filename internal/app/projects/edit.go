package projects

import (
	"context"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// save は、保存の中でいまのプロジェクトを読み直し、drafts が返す行を 1 回で保存する(02「概要の編集」)。
// 削除済み(root: true)なら、先に削除を取り消す(06「他の書き手との重なり」)。行がなければ何も残さず、削除済みなら断る。drafts が断ったら何も保存しない。summary はアクティビティの 1 行で、保存の前に決める。
func (s Service) save(ctx context.Context, store port.StoreID, id project.ID, summary string, drafts func(tx port.Tx, p project.Project) ([]change.Draft, error)) (project.Project, error) {
	me, err := s.Identity.Me(ctx, store)
	if err != nil {
		return project.Project{}, err
	}
	op := port.Operation{Actor: change.User(me), Summary: summary}
	err = s.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		p, err := port.EditableProject(tx, id)
		if err != nil {
			return err
		}
		if err := port.ReviveProject(ctx, tx, id); err != nil {
			return err
		}
		ds, err := drafts(tx, p)
		if err != nil {
			return err
		}
		if len(ds) == 0 {
			return port.Unchanged(p.Deleted) // 何も変わらない保存は、削除済みなら断る(決定 S4)。tx は取り消される
		}
		return tx.WriteProjects().Append(id, ds...)
	})
	if err != nil {
		return project.Project{}, err
	}
	return s.Get(ctx, store, id)
}

// label は、アクティビティの 1 行に書くタイトル。タイトルは改行を含められる(作成と同じ)ので、
// 改行を含む連続する空白を 1 つの空白にまとめる(1 行でないと保存できない)。
func label(title string) string { return strings.Join(strings.Fields(title), " ") }
