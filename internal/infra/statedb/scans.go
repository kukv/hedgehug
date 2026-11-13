package statedb

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
)

func init() {
	register("github/004", `CREATE TABLE github_scans (
		store   TEXT NOT NULL,
		repo    TEXT NOT NULL,
		read    INTEGER NOT NULL,
		since   TEXT NOT NULL,
		failure TEXT NOT NULL,
		PRIMARY KEY (store, repo)
	)`)
	register("github/005", `CREATE TABLE github_app_missing (
		account TEXT NOT NULL,
		org_key TEXT NOT NULL,
		org     TEXT NOT NULL,
		repo    TEXT NOT NULL,
		seq     INTEGER NOT NULL,
		PRIMARY KEY (account, org_key)
	)`)
}

// PRScans は、PR を探した記録(port.PRScans。D4b-4)。
func (d *DB) PRScans() port.PRScans { return prScans{d.db} }

type prScans struct{ db *sql.DB }

func (p prScans) Scans(ctx context.Context, store port.StoreID) (map[string]port.ScanRecord, error) {
	rows, err := p.db.QueryContext(ctx, `SELECT repo, read, since, failure FROM github_scans WHERE store = ?`, string(store))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[string]port.ScanRecord{}
	for rows.Next() {
		var repo, since string
		var r port.ScanRecord
		if err := rows.Scan(&repo, &r.Read, &since, &r.Failure); err != nil {
			return nil, err
		}
		if r.Since, err = parseTime(since); err != nil {
			return nil, err
		}
		out[repo] = r
	}
	return out, rows.Err()
}

// SaveScan は、読んで比べてから書くまでを 1 つのトランザクションで行う。
func (p prScans) SaveScan(ctx context.Context, store port.StoreID, repo string, r port.ScanRecord) (bool, error) {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	changed, err := saveScanIn(ctx, tx, store, strings.ToLower(repo), r)
	if err != nil {
		return false, errors.Join(err, tx.Rollback())
	}
	return changed, tx.Commit()
}

func saveScanIn(ctx context.Context, tx *sql.Tx, store port.StoreID, repo string, r port.ScanRecord) (bool, error) {
	var read bool
	var since, failure string
	err := tx.QueryRowContext(ctx, `SELECT read, since, failure FROM github_scans WHERE store = ? AND repo = ?`,
		string(store), repo).Scan(&read, &since, &failure)
	if err == nil && read == r.Read && since == timeText(r.Since) && failure == r.Failure {
		return false, nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO github_scans (store, repo, read, since, failure) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (store, repo) DO UPDATE SET read = excluded.read, since = excluded.since, failure = excluded.failure`,
		string(store), repo, r.Read, timeText(r.Since), r.Failure)
	return err == nil, err
}

// AppMissing は、GitHub App 未導入の組織の記録(port.AppMissing。D4b-12)。
func (d *DB) AppMissing() port.AppMissing { return appMissing{d.db} }

type appMissing struct{ db *sql.DB }

func (a appMissing) List(ctx context.Context) (map[account.ID][]port.MissingOrg, error) {
	rows, err := a.db.QueryContext(ctx, `SELECT account, org, repo FROM github_app_missing ORDER BY seq`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[account.ID][]port.MissingOrg{}
	for rows.Next() {
		var id string
		var m port.MissingOrg
		if err := rows.Scan(&id, &m.Org, &m.Repo); err != nil {
			return nil, err
		}
		out[account.ID(id)] = append(out[account.ID(id)], m)
	}
	return out, rows.Err()
}

// Put は、同じ組織(小文字)があれば Repo だけ置き換える(順番と表示名は最初のまま)。
func (a appMissing) Put(ctx context.Context, id account.ID, m port.MissingOrg) (bool, error) {
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	changed, err := putMissingIn(ctx, tx, id, m)
	if err != nil {
		return false, errors.Join(err, tx.Rollback())
	}
	return changed, tx.Commit()
}

func putMissingIn(ctx context.Context, tx *sql.Tx, id account.ID, m port.MissingOrg) (bool, error) {
	key := strings.ToLower(m.Org)
	res, err := tx.ExecContext(ctx, `UPDATE github_app_missing SET repo = ? WHERE account = ? AND org_key = ?`, m.Repo, string(id), key)
	if err != nil {
		return false, err
	}
	if n, err := res.RowsAffected(); err != nil || n > 0 {
		return false, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO github_app_missing (account, org_key, org, repo, seq)
		VALUES (?, ?, ?, ?, (SELECT COALESCE(MAX(seq), 0) + 1 FROM github_app_missing))`, string(id), key, m.Org, m.Repo)
	return err == nil, err
}

func (a appMissing) Clear(ctx context.Context, id account.ID, org string) (bool, error) {
	res, err := a.db.ExecContext(ctx, `DELETE FROM github_app_missing WHERE account = ? AND org_key = ?`, string(id), strings.ToLower(org))
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}
