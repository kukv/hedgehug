package descriptions

import (
	"context"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/textmerge"
)

// slot は、アイテムとプロジェクトで違う、説明の読み書き。write は、本文と履歴の desc の行を書く。
type slot struct {
	read  func(r port.Reader) (string, error)
	write func(tx port.Tx, text string) error
}

// save は、保存の中で読んだいまの説明を older、入力を newer にして、編集を始めたときの説明(base)と
// 行ごとに合わせる(A4a-3、06「同時に編集したとき」。保存してあるほうが先)。合わせた結果がいまの
// 説明と同じなら何も保存しない。
func (s Service) save(ctx context.Context, store port.StoreID, summary, base, text string, sl slot) (Result, error) {
	me, err := s.Identity.Me(ctx, store)
	if err != nil {
		return Result{}, err
	}
	op := port.Operation{Actor: change.User(me), Summary: summary}
	var res Result
	err = s.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		saved, err := sl.read(tx)
		if err != nil {
			return err
		}
		merged, conflict := textmerge.Merge(blankIsEmpty(base), saved, blankIsEmpty(text))
		res = Result{Text: merged, Conflict: conflict}
		if merged == saved {
			return nil // 行のない Write は何も残さない
		}
		return sl.write(tx, merged)
	})
	if err != nil {
		return Result{}, err
	}
	return res, nil
}

// blankIsEmpty は、空白と改行だけの説明を空にする(空の説明はファイルを置かない。A4a-1)。
func blankIsEmpty(s string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	return s
}
