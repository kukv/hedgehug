package projects

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// SetOwner は、オーナーを変える(02「概要の編集」)。login が空なら「なし」。メンバーでなければ port.ErrNotFound。
// ログイン名は大文字と小文字を区別せずに比べ、保存するのはメンバーのログイン名。前と同じなら何も保存しない。
func (s Service) SetOwner(ctx context.Context, store port.StoreID, id project.ID, login string) (project.Project, error) {
	p, err := s.Get(ctx, store, id)
	if err != nil {
		return project.Project{}, err
	}
	owner, name := "", "なし"
	if login != "" {
		m, err := s.memberOf(ctx, store, login)
		if err != nil {
			return project.Project{}, err
		}
		owner, name = m.Login, m.Name
	}
	summary := fmt.Sprintf("%s のオーナーを %s に変更", label(p.Title), label(name))
	return s.save(ctx, store, id, summary, func(_ port.Tx, cur project.Project) ([]change.Draft, error) {
		if cur.Owner == owner {
			return nil, nil
		}
		return []change.Draft{change.NewSet("owner", cur.Owner, owner)}, nil
	})
}

func (s Service) memberOf(ctx context.Context, store port.StoreID, login string) (member.Member, error) {
	var found member.Member
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		ms, err := r.Members().List()
		if err != nil {
			return err
		}
		i := slices.IndexFunc(ms, func(m member.Member) bool { return strings.EqualFold(m.Login, login) })
		if i < 0 {
			return port.ErrNotFound
		}
		found = ms[i]
		return nil
	})
	return found, err
}
