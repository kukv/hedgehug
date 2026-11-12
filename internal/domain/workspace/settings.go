package workspace

import "github.com/kukv/hedgehug/internal/domain/change"

// Category はステータスの分類(03「分類」)。
type Category string

const (
	CategoryTodo     Category = "todo"
	CategoryDoing    Category = "doing"
	CategoryDone     Category = "done"
	CategoryCanceled Category = "canceled"
)

// Categories は分類の並び(未着手・進行中・完了・中止)。
var Categories = []Category{CategoryTodo, CategoryDoing, CategoryDone, CategoryCanceled}

// Color は値の色。名前は画面のトークン --c-<色>-* と同じ。
type Color string

// Colors は 03「色」の 18 色を、03 の並び(グレー、青灰、茶、赤、ローズ、桃、赤紫、紫、藍、青、
// 空色、水色、青緑、緑、オリーブ、山吹、黄、橙)で持つ。
var Colors = []Color{
	"gray", "slate", "brown", "red", "rose", "pink", "magenta", "purple", "indigo",
	"blue", "sky", "cyan", "teal", "green", "olive", "amber", "yellow", "orange",
}

// PropertyType はプロパティの型(03「組み込みの 8 種」「追加プロパティ」)。
type PropertyType string

const (
	TypeStatus   PropertyType = "status"
	TypeUser     PropertyType = "user"
	TypePriority PropertyType = "priority"
	TypeDate     PropertyType = "date"
	TypeNumber   PropertyType = "number"
	TypeDuration PropertyType = "duration" // 組み込みの見積もり時間だけが持つ
	TypeSystems  PropertyType = "systems"
	TypeText     PropertyType = "text"
	TypeSingle   PropertyType = "single"
	TypeMulti    PropertyType = "multi"
	TypeCheck    PropertyType = "check"
	TypeURL      PropertyType = "url"
)

// Kinds は、プロパティを使う種類に選べるアイテムの種類。item は workspace を import するので、
// item.Kind ではなく同じ文字で持つ。
var Kinds = []string{"Epic", "Story", "Task", "Bug", "Step"}

// Value は、ステータス・優先度・関連システムの値の 1 つ。Category はステータスだけが持つ。
type Value struct {
	ID       change.ID
	Name     string
	Color    Color
	Category Category
	Rank     string
}

// Property は、アイテムの項目の定義。組み込みの ID はキーと同じ。
type Property struct {
	ID      change.ID
	Key     string
	Name    string
	Type    PropertyType
	Builtin bool
	Kinds   []string // 使う種類(Kinds の並び)
	Hidden  bool
	Rank    string
	Choices []Value // 選択肢(単一選択・複数選択だけ)。Rank の順
}

// Settings は、ワークスペースの設定。一覧はどれも Rank の順。
type Settings struct {
	Properties       []Property
	Statuses         []Value
	Priorities       []Value
	Systems          []Value
	InitialStatus    change.ID
	InitialPriority  change.ID
	CategoryDefaults map[Category]change.ID
}

func validColor(c Color) bool {
	for _, x := range Colors {
		if x == c {
			return true
		}
	}
	return false
}

func validCategory(c Category) bool {
	for _, x := range Categories {
		if x == c {
			return true
		}
	}
	return false
}
