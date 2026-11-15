package statedb

import (
	"context"
	"database/sql"
	"errors"

	"github.com/kukv/hedgehug/internal/app/port"
)

func init() {
	register("sync/004", `CREATE TABLE clock_skew (
		id      INTEGER PRIMARY KEY CHECK (id = 1),
		minutes INTEGER NOT NULL
	)`)
}

// ClockSkews は、時計のずれの警告の分(06「時計のずれの警告」)。
func (d *DB) ClockSkews() port.ClockSkews { return clockSkews{d.db} }

type clockSkews struct{ db *sql.DB }

// Skew は、覚えた分を返す。覚えていなければ 0。
func (s clockSkews) Skew(ctx context.Context) (int, error) {
	var minutes int
	err := s.db.QueryRowContext(ctx, `SELECT minutes FROM clock_skew WHERE id = 1`).Scan(&minutes)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return minutes, err
}

// SetSkew は、分を置く(あれば上書きする)。
func (s clockSkews) SetSkew(ctx context.Context, minutes int) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO clock_skew (id, minutes) VALUES (1, ?)
		ON CONFLICT (id) DO UPDATE SET minutes = excluded.minutes`, minutes)
	return err
}
