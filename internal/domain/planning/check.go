package planning

import (
	"errors"
	"fmt"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/refusal"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

var (
	// ErrNotWork は、スプリントに入れられない種類(作業項目でない)を伝える。
	ErrNotWork = errors.New("planning: not a work item")
	// ErrNoSprint は、使えるスプリントがない(削除済み・ない・別のワークスペース)ことを伝える。
	ErrNoSprint = errors.New("planning: no such sprint")
)

// AssignableSprint は、種類 kind のアイテム(いまのスプリントは current)を入れられるスプリント id
// (F1-5・F1-15)。作業項目でなければ ErrNotWork、使えるスプリントでなければ ErrNoSprint。
// 完了したスプリントは、いま入っているときだけ返し、ほかは断る。
func AssignableSprint(p Plan, id change.ID, kind item.Kind, current change.ID) (Sprint, error) {
	if !IsWork(kind) {
		return Sprint{}, fmt.Errorf("%w: %s はスプリントに入れられない", ErrNotWork, kind)
	}
	sp, ok := p.UsableSprint(id)
	if !ok {
		return Sprint{}, ErrNoSprint
	}
	if sp.State == Closed && current != sp.ID {
		return Sprint{}, refusal.New("完了したスプリントには入れられません")
	}
	return sp, nil
}

// NormalizeName は、保存と比べに使う名前の形(前後の空白を除く。F1-6)。
func NormalizeName(s string) string { return workspace.NormalizeName(s) }

// CheckSprint は、スプリント s の入力の検査(05 の表の順に最初の 1 つ)。live に s 自身が入っていてもよい。
func CheckSprint(s Sprint, live []Sprint, today string) error {
	name := NormalizeName(s.Name)
	if name == "" {
		return refusal.New("名前を入力してください")
	}
	for _, o := range live {
		if o.ID != s.ID && NormalizeName(o.Name) == name {
			return refusal.New("同じ名前のスプリントがあります")
		}
	}
	started := s.State == Active || s.State == Closed
	if started && (s.Start == "" || s.End == "") {
		return refusal.New("実行中のスプリントには開始日と終了日が必要です")
	}
	if s.Start != "" && s.End != "" && s.End < s.Start {
		return refusal.New("終了日は開始日以降にしてください")
	}
	if started && s.Start > today {
		return refusal.New("開始日は今日以前にしてください")
	}
	return nil
}

// CheckStart は、開始のダイアログの開始日と終了日の検査(05 の開始)。
func CheckStart(start, end, today string) error {
	switch {
	case start == "" || end == "":
		return refusal.New("開始日と終了日を入力してください")
	case end < start:
		return refusal.New("終了日は開始日以降にしてください")
	case start > today:
		return refusal.New("開始日は今日以前にしてください")
	}
	return nil
}

// CheckRelease は、リリース r の入力の検査。live に r 自身が入っていてもよい。
func CheckRelease(r Release, live []Release) error {
	name := NormalizeName(r.Name)
	if name == "" {
		return refusal.New("名前を入力してください")
	}
	for _, o := range live {
		if o.ID != r.ID && NormalizeName(o.Name) == name {
			return refusal.New("同じ名前のリリースがあります")
		}
	}
	return nil
}
