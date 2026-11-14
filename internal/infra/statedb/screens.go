package statedb

import (
	"context"
	"database/sql"
	"errors"
)

func init() {
	register("screens/001", `CREATE TABLE screen_states (
		screen TEXT PRIMARY KEY,
		body   TEXT NOT NULL
	)`)
}

// ScreenStates は、画面の状態(E1-16)。
func (d *DB) ScreenStates() ScreenStates { return ScreenStates{d.db} }

// ScreenStates は、画面ごとの状態の文字列を state.db に置く。中身は見ない。
type ScreenStates struct{ db *sql.DB }

// Get は、保存した文字列を返す。なければ 2 つ目の値が false。
func (s ScreenStates) Get(ctx context.Context, screen string) (string, bool, error) {
	var body string
	err := s.db.QueryRowContext(ctx, `SELECT body FROM screen_states WHERE screen = ?`, screen).Scan(&body)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return body, true, nil
}

// Put は、文字列を置く(あれば上書きする)。
func (s ScreenStates) Put(ctx context.Context, screen, body string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO screen_states (screen, body) VALUES (?, ?)
		ON CONFLICT (screen) DO UPDATE SET body = excluded.body`, screen, body)
	return err
}
