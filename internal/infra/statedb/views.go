package statedb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/kukv/hedgehug/internal/domain/savedview"
)

func init() {
	register("views/001", `CREATE TABLE views (
		id   TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		body TEXT NOT NULL
	)`)
}

// Views は、保存したビュー(E3b)。
func (d *DB) Views() Views { return Views{d.db} }

// Views は、保存したビューを state.db に置く(app/views.Store)。body は内容の JSON。
type Views struct{ db *sql.DB }

// List は、すべてのビューを返す(並びは決まっていない)。
func (v Views) List(ctx context.Context) ([]savedview.View, error) {
	rows, err := v.db.QueryContext(ctx, `SELECT id, name, body FROM views`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []savedview.View
	for rows.Next() {
		var id, name, body string
		if err := rows.Scan(&id, &name, &body); err != nil {
			return nil, err
		}
		view, err := decodeView(id, name, body)
		if err != nil {
			return nil, err
		}
		out = append(out, view)
	}
	return out, rows.Err()
}

// Get は、id のビューを返す。なければ 2 つ目の値が false。
func (v Views) Get(ctx context.Context, id string) (savedview.View, bool, error) {
	var name, body string
	err := v.db.QueryRowContext(ctx, `SELECT name, body FROM views WHERE id = ?`, id).Scan(&name, &body)
	if errors.Is(err, sql.ErrNoRows) {
		return savedview.View{}, false, nil
	}
	if err != nil {
		return savedview.View{}, false, err
	}
	view, err := decodeView(id, name, body)
	return view, err == nil, err
}

// Put は、ビューを置く(あれば上書きする)。
func (v Views) Put(ctx context.Context, view savedview.View) error {
	body, err := json.Marshal(view.Content)
	if err != nil {
		return err
	}
	_, err = v.db.ExecContext(ctx, `INSERT INTO views (id, name, body) VALUES (?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET name = excluded.name, body = excluded.body`,
		view.ID, view.Name, string(body))
	return err
}

// Delete は、ビューと、その画面の状態 view:{id} の行を 1 つのトランザクションで消す。
func (v Views) Delete(ctx context.Context, id string) error {
	tx, err := v.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM views WHERE id = ?`, id); err != nil {
		return errors.Join(err, tx.Rollback())
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM screen_states WHERE screen = 'view:' || ?`, id); err != nil {
		return errors.Join(err, tx.Rollback())
	}
	return tx.Commit()
}

func decodeView(id, name, body string) (savedview.View, error) {
	view := savedview.View{ID: id, Name: name}
	if err := json.Unmarshal([]byte(body), &view.Content); err != nil {
		return savedview.View{}, err
	}
	return view, nil
}
