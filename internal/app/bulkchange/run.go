package bulkchange

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// judged は、選んだアイテム 1 つの判定と、その材料。ws は、アイテムのワークスペース(削除済み・閲覧のみは nil)。
type judged struct {
	t  Target
	c  itemContext
	v  verdict
	ws *wsChoices
}

// changes は、このアイテムを変えるか。
func (j judged) changes() bool { return len(j.v.drafts) > 0 }

// Preview は、保存せずに、実行と同じ判定をする(F2b2-16)。
func (s Service) Preview(ctx context.Context, targets []Target, in Input) (Result, error) {
	return s.apply(ctx, targets, in, false)
}

// Run は、Store ごとに最初に出てきた順で 1 回ずつ保存する(F2b2-12・F2b2-13・F2b2-17)。保存できなかった
// Store は Failed に入れて、次の Store へ進む。
func (s Service) Run(ctx context.Context, targets []Target, in Input) (Result, error) {
	return s.apply(ctx, targets, in, true)
}

// apply は、入力を検査し、全部の Store を読んで判定してから、write なら Store ごとに保存する。
func (s Service) apply(ctx context.Context, targets []Target, in Input, write bool) (Result, error) {
	p, err := parseInput(in)
	if err != nil {
		return Result{}, err
	}
	groups, err := s.groups(ctx, targets)
	if err != nil {
		return Result{}, err
	}
	before, err := s.judgeStores(ctx, groups, p)
	if err != nil {
		return Result{}, err
	}
	res := Result{ValueText: valueText(p, firstContext(p, before), s.Clock.Now())}
	kept := map[Target]string{}
	for i, g := range groups {
		js := before[i]
		if write && changed(js) > 0 {
			if js, err = s.commit(ctx, g, js, p, &res); err != nil {
				return Result{}, err
			}
		}
		tally(&res, kept, js)
	}
	res.Kept = keptInOrder(targets, kept)
	return res, nil
}

