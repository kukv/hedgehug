package apptest

import (
	"context"
	"fmt"
	"maps"
	"strings"
	"sync"
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
)

// GitHub は port.GitHub の偽物。Users はトークンからログイン名への対応。Err があれば、どの問い合わせもそれを返す。
// Items は Lookup の答え(キーは小文字のリポジトリ名と番号。Put で入れる)。LookupErr は同じキーの失敗。
// Lookups は Lookup を呼んだ回数。Rate は ReadMany が返す利用上限。
// ReadMany では、Fail(repo, 0, err) はリポジトリ全体の失敗。Fail(repo, n, err) は、err が NotFound ならその番号を
// Missing にし(port.GitHub.ReadMany の約束)、それ以外の失敗なら問い合わせ全体をその失敗にする。
type GitHub struct {
	mu        sync.Mutex
	Users     map[string]string
	Err       error
	Items     map[string]port.Found
	LookupErr map[string]error
	Lookups   int
	Rate      port.Rate
	many      map[string]int
	pulls     map[string][]port.Pull // 小文字のリポジトリ名から引く。PutPull で入れる
	pullCalls map[string]int
	missed    map[string]bool // App 未導入の owner(小文字)
}

// MissApp は、owner のリポジトリの問い合わせ(Lookup・ReadMany・ReadPulls・CheckRepo)が
// AppNotInstalled を返すようにする(false で戻す)。
func (g *GitHub) MissApp(owner string, missing bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.missed == nil {
		g.missed = map[string]bool{}
	}
	g.missed[strings.ToLower(owner)] = missing
}

// appMissing は、repo の owner が App 未導入なら、その失敗(Org は repo の書き方のまま)。g.mu を持って呼ぶ。
func (g *GitHub) appMissing(repo string) error {
	owner, _, _ := strings.Cut(repo, "/")
	if g.missed[strings.ToLower(owner)] {
		return &ghcall.Failure{Kind: ghcall.AppNotInstalled, Org: owner, Repo: repo}
	}
	return nil
}

// CheckRepo は、repo を読めるか。Fail(repo, 0, err) があればその失敗、なければ nil。
func (g *GitHub) CheckRepo(_ context.Context, token, repo string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.Err != nil {
		return g.Err
	}
	if _, ok := g.Users[token]; !ok {
		return &ghcall.Failure{Kind: ghcall.TokenRejected}
	}
	if err := g.appMissing(repo); err != nil {
		return err
	}
	return g.LookupErr[itemKey(repo, 0)]
}

func itemKey(repo string, number int) string {
	return fmt.Sprintf("%s#%d", strings.ToLower(repo), number)
}

// Put は、repo(引くときの名前)の f.Number を f にする。f.Repo が repo と違えば、改名・移管のあとの名前。
func (g *GitHub) Put(repo string, f port.Found) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.Items == nil {
		g.Items = map[string]port.Found{}
	}
	g.Items[itemKey(repo, f.Number)] = f
}

// Fail は、repo の number を引くと err を返すようにする(nil で戻す)。
func (g *GitHub) Fail(repo string, number int, err error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.LookupErr == nil {
		g.LookupErr = map[string]error{}
	}
	if err == nil {
		delete(g.LookupErr, itemKey(repo, number))
		return
	}
	g.LookupErr[itemKey(repo, number)] = err
}

func (g *GitHub) Lookup(_ context.Context, token, repo string, number int) (port.Found, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.Lookups++
	if g.Err != nil {
		return port.Found{}, g.Err
	}
	if _, ok := g.Users[token]; !ok {
		return port.Found{}, &ghcall.Failure{Kind: ghcall.TokenRejected}
	}
	if err := g.appMissing(repo); err != nil {
		return port.Found{}, err
	}
	if err := g.LookupErr[itemKey(repo, number)]; err != nil {
		return port.Found{}, err
	}
	f, ok := g.Items[itemKey(repo, number)]
	if !ok {
		return port.Found{}, &ghcall.Failure{Kind: ghcall.NotFound, Repo: repo, Number: number}
	}
	return f, nil
}

