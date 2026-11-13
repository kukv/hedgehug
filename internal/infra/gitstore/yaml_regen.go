package gitstore

import (
	"os"
	"path/filepath"
	"reflect"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
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
	s  workspace.Settings
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

func (c *txSettings) lookup(id workspace.ID) (workspace.Settings, bool) {
	if e, ok := c.cache[id]; ok {
		return e.s, e.ok
	}
	log, err := c.t.log(change.Ref{Kind: change.KindWorkspace, ID: id})
	if err != nil && c.err == nil {
		c.err = err
	}
	s, ok := workspace.SettingsFromLog(log)
	c.cache[id] = settingsEntry{s, ok}
	return s, ok
}

// changedWorkspaces は、この Tx で設定が変わるワークスペース。採番の行だけを足すような保存では変わらない。
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
		before, ok := workspace.SettingsFromLog(base)
		if after, _ := c.lookup(ref.ID); ok && !reflect.DeepEqual(before, after) {
			out[ref.ID] = true
		}
	}
	return out, c.err
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
		if _, writing := c.t.pending[ref]; writing || c.t.removed[ref] {
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
