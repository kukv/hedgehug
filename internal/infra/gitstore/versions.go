package gitstore

import (
	"context"
	"maps"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/infra/gitstore/saves"
)

// Versions は、取り付けた Store ごとの先端のコミット(port.StoreVersions)。保存・同期・ほかのプログラムの
// コミットのどれでも変わる。読めない Store は含めない。
func (s *Stores) Versions(context.Context) (map[port.StoreID]string, error) {
	s.mu.Lock()
	dirs := maps.Clone(s.dirs)
	s.mu.Unlock()
	out := make(map[port.StoreID]string, len(dirs))
	for id, dir := range dirs {
		repo, err := git.PlainOpen(dir)
		if err != nil {
			continue
		}
		head, err := repo.Head()
		if err != nil {
			continue
		}
		out[id] = head.Hash().String()
	}
	return out, nil
}

// Commits は、Store の保存のコミットを新しい順に返す(port.StoreCommits。決定 C4d-2)。ロックは先端を読むあいだだけ
// 持ち、コミットを辿るあいだは持たない(コミットは書き換わらないので、辿っているあいだに保存が終わってもよい)。
// 読んだコミットは SHA ごとに覚え、2 回目からは差分を取らない。
func (s *Stores) Commits(_ context.Context, id port.StoreID) ([]port.Commit, error) {
	root, err := s.dir(id)
	if err != nil {
		return nil, err
	}
	repo, head, err := openHead(root)
	if err != nil {
		return nil, err
	}
	if s.duringCommits != nil {
		s.duringCommits()
	}
	s.mu.Lock()
	seen := maps.Clone(s.commits)
	s.mu.Unlock()
	out, err := saves.History(repo, head, seen)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	maps.Copy(s.commits, seen)
	s.mu.Unlock()
	return out, nil
}

// openHead は、Store のロックを取って先端を読み、すぐに手放す。途中で止まった保存が残っていれば、先に戻す。
func openHead(root string) (*git.Repository, plumbing.Hash, error) {
	unlock, err := open(root)
	if err != nil {
		return nil, plumbing.ZeroHash, err
	}
	defer unlock()
	repo, err := git.PlainOpen(root)
	if err != nil {
		return nil, plumbing.ZeroHash, err
	}
	head, err := repo.Head()
	if err != nil {
		return nil, plumbing.ZeroHash, err
	}
	return repo, head.Hash(), nil
}
