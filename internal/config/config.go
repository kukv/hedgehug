package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/model"
)

// ErrNoStores means config.yaml lists no store.
var ErrNoStores = errors.New("no store is set up; run hedgehug store init <name>")

// StoreEntry is one store in config.yaml.
type StoreEntry struct {
	Name    string
	Path    string   // absolute, with ~/ expanded
	Remotes []string // as written: a path or a URL, so ~/ is not expanded
	Account string   // the name of an entry of accounts, or empty
}

// Account is a person on a host. Commits of a store that has an account
// are authored as the login.
type Account struct {
	Name  string
	Host  string
	Login string
	Token TokenSource
}

// TokenSource is where the token of an account comes from: gh, an
// environment variable, the keychain, or the device flow. Kind is empty for
// an account that uses no token.
type TokenSource struct {
	Kind string // "", "gh", "env", "keychain" or "device"
	Env  string // the variable, for env
}

// String is the value of token: in config.yaml.
func (t TokenSource) String() string {
	if t.Kind == "env" {
		return "env:" + t.Env
	}
	return t.Kind
}

func parseTokenSource(s string) (TokenSource, bool) {
	switch s {
	case "gh", "keychain", "device":
		return TokenSource{Kind: s}, true
	}
	if v, ok := strings.CutPrefix(s, "env:"); ok && v != "" {
		return TokenSource{Kind: "env", Env: v}, true
	}
	return TokenSource{}, false
}

// ID is host/login, which names the account in Hedgehug-Author trailers.
func (a Account) ID() string { return a.Host + "/" + a.Login }

// SyncConfig is the sync section: how often a store is pulled and pushed.
type SyncConfig struct {
	Interval     time.Duration
	PushInterval time.Duration
}

// Default sync intervals.
const (
	defaultSyncInterval = 2 * time.Minute
	defaultPushInterval = 30 * time.Second
)

// Config is config.yaml. Only stores, accounts, hosts, default_store,
// url_rewrites, repo_accounts and sync are read; the rest of the file,
// comments included, is kept as it was by Save.
type Config struct {
	Stores       []StoreEntry
	Accounts     []Account
	Hosts        []Host
	DefaultStore string
	URLRewrites  []URLRewrite
	RepoAccounts []RepoAccount
	Sync         SyncConfig

	file string
	home string
	doc  *yaml.Node // the document; nil when the file does not exist

	// flush is true when the file writes its list items at the column of
	// the key ("stores:\n- name: a"), and false for the indented style.
	flush bool
}

// Load reads config.yaml. A missing file is a config with no stores.
func Load(p Paths) (*Config, error) {
	c := &Config{file: p.ConfigFile, home: p.Home, Sync: SyncConfig{Interval: defaultSyncInterval, PushInterval: defaultPushInterval}}
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
			// Only comments (or nothing): keep the comments for Save.
			if t := strings.TrimSpace(string(b)); t != "" {
				c.doc = &yaml.Node{Kind: yaml.DocumentNode, HeadComment: t}
			}
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
	c.flush = detectFlush(root)
	if err := c.loadAccounts(valueOf(root, "accounts")); err != nil {
		return nil, err
	}
	if err := c.loadHosts(valueOf(root, "hosts")); err != nil {
		return nil, err
	}
	if err := c.loadRewrites(valueOf(root, "url_rewrites")); err != nil {
		return nil, err
	}
	if err := c.loadSync(valueOf(root, "sync")); err != nil {
		return nil, err
	}
	if err := c.loadStores(valueOf(root, "stores")); err != nil {
		return nil, err
	}
	if err := c.loadRepoAccounts(valueOf(root, "repo_accounts")); err != nil {
		return nil, err
	}
	if d := valueOf(root, "default_store"); d != nil && d.Tag != "!!null" {
		if !slices.ContainsFunc(c.Stores, func(s StoreEntry) bool { return s.Name == d.Value }) {
			return nil, c.errAt(d, fmt.Sprintf("default_store: no store is named %q", d.Value))
		}
		c.DefaultStore = d.Value
	}
	return c, nil
}

