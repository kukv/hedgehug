package repairing

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/repair"
)

// unlinkIssues は、重なった Issue のリンクを外す(決定 C5c2-2〜C5c2-5)。外すリンク 1 本を 1 回の保存にする。
func unlinkIssues(ctx context.Context, w port.RepairWriter) error {
	s, err := readSnapshot(w)
	if err != nil {
		return err
	}
	displays := map[item.ID]string{}
	var all []repair.IssueLink
	for _, en := range s.entities {
		if en.ref.Kind != change.KindItem {
			continue
		}
		if it, ok := item.FromLog(en.log); !ok || it.Deleted != nil {
			continue
		}
		displays[en.ref.ID] = en.display
		all = append(all, issueLinksOf(en)...)
	}
	for _, l := range repair.IssueUnlinks(all, w.Facts()) {
		if err := ctx.Err(); err != nil {
			return err
		}
		ref := change.Ref{Kind: change.KindItem, ID: l.Item}
		op := port.Operation{Actor: syncActor, Summary: itemlink.RemoveSummary(displays[l.Item] + " の " + l.Ref.Display())}
		drafts := itemlink.Set{}.Detach(itemlink.Link{Ref: l.Ref})
		if err := w.Write(op, func(tx port.Tx) error { return appendTo(tx, ref, drafts...) }); err != nil {
			return err
		}
	}
	return nil
}

// issueLinksOf は、アイテム en のいまの Issue のリンクを、リンクした行(決定 C5c2-17)つきで返す。
func issueLinksOf(en entity) []repair.IssueLink {
	var out []repair.IssueLink
	for _, l := range itemlink.FromLog(en.log).Links {
		if l.Kind != itemlink.Issue {
			continue
		}
		raw, err := json.Marshal(l.Ref)
		if err != nil {
			continue
		}
		row, ok := change.ListRow(en.log, itemlink.LinksField, raw)
		if !ok {
			continue
		}
		out = append(out, repair.IssueLink{Item: en.ref.ID, Ref: l.Ref, Linking: itemlink.Origin(en.log, row)})
	}
	return out
}

// mergeMembers は、大文字小文字だけが違うメンバーの綴りを統合する(決定 C5c2-6)。1 組を 1 回の保存にする。
func mergeMembers(ctx context.Context, w port.RepairWriter) error {
	ref := change.Ref{Kind: change.KindMembers}
	var merges []repair.MemberMerge
	err := w.Read(func(r port.Reader) error {
		log, err := r.Changes().Of(ref)
		if err != nil && !errors.Is(err, port.ErrNotFound) {
			return err
		}
		merges = repair.MemberMerges(spellingsOf(log), w.Facts())
		return nil
	})
	if err != nil {
		return err
	}
	for _, m := range merges {
		if err := ctx.Err(); err != nil {
			return err
		}
		drafts := make([]change.Draft, 0, len(m.Drop))
		for _, login := range m.Drop {
			drafts = append(drafts, change.NewRemove("members", login))
		}
		op := port.Operation{Actor: syncActor, Summary: "メンバー " + m.Keep + " の重複を統合"}
		if err := w.Write(op, func(tx port.Tx) error { return appendTo(tx, ref, drafts...) }); err != nil {
			return err
		}
	}
	return nil
}

// spellingsOf は、メンバーの一覧のログのいまの綴り(使える綴りだけ)と、綴りを決めた行。
func spellingsOf(log []change.Record) []repair.Spelling {
	var out []repair.Spelling
	for _, raw := range change.ListOf(log, "members") {
		var login string
		if json.Unmarshal(raw, &login) != nil || !member.ValidLogin(login) {
			continue
		}
		if row, ok := change.ListRow(log, "members", raw); ok {
			out = append(out, repair.Spelling{Login: login, Row: row})
		}
	}
	return out
}
