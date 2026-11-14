package statedb

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

func init() {
	register("palette/001", `CREATE TABLE recent_items (
		item  TEXT PRIMARY KEY,
		store TEXT NOT NULL,
		at    TEXT NOT NULL
	)`)
}

// Recent は、このマシンで最近開いたアイテム(port.RecentItems。A5c-18)。
func (d *DB) Recent() port.RecentItems { return recent{d.db} }

type recent struct{ db *sql.DB }

// Touch は、記録を置き換えたあと、新しい順で keep 件を超えた行を消す(1 つのトランザクション)。
// 時刻の文字列は桁数が揃わず、文字の順では新しさにならないので、並べるのは時刻にしてから。
func (r recent) Touch(ctx context.Context, store port.StoreID, item change.ID, at time.Time, keep int) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := touchIn(ctx, tx, store, item, at, keep); err != nil {
		return errors.Join(err, tx.Rollback())
	}
	return tx.Commit()
}

func touchIn(ctx context.Context, tx *sql.Tx, store port.StoreID, item change.ID, at time.Time, keep int) error {
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO recent_items (item, store, at) VALUES (?, ?, ?)
		 ON CONFLICT(item) DO UPDATE SET store = excluded.store, at = excluded.at`,
		string(item), string(store), timeText(at)); err != nil {
		return err
	}
	all, err := listIn(ctx, tx)
	if err != nil {
		return err
	}
	for _, old := range all[min(keep, len(all)):] {
		if _, err := tx.ExecContext(ctx, `DELETE FROM recent_items WHERE item = ?`, string(old.Item)); err != nil {
			return err
		}
	}
	return nil
}

func (r recent) List(ctx context.Context) ([]port.Recent, error) { return listIn(ctx, r.db) }

type querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func listIn(ctx context.Context, q querier) ([]port.Recent, error) {
	rows, err := q.QueryContext(ctx, `SELECT store, item, at FROM recent_items`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []port.Recent
	for rows.Next() {
		var store, item, at string
		if err := rows.Scan(&store, &item, &at); err != nil {
			return nil, err
		}
		t, err := parseTime(at)
		if err != nil {
			return nil, err
		}
		out = append(out, port.Recent{Store: port.StoreID(store), Item: change.ID(item), At: t})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].At.Equal(out[j].At) {
			return out[i].At.After(out[j].At)
		}
		return out[i].Item < out[j].Item
	})
	return out, nil
}

func (r recent) Forget(ctx context.Context, items []change.ID) error {
	if len(items) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	for _, item := range items {
		if _, err := tx.ExecContext(ctx, `DELETE FROM recent_items WHERE item = ?`, string(item)); err != nil {
			return errors.Join(err, tx.Rollback())
		}
	}
	return tx.Commit()
}
