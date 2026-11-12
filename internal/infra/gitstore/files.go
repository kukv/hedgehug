package gitstore

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
)

var safeID = regexp.MustCompile(`^[0-9A-Za-z]+$`)

// kindDirs は、種類ごとのフォルダと、読む用の YAML の名前。
var kindDirs = map[change.Kind]struct{ dir, yaml string }{
	change.KindMembers:   {"members", "members.yaml"},
	change.KindWorkspace: {"workspaces", "workspace.yaml"},
	change.KindProject:   {"projects", "project.yaml"},
	change.KindItem:      {"items", "item.yaml"},
}

// dirOf は、エンティティのフォルダ(Store の根からの相対パス)。
func dirOf(ref change.Ref) (string, error) {
	k, ok := kindDirs[ref.Kind]
	if !ok {
		return "", fmt.Errorf("unknown kind %q", ref.Kind)
	}
	if ref.Kind == change.KindMembers {
		return k.dir, nil
	}
	if !safeID.MatchString(string(ref.ID)) {
		return "", fmt.Errorf("id %q cannot be a folder name", ref.ID)
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
		if e.IsDir() && safeID.MatchString(e.Name()) {
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
