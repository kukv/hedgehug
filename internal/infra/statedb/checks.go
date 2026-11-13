package statedb

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
)

func init() {
	register("github/001", `CREATE TABLE github_checks (
		store  TEXT NOT NULL,
		link   TEXT NOT NULL,
		checks TEXT NOT NULL,
		at     TEXT NOT NULL,
		PRIMARY KEY (store, link)
	)`)
}

// Checks は、このマシンが読んだ PR のチェックの結果(port.Checks。D3-7)。
func (d *DB) Checks() port.Checks { return checks{d.db} }

type checks struct{ db *sql.DB }

func (c checks) Read(ctx context.Context, store port.StoreID) (map[string]itemlink.Checks, error) {
	rows, err := c.db.QueryContext(ctx, `SELECT link, checks FROM github_checks WHERE store = ?`, string(store))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[string]itemlink.Checks{}
	for rows.Next() {
		var link, v string
		if err := rows.Scan(&link, &v); err != nil {
			return nil, err
		}
		out[link] = itemlink.Checks(v)
	}
	return out, rows.Err()
}

func (c checks) Save(ctx context.Context, store port.StoreID, key string, v itemlink.Checks) (bool, error) {
	var old string
	err := c.db.QueryRowContext(ctx, `SELECT checks FROM github_checks WHERE store = ? AND link = ?`, string(store), key).Scan(&old)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		old = ""
	case err != nil:
		return false, err
	}
	if old == string(v) {
		return false, nil
	}
	if v == itemlink.NoChecks {
		_, err = c.db.ExecContext(ctx, `DELETE FROM github_checks WHERE store = ? AND link = ?`, string(store), key)
		return err == nil, err
	}
	_, err = c.db.ExecContext(ctx, `INSERT INTO github_checks (store, link, checks, at) VALUES (?, ?, ?, ?)
		ON CONFLICT (store, link) DO UPDATE SET checks = excluded.checks, at = excluded.at`,
		string(store), key, string(v), timeText(time.Now()))
	return err == nil, err
}
