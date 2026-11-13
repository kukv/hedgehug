package itemlink

import (
	"github.com/kukv/hedgehug/internal/domain/change"
)

// Attach は、r を v の値でリンクする行(07「追加」)。値は、いまの値(外したリンクのものを含む)と違う項目だけ
// 書く。解除の記録にあれば、同じ保存で記録から消す(07「解除」の付け直し)。Issue はレビューの行を書かない。
func (s Set) Attach(r Ref, v Values) []change.Draft {
	out := []change.Draft{change.NewAdd(LinksField, r)}
	out = append(out, s.valueDrafts(r, v)...)
	for _, o := range s.Off {
		if o.Kind == r.Kind && o.Same(r) {
			out = append(out, change.NewRemove(OffField, o))
		}
	}
	return out
}

// Detach は、l を外す行。PR なら解除の記録に足す(自動でもう付けない。07「解除」)。
func (s Set) Detach(l Link) []change.Draft {
	out := []change.Draft{change.NewRemove(LinksField, l.Ref)}
	if l.Kind == PR {
		out = append(out, change.NewAdd(OffField, l.Ref))
	}
	return out
}

// Refresh は、GitHub から読んだ v のうち、l の値と違う項目の行(07「表示と更新」)。
func (s Set) Refresh(l Link, v Values) []change.Draft { return s.valueDrafts(l.Ref, v) }

// Rename は、改名・移管を追って、l を repo のリンクに書き換える行(D3-11)。新しい項目の値は、変わらない項目も
// すべて書き、from は旧いリンクの値にする(履歴は from と to が同じ行を出さず、変わった項目だけが
// 「…の状態を {状態} に変更」として出る。D4 の自動化のきっかけもこの行で判定する)。
func (s Set) Rename(l Link, repo string, v Values) []change.Draft {
	moved := Ref{Kind: l.Kind, Repo: repo, Number: l.Number}
	out := []change.Draft{change.NewRemove(LinksField, l.Ref), change.NewAdd(LinksField, moved)}
	out = append(out,
		change.NewSet(ValueField(moved, "title"), l.Title, v.Title),
		change.NewSet(ValueField(moved, "state"), string(l.State), string(v.State)))
	if l.Kind == PR {
		out = append(out, change.NewSet(ValueField(moved, "review"), string(l.Review), string(v.Review)))
	}
	return out
}

// valueDrafts は、r の値の項目のうち、いまの値と違うものの set。from は、いまの値がなければ null。
func (s Set) valueDrafts(r Ref, v Values) []change.Draft {
	cur, had := s.known[r.Key()]
	var out []change.Draft
	add := func(name, from, to string) {
		if from == to && had {
			return
		}
		var f any = from
		if from == "" {
			f = nil
		}
		out = append(out, change.NewSet(ValueField(r, name), f, to))
	}
	add("title", cur.Title, v.Title)
	add("state", string(cur.State), string(v.State))
	if r.Kind == PR {
		add("review", string(cur.Review), string(v.Review))
	}
	return out
}
