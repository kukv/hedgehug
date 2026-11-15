package machineconfig

import (
	"context"
	"regexp"
	"slices"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
)

// storeYAML は config.yaml の stores の 1 行(G1-1)。並びがサイドバーの並び。
type storeYAML struct {
	ID       string   `yaml:"id"`
	Name     string   `yaml:"name"`
	Path     string   `yaml:"path"`
	Remote   string   `yaml:"remote,omitempty"`
	Mirrors  []string `yaml:"mirrors,omitempty"`
	Account  string   `yaml:"account,omitempty"`
	Color    string   `yaml:"color,omitempty"`
	Hidden   bool     `yaml:"hidden,omitempty"`
	ReadOnly bool     `yaml:"read_only,omitempty"`
}

// storeColors は、画面のトークン --t-<色> の名前。色が空・この中にないときは、この順で使われていない色を選ぶ(G1-19)。
var storeColors = []string{"teal", "orange", "purple", "sky", "pink", "olive", "brown"}

// sshRemote は SSH の形の GitHub の URL(G1-9)。
var sshRemote = regexp.MustCompile(`^git@github\.com:([^/]+)/(.+?)(\.git)?/?$`)

// StoreEntry は、起動のときに開く Store の行。前回の版の形の行は ID が空。
type StoreEntry struct {
	ID   port.StoreID
	Path string
}

// StoreEntries は、config.yaml の stores のすべての行を、並びのまま返す。id や path が空の行も返し、
// 開かない行を知らせるのは開く側(G1-2)。
func (f *File) StoreEntries() ([]StoreEntry, error) {
	c, err := f.read()
	if err != nil {
		return nil, err
	}
	out := make([]StoreEntry, 0, len(c.Stores))
	for _, s := range c.Stores {
		out = append(out, StoreEntry{ID: port.StoreID(s.ID), Path: s.Path})
	}
	return out, nil
}

// StoresView は、config.yaml の stores と login を読む口の組。
// 3 つの口は List の戻り値が違い、1 つの型では実装できないので、口ごとの値を返す。
type StoresView struct {
	f    *File
	open func(id port.StoreID) bool
}

// Stores は、開けた Store(open が true の ID)だけを見せる StoresView を返す。
func (f *File) Stores(open func(id port.StoreID) bool) StoresView {
	return StoresView{f: f, open: open}
}

func (v StoresView) StoreList() port.StoreList     { return storeList{v} }
func (v StoresView) SyncTargets() port.SyncTargets { return syncTargets{v} }
func (v StoresView) StoreGitHub() port.StoreGitHub { return storeGitHub{v} }

// rows は、開けた行(id がなく開けない行を含まない)を config の並びで返す。
func (v StoresView) rows() ([]storeYAML, error) {
	c, err := v.f.read()
	if err != nil {
		return nil, err
	}
	var out []storeYAML
	for _, s := range c.Stores {
		if s.ID != "" && v.open(port.StoreID(s.ID)) {
			out = append(out, s)
		}
	}
	return out, nil
}

type storeList struct{ v StoresView }

var _ port.StoreList = storeList{}

func (l storeList) List(context.Context) ([]port.StoreInfo, error) {
	rows, err := l.v.rows()
	if err != nil {
		return nil, err
	}
	var out []port.StoreInfo
	var used []string
	for _, s := range rows {
		if s.Hidden {
			continue
		}
		color := pickColor(s.Color, used)
		used = append(used, color)
		out = append(out, port.StoreInfo{ID: port.StoreID(s.ID), Name: s.Name, Color: color, ReadOnly: s.ReadOnly})
	}
	return out, nil
}

// pickColor は、color が 7 色のどれかならそのまま、そうでなければ used にない最初の色(なければ teal)。
func pickColor(color string, used []string) string {
	if slices.Contains(storeColors, color) {
		return color
	}
	for _, c := range storeColors {
		if !slices.Contains(used, c) {
			return c
		}
	}
	return storeColors[0]
}

type syncTargets struct{ v StoresView }

var _ port.SyncTargets = syncTargets{}

func (t syncTargets) List(context.Context) ([]port.SyncTarget, error) {
	rows, err := t.v.rows()
	if err != nil {
		return nil, err
	}
	out := make([]port.SyncTarget, 0, len(rows))
	for _, s := range rows {
		var mirrors []string
		for _, m := range s.Mirrors {
			mirrors = append(mirrors, httpsRemote(m))
		}
		out = append(out, port.SyncTarget{
			ID: port.StoreID(s.ID), Name: s.Name, Remote: httpsRemote(s.Remote), Mirrors: mirrors,
			Account: account.ID(s.Account), Hidden: s.Hidden, ReadOnly: s.ReadOnly,
		})
	}
	return out, nil
}

// httpsRemote は、SSH の形の GitHub の URL を HTTPS に直す。それ以外はそのまま返す(G1-9)。
func httpsRemote(url string) string {
	return sshRemote.ReplaceAllString(url, "https://github.com/$1/$2.git")
}

type storeGitHub struct{ v StoresView }

var _ port.StoreGitHub = storeGitHub{}

func (g storeGitHub) List(context.Context) ([]port.StoreGitHubInfo, error) {
	rows, err := g.v.rows()
	if err != nil {
		return nil, err
	}
	out := make([]port.StoreGitHubInfo, 0, len(rows))
	for _, s := range rows {
		out = append(out, port.StoreGitHubInfo{
			ID: port.StoreID(s.ID), Name: s.Name, Account: account.ID(s.Account),
			Remote: s.Remote != "", ReadOnly: s.ReadOnly, Hidden: s.Hidden,
		})
	}
	return out, nil
}

func (g storeGitHub) Unassign(_ context.Context, id account.ID) error {
	return g.v.f.update(func(c *config) error {
		for i := range c.Stores {
			if c.Stores[i].Account == string(id) {
				c.Stores[i].Account = ""
			}
		}
		return nil
	})
}

func (g storeGitHub) SetMachineLogin(_ context.Context, login string) error {
	return g.v.f.update(func(c *config) error {
		c.Login = login
		return nil
	})
}

func (g storeGitHub) MachineLogin(context.Context) (string, error) {
	c, err := g.v.f.read()
	return c.Login, err
}
