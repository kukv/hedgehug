package inbox

import (
	"context"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
)

// pageSize は、一度に読む行の数(06「件数」。C4a-4)。
const pageSize = 300

// Page は、受信箱の一覧の 1 回分。
type Page struct {
	Rows   []Row
	More   bool // 続きがある(「さらに読み込む」を出す)
	Unread map[Tab]int
}

// Row は、一覧の 1 行。対象とボタンは、読むときのいまの値で決める(C4a-8・C4a-10)。
type Row struct {
	ID     port.NoticeID
	Kind   port.NoticeKind
	At     time.Time
	Read   bool
	Text   string
	Reason string
	Sender string // 差出人のログイン名。空は「hedgehug」
	Store  port.StoreInfo
	Target Target
	Action *Action // 止まった通知だけ
}

// Page は、タブの通知を新しい順に、before より古いものから 300 件返す。before が nil なら先頭から。
func (s Service) Page(ctx context.Context, tab Tab, before *port.NoticeCursor) (Page, error) {
	infos, err := s.visible(ctx)
	if err != nil {
		return Page{}, err
	}
	stores := ids(infos)
	var notices []port.NoticeRow
	if len(stores) > 0 {
		notices, err = s.Book.List(ctx, port.NoticeQuery{Stores: stores, Kinds: KindsOf(tab), Before: before, Limit: pageSize + 1})
		if err != nil {
			return Page{}, err
		}
	}
	p := Page{More: len(notices) > pageSize}
	if p.More {
		notices = notices[:pageSize]
	}
	if p.Rows, err = s.rows(ctx, infos, notices); err != nil {
		return Page{}, err
	}
	if p.Unread, err = s.counts(ctx, stores); err != nil {
		return Page{}, err
	}
	return p, nil
}

// rows は、通知を行にする。Store ごとに 1 回だけ読む。
func (s Service) rows(ctx context.Context, infos []port.StoreInfo, notices []port.NoticeRow) ([]Row, error) {
	out := make([]Row, len(notices))
	for _, info := range infos {
		err := s.Stores.Read(ctx, info.ID, func(r port.Reader) error {
			for k, n := range notices {
				if n.Store != info.ID {
					continue
				}
				row, err := resolve(r, info, n)
				if err != nil {
					return err
				}
				out[k] = row
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}
