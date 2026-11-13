package githublink

import (
	"context"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// pass は、1 回の読み取り(D4a-14)。同じアイテムの同じきっかけを 1 回だけ起こすために、起こしたものを覚える。
// fetched は、今回 GitHub から読んだ値(Ref.Key() から引く)。AllMerged をまだ書いていないリンクも含めて数えるのに使う。
type pass struct {
	fired   map[item.ID]map[automation.Trigger]bool
	fetched map[string]itemlink.Values
}

func newPass() *pass {
	return &pass{fired: map[item.ID]map[automation.Trigger]bool{}, fetched: map[string]itemlink.Values{}}
}

// fresh は、ts のうち、この読み取りでまだ id に起こしていないきっかけ。返したものは起こしたと覚える。
// p が nil(1 回きりの操作)なら ts のまま。覚えるのは保存の中(保存が成功する前)なので、保存が失敗したら、
// その pass は捨てる(同じ pass で読み取りを続けると、保存されなかったきっかけを起こしたことにしてしまう)。
func (p *pass) fresh(id item.ID, ts []automation.Trigger) []automation.Trigger {
	if p == nil {
		return ts
	}
	done := p.fired[id]
	if done == nil {
		done = map[automation.Trigger]bool{}
		p.fired[id] = done
	}
	var out []automation.Trigger
	for _, t := range ts {
		if !done[t] {
			done[t] = true
			out = append(out, t)
		}
	}
	return out
}

// fired は、1 つのアイテムで動いたルールの結果と、通知の文を作るために保存の中で読んだもの。
type fired struct {
	item     item.Item
	outcomes []automation.Outcome
	settings workspace.Settings
	names    item.Names
}

// fire は、保存 tx の中で、アイテム id のリンクが before から after になったきっかけでルールを動かし、
// 変えた行を書く。p が nil なら 1 回きりの操作(手でのリンク)。削除済みのアイテムで動かさない判定は、
// 削除の印とともに A4b が足す。呼ぶのはリンクの行を書いたあと(tx の読み取りは書いた行を含む)。
// before は、tx の中で書く前に読んだリンク(いまリンクしたなら nil)、after は書いたあとの値。
// 1 回の変更で起きたきっかけのルールは、一覧の上から順に 1 回で動かす(03「実行の規則」)。
func fire(tx port.Tx, id item.ID, before *itemlink.Link, after itemlink.Link, p *pass) ([]fired, error) {
	ts := p.fresh(id, automation.LinkEvents(before, after))
	if len(ts) == 0 {
		return nil, nil
	}
	// 削除済みのアイテムでは動かさない(03)。削除の印は A4b が足す
	it, err := tx.Items().Get(id)
	if err != nil {
		return nil, err
	}
	rules, st, all, err := fireInputs(tx, it, p)
	if err != nil {
		return nil, err
	}
	evs := make([]automation.Event, len(ts))
	for i, t := range ts {
		evs[i] = automation.Event{Trigger: t, AllMerged: all}
	}
	os := automation.RunAll(st, rules, it, evs)
	var ds []change.Draft
	for _, o := range os {
		if o.Draft != nil {
			ds = append(ds, *o.Draft)
		}
	}
	if len(os) == 0 {
		return nil, nil
	}
	if err := tx.WriteItems().Append(id, ds...); err != nil {
		return nil, err
	}
	names, err := memberNames(tx)
	return []fired{{item: it, outcomes: os, settings: st, names: names}}, err
}

// fireInputs は、ルール・ワークスペースの設定と、「すべてマージ済み」か(D4a-13・14)。リンクは tx の中で読む
// (書いたリンクの行を含む)。まだ書いていない同じアイテムのリンクは、今回読んだ値(p.fetched)で数える。
func fireInputs(tx port.Tx, it item.Item, p *pass) ([]automation.Rule, workspace.Settings, bool, error) {
	wlog, err := tx.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: it.Place.Workspace})
	if err != nil {
		return nil, workspace.Settings{}, false, err
	}
	st, err := tx.Workspaces().Settings(it.Place.Workspace)
	if err != nil {
		return nil, workspace.Settings{}, false, err
	}
	ilog, err := tx.Changes().Of(change.Ref{Kind: change.KindItem, ID: it.ID})
	if err != nil {
		return nil, workspace.Settings{}, false, err
	}
	links := itemlink.FromLog(ilog).Links
	for i, l := range links {
		if v, ok := p.valueOf(l.Ref); ok {
			links[i].Values = v
		}
	}
	return automation.FromLog(wlog), st, automation.AllMerged(links), nil
}

// valueOf は、今回 GitHub から読んだ r の値。p が nil なら読んでいない。
func (p *pass) valueOf(r itemlink.Ref) (itemlink.Values, bool) {
	if p == nil {
		return itemlink.Values{}, false
	}
	v, ok := p.fetched[r.Key()]
	return v, ok
}

// memberNames は、メンバーの表示名を引く関数(app/items の editContext と同じ)。
func memberNames(r port.Reader) (item.Names, error) {
	ms, err := r.Members().List()
	if err != nil {
		return nil, err
	}
	names := map[string]string{}
	for _, m := range ms {
		names[strings.ToLower(m.Login)] = m.Name
	}
	return func(login string) string {
		if n, ok := names[strings.ToLower(login)]; ok {
			return n
		}
		return "不明なユーザー" // 01「画面の移り方と、共通の表示」
	}, nil
}

// notify は、保存のあとに、動いたルールの通知を受信箱に書く(B4-10・B4-14 と同じ形)。
func (l Links) notify(ctx context.Context, store port.StoreID, fs []fired) error {
	for _, f := range fs {
		for _, o := range f.outcomes {
			n := port.Notice{Store: store, Workspace: f.item.Place.Workspace, Item: f.item.ID, Rule: o.Rule.ID}
			if o.Draft == nil {
				n.Kind, n.Text, n.Reason = port.NoticeAutomationStopped, automation.StoppedText(o.Rule.Name), o.Stopped
				n.Status = change.ID(o.Rule.Value)
			} else {
				value := item.Display(o.Prop, o.To, f.settings, f.names, l.now())
				n.Kind, n.Text = port.NoticeAutomation, automation.AppliedText(o.Rule.Name, automation.ChangeText(o.Prop.Name, value))
			}
			if err := l.Inbox.Add(ctx, n); err != nil {
				return err
			}
		}
	}
	return nil
}
