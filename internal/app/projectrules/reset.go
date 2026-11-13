package projectrules

import (
	"context"
	"fmt"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
)

// Reset は、区画 sec をワークスペースの設定に戻す。1 回の保存で、自動化では専用ルールの削除も含む(B5a-14)。
// 区画に行がなければ ErrNotFound。
func (s Service) Reset(ctx context.Context, store port.StoreID, p project.ID, sec projectrule.Section) error {
	return s.edit(ctx, store, p, func(st state) (plan, error) {
		ds := projectrule.ResetDrafts(st.mine(), p, sec)
		if sec == projectrule.SectionAutomation {
			for _, r := range st.own() {
				ds = append(ds, automation.DeleteDraft(r.ID))
			}
		}
		if len(ds) == 0 {
			return plan{}, port.ErrNotFound
		}
		return plan{fmt.Sprintf("プロジェクト「%s」の%sをワークスペースの設定に戻す", st.project.Title, sec.Label()), ds}, nil
	})
}
