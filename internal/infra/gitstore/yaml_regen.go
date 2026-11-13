package gitstore

import (
	"maps"
	"os"
	"path/filepath"
	"reflect"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// txSettings は、書く道で設定を引く。この Tx が書くワークスペースは、ログに行を足す前に(書いた行を
// 含めて)引いて覚えておく。足したあとにもう一度読むと、書いた行を二重に数えるため。
type txSettings struct {
	t     *tx
	cache map[workspace.ID]settingsEntry
	err   error // lookup が読めなかったときの最初のエラー
}

type settingsEntry struct {
	s  wsInfo
	ok bool
}

func newTxSettings(t *tx) (*txSettings, error) {
	c := &txSettings{t: t, cache: map[workspace.ID]settingsEntry{}}
	for ref := range t.pending {
		if ref.Kind != change.KindWorkspace {
			continue
		}
		c.lookup(ref.ID)
	}
	return c, c.err
}

func (c *txSettings) lookup(id workspace.ID) (wsInfo, bool) {
	if e, ok := c.cache[id]; ok {
		return e.s, e.ok
	}
	log, err := c.t.log(change.Ref{Kind: change.KindWorkspace, ID: id})
	if err != nil && c.err == nil {
		c.err = err
	}
	s, ok := wsInfoFromLog(log)
	c.cache[id] = settingsEntry{s, ok}
	return s, ok
}

// changedWorkspaces は、この Tx で設定か、スプリント・リリースの注釈が変わるワークスペース。採番の行だけを
// 足すような保存や、スプリント・リリースの状態・日付だけを変える保存では変わらない。
func (c *txSettings) changedWorkspaces() (map[workspace.ID]bool, error) {
	out := map[workspace.ID]bool{}
	for ref := range c.t.pending {
		if ref.Kind != change.KindWorkspace {
			continue
		}
		base, err := readLog(c.t.root, ref)
		if err != nil {
			return nil, err
		}
		before, ok := wsInfoFromLog(base)
		after, _ := c.lookup(ref.ID)
		if ok && (!reflect.DeepEqual(before.Settings, after.Settings) || !maps.Equal(planNames(before.Plan), planNames(after.Plan))) {
			out[ref.ID] = true
		}
	}
	return out, c.err
}

// planNames は、item.yaml の sprint・release の行末に書く名前(内部 ID ごと)。削除済みのものは使えず(F1-5)
// 注釈を付けないので入れない。名前の変更のほか、削除・復元・完全な削除でも変わる。
func planNames(p planning.Plan) map[change.ID]string {
	out := map[change.ID]string{}
	for _, s := range p.Sprints {
		if s.Deleted == nil {
			out[s.ID] = s.Name
		}
	}
	for _, r := range p.Releases {
		if r.Deleted == nil {
			out[r.ID] = r.Name
		}
	}
	return out
}

// staleItems は、設定が変わるワークスペースのアイテムのうち、この Tx では書かないもの(item.yaml の
// 注釈だけを作り直す。ログは書かない)。ファイルを書く前に呼ぶ。
func (c *txSettings) staleItems() ([]change.ID, error) {
	changed, err := c.changedWorkspaces()
	if err != nil || len(changed) == 0 {
		return nil, err
	}
	ids, err := c.t.view.ids(change.KindItem)
	if err != nil {
		return nil, err
	}
	var out []change.ID
	for _, id := range ids {
		ref := change.Ref{Kind: change.KindItem, ID: id}
		if _, writing := c.t.pending[ref]; writing {
			continue
		}
		log, err := readLog(c.t.root, ref)
		if err != nil {
			return nil, err
		}
		if it, ok := item.FromLog(log); ok && changed[it.Place.Workspace] {
			out = append(out, id)
		}
	}
	return out, nil
}

// rewriteItemYAML は、アイテムの item.yaml を、いまのログと設定から作り直す。書いたファイルを返す。
func rewriteItemYAML(t *tx, id change.ID, settings settingsOf) (string, error) {
	ref := change.Ref{Kind: change.KindItem, ID: id}
	d, err := dirOf(ref)
	if err != nil {
		return "", err
	}
	log, err := readLog(t.root, ref)
	if err != nil {
		return "", err
	}
	y, err := renderYAML(change.KindItem, log, settings)
	if err != nil || y == nil {
		return "", err
	}
	name := filepath.Join(d, kindDirs[change.KindItem].yaml)
	return name, os.WriteFile(filepath.Join(t.root, name), y, 0o644) //nolint:gosec // G306: Store のファイルは共有するもので秘密ではない
}
