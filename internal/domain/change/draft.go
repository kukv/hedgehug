package change

import "encoding/json"

// Draft は usecase が書く変更。ID・操作・時刻・実行者・prev は、Stores の実装が Stamp で付ける。
type Draft struct {
	T    Type
	F    string
	From json.RawMessage
	To   json.RawMessage
	Root *bool
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
