package statedb

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

func init() {
	register("inbox/001", `CREATE TABLE inbox (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		store       TEXT NOT NULL,
		workspace   TEXT NOT NULL,
		kind        TEXT NOT NULL,
		item        TEXT NOT NULL,
		rule        TEXT NOT NULL,
		text        TEXT NOT NULL,
		reason      TEXT NOT NULL,
		status      TEXT NOT NULL,
		from_status TEXT NOT NULL,
		display     TEXT NOT NULL,
		sender      TEXT NOT NULL,
		at          TEXT NOT NULL,
		read        INTEGER NOT NULL DEFAULT 0,
		result      TEXT NOT NULL DEFAULT ''
	);
	CREATE INDEX inbox_by_at ON inbox (store, at, id)`)
	register("inbox/002", `ALTER TABLE inbox ADD COLUMN project TEXT NOT NULL DEFAULT '';
	ALTER TABLE inbox ADD COLUMN source TEXT NOT NULL DEFAULT '';
	CREATE INDEX inbox_by_source ON inbox (store, source)`)
}

// Inbox は、このマシンの受信箱(port.InboxBook)。時刻は Add のときに clock で刻む。
func (d *DB) Inbox(clock port.Clock) port.InboxBook { return inbox{d.db, clock} }

type inbox struct {
	db    *sql.DB
	clock port.Clock
}

// 並べ替えと Before の比較は文字列で行うので、at は桁が揃った UTC で書く(timeText は末尾の 0 を削るので使えない)。
const inboxTimeLayout = "2006-01-02T15:04:05.000000000Z"

func inboxTime(t time.Time) string { return t.UTC().Format(inboxTimeLayout) }

const inboxColumns = `id, store, workspace, project, kind, item, rule, text, reason, status, from_status, display, sender, source, at, read, result`

func (b inbox) Add(ctx context.Context, n port.Notice) error {
	return b.insert(ctx, n, b.clock.Now())
}

