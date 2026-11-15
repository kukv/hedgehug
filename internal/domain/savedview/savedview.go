// Package savedview は、保存したビュー(E3b)の値。名前と内容(検索の画面の状態と同じ形)を持つ。
// infra が app を import できないので、infra(statedb)と app/views が共有する型をここに置く。
package savedview

// Sort は、テーブルの並べ替え。
type Sort struct {
	Key  string `json:"key"`
	Desc bool   `json:"desc"`
}

// Table は、テーブルの設定。
type Table struct {
	Columns []string `json:"columns"`
	Group   string   `json:"group"`
	Sort    *Sort    `json:"sort"`
}

// Content は、ビューの内容(E3b-1)。
type Content struct {
	Query  string `json:"query"`
	Layout string `json:"layout"`
	Table  Table  `json:"table"`
}

// View は、保存したビュー。ID は ULID を小文字にした 26 文字(E3b-2)。
type View struct {
	ID      string
	Name    string
	Content Content
}
