package statedb

import (
	"context"
	"database/sql"
	"errors"
	"slices"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

func init() {
	register("judged/001", `CREATE TABLE judged_saves (
		store   TEXT NOT NULL,
		version TEXT NOT NULL,
		op      TEXT NOT NULL,
		author  TEXT NOT NULL,
		at      TEXT NOT NULL,
		PRIMARY KEY (store, version)
	);
	CREATE INDEX judged_by_save ON judged_saves (store, op, author, at)`)
	register("judged/002", "ALTER TABLE judged_saves ADD COLUMN own INTEGER NOT NULL DEFAULT 0")
}

// JudgedSaves は、通知を判定した保存の記録(port.JudgedSaves)。時刻は inboxTime の固定幅で書く。
func (d *DB) JudgedSaves() port.JudgedSaves { return judged{d.db} }

type judged struct{ db *sql.DB }

func (j judged) exists(ctx context.Context, query string, args ...any) (bool, error) {
	var one int
	err := j.db.QueryRowContext(ctx, query, args...).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (j judged) Started(ctx context.Context, store port.StoreID) (bool, error) {
	return j.exists(ctx, `SELECT 1 FROM judged_saves WHERE store = ? LIMIT 1`, string(store))
}

func (j judged) Known(ctx context.Context, store port.StoreID, k port.SaveKey) (port.Match, error) {
	if ok, err := j.exists(ctx, `SELECT 1 FROM judged_saves WHERE store = ? AND version = ?`, string(store), k.Version); err != nil || ok {
		return port.SameVersion, err
	}
	if k.Op == "" {
		return port.NotJudged, nil
	}
	ok, err := j.exists(ctx, `SELECT 1 FROM judged_saves WHERE store = ? AND op = ? AND author = ? AND at = ? LIMIT 1`,
		string(store), string(k.Op), k.Author, inboxTime(k.At))
	if err != nil || !ok {
		return port.NotJudged, err
	}
	return port.SameSave, nil
}

func (j judged) Mark(ctx context.Context, store port.StoreID, keys []port.SaveKey) error {
	return j.insert(ctx, store, keys, `INSERT OR IGNORE INTO judged_saves (store, version, op, author, at) VALUES (?, ?, ?, ?, ?)`)
}

// MarkOwn は、Mark と同じく書き、すでにある記録も own = 1 にする(決定 C4c1-4)。
func (j judged) MarkOwn(ctx context.Context, store port.StoreID, keys []port.SaveKey) error {
	return j.insert(ctx, store, keys, `INSERT INTO judged_saves (store, version, op, author, at, own) VALUES (?, ?, ?, ?, ?, 1)
		ON CONFLICT (store, version) DO UPDATE SET own = 1`)
}

// insert は、keys のそれぞれを query で 1 回の書き込みにまとめて書く。
func (j judged) insert(ctx context.Context, store port.StoreID, keys []port.SaveKey, query string) error {
	tx, err := j.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, k := range keys {
		if _, err := tx.ExecContext(ctx, query, string(store), k.Version, string(k.Op), k.Author, inboxTime(k.At)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (j judged) Own(ctx context.Context, store port.StoreID, op change.ID) (bool, error) {
	if op == "" {
		return false, nil
	}
	return j.exists(ctx, `SELECT 1 FROM judged_saves WHERE store = ? AND op = ? AND own = 1 LIMIT 1`, string(store), string(op))
}

// Received は、ops のうち、同期で受け取った保存のもの(記録があり、同じ op にこのマシンの保存の記録がない。決定 C4d-16)
// を true にして返す。空の op は数えない。
func (d *DB) Received(ctx context.Context, store port.StoreID, ops []change.ID) (map[change.ID]bool, error) {
	out := map[change.ID]bool{}
	ops = slices.DeleteFunc(slices.Clone(ops), func(op change.ID) bool { return op == "" })
	if len(ops) == 0 {
		return out, nil
	}
	marks, args := inClause(ops)
	rows, err := d.db.QueryContext(ctx, `SELECT op FROM judged_saves WHERE store = ? AND op IN `+marks+` GROUP BY op HAVING MAX(own) = 0`, //nolint:gosec // G202: つなぐのは ? だけの IN 句。値は args で渡す
		append([]any{string(store)}, args...)...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var op string
		if err := rows.Scan(&op); err != nil {
			return nil, err
		}
		out[change.ID(op)] = true
	}
	return out, rows.Err()
}

func (j judged) ForgetStore(ctx context.Context, store port.StoreID) error {
	_, err := j.db.ExecContext(ctx, `DELETE FROM judged_saves WHERE store = ?`, string(store))
	return err
}
