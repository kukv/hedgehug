package automations

import (
	"bytes"
	"context"
	"slices"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// recentMax は、最近の実行の件数(03「最近の実行」)。
const recentMax = 8

// View は、自動化のサブタブの中身。
type View struct {
	Rules  []automation.Rule
	Recent []Run
}

// Run は、最近の実行の 1 行(B4-15)。
type Run struct {
	At      time.Time
	Item    item.ID
	Display string // HH-12
	Text    string // 「{ルール名}」で{プロパティ名}を {値} に変更
}

// List は、ワークスペースのルールと最近の実行。
func (s Service) List(ctx context.Context, store port.StoreID, ws workspace.ID) (View, error) {
	var v View
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		rules, err := rulesOf(r, ws)
		if err != nil {
			return err
		}
		log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: ws})
		if err != nil {
			return err
		}
		v.Rules = rules
		v.Recent, err = s.recent(r, ws, automation.Names(log))
		return err
	})
	return v, err
}

type ruleRow struct {
	rec  change.Record
	item item.Item
}

// recent は、ワークスペースのすべてのアイテムの、自動化の行(B4-15)。
func (s Service) recent(r port.Reader, ws workspace.ID, names map[change.ID]string) ([]Run, error) {
	rows, err := ruleRows(r, ws)
	if err != nil {
		return nil, err
	}
	w, err := r.Workspaces().Get(ws)
	if err != nil {
		return nil, err
	}
	st, err := r.Workspaces().Settings(ws)
	if err != nil {
		return nil, err
	}
	who, err := memberNames(r)
	if err != nil {
		return nil, err
	}
	now := s.Clock.Now()
	out := make([]Run, 0, min(len(rows), recentMax))
	for _, row := range rows[:min(len(rows), recentMax)] {
		p := propertyOf(st, row.rec.F)
		value := item.Display(p, row.rec.To, st, who, now)
		out = append(out, Run{
			At: row.rec.At, Item: row.item.ID, Display: row.item.DisplayID(w.Prefix),
			Text: automation.ActivityText(names[row.rec.Actor.Rule], automation.ChangeText(p.Name, value)),
		})
	}
	return out, nil
}

// ruleRows は、自動化の行を新しい順に並べる。同じアイテム・項目・変更前・変更後の行は、最も早い 1 行だけ残す。
func ruleRows(r port.Reader, ws workspace.ID) ([]ruleRow, error) {
	ps, err := r.Projects().List(ws)
	if err != nil {
		return nil, err
	}
	var rows []ruleRow
	for _, p := range ps {
		its, err := r.Items().List(p.ID)
		if err != nil {
			return nil, err
		}
		for _, it := range its {
			log, err := r.Changes().Of(change.Ref{Kind: change.KindItem, ID: it.ID})
			if err != nil {
				return nil, err
			}
			rows = append(rows, earliest(it, log)...)
		}
	}
	slices.SortStableFunc(rows, func(a, b ruleRow) int { return b.rec.At.Compare(a.rec.At) })
	return rows, nil
}

// earliest は、アイテムのログの自動化の行。同じ変更は最も早い 1 行だけ。新しい行を先にする。
func earliest(it item.Item, log []change.Record) []ruleRow {
	var out []ruleRow
	for i := len(log) - 1; i >= 0; i-- {
		rec := log[i]
		if rec.Actor.Kind != change.ActorAutomation {
			continue
		}
		dup := slices.IndexFunc(out, func(o ruleRow) bool {
			return o.rec.F == rec.F && bytes.Equal(o.rec.From, rec.From) && bytes.Equal(o.rec.To, rec.To)
		})
		if dup >= 0 {
			out[dup].rec = rec // 古いほうへ置き換える(ログは古い順なので、さかのぼるほど早い)
			continue
		}
		out = append(out, ruleRow{rec: rec, item: it})
	}
	return out
}

func propertyOf(st workspace.Settings, f string) workspace.Property {
	for _, p := range st.Properties {
		if item.Field(p) == f {
			return p
		}
	}
	return workspace.Property{Name: f}
}

// memberNames は、ログイン名から表示名を引く。メンバーにいなければ「不明なユーザー」(01)。
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
		return "不明なユーザー"
	}, nil
}