// judgeStores は、保存の前に、すべての Store の選んだアイテムを読んで判定する。閲覧のみの Store は読まない。
func (s Service) judgeStores(ctx context.Context, groups []storeGroup, p parsed) ([][]judged, error) {
	out := make([][]judged, len(groups))
	for i, g := range groups {
		if g.readOnly {
			out[i] = readOnly(g.targets, p)
			continue
		}
		err := s.Stores.Read(ctx, g.id, func(r port.Reader) (err error) {
			out[i], err = judgeAll(r, g.targets, p)
			return err
		})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// tally は、判定 js を、変えた数と、残したもの(kept)に足す。
func tally(res *Result, kept map[Target]string, js []judged) {
	for _, j := range js {
		if j.changes() {
			res.Changed++
		} else if j.v.reason != "" {
			kept[j.t] = j.v.reason
		}
	}
}

// commit は、Store g を保存する。保存できなければ、保存の前の判定 before で変わるはずだった数を Failed に足し
// (F2b2-13)、ほかの理由で残したものだけを返す。保存できたら、自動化の通知を出し、OffBacklog を足して、
// 保存の中の判定を返す。通知・読み直しのエラーは返す。
func (s Service) commit(ctx context.Context, g storeGroup, before []judged, p parsed, res *Result) ([]judged, error) {
	js, fs, err := s.save(ctx, g, p)
	if err != nil {
		res.Failed = append(res.Failed, Failed{Store: g.id, Name: g.name, Count: changed(before)})
		return slices.DeleteFunc(slices.Clone(before), judged.changes), nil
	}
	if err := s.notify(ctx, g.id, fs); err != nil {
		return nil, err
	}
	return js, s.offBacklog(ctx, g.id, js, &res.OffBacklog)
}

// readOnly は、閲覧のみの Store のアイテムの判定(読まない)。
func readOnly(targets []Target, p parsed) []judged {
	out := make([]judged, len(targets))
	for i, t := range targets {
		c := itemContext{it: item.Item{ID: t.ID}, readOnly: true}
		out[i] = judged{t: t, c: c, v: judge(c, p)}
	}
	return out
}

// judgeAll は、Store の選んだアイテムを読んで判定する。削除済み・存在しない・ワークスペースが削除済みのアイテムは
// 「削除済み」(削除は取り消さない。F2b2-7)。
func judgeAll(r port.Reader, targets []Target, p parsed) ([]judged, error) {
	ms, err := r.Members().List()
	if err != nil {
		return nil, err
	}
	wss := map[workspace.ID]*wsChoices{}
	out := make([]judged, 0, len(targets))
	for _, t := range targets {
		j := judged{t: t, c: itemContext{it: item.Item{ID: t.ID}, deleted: true}}
		it, err := port.LiveItem(r, t.ID)
		switch {
		case err == nil:
			if j.ws, err = cachedWorkspace(r, wss, it.Place.Workspace); err != nil {
				return nil, err
			}
			if !j.ws.deleted {
				j.c = itemContext{it: it, settings: j.ws.settings, rules: j.ws.rules.Of(it.Place.Project), plan: j.ws.plan, members: ms}
			}
		case !errors.Is(err, port.ErrNotFound):
			return nil, err
		}
		j.v = judge(j.c, p)
		out = append(out, j)
	}
	return out, nil
}

// cachedWorkspace は、ワークスペース ws を、初めてのときだけ読む。
func cachedWorkspace(r port.Reader, wss map[workspace.ID]*wsChoices, ws workspace.ID) (*wsChoices, error) {
	if w := wss[ws]; w != nil {
		return w, nil
	}
	w, err := readWorkspace(r, ws)
	if err != nil {
		return nil, err
	}
	wss[ws] = w
	return w, nil
}

// changed は、js のうち変えるアイテムの数。
func changed(js []judged) int {
	n := 0
	for _, j := range js {
		if j.changes() {
			n++
		}
	}
	return n
}

// firstContext は、確認の本文の {値} に使う、最初の読めたアイテムの材料。担当者なら、そのログイン名の
// メンバーがいる最初のもの。
func firstContext(p parsed, groups [][]judged) itemContext {
	for _, js := range groups {
		for _, j := range js {
			if j.c.readOnly || j.c.deleted {
				continue
			}
			if p.field != FieldAssignee || slices.ContainsFunc(j.c.members, func(m member.Member) bool { return sameName(m.Login, p.text) }) {
				return j.c
			}
		}
	}
	return itemContext{}
}

// keptInOrder は、残したアイテムを、選んだ順に並べる。
func keptInOrder(targets []Target, kept map[Target]string) []Kept {
	var out []Kept
	for _, t := range targets {
		if r, ok := kept[t]; ok {
			out = append(out, Kept{Store: t.Store, ID: t.ID, Reason: r})
			delete(kept, t)
		}
	}
	return out
}

// save は、Store g の 1 回の保存。保存の中でもう一度判定して書き(F2b2-12)、スプリントの順位(F2b2-8)、
// 自動化(F2b2-14)の順に行う。保存の中で変わるものがなくなったら、何も書かない。
func (s Service) save(ctx context.Context, g storeGroup, p parsed) ([]judged, []fired, error) {
	me, err := s.Identity.Me(ctx, g.id)
	if err != nil {
		return nil, nil, err
	}
	op := port.Operation{Actor: change.User(me), Summary: fmt.Sprintf("%d 件の%sを一括変更", len(g.targets), fieldName(p.field, true))}
	var js []judged
	var fs []fired
	err = s.Stores.Write(ctx, g.id, op, func(tx port.Tx) error {
		var err error
		if js, err = judgeAll(tx, g.targets, p); err != nil {
			return err
		}
		if err := writeDrafts(tx, js); err != nil {
			return err
		}
		if p.field == FieldStatus {
			fs, err = automate(tx, js)
		}
		return err
	})
	return js, fs, err
}
