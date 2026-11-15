package statedb

import (
	"context"
	"database/sql"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
)

func init() {
	// id は AUTOINCREMENT にして、ForgetStore で消したあとも番号を使い回さない(アクティビティの行の鍵 sync:{id})。
	register("sync/003", `CREATE TABLE sync_log (
		id       INTEGER PRIMARY KEY AUTOINCREMENT,
		store    TEXT NOT NULL,
		at       TEXT NOT NULL,
		sent     INTEGER NOT NULL,
		received INTEGER NOT NULL
	);
	CREATE INDEX sync_log_by_store ON sync_log (store, at)`)
}

// SyncLog は、同期の行の記録(port.SyncLog。決定 C4d-13)。並べ替えは文字列で行うので、時刻は inboxTime の固定幅で書く。
func (d *DB) SyncLog() port.SyncLog { return syncLog{d.db} }

type syncLog struct{ db *sql.DB }

func (l syncLog) Add(ctx context.Context, store port.StoreID, at time.Time, sent, received int) error {
	if sent == 0 && received == 0 {
		return nil
	}
	_, err := l.db.ExecContext(ctx, `INSERT INTO sync_log (store, at, sent, received) VALUES (?, ?, ?, ?)`,
		string(store), inboxTime(at), sent, received)
	return err
}

func (l syncLog) List(ctx context.Context, store port.StoreID) ([]port.SyncLine, error) {
	rows, err := l.db.QueryContext(ctx, `SELECT id, at, sent, received FROM sync_log WHERE store = ? ORDER BY at DESC, id DESC`, string(store))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []port.SyncLine{}
	for rows.Next() {
		line := port.SyncLine{Store: store}
		var at string
		if err := rows.Scan(&line.ID, &at, &line.Sent, &line.Received); err != nil {
			return nil, err
		}
		if line.At, err = time.Parse(inboxTimeLayout, at); err != nil {
			return nil, err
		}
		out = append(out, line)
	}
	return out, rows.Err()
}

func (l syncLog) ForgetStore(ctx context.Context, store port.StoreID) error {
	_, err := l.db.ExecContext(ctx, `DELETE FROM sync_log WHERE store = ?`, string(store))
	return err
}
