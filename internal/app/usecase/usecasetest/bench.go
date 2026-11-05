package usecasetest

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/storage/config"
	"github.com/kukv/hedgehug/internal/app/storage/history/historytest"
	"github.com/kukv/hedgehug/internal/app/usecase"
)

// BenchSession is a session on the store at dir, as the account (by name: one
// of historytest.Accounts, or dave, who wrote nothing), at a time after every
// commit of the synthesized store of the benchmarks.
func BenchSession(b *testing.B, dir, account string) *usecase.Session {
	b.Helper()
	home := b.TempDir()
	env := map[string]string{"HOME": home, "HEDGEHUG_HOME": filepath.Join(home, "hh"), "HEDGEHUG_AUTO_SYNC": "0"}
	now := historytest.Start.Add(2 * historytest.BenchCommits * time.Minute)
	ss := &usecase.Session{
		Getenv: func(k string) string { return env[k] },
		Now:    func() time.Time { return now },
		Warn:   func(string) {},
	}
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
	return ss
}
