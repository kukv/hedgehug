package statedb

import (
	"cmp"
	"database/sql"
	"errors"
	"fmt"
	"slices"
)

// migration は、表を作る 1 つの手順。名前は「機能/連番」(例 "sync/001")。一度当てた手順は変えない
// (変えるときは、新しい手順を足す)。
type migration struct {
	name string
	sql  string
}

// migrations は、機能ごとのファイルの init で足す。
var migrations []migration

func register(name, stmt string) { migrations = append(migrations, migration{name: name, sql: stmt}) }

// migrate は、まだ当てていない手順を、名前の順に 1 つずつ当てる。
func migrate(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS migrations (name TEXT PRIMARY KEY)`); err != nil {
		return err
	}
	ms := slices.Clone(migrations)
	slices.SortFunc(ms, func(a, b migration) int { return cmp.Compare(a.name, b.name) })
	for _, m := range ms {
		if err := apply(db, m); err != nil {
			return fmt.Errorf("state.db %s: %w", m.name, err)
		}
	}
	return nil
}

func apply(db *sql.DB, m migration) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	var name string
	err = tx.QueryRow(`SELECT name FROM migrations WHERE name = ?`, m.name).Scan(&name)
	switch {
	case err == nil:
		return tx.Rollback()
	case !errors.Is(err, sql.ErrNoRows):
		return errors.Join(err, tx.Rollback())
	}
	if _, err := tx.Exec(m.sql); err != nil {
		return errors.Join(err, tx.Rollback())
	}
	if _, err := tx.Exec(`INSERT INTO migrations (name) VALUES (?)`, m.name); err != nil {
		return errors.Join(err, tx.Rollback())
	}
	return tx.Commit()
}
