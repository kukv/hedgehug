package apptest

import (
	"context"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

// StoreGitHub は port.StoreGitHub の偽物。
type StoreGitHub struct {
	mu     sync.Mutex
	Stores []port.StoreGitHubInfo
	Login  string // マシンの設定「ログイン名」
}

func (s *StoreGitHub) List(context.Context) ([]port.StoreGitHubInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.Stores), nil
}

func (s *StoreGitHub) Unassign(_ context.Context, id account.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.Stores {
		if s.Stores[i].Account == id {
			s.Stores[i].Account = ""
		}
	}
	return nil
}

func (s *StoreGitHub) MachineLogin(context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Login, nil
}

// 以下は GitHub(github.go)の PR の偽物。

// PutPull は、repo の PR p を置く(同じ番号があれば置き換える)。ReadPulls が返す。
func (g *GitHub) PutPull(repo string, p port.Pull) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.pulls == nil {
		g.pulls = map[string][]port.Pull{}
	}
	key := strings.ToLower(repo)
	g.pulls[key] = slices.DeleteFunc(g.pulls[key], func(o port.Pull) bool { return o.Number == p.Number })
	g.pulls[key] = append(g.pulls[key], p)
}

// ReadPulls は、PutPull した PR を更新の新しい順に返す。since がゼロなら 100 件まで、そうでなければ UpdatedAt が
// since 以上のもの。Fail(repo, 0, err) があればその失敗。
func (g *GitHub) ReadPulls(_ context.Context, token, repo string, since time.Time) (port.Pulls, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.pullCalls == nil {
		g.pullCalls = map[string]int{}
	}
	g.pullCalls[strings.ToLower(repo)]++
	if g.Err != nil {
		return port.Pulls{}, g.Err
	}
	if _, ok := g.Users[token]; !ok {
		return port.Pulls{}, &ghcall.Failure{Kind: ghcall.TokenRejected}
	}
	if err := g.appMissing(repo); err != nil {
		return port.Pulls{}, err
	}
	if err := g.LookupErr[itemKey(repo, 0)]; err != nil {
		return port.Pulls{}, err
	}
	all := slices.Clone(g.pulls[strings.ToLower(repo)])
	slices.SortStableFunc(all, func(a, b port.Pull) int { return b.UpdatedAt.Compare(a.UpdatedAt) })
	out := port.Pulls{Repo: repo, Rate: g.Rate}
	for _, p := range all {
		if (since.IsZero() && len(out.Pulls) == 100) || (!since.IsZero() && p.UpdatedAt.Before(since)) {
			break
		}
		out.Pulls = append(out.Pulls, p)
	}
	return out, nil
}

// ReadPullsCalls は、repo(大文字小文字を問わない)で ReadPulls を呼んだ回数。
func (g *GitHub) ReadPullsCalls(repo string) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.pullCalls[strings.ToLower(repo)]
}