func (b inbox) insert(ctx context.Context, n port.Notice, at time.Time) error {
	_, err := b.db.ExecContext(ctx, `INSERT INTO inbox
		(store, workspace, project, kind, item, rule, text, reason, status, from_status, display, sender, source, at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		string(n.Store), string(n.Workspace), string(n.Project), string(n.Kind), string(n.Item), string(n.Rule), n.Text, n.Reason,
		string(n.Status), string(n.From), n.Display, n.Sender, n.Source, inboxTime(at))
	return err
}

// Mention は、同じ Store・Source の行があれば本文・差出人・時刻を書き換えて未読に戻す。なければ足す。
// 結果は触らない。
func (b inbox) Mention(ctx context.Context, n port.Notice) error {
	if n.Source == "" {
		return port.ErrInvalidOperation
	}
	res, err := b.db.ExecContext(ctx, `UPDATE inbox SET text = ?, sender = ?, at = ?, read = 0 WHERE store = ? AND source = ?`,
		n.Text, n.Sender, inboxTime(n.At), string(n.Store), n.Source)
	if err != nil {
		return err
	}
	if c, err := res.RowsAffected(); err != nil || c > 0 {
		return err
	}
	return b.insert(ctx, n, n.At)
}

// inClause は、件数ぶんの ? と値を返す。
func inClause[T ~string](vs []T) (string, []any) {
	args := make([]any, len(vs))
	for i, v := range vs {
		args[i] = string(v)
	}
	return "(" + strings.TrimSuffix(strings.Repeat("?,", len(vs)), ",") + ")", args
}

// inboxWhere は、Stores(空なら ok が false)と Kinds(空はすべて)の条件を返す。
func inboxWhere(stores []port.StoreID, kinds []port.NoticeKind) (cond string, args []any, ok bool) {
	if len(stores) == 0 {
		return "", nil, false
	}
	marks, args := inClause(stores)
	cond = "store IN " + marks
	if len(kinds) > 0 {
		km, ka := inClause(kinds)
		cond += " AND kind IN " + km
		args = append(args, ka...)
	}
	return cond, args, true
}

func (b inbox) List(ctx context.Context, q port.NoticeQuery) ([]port.NoticeRow, error) {
	cond, args, ok := inboxWhere(q.Stores, q.Kinds)
	if !ok {
		return nil, nil
	}
	if q.Before != nil {
		at := inboxTime(q.Before.At)
		cond += " AND (at < ? OR (at = ? AND id < ?))"
		args = append(args, at, at, int64(q.Before.ID))
	}
	stmt := `SELECT ` + inboxColumns + ` FROM inbox WHERE ` + cond + ` ORDER BY at DESC, id DESC` //nolint:gosec // G202: つなぐのは固定の列名と、? だけの IN 句。値は args で渡す
	if q.Limit > 0 {
		stmt += " LIMIT ?"
		args = append(args, q.Limit)
	}
	rows, err := b.db.QueryContext(ctx, stmt, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []port.NoticeRow
	for rows.Next() {
		r, err := scanNotice(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func scanNotice(s interface{ Scan(...any) error }) (port.NoticeRow, error) {
	var r port.NoticeRow
	var store, workspace, project, kind, itemID, rule, status, from, result, at string
	var read int
	err := s.Scan(&r.ID, &store, &workspace, &project, &kind, &itemID, &rule, &r.Text, &r.Reason, &status, &from, &r.Display, &r.Sender, &r.Source, &at, &read, &result)
	if err != nil {
		return r, err
	}
	r.Store, r.Workspace, r.Kind, r.Item, r.Rule = port.StoreID(store), change.ID(workspace), port.NoticeKind(kind), change.ID(itemID), change.ID(rule)
	r.Project = change.ID(project)
	r.Status, r.From, r.Read, r.Result = change.ID(status), change.ID(from), read != 0, port.NoticeResult(result)
	r.At, err = parseTime(at)
	return r, err
}

func (b inbox) Unread(ctx context.Context, stores []port.StoreID) (map[port.NoticeKind]int, error) {
	out := map[port.NoticeKind]int{}
	cond, args, ok := inboxWhere(stores, nil)
	if !ok {
		return out, nil
	}
	rows, err := b.db.QueryContext(ctx, `SELECT kind, COUNT(*) FROM inbox WHERE `+cond+` AND read = 0 GROUP BY kind`, args...) //nolint:gosec // G202: つなぐのは ? だけの条件。値は args で渡す
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var kind string
		var n int
		if err := rows.Scan(&kind, &n); err != nil {
			return nil, err
		}
		out[port.NoticeKind(kind)] = n
	}
	return out, rows.Err()
}

func (b inbox) Get(ctx context.Context, id port.NoticeID) (port.NoticeRow, error) {
	r, err := scanNotice(b.db.QueryRowContext(ctx, `SELECT `+inboxColumns+` FROM inbox WHERE id = ?`, int64(id)))
	if errors.Is(err, sql.ErrNoRows) {
		return port.NoticeRow{}, port.ErrNotFound
	}
	return r, err
}

func (b inbox) SetRead(ctx context.Context, id port.NoticeID, read bool) error {
	res, err := b.db.ExecContext(ctx, `UPDATE inbox SET read = ? WHERE id = ?`, read, int64(id))
	return foundOne(res, err)
}

func foundOne(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil || n == 0 {
		return errors.Join(err, port.ErrNotFound)
	}
	return nil
}

func (b inbox) ReadAll(ctx context.Context, stores []port.StoreID, kinds []port.NoticeKind) error {
	cond, args, ok := inboxWhere(stores, kinds)
	if !ok {
		return nil
	}
	_, err := b.db.ExecContext(ctx, `UPDATE inbox SET read = 1 WHERE `+cond, args...) //nolint:gosec // G202: つなぐのは ? だけの条件。値は args で渡す
	return err
}

// Resolve は、結果がまだ空の行にだけ書く。書けなかったときに、ない行と処理済みの行を区別する。
func (b inbox) Resolve(ctx context.Context, id port.NoticeID, r port.NoticeResult) error {
	if r != port.NoticeApplied && r != port.NoticeKept {
		return port.ErrInvalidOperation
	}
	res, err := b.db.ExecContext(ctx, `UPDATE inbox SET read = 1, result = ? WHERE id = ? AND result = ''`, string(r), int64(id))
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil || n > 0 {
		return err
	}
	if _, err := b.Get(ctx, id); err != nil {
		return err
	}
	return refusal.New("この通知はすでに処理されています")
}

func (b inbox) Prune(ctx context.Context, store port.StoreID, targets []change.ID) error {
	if len(targets) == 0 {
		return nil
	}
	marks, ids := inClause(targets)
	args := append([]any{string(store)}, ids...)
	args = append(args, ids...)
	args = append(args, ids...)
	_, err := b.db.ExecContext(ctx, `DELETE FROM inbox WHERE store = ? AND (item IN `+marks+` OR project IN `+marks+` OR (item = '' AND project = '' AND workspace IN `+marks+`))`, args...) //nolint:gosec // G202: つなぐのは ? だけの IN 句。値は args で渡す
	return err
}

func (b inbox) ForgetStore(ctx context.Context, store port.StoreID) error {
	_, err := b.db.ExecContext(ctx, `DELETE FROM inbox WHERE store = ?`, string(store))
	return err
}
