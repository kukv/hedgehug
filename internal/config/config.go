package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v4"
)

// ErrNoStores means config.yaml lists no store.
var ErrNoStores = errors.New("no store is set up; run hedgehug store init <name>")

// StoreEntry is one store in config.yaml.
type StoreEntry struct {
	Name string
	Path string // absolute, with ~/ expanded
}

// Config is config.yaml. Only stores[].name and stores[].path are read;
// the rest of the file, comments included, is kept as it was by Save.
type Config struct {
	Stores []StoreEntry

	file string
	home string
	doc  *yaml.Node // the document; nil when the file does not exist
}

// Load reads config.yaml. A missing file is a config with no stores.
func Load(p Paths) (*Config, error) {
	c := &Config{file: p.ConfigFile, home: p.Home}
	b, err := os.ReadFile(p.ConfigFile) //nolint:gosec // the path is the user's own config file
	if errors.Is(err, fs.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return nil, err
	}
	var doc yaml.Node
	if err := yaml.Load(b, &doc); err != nil {
		if strings.Contains(err.Error(), "no documents") {
			return c, nil
		}
		return nil, fmt.Errorf("%s: %w", c.file, err)
	}
	if doc.Kind == 0 || len(doc.Content) == 0 {
		return c, nil
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, c.errAt(root, "want a mapping")
	}
	c.doc = &doc
	seq := valueOf(root, "stores")
	if seq == nil {
		return c, nil
	}
	if seq.Kind != yaml.SequenceNode {
		return nil, c.errAt(seq, "stores: want a list")
	}
	seen := map[string]bool{}
	for _, it := range seq.Content {
		if it.Kind != yaml.MappingNode {
			return nil, c.errAt(it, "stores: want a mapping with name and path")
		}
		name, path := valueOf(it, "name"), valueOf(it, "path")
		if name == nil || name.Value == "" {
			return nil, c.errAt(it, "stores: name is missing")
		}
		if path == nil || path.Value == "" {
			return nil, c.errAt(it, "stores: path is missing")
		}
		if seen[name.Value] {
			return nil, c.errAt(it, fmt.Sprintf("stores: %q is listed twice", name.Value))
		}
		seen[name.Value] = true
		c.Stores = append(c.Stores, StoreEntry{Name: name.Value, Path: c.expand(path.Value)})
	}
	return c, nil
}

// Select picks the store a command works on: the one named name, or the
// only one when name is empty.
func (c *Config) Select(name string) (StoreEntry, error) {
	if len(c.Stores) == 0 {
		return StoreEntry{}, ErrNoStores
	}
	names := make([]string, len(c.Stores))
	for i, s := range c.Stores {
		if s.Name == name {
			return s, nil
		}
		names[i] = s.Name
	}
	if name != "" {
		return StoreEntry{}, fmt.Errorf("no store is named %q (stores: %s)", name, strings.Join(names, ", "))
	}
	if len(c.Stores) == 1 {
		return c.Stores[0], nil
	}
	return StoreEntry{}, fmt.Errorf("several stores are set up; pick one with --store (stores: %s)", strings.Join(names, ", "))
}

// AddStore adds a store to the end of stores. Save writes it.
func (c *Config) AddStore(name, path string) error {
	for _, s := range c.Stores {
		if s.Name == name {
			return fmt.Errorf("a store named %q is already set up", name)
		}
	}
	if c.doc == nil {
		c.doc = &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}}
	}
	root := c.doc.Content[0]
	seq := valueOf(root, "stores")
	if seq == nil {
		seq = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		root.Content = append(root.Content, scalar("stores"), seq)
	}
	seq.Content = append(seq.Content, &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{
		scalar("name"), scalar(name),
		scalar("path"), scalar(c.shorten(path)),
	}})
	c.Stores = append(c.Stores, StoreEntry{Name: name, Path: path})
	return nil
}

// Save writes config.yaml through a temporary file and a rename.
func (c *Config) Save() error {
	if c.doc == nil {
		return nil
	}
	b, err := yaml.Dump(c.doc, yaml.WithLineWidth(-1))
	if err != nil {
		return err
	}
	dir := filepath.Dir(c.file)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".config.yaml.tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	_, err = f.Write(b)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp, c.file)
	}
	if err != nil {
		_ = os.Remove(tmp)
	}
	return err
}

var storeNameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// ValidStoreName reports whether name can name a store: lower-case letters,
// digits, - and _, starting with a letter or a digit. The name is also the
// directory of the store.
func ValidStoreName(name string) error {
	if !storeNameRE.MatchString(name) {
		return fmt.Errorf("%q cannot name a store: use lower-case letters, digits, - and _, starting with a letter or a digit", name)
	}
	return nil
}

func (c *Config) errAt(n *yaml.Node, msg string) error {
	return fmt.Errorf("%s:%d: %s", c.file, n.Line, msg)
}

// expand turns a leading ~/ into the home directory.
func (c *Config) expand(p string) string {
	if rest, ok := strings.CutPrefix(p, "~/"); ok {
		return filepath.Join(c.home, filepath.FromSlash(rest))
	}
	return filepath.Clean(filepath.FromSlash(p))
}

// shorten writes p under the home directory as ~/..., with slashes.
func (c *Config) shorten(p string) string {
	if rel, err := filepath.Rel(c.home, p); err == nil && rel != "." && !strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel) {
		return "~/" + filepath.ToSlash(rel)
	}
	return filepath.ToSlash(p)
}

// valueOf returns the value of key in the mapping m, or nil.
func valueOf(m *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

func scalar(v string) *yaml.Node { return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v} }