// ReadMany は、numbers を Put した値から引く。Put になければ Missing(NotFound の Fail も Missing)。
func (g *GitHub) ReadMany(_ context.Context, token, repo string, numbers []int) (port.Batch, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.many == nil {
		g.many = map[string]int{}
	}
	g.many[strings.ToLower(repo)]++
	if g.Err != nil {
		return port.Batch{}, g.Err
	}
	if _, ok := g.Users[token]; !ok {
		return port.Batch{}, &ghcall.Failure{Kind: ghcall.TokenRejected}
	}
	if err := g.appMissing(repo); err != nil {
		return port.Batch{}, err
	}
	if err := g.LookupErr[itemKey(repo, 0)]; err != nil {
		return port.Batch{}, err
	}
	b := port.Batch{Repo: repo, Found: map[int]port.Found{}, Rate: g.Rate}
	for _, n := range numbers {
		err := g.LookupErr[itemKey(repo, n)]
		if err != nil && !ghcall.Is(err, ghcall.NotFound) {
			return port.Batch{}, err
		}
		f, ok := g.Items[itemKey(repo, n)]
		if err != nil || !ok {
			b.Missing = append(b.Missing, n)
			continue
		}
		b.Repo = f.Repo
		b.Found[n] = f
	}
	return b, nil
}

// ReadManyCalls は、repo(大文字小文字を問わない)で ReadMany を呼んだ回数。
func (g *GitHub) ReadManyCalls(repo string) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.many[strings.ToLower(repo)]
}

func (g *GitHub) Viewer(_ context.Context, token string) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.Err != nil {
		return "", g.Err
	}
	login, ok := g.Users[token]
	if !ok {
		return "", &ghcall.Failure{Kind: ghcall.TokenRejected}
	}
	return login, nil
}

// AddUser は、トークン token をログイン名 login の人のものにする(通しのテストで、デバイスフローの許可に使う)。
func (g *GitHub) AddUser(token, login string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.Users == nil {
		g.Users = map[string]string{}
	}
	g.Users[token] = login
}

// Checks は port.Checks の偽物。
type Checks struct {
	mu   sync.Mutex
	rows map[port.StoreID]map[string]itemlink.Checks
}

func NewChecks() *Checks { return &Checks{rows: map[port.StoreID]map[string]itemlink.Checks{}} }

func (c *Checks) Read(_ context.Context, store port.StoreID) (map[string]itemlink.Checks, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return maps.Clone(c.rows[store]), nil
}

func (c *Checks) Save(_ context.Context, store port.StoreID, key string, v itemlink.Checks) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.rows[store] == nil {
		c.rows[store] = map[string]itemlink.Checks{}
	}
	if c.rows[store][key] == v {
		return false, nil
	}
	if v == itemlink.NoChecks {
		delete(c.rows[store], key)
	} else {
		c.rows[store][key] = v
	}
	return true, nil
}

// RunChecksContract は、port.Checks の実装が守ること(偽物と state.db で同じテストを流す)。
func RunChecksContract(t *testing.T, open func(t *testing.T) port.Checks) {
	t.Run("save, read, change and clear", func(t *testing.T) {
		c, ctx := open(t), t.Context()
		if got, err := c.Read(ctx, "S1"); err != nil || len(got) != 0 {
			t.Fatalf("empty Read = %v, %v", got, err)
		}
		mustSave := func(key string, v itemlink.Checks, want bool) {
			t.Helper()
			if changed, err := c.Save(ctx, "S1", key, v); err != nil || changed != want {
				t.Fatalf("Save(%s, %q) = %v, %v; want changed %v", key, v, changed, err, want)
			}
		}
		mustSave("pr:org/web#1", itemlink.Running, true)
		mustSave("pr:org/web#1", itemlink.Running, false)
		mustSave("pr:org/web#1", itemlink.Success, true)
		mustSave("pr:org/web#2", itemlink.Failure, true)
		if got, _ := c.Read(ctx, "S1"); len(got) != 2 || got["pr:org/web#1"] != itemlink.Success || got["pr:org/web#2"] != itemlink.Failure {
			t.Errorf("Read = %v", got)
		}
		if got, _ := c.Read(ctx, "S2"); len(got) != 0 {
			t.Errorf("another store sees %v", got)
		}
		mustSave("pr:org/web#2", itemlink.NoChecks, true)
		mustSave("pr:org/web#2", itemlink.NoChecks, false)
		if got, _ := c.Read(ctx, "S1"); len(got) != 1 {
			t.Errorf("after clearing: %v", got)
		}
	})
}