// loadStores reads the stores list.
func (c *Config) loadStores(seq *yaml.Node) error {
	if seq == nil || seq.Tag == "!!null" {
		return nil
	}
	if seq.Kind != yaml.SequenceNode {
		return c.errAt(seq, "stores: want a list")
	}
	seen := map[string]bool{}
	for _, it := range seq.Content {
		if it.Kind != yaml.MappingNode {
			return c.errAt(it, "stores: want a mapping with name and path")
		}
		name, storePath := valueOf(it, "name"), valueOf(it, "path")
		if name == nil || name.Value == "" {
			return c.errAt(it, "stores: name is missing")
		}
		if storePath == nil || storePath.Value == "" {
			return c.errAt(it, "stores: path is missing")
		}
		if seen[name.Value] {
			return c.errAt(it, fmt.Sprintf("stores: %q is listed twice", name.Value))
		}
		seen[name.Value] = true
		e := StoreEntry{Name: name.Value, Path: c.expand(storePath.Value)}
		if rs := valueOf(it, "remotes"); rs != nil && rs.Tag != "!!null" {
			if rs.Kind != yaml.SequenceNode {
				return c.errAt(rs, "stores: remotes: want a list")
			}
			for _, r := range rs.Content {
				if r.Kind != yaml.ScalarNode || r.Value == "" {
					return c.errAt(r, "stores: remotes: want a list of strings")
				}
				e.Remotes = append(e.Remotes, r.Value)
			}
		}
		if a := valueOf(it, "account"); a != nil && a.Tag != "!!null" {
			if a.Kind != yaml.ScalarNode || a.Value == "" {
				return c.errAt(a, "stores: account: want the name of an account")
			}
			if !c.hasAccount(a.Value) {
				return c.errAt(a, fmt.Sprintf("stores: account %q is not in accounts", a.Value))
			}
			e.Account = a.Value
		}
		c.Stores = append(c.Stores, e)
	}
	return nil
}

// loadAccounts reads the accounts list. Keys other than name, host, login
// and token are ignored and stay in the file.
func (c *Config) loadAccounts(seq *yaml.Node) error {
	if seq == nil || seq.Tag == "!!null" {
		return nil
	}
	if seq.Kind != yaml.SequenceNode {
		return c.errAt(seq, "accounts: want a list")
	}
	for _, it := range seq.Content {
		if it.Kind != yaml.MappingNode {
			return c.errAt(it, "accounts: want a mapping with name, host and login")
		}
		var a Account
		for _, f := range []struct {
			key string
			dst *string
		}{{"name", &a.Name}, {"host", &a.Host}, {"login", &a.Login}} {
			v := valueOf(it, f.key)
			if v == nil || v.Value == "" {
				return c.errAt(it, fmt.Sprintf("accounts: %s is missing", f.key))
			}
			*f.dst = v.Value
		}
		if v := valueOf(it, "token"); v != nil && v.Tag != "!!null" {
			ts, ok := parseTokenSource(v.Value)
			if !ok {
				return c.errAt(v, fmt.Sprintf("accounts: token %q: want gh, env:<VAR>, keychain or device", v.Value))
			}
			a.Token = ts
		}
		if c.hasAccount(a.Name) {
			return c.errAt(it, fmt.Sprintf("accounts: %q is listed twice", a.Name))
		}
		c.Accounts = append(c.Accounts, a)
	}
	return nil
}

// loadSync reads sync.interval and sync.push_interval over the defaults.
func (c *Config) loadSync(m *yaml.Node) error {
	if m == nil || m.Tag == "!!null" {
		return nil
	}
	if m.Kind != yaml.MappingNode {
		return c.errAt(m, "sync: want a mapping")
	}
	for _, f := range []struct {
		key string
		dst *time.Duration
	}{{"interval", &c.Sync.Interval}, {"push_interval", &c.Sync.PushInterval}} {
		v := valueOf(m, f.key)
		if v == nil {
			continue
		}
		d, err := time.ParseDuration(v.Value)
		if err != nil || d <= 0 {
			return c.errAt(v, fmt.Sprintf("sync: %s: want a positive duration such as 2m", f.key))
		}
		*f.dst = d
	}
	return nil
}

func (c *Config) hasAccount(name string) bool {
	for _, a := range c.Accounts {
		if a.Name == name {
			return true
		}
	}
	return false
}

// Writer returns the account that authors the commits of e: the one e names,
// or the only account when there is just one.
func (c *Config) Writer(e StoreEntry) (Account, bool) {
	for _, a := range c.Accounts {
		if a.Name == e.Account {
			return a, true
		}
	}
	if e.Account == "" && len(c.Accounts) == 1 {
		return c.Accounts[0], true
	}
	return Account{}, false
}

