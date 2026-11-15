package activity

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const unknownUser = "不明なユーザー"

// entityRow は、コミットで足された行 1 つと、それを持つエンティティ。
type entityRow struct {
	ref change.Ref
	rec change.Record
}

// commitRows は、保存のコミット 1 つから、本体・自動化・メンバーの追加の行を作る(C4d-3〜C4d-9・C4d-12)。
func commitRows(r port.Reader, store port.StoreInfo, c port.Commit) ([]Row, error) {
	v, err := readView(r)
	if err != nil {
		return nil, err
	}
	return v.rows(store, c)
}

// rows は、コミット c の行。並びは本体、自動化の行、メンバーの追加の行で、鍵の連番もこの順。
func (v *view) rows(store port.StoreInfo, c port.Commit) ([]Row, error) {
	recs := opRows(c)
	actor := mainActor(recs, c.Author, c.Summary)
	base := Row{At: c.At, Store: store, SHA: c.SHA[:min(len(c.SHA), 7)]}
	main := base
	main.Text = c.Summary
	setActor(&main, actor, v.names)
	refs := refsOf(recs, c.Removed)
	if err := v.place(&main, refs, hasMembers(refs) || c.StoreLog || actor.Kind == change.ActorNone); err != nil {
		return nil, err
	}
	if actor.Kind != change.ActorUser {
		main.dedup = dedupKey(store.ID, actor, recs, c.Summary, c.Removed)
	}
	out := []Row{main}
	for _, er := range recs {
		if er.rec.Actor.Kind != change.ActorAutomation {
			continue
		}
		row, err := v.automationRow(base, store.ID, er)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	out = append(out, memberRows(base, store.ID, actor, recs, refs, v.names)...)
	for i := range out {
		out[i].Key = fmt.Sprintf("%s:%06d", c.SHA, i)
	}
	return out, nil
}

// opRows は、行に使う変更記録(C4d-3): コミットの op と同じ op の行。1 行もなければ、足されたすべての行。
func opRows(c port.Commit) []entityRow {
	var same, all []entityRow
	for _, e := range c.Changes {
		for _, rec := range e.Rows {
			er := entityRow{ref: e.Ref, rec: rec}
			all = append(all, er)
			if rec.Op == c.Op {
				same = append(same, er)
			}
		}
	}
	if len(same) > 0 {
		return same
	}
	return all
}

// purgeSummary は、保持日数を過ぎたものの完全な削除の件名の書き出し(06 の文)。
const purgeSummary = "保持日数を過ぎた削除済み "

// mainActor は、本体の実行者(C4d-5)。自動化でない最初の行の実行者。なければ、作者が bot なら同期、ほかは作者の人。
// 行が 1 行もない bot のコミットのうち、件名が保持日数を過ぎたものの完全な削除のものは実行者なし(完全な削除の記録は
// 実行者を持たないので、件名で同期の修復の完全な削除と見分ける)。
// op の行がすべて自動化の行で作者が bot のコミットは、GitHub か同期かを件名からも行からも決められないが、起きない:
// ルールの行を書けるのは人と GitHub の操作だけ(port.CheckRuleDrafts)。人の操作は作者が人なので人になり(止まった
// 自動化の「制約を無視して…」は自動化の行だけを人が書く)、GitHub の取り直しと自動リンクは、ルールを動かす前に
// 必ず同じ保存でリンクの行(実行者 GitHub)を書く(githublink の fire)。
func mainActor(recs []entityRow, author, summary string) change.Actor {
	for _, er := range recs {
		if er.rec.Actor.Kind != change.ActorAutomation {
			return er.rec.Actor
		}
	}
	if author == change.BotLogin {
		if len(recs) == 0 && strings.HasPrefix(summary, purgeSummary) {
			return change.Actor{Kind: change.ActorNone}
		}
		return change.Actor{Kind: change.ActorSync}
	}
	return change.User(author)
}

// setActor は、行の実行者と名前とアバター(C4d-12)。
func setActor(row *Row, a change.Actor, names item.Names) {
	row.Actor = ActorKind(a.Kind)
	switch a.Kind {
	case change.ActorUser:
		row.Who = names(a.Login)
		if row.Who != unknownUser {
			row.Avatar = row.Who
		}
	case change.ActorAutomation:
		row.Who = "自動化"
	case change.ActorGitHub:
		row.Who = "GitHub"
	case change.ActorSync:
		row.Who = "同期"
	}
}

// refsOf は、対象とワークスペースを決めるエンティティ(C4d-3): 行を持つものと Removed のもの。重ねず、出てきた順。
func refsOf(recs []entityRow, removed []change.Ref) []change.Ref {
	var out []change.Ref
	for _, er := range recs {
		if !slices.Contains(out, er.ref) {
			out = append(out, er.ref)
		}
	}
	for _, ref := range removed {
		if !slices.Contains(out, ref) {
			out = append(out, ref)
		}
	}
	return out
}

func hasMembers(refs []change.Ref) bool {
	return slices.ContainsFunc(refs, func(r change.Ref) bool { return r.Kind == change.KindMembers })
}

// automationRow は、自動化の行 1 つの行(C4d-4 の (2))。ルール名と値の名前は、ルールを持つワークスペースで引く
// (同じ Store の別のワークスペースへ移したアイテムでも、値の ID はルールのワークスペースのもの)。
func (v *view) automationRow(base Row, store port.StoreID, er entityRow) (Row, error) {
	row := base
	setActor(&row, er.rec.Actor, v.names)
	refs := []change.Ref{er.ref}
	if err := v.place(&row, refs, false); err != nil {
		return Row{}, err
	}
	rule := v.rules[er.rec.Actor.Rule]
	st := v.settings[rule.workspace]
	if rule.workspace == "" && len(row.Workspaces) == 1 {
		st = v.settings[row.Workspaces[0]] // 名前の分からないルールは、アイテムのいまのワークスペースで引く
	}
	p := propertyOf(st, er.rec.F)
	value := item.Display(p, er.rec.To, st, v.names, base.At)
	row.Text = automation.ActivityText(rule.name, automation.ChangeText(p.Name, value))
	row.dedup = dedupKey(store, er.rec.Actor, []entityRow{er}, "", nil)
	return row, nil
}

// memberRows は、メンバーの追加の行(C4d-4 の (3))。members のほかにも行を足したコミットの、members の add の行ごとに
// 1 行。members だけのコミットは本体の件名がすでに「メンバー…」なので作らない。Store 全体の記録で、対象はない。
func memberRows(base Row, store port.StoreID, actor change.Actor, recs []entityRow, refs []change.Ref, names item.Names) []Row {
	if !slices.ContainsFunc(refs, func(r change.Ref) bool { return r.Kind != change.KindMembers }) {
		return nil
	}
	var out []Row
	for _, er := range recs {
		if er.ref.Kind != change.KindMembers || er.rec.T != change.Add || er.rec.F != "members" {
			continue
		}
		var login string
		if json.Unmarshal(er.rec.To, &login) != nil {
			continue
		}
		row := base
		setActor(&row, actor, names)
		row.Text = fmt.Sprintf("メンバー %s を追加", login)
		row.Storewide = true
		if actor.Kind != change.ActorUser {
			row.dedup = dedupKey(store, actor, []entityRow{er}, "", nil)
		}
		out = append(out, row)
	}
	return out
}

// dedupKey は、重なりの鍵(C4d-11): Store・実行者(自動化はルールまで)と、行の (エンティティ・項目・from・to・Prev)
// の集合。行がなければ件名と、消したもの(removed)の集合。06 がまとめるのは複数のマシンから届いた同じ変更だけ:
// 2 台が同じ版から作った変更は Prev が同じで、順に起きた繰り返しは Prev が違う。同じものを 2 度消すことはないので、
// 別のときの同じ件名の完全な削除は、消したものが違う。
func dedupKey(store port.StoreID, actor change.Actor, recs []entityRow, summary string, removed []change.Ref) string {
	parts := make([]string, 0, len(recs))
	for _, er := range recs {
		prev := make([]string, len(er.rec.Prev))
		for i, id := range er.rec.Prev {
			prev[i] = string(id)
		}
		slices.Sort(prev)
		parts = append(parts, strings.Join([]string{
			string(er.ref.Kind), string(er.ref.ID), er.rec.F, string(compact(er.rec.From)), string(compact(er.rec.To)),
			strings.Join(prev, ","),
		}, "\x1f"))
	}
	if len(parts) == 0 {
		parts = append(parts, "summary\x1f"+summary)
		for _, ref := range removed {
			parts = append(parts, "removed\x1f"+string(ref.Kind)+"\x1f"+string(ref.ID))
		}
	}
	slices.Sort(parts)
	return string(store) + "\x1e" + actor.Code() + "\x1e" + strings.Join(parts, "\x1e")
}

// compact は、JSON の空白をそろえる(同じ値を同じ鍵にする)。読めなければそのまま。
func compact(raw json.RawMessage) []byte {
	var b bytes.Buffer
	if json.Compact(&b, raw) != nil {
		return raw
	}
	return b.Bytes()
}

// propertyOf は、項目名 f のプロパティ。設定になければ、名前を f にしたもの(app/automations と同じ)。
func propertyOf(st workspace.Settings, f string) workspace.Property {
	for _, p := range st.Properties {
		if item.Field(p) == f {
			return p
		}
	}
	return workspace.Property{Name: f}
}
