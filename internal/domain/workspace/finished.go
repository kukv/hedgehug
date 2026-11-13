package workspace

import "github.com/kukv/hedgehug/internal/domain/change"

// Finished は、ステータス id が完了か中止の分類か(「ブロック中」の判定。02「上から順の構成」)。
// 一覧にない値と空は false。
func (s Settings) Finished(id change.ID) bool {
	for _, v := range s.Statuses {
		if v.ID == id {
			return v.Category == CategoryDone || v.Category == CategoryCanceled
		}
	}
	return false
}