// Me returns the ID of every account.
func (c *Config) Me() []string {
	ids := make([]string, len(c.Accounts))
	for i, a := range c.Accounts {
		ids[i] = a.ID()
	}
	return ids
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
func (c *Config) AddStore(name, path string, remotes []string) error {
	if slices.Contains(remotes, "") {
		return errors.New("a remote must not be empty")
	}
	for _, s := range c.Stores {
		if s.Name == name {
			return fmt.Errorf("a store named %q is already set up", name)
		}
	}
	root := c.root()
	seq := valueOf(root, "stores")
	switch {
	case seq == nil:
		seq = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		root.Content = append(root.Content, scalar("stores"), seq)
	case seq.Tag == "!!null":
		// A bare "stores:" has a null value; put a list in its place.
		*seq = yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	}
	item := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{
		scalar("name"), scalar(name),
		scalar("path"), scalar(c.shorten(path)),
	}}
	if len(remotes) > 0 {
		rs := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Style: yaml.FlowStyle}
		for _, r := range remotes {
			rs.Content = append(rs.Content, scalar(r))
		}
		item.Content = append(item.Content, scalar("remotes"), rs)
	}
	seq.Content = append(seq.Content, item)
	c.Stores = append(c.Stores, StoreEntry{Name: name, Path: path, Remotes: slices.Clone(remotes)})
	if c.DefaultStore == "" {
		return c.SetDefaultStore(name)
	}
	return nil
}

// Account returns the account named name.
func (c *Config) Account(name string) (Account, bool) {
	for _, a := range c.Accounts {
		if a.Name == name {
			return a, true
		}
	}
	return Account{}, false
}

// root returns the top mapping of the document, making one when the file
// is empty.
func (c *Config) root() *yaml.Node {
	if c.doc == nil {
		c.doc = &yaml.Node{Kind: yaml.DocumentNode}
	}
	if len(c.doc.Content) == 0 {
		c.doc.Content = []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}
	}
	return c.doc.Content[0]
}

// setKey sets key of the mapping m to v, adding it at the end when missing.
func setKey(m *yaml.Node, key string, v *yaml.Node) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content[i+1] = v
			return
		}
	}
	m.Content = append(m.Content, scalar(key), v)
}

// deleteKey removes key from the mapping m.
func deleteKey(m *yaml.Node, key string) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content = slices.Delete(m.Content, i, i+2)
			return
		}
	}
}

// AddAccount adds an account to the end of accounts. Save writes it.
func (c *Config) AddAccount(a Account) error {
	if c.hasAccount(a.Name) {
		return fmt.Errorf("an account named %q is already set up", a.Name)
	}
	root := c.root()
	seq := valueOf(root, "accounts")
	switch {
	case seq == nil:
		seq = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		root.Content = append(root.Content, scalar("accounts"), seq)
	case seq.Tag == "!!null":
		*seq = yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	}
	item := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{
		scalar("name"), scalar(a.Name), scalar("host"), scalar(a.Host), scalar("login"), scalar(a.Login),
	}}
	if a.Token.Kind != "" {
		item.Content = append(item.Content, scalar("token"), scalar(a.Token.String()))
	}
	seq.Content = append(seq.Content, item)
	c.Accounts = append(c.Accounts, a)
	return nil
}

// SetAccountToken changes where the token of the account name comes from;
// an empty Kind removes token. Save writes it.
func (c *Config) SetAccountToken(name string, t TokenSource) error {
	i := slices.IndexFunc(c.Accounts, func(a Account) bool { return a.Name == name })
	if i < 0 {
		return fmt.Errorf("no account is named %q", name)
	}
	for _, it := range valueOf(c.root(), "accounts").Content {
		if n := valueOf(it, "name"); n != nil && n.Value == name {
			if t.Kind == "" {
				deleteKey(it, "token")
			} else {
				setKey(it, "token", scalar(t.String()))
			}
		}
	}
	c.Accounts[i].Token = t
	return nil
}

