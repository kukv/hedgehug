// Package statedb は、このマシンのデータ(~/.local/share/hedgehug/state.db、SQLite)を持つ(保存形式「このマシンに持つもの」)。
// 機能ごとに表を分け、表を作る手順は機能のファイルの init で register する。
package statedb

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // database/sql の "sqlite"
)

// DB は state.db。
type DB struct {
	db *sql.DB
}

// Open は、path の state.db を開き(なければフォルダごと作り)、まだ当てていない表を作る手順を当てる。
func Open(path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, err
	}
	// 書き込みは 1 本の接続で順に行う(SQLite は同時に 1 つしか書けない)。
	db.SetMaxOpenConns(1)
	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &DB{db: db}, nil
}

func (d *DB) Close() error { return d.db.Close() }
