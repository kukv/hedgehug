package statedb

import (
	"context"
	"errors"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
)

func init() {
	register("github/006", `CREATE TABLE github_renames (
		store    TEXT NOT NULL,
		repo     TEXT NOT NULL,
		moved_to TEXT NOT NULL,
		PRIMARY KEY (store, repo)
	)`)
}

func (p prScans) Renames(ctx context.Context, store port.StoreID) (map[string]string, error) {
	rows, err := p.db.QueryContext(ctx, `SELECT repo, moved_to FROM github_renames WHERE store = ?`, string(store))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[string]string{}
	for rows.Next() {
		var from, to string
		if err := rows.Scan(&from, &to); err != nil {
			return nil, err
		}
		out[from] = to
	}
	return out, rows.Err()
}

// SaveRename は、to から先の対応を消してから from → to を置くまでを 1 つのトランザクションで行う。
func (p prScans) SaveRename(ctx context.Context, store port.StoreID, from, to string) error {
	from, to = strings.ToLower(from), strings.ToLower(to)
	if from == to {
		return nil
	}
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM github_renames WHERE store = ? AND repo = ?`, string(store), to); err != nil {
		return errors.Join(err, tx.Rollback())
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO github_renames (store, repo, moved_to) VALUES (?, ?, ?)
		ON CONFLICT (store, repo) DO UPDATE SET moved_to = excluded.moved_to`, string(store), from, to)
	if err != nil {
		return errors.Join(err, tx.Rollback())
	}
	return tx.Commit()
}
