package search

import (
	"encoding/json"
	"reflect"

	"github.com/danielgtaylor/huma/v2"
)

// Nullable は、null になりうるオブジェクト。huma はオブジェクトの型への nullable タグを受け付けないので、
// スキーマを oneOf(その型 1 つ)に nullable を付けて出す(OpenAPI 3.0 に type: "null" はない)。値がなければ JSON の null を書く。
type Nullable[T any] struct{ V *T }

func nullable[T any](v T) Nullable[T] { return Nullable[T]{&v} }

// MarshalJSON は、値がなければ null を、あればその値を書く。
func (n Nullable[T]) MarshalJSON() ([]byte, error) {
	if n.V == nil {
		return []byte("null"), nil
	}
	return json.Marshal(n.V)
}

// Schema は、huma に渡すスキーマ。
func (Nullable[T]) Schema(r huma.Registry) *huma.Schema {
	return &huma.Schema{Type: huma.TypeObject, OneOf: []*huma.Schema{r.Schema(reflect.TypeFor[T](), true, "")}, Nullable: true}
}
