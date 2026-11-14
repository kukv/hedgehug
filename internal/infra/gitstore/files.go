package gitstore

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/infra/gitstore/saves"
)

// kindDirs は、種類ごとのフォルダ(名前は saves.Dirs が決める)と、読む用の YAML の名前。
var kindDirs = map[change.Kind]struct{ dir, yaml string }{
	change.KindMembers:   {saves.Dirs[change.KindMembers], "members.yaml"},
	change.KindWorkspace: {saves.Dirs[change.KindWorkspace], "workspace.yaml"},
	change.KindProject:   {saves.Dirs[change.KindProject], "project.yaml"},
	change.KindItem:      {saves.Dirs[change.KindItem], "item.yaml"},
}

// dirOf は、エンティティのフォルダ(Store の根からの相対パス)。ref が change.Ref.Valid でなければ
// エラー(ID は change.ValidID なので、フォルダの名前にしても Store の外を指さない)。
func dirOf(ref change.Ref) (string, error) {
	if !ref.Valid() {
		return "", fmt.Errorf("ref %+v cannot be a folder", ref)
	}
	k := kindDirs[ref.Kind]
	if ref.Kind == change.KindMembers {
		return k.dir, nil
	}
	return filepath.Join(k.dir, string(ref.ID)), nil
}

// readLog は、log.jsonl を読む。ファイルがなければ空。形の違う行(GitHub 上で手で書き換えたもの)は読み飛ばす。
func readLog(root string, ref change.Ref) ([]change.Record, error) {
	d, err := dirOf(ref)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(filepath.Join(root, d, "log.jsonl")) //nolint:gosec // G304: パスは Store の根と dirOf で検査済みの ID から作る
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	var out []change.Record
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		var r change.Record
		if json.Unmarshal(sc.Bytes(), &r) == nil {
			out = append(out, r)
		}
	}
	return out, sc.Err()
}

// listIDs は、種類のフォルダの下のエンティティの ID を、辞書順で返す。
func listIDs(root string, kind change.Kind) ([]change.ID, error) {
	entries, err := os.ReadDir(filepath.Join(root, kindDirs[kind].dir))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var ids []change.ID
	for _, e := range entries {
		if e.IsDir() && change.ValidID(change.ID(e.Name())) {
			ids = append(ids, change.ID(e.Name()))
		}
	}
	slices.SortFunc(ids, func(a, b change.ID) int { return strings.Compare(string(a), string(b)) })
	return ids, nil
}

// appendLog は、log.jsonl に行を足す。
func appendLog(root string, ref change.Ref, recs []change.Record) error {
	d, err := dirOf(ref)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil { //nolint:gosec // G301: Store のフォルダは共有するもので秘密ではない
		return err
	}
	f, err := os.OpenFile(filepath.Join(root, d, "log.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644) //nolint:gosec // G302・G304: パスは dirOf で検査済みの ID から作り、中身は共有するもので秘密ではない
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	for _, r := range recs {
		b, err := json.Marshal(r)
		if err != nil {
			_ = f.Close()
			return err
		}
		_, _ = w.Write(append(b, '\n'))
	}
	if err := w.Flush(); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// descriptionPath は、エンティティの説明のファイル(Store の根からの相対パス)。
func descriptionPath(ref change.Ref) (string, error) {
	d, err := dirOf(ref)
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "description.md"), nil
}

// readDescription は、作業ツリーの説明を読む。ファイルがなければ ""。
func readDescription(root string, ref change.Ref) (string, error) {
	p, err := descriptionPath(ref)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(filepath.Join(root, p)) //nolint:gosec // G304: パスは Store の根と dirOf で検査済みの ID から作る
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	return string(b), err
}

// transfersFile・purgedFile は、転送記録と、完全に削除したものの記録(Store の根からの相対パス)。
const (
	transfersFile = "transfers.jsonl"
	purgedFile    = "purged.jsonl"
)

type transfers struct{ v *view }

// List は、transfers.jsonl の行と、同じ Tx で書いた行を、この順で返す。
func (r transfers) List() ([]transfer.Record, error) {
	b, err := os.ReadFile(filepath.Join(r.v.root, transfersFile))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return append(transfer.ParseRecords(b), r.v.transfers...), nil
}

type purged struct{ v *view }

// List は、purged.jsonl の行を返す。JSON として読めない行・id が空の行・at が RFC 3339 でない行は飛ばす。
func (r purged) List() ([]port.Purged, error) {
	b, err := os.ReadFile(filepath.Join(r.v.root, purgedFile))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	var out []port.Purged
	for _, raw := range bytes.Split(b, []byte("\n")) {
		var l struct {
			ID      string    `json:"id"`
			At      string    `json:"at"`
			Entity  string    `json:"entity"`
			Target  change.ID `json:"target"`
			Display string    `json:"display"`
		}
		if json.Unmarshal(raw, &l) != nil || l.ID == "" {
			continue
		}
		at, err := time.Parse(time.RFC3339, l.At)
		if err != nil {
			continue
		}
		out = append(out, port.Purged{Entity: l.Entity, Target: l.Target, Display: l.Display, At: at})
	}
	return out, nil
}

// appendTransfers は、transfers.jsonl に行を足す(なければ作る)。
func appendTransfers(root string, recs []transfer.Record) error {
	var b []byte
	for _, r := range recs {
		b = append(append(b, r.JSON()...), '\n')
	}
	f, err := os.OpenFile(filepath.Join(root, transfersFile), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644) //nolint:gosec // G302・G304: パスは Store の根から作り、中身は共有するもので秘密ではない
	if err != nil {
		return err
	}
	if _, err := f.Write(b); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// writeExtras は、この Tx で消したアイテムのフォルダを消し、書いた転送記録を transfers.jsonl に足す。
// 書き直し(rewriteStale)のあとに呼ぶ。
func writeExtras(wt *git.Worktree, t *tx) error {
	refs := make([]change.Ref, 0, len(t.removed))
	for ref := range t.removed {
		refs = append(refs, ref)
	}
	slices.SortFunc(refs, func(a, b change.Ref) int { return strings.Compare(string(a.ID), string(b.ID)) })
	for _, ref := range refs {
		if err := removeFolder(wt, t.root, ref); err != nil {
			return err
		}
	}
	if len(t.transfers) == 0 {
		return nil
	}
	if err := appendTransfers(t.root, t.transfers); err != nil {
		return err
	}
	return addPaths(wt, transfersFile)
}

// removeFolder は、エンティティのフォルダの中のファイルをインデックスと作業ツリーから消し、フォルダを消す。
// フォルダがなければ(同じ Tx で写しただけのもの)何もしない。
func removeFolder(wt *git.Worktree, root string, ref change.Ref) error {
	d, err := dirOf(ref)
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(filepath.Join(root, d))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, e := range entries {
		if _, err := wt.Remove(filepath.Join(d, e.Name())); err != nil {
			return err
		}
	}
	return os.RemoveAll(filepath.Join(root, d))
}
