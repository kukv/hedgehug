package port

import (
	"context"
	"slices"
)

// RevivalKind は、取り消したものの種類。画面が出すトーストの文を分ける。
type RevivalKind int

const (
	RevivedID      RevivalKind = iota // アイテム・プロジェクト。Text は表示 ID
	RevivedName                       // 設定の値・プロパティ・テンプレートなど。Text は名前
	RevivedComment                    // コメント。Text はコメントの持ち主の表示 ID
)

// Revival は、取り消したもの 1 つ。
type Revival struct {
	Kind RevivalKind
	Text string
}

type revivedKey struct{}

// revived は、取り消したものを、積んだ順に持つ。
type revived struct{ list []Revival }

// WithRevived は、取り消したものを積む入れ物を置いた context を返す。
func WithRevived(ctx context.Context) context.Context {
	return context.WithValue(ctx, revivedKey{}, &revived{})
}

// Revived は、入れ物に積んだものを、積んだ順に返す。入れ物がなければ nil。
func Revived(ctx context.Context) []Revival {
	v, _ := ctx.Value(revivedKey{}).(*revived)
	if v == nil {
		return nil
	}
	return slices.Clone(v.list)
}

// NoteRevived は、入れ物に r を積む(同じものは 1 つ)。入れ物がなければ捨てる。
func NoteRevived(ctx context.Context, r Revival) {
	if v, _ := ctx.Value(revivedKey{}).(*revived); v != nil && !slices.Contains(v.list, r) {
		v.list = append(v.list, r)
	}
}
