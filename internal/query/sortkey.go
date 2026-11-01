package query

import (
	"cmp"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/index"
)

// SortKey is the value of the sort: field of one task, which mount.Query
// compares across stores.
type SortKey struct {
	None  bool    // no value; last in either direction
	Class int     // status: the category's place (todo 0, doing 1, done 2, canceled 3); kind: the role's place (container 0, work 1); 0 for the other fields
	Place int     // status, kind, enum, multi_enum: the value's place in the task's own schema (len when not in it); 0 for the other fields
	Num   float64 // a number, date or datetime that parses
	NumOK bool
	Text  string // the value as written
	By    string // "place", "number" or "text": how Text and Num compare, as the field compares now
}

// CompareSortKeys orders two keys from any stores: None last, then Class,
// then by By as the sort compares now (place: Place then Text; number: Num
// (parsed before not) then Text; text: lower-cased Text). desc reverses all
// but None.
func CompareSortKeys(a, b SortKey, desc bool) int {
	switch {
	case a.None && b.None:
		return 0
	case a.None:
		return 1
	case b.None:
		return -1
	}
	n := cmp.Compare(a.Class, b.Class)
	if n == 0 {
		switch a.By {
		case "place":
			if n = cmp.Compare(a.Place, b.Place); n == 0 {
				n = strings.Compare(a.Text, b.Text)
			}
		case "number":
			switch {
			case a.NumOK && b.NumOK && a.Num != b.Num:
				n = cmp.Compare(a.Num, b.Num)
			case a.NumOK && !b.NumOK:
				n = -1
			case !a.NumOK && b.NumOK:
				n = 1
			default:
				n = strings.Compare(a.Text, b.Text)
			}
		default:
			n = strings.Compare(strings.ToLower(a.Text), strings.ToLower(b.Text))
		}
	}
	if desc {
		n = -n
	}
	return n
}

// sortKey is the key of e for the field.
func (f field) sortKey(e *index.Entry) SortKey {
	v := first(f.values(e))
	if v == "" {
		return SortKey{None: true}
	}
	k := SortKey{Text: v, By: f.by}
	if f.class != nil {
		k.Class = f.class(e)
	}
	switch f.by {
	case "place":
		k.Place = slices.Index(f.order, v)
		if k.Place < 0 {
			k.Place = len(f.order)
		}
	case "number":
		if x, err := f.parse(v); err == nil {
			k.Num, k.NumOK = x, true
		}
	}
	return k
}

// placed, numbered and texted say how a field compares: by the place of a
// value in order, by what parse makes of it, or as lower-cased text.
func placed(f field, order []string) field {
	f.by, f.order, f.less = "place", order, byPlace(order)
	return f
}

func numbered(f field, parse func(string) (float64, error)) field {
	f.by, f.parse, f.less = "number", parse, byNumber(parse)
	return f
}

func texted(f field) field {
	f.by, f.less = "text", byText
	return f
}
