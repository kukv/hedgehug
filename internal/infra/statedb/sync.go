package statedb

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/syncstatus"
)

func init() {
	register("sync/001", `CREATE TABLE sync_status (
		store        TEXT PRIMARY KEY,
		last_attempt TEXT NOT NULL DEFAULT '',
		last_success TEXT NOT NULL DEFAULT '',
		reason       TEXT NOT NULL DEFAULT '',
		remote_key   TEXT NOT NULL DEFAULT '',
		seen         TEXT NOT NULL DEFAULT ''
	)`)
	register("sync/002", `CREATE TABLE mirror_status (
		store  TEXT NOT NULL,
		mirror TEXT NOT NULL,
		at     TEXT NOT NULL,
		reason TEXT NOT NULL DEFAULT '',
		PRIMARY KEY (store, mirror)
	)`)
}

// SyncState は、同期の状態(port.SyncState)。
func (d *DB) SyncState() port.SyncState { return syncState{d.db} }

type syncState struct{ db *sql.DB }

// 時刻は UTC の RFC 3339(ナノ秒まで)で書く。ゼロは空。
func timeText(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func parseTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339Nano, s)
}

func (s syncState) Records(ctx context.Context) ([]port.SyncRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT store, last_attempt, last_success, reason FROM sync_status ORDER BY store`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []port.SyncRecord{}
	for rows.Next() {
		var store, attempt, success, reason string
		if err := rows.Scan(&store, &attempt, &success, &reason); err != nil {
			return nil, err
		}
		r := port.SyncRecord{Store: port.StoreID(store), Reason: syncstatus.Reason(reason)}
		if r.LastAttempt, err = parseTime(attempt); err != nil {
			return nil, err
		}
		if r.LastSuccess, err = parseTime(success); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s syncState) Failed(ctx context.Context, store port.StoreID, at time.Time, reason syncstatus.Reason) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO sync_status (store, last_attempt, reason) VALUES (?, ?, ?)
		ON CONFLICT (store) DO UPDATE SET last_attempt = excluded.last_attempt, reason = excluded.reason`,
		string(store), timeText(at), string(reason))
	return err
}

func (s syncState) Succeeded(ctx context.Context, store port.StoreID, at time.Time, remoteKey, seen string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO sync_status (store, last_attempt, last_success, reason, remote_key, seen)
		VALUES (?, ?, ?, '', ?, ?)
		ON CONFLICT (store) DO UPDATE SET last_attempt = excluded.last_attempt, last_success = excluded.last_success,
			reason = '', remote_key = excluded.remote_key, seen = excluded.seen`,
		string(store), timeText(at), timeText(at), remoteKey, seen)
	return err
}

func (s syncState) Seen(ctx context.Context, store port.StoreID, remoteKey string) (string, error) {
	var key, seen string
	err := s.db.QueryRowContext(ctx, `SELECT remote_key, seen FROM sync_status WHERE store = ?`, string(store)).Scan(&key, &seen)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return "", nil
	case err != nil:
		return "", err
	case key != remoteKey:
		return "", nil
	}
	return seen, nil
}

func (s syncState) Mirrored(ctx context.Context, store port.StoreID, mirror string, at time.Time, reason syncstatus.Reason) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO mirror_status (store, mirror, at, reason) VALUES (?, ?, ?, ?)
		ON CONFLICT (store, mirror) DO UPDATE SET at = excluded.at, reason = excluded.reason`,
		string(store), mirror, timeText(at), string(reason))
	return err
}

func (s syncState) Mirrors(ctx context.Context, store port.StoreID) ([]port.MirrorRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT mirror, at, reason FROM mirror_status WHERE store = ? ORDER BY mirror`, string(store))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []port.MirrorRecord{}
	for rows.Next() {
		var m, at, reason string
		if err := rows.Scan(&m, &at, &reason); err != nil {
			return nil, err
		}
		t, err := parseTime(at)
		if err != nil {
			return nil, err
		}
		out = append(out, port.MirrorRecord{Mirror: m, At: t, Reason: syncstatus.Reason(reason)})
	}
	return out, rows.Err()
}