// SetDefaultStore makes name the default store; "" removes the default.
// Save writes it.
func (c *Config) SetDefaultStore(name string) error {
	if name == "" {
		if c.doc != nil && len(c.doc.Content) > 0 {
			deleteKey(c.doc.Content[0], "default_store")
		}
		c.DefaultStore = ""
		return nil
	}
	if !slices.ContainsFunc(c.Stores, func(s StoreEntry) bool { return s.Name == name }) {
		return fmt.Errorf("no store is named %q", name)
	}
	setKey(c.root(), "default_store", scalar(name))
	c.DefaultStore = name
	return nil
}

// SetStoreAccount makes the account the one that authors the commits of the
// store name. Save writes it.
func (c *Config) SetStoreAccount(name, account string) error {
	i := slices.IndexFunc(c.Stores, func(s StoreEntry) bool { return s.Name == name })
	if i < 0 {
		return fmt.Errorf("no store is named %q", name)
	}
	if !c.hasAccount(account) {
		return fmt.Errorf("account %q is not in accounts", account)
	}
	for _, it := range valueOf(c.doc.Content[0], "stores").Content {
		if n := valueOf(it, "name"); n == nil || n.Value != name {
			continue
		}
		if a := valueOf(it, "account"); a != nil {
			*a = *scalar(account)
		} else {
			it.Content = append(it.Content, scalar("account"), scalar(account))
		}
	}
	c.Stores[i].Account = account
	return nil
}

// RemoveStore removes a store from stores, with the comments on its item.
// Save writes it.
func (c *Config) RemoveStore(name string) error {
	i := slices.IndexFunc(c.Stores, func(s StoreEntry) bool { return s.Name == name })
	if i < 0 {
		return fmt.Errorf("no store is named %q", name)
	}
	seq := valueOf(c.doc.Content[0], "stores")
	for j, it := range seq.Content {
		if n := valueOf(it, "name"); n != nil && n.Value == name {
			seq.Content = slices.Delete(seq.Content, j, j+1)
			break
		}
	}
	c.Stores = slices.Delete(c.Stores, i, i+1)
	if c.DefaultStore == name {
		return c.SetDefaultStore("")
	}
	return nil
}

// Save writes config.yaml through a temporary file and a rename.
func (c *Config) Save() error {
	if c.doc == nil || len(c.doc.Content) == 0 {
		return nil
	}
	b, err := yaml.Dump(c.doc, yaml.WithLineWidth(-1), yaml.WithIndent(2), yaml.WithCompactSeqIndent(c.flush))
	if err != nil {
		return err
	}
	file := c.file
	if r, err := filepath.EvalSymlinks(file); err == nil {
		file = r // write the link's target and keep the link
	}
	dir := filepath.Dir(file)
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
		err = os.Rename(tmp, file)
	}
	if err != nil {
		_ = os.Remove(tmp)
	}
	return err
}

// ValidStoreName reports whether name can name a store, by the rule of
// model.ValidName. The name is also the directory of the store.
func ValidStoreName(name string) error { return model.ValidName("store", name) }

func (c *Config) errAt(n *yaml.Node, msg string) error {
	return fmt.Errorf("%s:%d: %s", c.file, n.Line, msg)
}

// expand turns a leading ~/ into the home directory.
func (c *Config) expand(p string) string {
	if rest, ok := strings.CutPrefix(p, "~/"); ok {
		return filepath.Join(c.home, filepath.FromSlash(rest))
	}
	p = filepath.FromSlash(p)
	if !filepath.IsAbs(p) {
		// A relative path is relative to config.yaml, not the working directory.
		p = filepath.Join(filepath.Dir(c.file), p)
	}
	return filepath.Clean(p)
}

// shorten writes p under the home directory as ~/..., with slashes.
func (c *Config) shorten(p string) string {
	if rel, err := filepath.Rel(c.home, p); err == nil && rel != "." && !strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel) {
		return "~/" + filepath.ToSlash(rel)
	}
	return filepath.ToSlash(p)
}

// detectFlush reports whether the file writes block lists flush with their
// key. It looks at stores first, then at the first block list in document
// order; a file without one is not flush.
func detectFlush(root *yaml.Node) bool {
	if k := keyNode(root, "stores"); k != nil {
		if flush, ok := model.ListStyle(k, valueOf(root, "stores")); ok {
			return flush
		}
	}
	flush, _ := model.FirstListStyle(root)
	return flush
}

// keyNode returns the key node of key in the mapping m, or nil.
func keyNode(m *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i]
		}
	}
	return nil
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
