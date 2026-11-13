package statedb

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/ghread"
)

func init() {
	register("github/002", `CREATE TABLE github_reads (
		store      TEXT NOT NULL,
		repo       TEXT NOT NULL,
		attempt    TEXT NOT NULL,
		failed     INTEGER NOT NULL,
		rate_limit INTEGER NOT NULL,
		remaining  INTEGER NOT NULL,
		reset_at   TEXT NOT NULL,
		not_before TEXT NOT NULL,
		PRIMARY KEY (store, repo)
	)`)
	register("github/003", `CREATE TABLE github_link_failures (
		store TEXT NOT NULL,
		link  TEXT NOT NULL,
		text  TEXT NOT NULL,
		PRIMARY KEY (store, link)
	)`)
}

// GitHubReads は、GitHub を読んだ記録と、取り直しの失敗の文言(port.GitHubReads。D4a-5・D4a-11)。
func (d *DB) GitHubReads() port.GitHubReads { return githubReads{d.db} }

type githubReads struct{ db *sql.DB }

func (g githubReads) Records(ctx context.Context, store port.StoreID) (map[string]ghread.Record, error) {
	rows, err := g.db.QueryContext(ctx, `SELECT repo, attempt, failed, rate_limit, remaining, reset_at, not_before
		FROM github_reads WHERE store = ?`, string(store))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[string]ghread.Record{}
	for rows.Next() {
		var repo, attempt, resetAt, notBefore string
		var r ghread.Record
		if err := rows.Scan(&repo, &attempt, &r.Failed, &r.Limit, &r.Remaining, &resetAt, &notBefore); err != nil {
			return nil, err
		}
		for _, f := range []struct {
			text string
			to   *time.Time
		}{{attempt, &r.Attempt}, {resetAt, &r.ResetAt}, {notBefore, &r.NotBefore}} {
			if *f.to, err = parseTime(f.text); err != nil {
				return nil, err
			}
		}
		out[repo] = r
	}
	return out, rows.Err()
}

func (g githubReads) SaveRecord(ctx context.Context, store port.StoreID, repo string, r ghread.Record) error {
	_, err := g.db.ExecContext(ctx, `INSERT INTO github_reads
		(store, repo, attempt, failed, rate_limit, remaining, reset_at, not_before) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (store, repo) DO UPDATE SET attempt = excluded.attempt, failed = excluded.failed,
		rate_limit = excluded.rate_limit, remaining = excluded.remaining,
		reset_at = excluded.reset_at, not_before = excluded.not_before`,
		string(store), strings.ToLower(repo), timeText(r.Attempt), r.Failed, r.Limit, r.Remaining,
		timeText(r.ResetAt), timeText(r.NotBefore))
	return err
}

func (g githubReads) Failures(ctx context.Context, store port.StoreID) (map[string]string, error) {
	rows, err := g.db.QueryContext(ctx, `SELECT link, text FROM github_link_failures WHERE store = ?`, string(store))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[string]string{}
	for rows.Next() {
		var link, text string
		if err := rows.Scan(&link, &text); err != nil {
			return nil, err
		}
		out[link] = text
	}
	return out, rows.Err()
}

// SetFailure は、読んで比べてから書くまでを 1 つのトランザクションで行う(Checks.Save と同じ)。
func (g githubReads) SetFailure(ctx context.Context, store port.StoreID, key, text string) (bool, error) {
	tx, err := g.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	changed, err := setFailureIn(ctx, tx, store, key, text)
	if err != nil {
		return false, errors.Join(err, tx.Rollback())
	}
	return changed, tx.Commit()
}

func setFailureIn(ctx context.Context, tx *sql.Tx, store port.StoreID, key, text string) (bool, error) {
	var old string
	err := tx.QueryRowContext(ctx, `SELECT text FROM github_link_failures WHERE store = ? AND link = ?`, string(store), key).Scan(&old)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		old = ""
	case err != nil:
		return false, err
	}
	if old == text {
		return false, nil
	}
	if text == "" {
		_, err = tx.ExecContext(ctx, `DELETE FROM github_link_failures WHERE store = ? AND link = ?`, string(store), key)
		return err == nil, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO github_link_failures (store, link, text) VALUES (?, ?, ?)
		ON CONFLICT (store, link) DO UPDATE SET text = excluded.text`, string(store), key, text)
	return err == nil, err
}
