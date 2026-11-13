package change

import "encoding/json"

// Draft は usecase が書く変更。ID・操作・時刻・実行者・prev は、Stores の実装が Stamp で付ける。
type Draft struct {
	T    Type
	F    string
	From json.RawMessage
	To   json.RawMessage
	Root *bool
	// Rule は、自動化のルールが書いた行のルール(B4-1)。空なら操作の実行者の行。
	Rule ID
}

// ByRule は、自動化のルール rule が書いた行にする。
func (d Draft) ByRule(rule ID) Draft {
	d.Rule = rule
	return d
}

// Value は値を JSON にする。JSON にできない値を渡すのは呼び出しの誤りなので panic する。
func Value(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// NewCreate は作成の変更。to は作成時の値すべて。
func NewCreate(to any) Draft { return Draft{T: Create, To: Value(to)} }

// NewSet は、値を 1 つ持つ項目の変更。
func NewSet(f string, from, to any) Draft {
	return Draft{T: Set, F: f, From: Value(from), To: Value(to)}
}

// NewAdd は、値を複数持つ項目への、値 1 つの追加。
func NewAdd(f string, v any) Draft { return Draft{T: Add, F: f, To: Value(v)} }

// NewRemove は、値を複数持つ項目からの、値 1 つの取り除き。
func NewRemove(f string, v any) Draft { return Draft{T: Remove, F: f, From: Value(v)} }

// NewDelete は、f が指すものの削除。from も to も持たない。
func NewDelete(f string) Draft { return Draft{T: Delete, F: f} }

// NewDesc は、説明を保存した記録。本文は Describe で別に書く(description.md)。
// DeletedField は、アイテム・プロジェクトの削除の行の f(item.yaml・project.yaml の deleted と同じ名前)。
const DeletedField = "deleted"

// NewDeleteEntity は、アイテム・プロジェクト自身の削除(論理削除)の変更。from も to も持たない。
// root は、利用者が選んだものなら true、下にあるものなら false(保存形式「変更記録」の root)。
func NewDeleteEntity(root bool) Draft { return Draft{T: Delete, F: DeletedField, Root: &root} }

func NewDesc() Draft { return Draft{T: Desc} }

// NewComment は、コメント id の追加・編集・削除。to が nil なら削除(to は null で書く)。
func NewComment(id ID, from, to any) Draft {
	return Draft{T: Comment, F: string(id), From: Value(from), To: Value(to)}
}

// NewWorklog は、作業時間の記録 id の追加・編集・削除。to が nil なら削除。
func NewWorklog(id ID, from, to any) Draft {
	return Draft{T: Worklog, F: string(id), From: Value(from), To: Value(to)}
}
