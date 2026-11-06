package usecasetest

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/config"
	"github.com/kukv/hedgehug/internal/app/storage/history/historytest"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
)

// BenchSession is a session on the store at dir, as the account (by name: one
// of historytest.Accounts, or dave, who wrote nothing), at a time after every
// commit of the synthesized store of the benchmarks.
func BenchSession(b *testing.B, dir, account string) *usecase.Session {
	b.Helper()
	return benchSession(BenchEnv(b, dir, account))
}

func benchSession(env map[string]string) *usecase.Session {
	now := historytest.Start.Add(2 * historytest.BenchCommits * time.Minute)
	return &usecase.Session{
		Getenv: func(k string) string { return env[k] },
		Now:    func() time.Time { return now },
		Warn:   func(string) {},
	}
}

// BenchEnv is the environment of a new home with the store at dir mounted and
// the account (as in BenchSession) in its config.
func BenchEnv(b *testing.B, dir, account string) map[string]string {
	b.Helper()
	home := b.TempDir()
	env := map[string]string{"HOME": home, "HEDGEHUG_HOME": filepath.Join(home, "hh"), "HEDGEHUG_AUTO_SYNC": "0"}
	ss := benchSession(env)
	c, _, err := ss.Config()
	if err != nil {
		b.Fatal(err)
	}
	if err := c.AddStore("me", dir, nil); err != nil {
		b.Fatal(err)
	}
	if err := c.Save(); err != nil {
		b.Fatal(err)
	}
	var acct config.Account
	for _, a := range append(historytest.Accounts, config.Account{Name: "dave", Host: "github.com", Login: "dave"}) {
		if a.Name == account {
			acct = a
		}
	}
	f, err := os.OpenFile(filepath.Join(env["HEDGEHUG_HOME"], "config.yaml"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) //nolint:gosec // the config of the benchmark
	if err != nil {
		b.Fatal(err)
	}
	_, err = f.WriteString("accounts:\n  - name: " + acct.Name + "\n    host: " + acct.Host + "\n    login: " + acct.Login + "\n")
	if err := errors.Join(err, f.Close()); err != nil {
		b.Fatal(err)
	}
	return env
}

// LsCase is a situation in which a command lists the tasks: Prepare changes
// the store at dir as what happened before the command.
type LsCase struct {
	Name    string
	Args    []string // of hedgehug
	Tree    bool     // the args are ls --tree
	Prepare func(b *testing.B, dir string)
}

// LsCases are the situations of the targets of phase 7a (the spec of phase 7).
func LsCases() []LsCase {
	return []LsCase{
		{Name: "cached", Args: []string{"ls"}, Prepare: func(*testing.B, string) {}},
		{Name: "tree", Args: []string{"ls", "--tree"}, Tree: true, Prepare: func(*testing.B, string) {}},
		{Name: "afterCommit", Args: []string{"ls"}, Prepare: commitTitle},
		{Name: "afterReparent", Args: []string{"ls"}, Prepare: commitReparent},
		{Name: "schema", Args: []string{"ls"}, Prepare: commitSchema},
		{Name: "rebuild", Args: []string{"ls"}, Prepare: removeIndex},
	}
}

// commit writes one commit, as a sync brings one: the task index stays behind.
func commit(b *testing.B, dir string, op store.Op, fn func(s *store.Store, tx *store.Tx) error) {
	b.Helper()
	ss := BenchSession(b, dir, "alice")
	s, err := ss.OpenStore()
	if err != nil {
		b.Fatal(err)
	}
	if _, err := ss.Update(s, op, func(tx *store.Tx) error { return fn(s, tx) }); err != nil {
		b.Fatal(err)
	}
	_ = s.Repo().Close()
}

func storeTasks(b *testing.B, dir string) []*model.Task {
	b.Helper()
	s, err := store.Open(dir)
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = s.Repo().Close() }()
	var out []*model.Task
	for t, err := range s.Tasks() {
		if err != nil {
			b.Fatal(err)
		}
		out = append(out, t)
	}
	return out
}

func commitTitle(b *testing.B, dir string) {
	b.Helper()
	t := storeTasks(b, dir)[0]
	commit(b, dir, store.Op{Name: "edit", Task: t.ULID}, func(_ *store.Store, tx *store.Tx) error {
		t.Title += " (changed)"
		return tx.PutTask(t)
	})
}

// commitReparent moves a project with its children, about 100, under another
// project, as an epic (a project stays at the root): the index checks the
// descendants of what changed again.
func commitReparent(b *testing.B, dir string) {
	b.Helper()
	var projects []*model.Task
	for _, t := range storeTasks(b, dir) {
		if t.Kind == "project" {
			projects = append(projects, t)
		}
	}
	t, parent := projects[1], projects[0]
	commit(b, dir, store.Op{Name: "mv", Task: t.ULID}, func(_ *store.Store, tx *store.Tx) error {
		t.Kind, t.Parent = "epic", &parent.ULID
		return tx.PutTask(t)
	})
}

func commitSchema(b *testing.B, dir string) {
	b.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "schema.yaml")) //nolint:gosec // a store of the benchmark
	if err != nil {
		b.Fatal(err)
	}
	commit(b, dir, store.Op{Name: "schema"}, func(_ *store.Store, tx *store.Tx) error {
		return tx.PutFile("schema.yaml", append(data, "# changed\n"...))
	})
}

func removeIndex(b *testing.B, dir string) {
	b.Helper()
	if err := os.Remove(filepath.Join(dir, ".hedgehug", "local", "index")); err != nil {
		b.Fatal(err)
	}
}
