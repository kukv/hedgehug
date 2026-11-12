package gitstore

import (
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
	var newDirs []string
	err = s.writeAndCommit(wt, t, op, &newDirs)
	if err != nil {
		return errors.Join(err, rollback(root, wt, head.Hash(), newDirs))
	}
	return nil
}

func (s *Stores) writeAndCommit(wt *git.Worktree, t *tx, op port.Operation, newDirs *[]string) error {
	refs := make([]change.Ref, 0, len(t.pending))
	for ref := range t.pending {
		refs = append(refs, ref)
	}
	slices.SortFunc(refs, func(a, b change.Ref) int {
		return strings.Compare(string(a.Kind)+"/"+string(a.ID), string(b.Kind)+"/"+string(b.ID))
	})
	for _, ref := range refs {
		paths, created, err := writeEntity(t, ref)
		if created {
			d, _ := dirOf(ref)
			*newDirs = append(*newDirs, d)
		}
		if err != nil {
			return err
		}
		for _, p := range paths {
			if _, err := wt.Add(p); err != nil {
				return err
			}
		}
	}
	if s.beforeCommit != nil {
		if err := s.beforeCommit(); err != nil {
			return err
		}
	}
	sig := author(op.Actor, t.meta.At)
	_, err := wt.Commit(op.Summary+"\n\nHedgehug-Op: "+string(t.meta.Op)+"\n", &git.CommitOptions{Author: sig, Committer: sig})
	return err
}

// writeEntity は、1 つのエンティティの log.jsonl に行を足し、読む用の YAML を作り直す。
// 書いたファイル(Store の根からの相対パス)と、log.jsonl を新しく作ったかを返す。
func writeEntity(t *tx, ref change.Ref) (paths []string, created bool, err error) {
	d, err := dirOf(ref)
	if err != nil {
		return nil, false, err
	}
	base, err := readLog(t.root, ref)
	if err != nil {
		return nil, false, err
	}
	created, err = appendLog(t.root, ref, t.pending[ref])
	if err != nil {
		return nil, created, err
	}
	paths = append(paths, filepath.Join(d, "log.jsonl"))
	y, err := renderYAML(ref.Kind, append(base, t.pending[ref]...))
	if err != nil || y == nil {
		return paths, created, err
	}
	name := filepath.Join(d, kindDirs[ref.Kind].yaml)
	if err := os.WriteFile(filepath.Join(t.root, name), y, 0o644); err != nil { //nolint:gosec // G306: Store のファイルは共有するもので秘密ではない
		return paths, created, err
	}
	return append(paths, name), created, nil
}

// author は、保存の作者とコミッター(01「「あなた」の決まり方」)。
func author(a change.Actor, at time.Time) *object.Signature {
	if a.Kind == change.ActorUser {
		return signature(a.Login, a.Login+"@hedgehug.invalid", at)
	}
	return signature(botName, botEmail, at)
}

// rollback は、作業ツリーとインデックスを head に戻し、新しく作ったフォルダを消す。
func rollback(root string, wt *git.Worktree, head plumbing.Hash, newDirs []string) error {
	errs := []error{wt.Reset(&git.ResetOptions{Commit: head, Mode: git.HardReset})}
	for _, d := range newDirs {
		errs = append(errs, os.RemoveAll(filepath.Join(root, d)))
	}
	return errors.Join(errs...)
}
