package githublink

import (
	"context"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// repoLinks は、1 つのリポジトリの、間隔ごとに取り直すリンク(D4a-3)と、PR を探すか(D4b-3)。
type repoLinks struct {
	key     string // リポジトリ名の小文字(読み取りの記録のキー。D4a-5)
	repo    string // 問い合わせる名前(最初に見つけた書き方)
	numbers []int  // 頼む番号(重なりなし)
	asked   map[int]bool
	targets []target
	scan    bool // PR を探す(プロジェクトのリポジトリか、リンクのあるリポジトリ)
}

// target は、取り直す 1 本のリンクと、それを持つアイテム。
type target struct {
	item item.ID
	link itemlink.Link
}

// collector は、Store のリンクを、リポジトリごとに見つけた順に集める。
type collector struct {
	repos []*repoLinks
	index map[string]*repoLinks
}

// repo は、repo(大文字小文字を区別しない)の要素。なければ、この書き方で足す。どれも PR を探す(D4b-3)。
func (c *collector) repo(repo string) *repoLinks {
	key := strings.ToLower(repo)
	r := c.index[key]
	if r == nil {
		r = &repoLinks{key: key, repo: repo, asked: map[int]bool{}, scan: true}
		c.index[key] = r
		c.repos = append(c.repos, r)
	}
	return r
}

// add は、アイテム id のリンクのリポジトリを探すものに足し、状態が Merged でないリンクを取り直すものに足す
// (07「表示と更新」)。
func (c *collector) add(id item.ID, links []itemlink.Link) {
	for _, l := range links {
		r := c.repo(l.Repo)
		if l.State == itemlink.Merged {
			continue
		}
		r.targets = append(r.targets, target{item: id, link: l})
		if !r.asked[l.Number] {
			r.asked[l.Number] = true
			r.numbers = append(r.numbers, l.Number)
		}
	}
}

// collect は、store のすべてのワークスペースの削除済みでないプロジェクトのリポジトリと、アイテムのリンクを、
// リポジトリごとに集める。
func (l Links) collect(ctx context.Context, store port.StoreID) ([]*repoLinks, error) {
	var out []*repoLinks
	err := l.Stores.Read(ctx, store, func(r port.Reader) error {
		c := &collector{index: map[string]*repoLinks{}}
		wss, err := r.Workspaces().List()
		if err != nil {
			return err
		}
		for _, w := range wss {
			ps, err := port.LiveProjects(r, w.ID)
			if err != nil {
				return err
			}
			for _, p := range ps {
				if err := collectProject(r, p, c); err != nil {
					return err
				}
			}
		}
		out = c.repos
		return nil
	})
	return out, err
}

// collectProject は、プロジェクトのリポジトリと、プロジェクトの中のアイテムのリンクを c に足す。
func collectProject(r port.Reader, p project.Project, c *collector) error {
	for _, repo := range p.Repos {
		c.repo(repo)
	}
	// 削除済みのアイテムのリンクは頼まない(D4a-10)
	its, err := port.LiveItems(r, p.ID)
	if err != nil {
		return err
	}
	for _, it := range its {
		log, err := r.Changes().Of(change.Ref{Kind: change.KindItem, ID: it.ID})
		if err != nil {
			return err
		}
		c.add(it.ID, itemlink.FromLog(log).Links)
	}
	return nil
}
