package gitstore

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/app/port"
)

// Format は、この hedgehug が書く Store の形式の版。
const Format = 1

// defaultRetentionDays は削除済みの保持日数の既定(08「保持日数」)。
const defaultRetentionDays = 30

var (
	ErrNotStore    = errors.New("not a hedgehug store")
	ErrNewerFormat = errors.New("the store was made by a newer hedgehug")
)

const storeFile = ".hedgehug/store.yaml"

type storeYAML struct {
	Store         string `yaml:"store"`
	Format        int    `yaml:"format"`
	RetentionDays int    `yaml:"retention_days"`
}

// Create は、空のフォルダ(またはないフォルダ)に Store を作る。最初のコミットで store.yaml だけを書く。
// このコミットは保存ではないので Hedgehug-Op を持たない(保存形式「.hedgehug/store.yaml」)。
func Create(dir string, id port.StoreID, at time.Time) error {
	if entries, err := os.ReadDir(dir); err == nil && len(entries) > 0 {
		return fmt.Errorf("create store in %s: the folder is not empty", dir)
	}
	repo, err := git.PlainInitWithOptions(dir, &git.PlainInitOptions{
		InitOptions: git.InitOptions{DefaultBranch: plumbing.NewBranchReferenceName("main")},
	})
	if err != nil {
		return fmt.Errorf("create store: %w", err)
	}
	b, err := encodeYAML(storeYAML{Store: string(id), Format: Format, RetentionDays: defaultRetentionDays})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(dir, ".hedgehug"), 0o755); err != nil { //nolint:gosec // Store のファイルは共有するもので秘密ではない
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, storeFile), b, 0o644); err != nil { //nolint:gosec // Store のファイルは共有するもので秘密ではない
		return err
	}
	wt, err := repo.Worktree()
	if err != nil {
		return err
	}
	if _, err := wt.Add(storeFile); err != nil {
		return err
	}
	sig := signature(botName, botEmail, at)
	_, err = wt.Commit("Store を作成\n", &git.CommitOptions{Author: sig, Committer: sig})
	return err
}

func readStoreYAML(dir string) (storeYAML, error) {
	b, err := os.ReadFile(filepath.Join(dir, storeFile)) //nolint:gosec // G304: パスは Store の根と決まったファイル名から作る
	if errors.Is(err, os.ErrNotExist) {
		return storeYAML{}, ErrNotStore
	}
	if err != nil {
		return storeYAML{}, err
	}
	var y storeYAML
	if err := yaml.Unmarshal(b, &y); err != nil || y.Store == "" {
		return storeYAML{}, ErrNotStore
	}
	if y.Format > Format {
		return storeYAML{}, ErrNewerFormat
	}
	return y, nil
}

// encodeYAML は、字下げ 2 の YAML にする。
func encodeYAML(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
