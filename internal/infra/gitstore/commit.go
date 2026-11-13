package gitstore

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

// writeMarker は、保存の途中であることを示すファイル(.git の中)。ファイルを変える前に作り、
// コミットか戻しが済んだら消す。ロックを取ったときに残っていれば、前の保存が途中で止まった
// (プロセスが落ちた)ので、操作の前のコミットに戻す(保存形式「技術構成」)。
const writeMarker = "hedgehug-write"

func markerPath(root string) string { return filepath.Join(root, ".git", writeMarker) }

// commit は、t の pending を log.jsonl に足し、*.yaml を作り直して、1 つのコミットにする。
// 途中で失敗したら、作業ツリーとインデックスを操作の前のコミットに戻す(保存形式「技術構成」)。
func (s *Stores) commit(root string, t *tx, op port.Operation) error {
	repo, err := git.PlainOpen(root)
	if err != nil {
		return err
	}
	wt, err := repo.Worktree()
	if err != nil {
		return err
	}
	head, err := repo.Head()
	if err != nil {
		return err
	}
	if err := os.WriteFile(markerPath(root), nil, 0o600); err != nil {
		return err
	}
	if err := s.writeAndCommit(wt, t, op); err != nil {
		if rbErr := rollback(wt, head.Hash()); rbErr != nil {
			// 戻せなかったので印を残し、次の操作で戻す。
			return errors.Join(err, rbErr)
		}
		return errors.Join(err, os.Remove(markerPath(root)))
	}
	// コミットは済んでいる。印が消せなくても、次の操作がいまのコミットに戻すだけで、保存は失われない。
	_ = os.Remove(markerPath(root))
	return nil
}

func (s *Stores) writeAndCommit(wt *git.Worktree, t *tx, op port.Operation) error {
	sc, err := newTxSettings(t)
	if err != nil {
		return err
	}
	stale, err := sc.staleItems()
	if err != nil {
		return err
	}
	refs := make([]change.Ref, 0, len(t.pending))
	for ref := range t.pending {
		refs = append(refs, ref)
	}
	slices.SortFunc(refs, func(a, b change.Ref) int {
		return strings.Compare(string(a.Kind)+"/"+string(a.ID), string(b.Kind)+"/"+string(b.ID))
	})
	for _, ref := range refs {
		paths, err := writeEntity(t, ref, sc.lookup, sc.project)
		if err != nil {
			return err
		}
		if err := addPaths(wt, paths...); err != nil {
			return err
		}
		if err := s.hook(s.afterEntity); err != nil {
			return err
		}
	}
	if err := writeTexts(wt, t); err != nil {
		return err
	}
	if err := rewriteStale(wt, t, sc, stale); err != nil {
		return err
	}
	if err := writeExtras(wt, t); err != nil {
		return err
	}
	if err := writePurges(wt, t); err != nil {
		return err
	}
	if err := s.hook(s.beforeCommit); err != nil {
		return err
	}
	sig := author(op.Actor, t.meta.At)
	_, err = wt.Commit(op.Summary+"\n\nHedgehug-Op: "+string(t.meta.Op)+"\n", &git.CommitOptions{Author: sig, Committer: sig})
	return err
}

// addPaths は、書いたファイルをインデックスに足す。空のパス(書かなかったもの)は飛ばす。
func addPaths(wt *git.Worktree, paths ...string) error {
	for _, p := range paths {
		if p == "" {
			continue
		}
		if err := wt.AddWithOptions(&git.AddOptions{Path: p, SkipStatus: true}); err != nil {
			return err
		}
	}
	return nil
}

// writeTexts は、この Tx で書いた説明を description.md にする(空なら消す)。エンティティのフォルダを
// 作ったあと(writeEntity のあと)に呼ぶ。
func writeTexts(wt *git.Worktree, t *tx) error {
	refs := make([]change.Ref, 0, len(t.texts))
	for ref := range t.texts {
		refs = append(refs, ref)
	}
	slices.SortFunc(refs, func(a, b change.Ref) int {
		return strings.Compare(string(a.Kind)+"/"+string(a.ID), string(b.Kind)+"/"+string(b.ID))
	})
	for _, ref := range refs {
		p, err := descriptionPath(ref)
		if err != nil {
			return err
		}
		if t.texts[ref] == "" {
			// describe は、作業ツリーと同じ説明を入れないので、空のときはファイルがある。
			if _, err := wt.Remove(p); err != nil {
				return err
			}
			continue
		}
		if err := os.WriteFile(filepath.Join(t.root, p), []byte(t.texts[ref]), 0o644); err != nil { //nolint:gosec // G306: Store のファイルは共有するもので秘密ではない
			return err
		}
		if err := addPaths(wt, p); err != nil {
			return err
		}
	}
	return nil
}

