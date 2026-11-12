package statedb

import (
	"path/filepath"
	"testing"
)

// 表を作る手順は、開くたびに当てても 1 度しか当たらない。
func TestMigrationsRunOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	for range 2 {
		d, err := Open(path)
		if err != nil {
			t.Fatal(err)
		}
		var n int
		if err := d.db.QueryRow(`SELECT count(*) FROM migrations`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != len(migrations) {
			t.Errorf("migrations = %d, want %d", n, len(migrations))
		}
		if err := d.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

// 手順は名前の順に当てる(登録の順ではない)。
func TestMigrationsAreSortedByName(t *testing.T) {
	saved := migrations
	t.Cleanup(func() { migrations = saved })
	migrations = nil
	register("b/001", `CREATE TABLE b (x TEXT)`)
	register("a/001", `CREATE TABLE a (x TEXT)`)
	register("a/002", `ALTER TABLE a ADD COLUMN y TEXT`)
	d, err := Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = d.Close() }()
	if _, err := d.db.Exec(`INSERT INTO a (x, y) VALUES ('1', '2')`); err != nil {
		t.Errorf("a.y missing: %v", err)
	}
}
