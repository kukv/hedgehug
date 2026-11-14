package inbox

import (
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
)

// Overwrite は、上書きの通知の値と「自分の値に戻す」の状態(C4c2-4・C4c2-5)。
type Overwrite struct {
	Mine, Kept string // 「あなたの値 {Mine} → 残った値 {Kept}」
	Visible    bool   // ボタンを出す
	Enabled    bool
	Note       string
	Result     port.NoticeResult // 空でなければ、ボタンの代わりに結果を出す
}

// overwriteOf は、上書きの通知 n の値とボタンの状態を上から順に決める(C4c2-4)。値を出さない項目は nil。
func overwriteOf(r port.Reader, info port.StoreInfo, n port.NoticeRow, today time.Time) (*Overwrite, error) {
	s, state, err := slotOf(r, n, today)
	if err != nil || state == slotNone {
		return nil, err
	}
	o := &Overwrite{Mine: s.mine, Kept: s.kept, Result: n.Result}
	switch {
	case info.ReadOnly:
		o.Visible, o.Note = true, "閲覧のみの Store です"
	case state == slotShown:
	case !sameValue(s.now, n.Kept):
		o.Visible, o.Note = true, "そのあとさらに変更されています(いまは "+s.current+")"
	case s.reason != "":
		o.Visible, o.Note = true, s.reason
	default:
		o.Visible, o.Enabled = true, true
	}
	return o, nil
}
