package noticing

import (
	"bytes"
	"fmt"
	"slices"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/comment"
	"github.com/kukv/hedgehug/internal/domain/history"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// watched は、ウォッチの通知 1 行の本文と、それを決めた行(差出人と、自分の変更かを決める)。
type watched struct {
	text string
	row  change.Record
}

// watches は、あなたがウォッチ中のアイテムの、判定の前と後で値が違う項目と、判定の中で投稿されたコメントの通知
// (C4b1-9)。読み取りの失敗(ないもの以外)はエラー。
func watches(r port.Reader, b batch, store port.StoreID, me string) ([]port.Notice, error) {
	var out []port.Notice
	for _, ref := range b.refs {
		if ref.Kind != change.KindItem || b.movedIn(ref) {
			continue
		}
		ws, base, ok, err := watchTarget(r, b, ref, me)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		for _, w := range ws {
			if !fromOther(w.row, me) {
				continue
			}
			n := base
			n.Kind, n.Store, n.Text, n.Sender = port.NoticeWatch, store, w.text, sender(w.row)
			out = append(out, n)
		}
	}
	return out, nil
}

// watchTarget は、ウォッチの本文の候補と通知の下地。あなたがウォッチしていないか、判定の前にアイテムがなければ false。
func watchTarget(r port.Reader, b batch, ref change.Ref, me string) ([]watched, port.Notice, bool, error) {
	after, base, ok, err := itemTarget(r, b, ref)
	if err != nil || !ok || !slices.ContainsFunc(after.Watchers, func(w string) bool { return same(w, me) }) {
		return nil, port.Notice{}, false, err
	}
	before, ok := item.FromLog(b.before(ref))
	if !ok {
		return nil, port.Notice{}, false, nil
	}
	var ws []watched
	if before.Title != after.Title {
		if w, ok := change.Current(b.after(ref), change.Set, "title", func(change.Record) bool { return true }); ok {
			ws = append(ws, watched{fmt.Sprintf("タイトルを「%s」に変更", after.Title), w})
		}
	}
	// 別のワークスペースへの移動は、値の定義が変わるので、プロパティを比べない(06「移動では通知しない」)
	if before.Place.Workspace == after.Place.Workspace {
		ps, err := propertyTexts(r, b, ref, before, after)
		if err != nil {
			return nil, port.Notice{}, false, err
		}
		ws = append(ws, ps...)
	}
	return append(ws, commentTexts(b, ref)...), base, true, nil
}

// propertyTexts は、ワークスペースのプロパティ(組み込みを含む)のうち、前と後で値が違うものの本文を、設定の並びで返す。
func propertyTexts(r port.Reader, b batch, ref change.Ref, before, after item.Item) ([]watched, error) {
	st, err := r.Workspaces().Settings(after.Place.Workspace)
	if err != nil {
		return nil, notFoundIsSkip(err)
	}
	ms, err := r.Members().List()
	if err = notFoundIsSkip(err); err != nil {
		return nil, err
	}
	names := history.NamesOf(ms)
	var out []watched
	for _, p := range st.Properties {
		v := item.Current(after, p)
		if bytes.Equal(item.Current(before, p), v) {
			continue
		}
		w, ok := lastWriter(b, ref, p)
		if !ok {
			continue
		}
		// 日付の年は、変えた行の日で決める(保存する日。item.Display)
		c := history.Context{Settings: st, Names: names, Today: w.At}
		out = append(out, watched{history.WatchText(p, v, c), w})
	}
	return out, nil
}

// lastWriter は、プロパティ p のいまの値を決めた行。値を 1 つ持つものは set の勝ち、複数持つもの(関連システム・
// 複数選択)は、判定する add・remove の行のうち最もあとのもの。
func lastWriter(b batch, ref change.Ref, p workspace.Property) (change.Record, bool) {
	f := item.Field(p)
	if p.Type != workspace.TypeSystems && p.Type != workspace.TypeMulti {
		return change.Current(b.after(ref), change.Set, f, func(change.Record) bool { return true })
	}
	var w change.Record
	found := false
	for _, r := range b.after(ref) {
		if !b.judged[r.ID] || r.F != f || (r.T != change.Add && r.T != change.Remove) {
			continue
		}
		if !found || r.At.After(w.At) || (r.At.Equal(w.At) && r.ID > w.ID) {
			w, found = r, true
		}
	}
	return w, found
}

// commentTexts は、判定の中で初めて現れたコメント(いまもあるもの)の本文。行は投稿の行。
func commentTexts(b batch, ref change.Ref) []watched {
	var out []watched
	before := b.before(ref)
	for _, c := range comment.FromLog(b.after(ref)) {
		if _, ok := change.First(before, change.Comment, string(c.ID)); ok {
			continue
		}
		if first, ok := change.First(b.after(ref), change.Comment, string(c.ID)); ok {
			out = append(out, watched{history.CommentHead(c.Body), first})
		}
	}
	return out
}
