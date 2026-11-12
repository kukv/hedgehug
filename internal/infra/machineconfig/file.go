// Package machineconfig は、このマシンの設定(~/.config/hedgehug/config.yaml)を持つ。
// トークンはここに置かない(OS のキーチェーンなど。infra/secret)。
package machineconfig

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"go.yaml.in/yaml/v4"
)

// config は config.yaml の中身。項目は、持つトラックが足す(トラック G が Store の一覧などを足す)。
type config struct {
	Accounts []accountYAML `yaml:"accounts"`
}

// File は config.yaml。同じプロセスの中の読み書きは 1 つずつ行う。
type File struct {
	path string
	mu   sync.Mutex
}

func Open(path string) *File { return &File{path: path} }

// update は、config.yaml を読み、fn で変えて書く。fn がエラーなら書かない。
func (f *File) update(fn func(c *config) error) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, err := f.load()
	if err != nil {
		return err
	}
	if err := fn(&c); err != nil {
		return err
	}
	return f.save(c)
}

func (f *File) read() (config, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.load()
}

func (f *File) load() (config, error) {
	var c config
	b, err := os.ReadFile(f.path)
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	return c, yaml.Unmarshal(b, &c)
}

// save は、同じフォルダの一時ファイルに書いてから置き換える(途中で止まっても壊れた設定を残さない)。
func (f *File) save(c config) error {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(c); err != nil {
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	dir := filepath.Dir(f.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".config-*.yaml")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(buf.Bytes()); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), f.path)
}
