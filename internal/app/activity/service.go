package activity

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// pageSize は、一度に返す行の数(06「件数」)。重なりをまとめ、絞り込んだあとで数える。
const pageSize = 300

// Query は、アクティビティの一覧の絞り込みと続きの印。Store・Workspace が空なら「すべて」。Workspace は Store と一緒に渡す。
type Query struct {
	Store     port.StoreID
	Workspace workspace.ID
	Kind      Kind
	// Before は、前のページの最後の行の Cursor。空なら先頭から。
	Before string
}

// Page は、一覧の 1 回分。
type Page struct {
	Rows []Row
	More bool // 続きがある(「さらに読み込む」を出す)
}

// Service は、表示中の Store のコミットの行と同期の行を並べる(06「アクティビティ」)。
type Service struct {
	Stores  port.Stores
	Commits port.StoreCommits
	Log     port.SyncLog
	List    port.StoreList
}

// Page は、q に合う行を、時刻の新しい順(同じなら鍵の辞書順の逆)に、q.Before の後ろから 300 件返す(C4d-14)。
// 種類か続きの印が読めないとき、ワークスペースだけを渡したときは port.ErrInvalidOperation。
func (s Service) Page(ctx context.Context, q Query) (Page, error) {
	match, err := q.matcher()
	if err != nil {
		return Page{}, err
	}
	infos, err := s.List.List(ctx)
	if err != nil {
		return Page{}, err
	}
	var all []Row
	for _, info := range infos {
		if q.Store != "" && info.ID != q.Store {
			continue
		}
		rows, err := s.storeRows(ctx, info)
		if err != nil {
			return Page{}, err
		}
		all = append(all, rows...)
	}
	slices.SortFunc(all, func(a, b Row) int { return cmp.Or(b.At.Compare(a.At), strings.Compare(b.Key, a.Key)) })
	p := Page{Rows: []Row{}}
	for _, r := range all {
		if !match(r) {
			continue
		}
		if len(p.Rows) == pageSize {
			p.More = true
			break
		}
		p.Rows = append(p.Rows, r)
	}
	return p, nil
}

// storeRows は、Store 1 つのコミットの行(重なりをまとめたもの)と同期の行。コミットは、Store を 1 回読むあいだに
// 行にする(Commits は Store を読むので、その外で呼ぶ)。
func (s Service) storeRows(ctx context.Context, info port.StoreInfo) ([]Row, error) {
	commits, err := s.Commits.Commits(ctx, info.ID)
	if err != nil {
		return nil, err
	}
	var rows []Row
	err = s.Stores.Read(ctx, info.ID, func(r port.Reader) error {
		v, err := readView(r)
		if err != nil {
			return err
		}
		for _, c := range commits {
			got, err := v.rows(info, c)
			if err != nil {
				return err
			}
			rows = append(rows, got...)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	rows = dedup(rows)
	lines, err := s.Log.List(ctx, info.ID)
	if err != nil {
		return nil, err
	}
	for _, line := range lines {
		rows = append(rows, syncRow(info, line))
	}
	return rows, nil
}

// syncKeyPrefix は、同期の行の鍵の書き出し。
const syncKeyPrefix = "sync:"

// syncRow は、同期の行(06「同期の行」)。時刻は、続きの印と同じミリ秒にそろえる(そろえないと、ミリ秒より下の
// 違いで、並びと続きの印の比べ方が食い違う)。
func syncRow(info port.StoreInfo, line port.SyncLine) Row {
	return Row{
		Key:   fmt.Sprintf("%s%d", syncKeyPrefix, line.ID),
		At:    line.At.Truncate(time.Millisecond),
		Store: info,
		Actor: ActorSync,
		Who:   "同期",
		Text:  fmt.Sprintf("origin と同期 (送信 %d 件・受信 %d 件)", line.Sent, line.Received),
	}
}

// dedup は、重なりの鍵が同じ行のうち、時刻が最も早い行(同じなら鍵、つまり SHA の辞書順で前)だけを残す(C4d-11)。
// 鍵のない行(人の行)はすべて残す。並びは変えない。
func dedup(rows []Row) []Row {
	keep := map[string]int{}
	for i, r := range rows {
		if r.dedup == "" {
			continue
		}
		if j, ok := keep[r.dedup]; !ok || r.At.Before(rows[j].At) || r.At.Equal(rows[j].At) && r.Key < rows[j].Key {
			keep[r.dedup] = i
		}
	}
	out := rows[:0:0]
	for i, r := range rows {
		if r.dedup == "" || keep[r.dedup] == i {
			out = append(out, r)
		}
	}
	return out
}