// rewriteStale は、設定が変わったワークスペースの、ほかのアイテムの item.yaml を同じコミットで作り直す(B2-19)。
func rewriteStale(wt *git.Worktree, t *tx, sc *txSettings, stale []change.ID) error {
	for _, id := range stale {
		p, err := rewriteItemYAML(t, id, sc.lookup)
		if err != nil {
			return err
		}
		if err := addPaths(wt, p); err != nil {
			return err
		}
	}
	return sc.err
}

// writePurges は、この Tx で完全に削除したエンティティのフォルダのファイルを消し、purged.jsonl に
// 1 つにつき 1 行足す(保存形式「削除・復元・完全に削除」)。
func writePurges(wt *git.Worktree, t *tx) error {
	if len(t.purges) == 0 {
		return nil
	}
	var b bytes.Buffer
	for _, p := range t.purges {
		d, err := dirOf(change.Ref{Kind: p.Entity, ID: p.Target})
		if err != nil {
			return err
		}
		if _, err := os.Stat(filepath.Join(t.root, d)); err == nil {
			if _, err := wt.Remove(d); err != nil {
				return err
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		line, err := json.Marshal(p)
		if err != nil {
			return err
		}
		b.Write(append(line, '\n'))
	}
	f, err := os.OpenFile(filepath.Join(t.root, "purged.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644) //nolint:gosec // G302・G304: パスは Store の根と決まったファイル名で、中身は共有するもので秘密ではない
	if err != nil {
		return err
	}
	if _, err := f.Write(b.Bytes()); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return addPaths(wt, "purged.jsonl")
}

func (s *Stores) hook(f func() error) error {
	if f == nil {
		return nil
	}
	return f()
}

// writeEntity は、1 つのエンティティの log.jsonl に行を足し、読む用の YAML を作り直す。
// 書いたファイル(Store の根からの相対パス)を返す。
func writeEntity(t *tx, ref change.Ref, settings settingsOf, projects projectOf) ([]string, error) {
	d, err := dirOf(ref)
	if err != nil {
		return nil, err
	}
	base, err := readLog(t.root, ref)
	if err != nil {
		return nil, err
	}
	if err := appendLog(t.root, ref, t.pending[ref]); err != nil {
		return nil, err
	}
	paths := []string{filepath.Join(d, "log.jsonl")}
	y, err := renderYAML(ref.Kind, append(base, t.pending[ref]...), settings, projects)
	if err != nil || y == nil {
		return paths, err
	}
	name := filepath.Join(d, kindDirs[ref.Kind].yaml)
	if err := os.WriteFile(filepath.Join(t.root, name), y, 0o644); err != nil { //nolint:gosec // G306: Store のファイルは共有するもので秘密ではない
		return paths, err
	}
	return append(paths, name), nil
}

// author は、保存の作者とコミッター(01「「あなた」の決まり方」)。
func author(a change.Actor, at time.Time) *object.Signature {
	if a.Kind == change.ActorUser {
		return signature(a.Login, a.Login+"@hedgehug.invalid", at)
	}
	return signature(botName, botEmail, at)
}

// rollback は、作業ツリーとインデックスを head に戻し、head にないファイルとフォルダを消す。
// go-git v5 のハードリセットは、いまはインデックスにないファイルも消すが、それは git と違う振る舞いなので
// 頼らず、Clean で消すことをここで約束する(失敗したときだけ通るので、全体の状態を見る費用は気にしない)。
func rollback(wt *git.Worktree, head plumbing.Hash) error {
	if err := wt.Reset(&git.ResetOptions{Commit: head, Mode: git.HardReset}); err != nil {
		return err
	}
	return wt.Clean(&git.CleanOptions{Dir: true})
}

// recoverInterrupted は、途中で止まった保存の印があれば、作業ツリーとインデックスをいまのコミットに戻し、
// 印を消す。ロックを持っているあいだに呼ぶ。
func recoverInterrupted(root string) error {
	if _, err := os.Stat(markerPath(root)); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	repo, err := git.PlainOpen(root)
	if err != nil {
		return err
	}
	wt, err := repo.Worktree()
	if err != nil {
		return err
	}
	head, err := repo.Head()
	if err != nil {
		return err
	}
	if err := rollback(wt, head.Hash()); err != nil {
		return err
	}
	return os.Remove(markerPath(root))
}
