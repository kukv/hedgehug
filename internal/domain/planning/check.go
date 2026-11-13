package planning

import (
	"github.com/kukv/hedgehug/internal/domain/refusal"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

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
