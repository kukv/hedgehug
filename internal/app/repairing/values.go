package repairing

import (
	"context"
	"errors"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/repair"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// wsFix は、ワークスペース ws に書く修復の保存 1 つ。
type wsFix struct {
	ws      workspace.ID
	summary string
	drafts  []change.Draft
}

// dropDanglingRefs は、削除された値を指すプロジェクト別のルール・種類ごとの既定のテンプレート・ルールを直す
// (決定 C5b2-6)。RefRepair 1 つを 1 回の保存にする。
func dropDanglingRefs(ctx context.Context, w port.RepairWriter) error {
	return writeFixes(ctx, w, readRefFixes)
}

// resetFinishedInitial は、完了・中止の分類にされた初期ステータスを、未着手の既定に戻す。
// ワークスペースごとに 1 回の保存にする。
func resetFinishedInitial(ctx context.Context, w port.RepairWriter) error {
	return writeFixes(ctx, w, readInitialFixes)
}

// writeFixes は、read がワークスペースごとに返す保存を、ワークスペースの順に書く。
func writeFixes(ctx context.Context, w port.RepairWriter, read func(port.Reader, workspace.ID, []change.Record) ([]wsFix, error)) error {
	var fixes []wsFix
	err := w.Read(func(r port.Reader) error {
		var err error
		fixes, err = collectFixes(r, read)
		return err
	})
	if err != nil {
		return err
	}
	for _, f := range fixes {
		if err := ctx.Err(); err != nil {
			return err
		}
		op := port.Operation{Actor: syncActor, Summary: f.summary}
		if err := w.Write(op, func(tx port.Tx) error { return tx.WriteWorkspaces().Append(f.ws, f.drafts...) }); err != nil {
			return err
		}
	}
	return nil
}

// collectFixes は、すべてのワークスペースの保存を、ワークスペースの順に集める。
func collectFixes(r port.Reader, read func(port.Reader, workspace.ID, []change.Record) ([]wsFix, error)) ([]wsFix, error) {
	wss, err := r.Workspaces().List()
	if err != nil {
		return nil, err
	}
	var out []wsFix
	for _, ws := range wss {
		log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: ws.ID})
		if err != nil {
			return nil, err
		}
		fs, err := read(r, ws.ID, log)
		if err != nil {
			return nil, err
		}
		out = append(out, fs...)
	}
	return out, nil
}

// readRefFixes は、ワークスペース ws の log にある、削除された値を指すものの修復。
func readRefFixes(r port.Reader, ws workspace.ID, log []change.Record) ([]wsFix, error) {
	var out []wsFix
	for _, rr := range repair.DanglingRefs(log) {
		var s string
		switch rr.Kind {
		case repair.RefProject:
			title := string(rr.Project)
			if p, err := r.Projects().Get(rr.Project); err == nil {
				title = p.Title
			} else if !errors.Is(err, port.ErrNotFound) {
				return nil, err
			}
			s = "プロジェクト「" + oneLine(title) + "」のルールから「" + oneLine(deletedName(log, rr)) + "」を削除(削除された値)"
		case repair.RefTemplateDefault:
			s = rr.TemplateKind + " の既定のテンプレートを変更(削除された値)"
		case repair.RefRule:
			s = "自動化「" + oneLine(automation.Names(log)[rr.Rule]) + "」を無効にする(削除された値)"
		}
		out = append(out, wsFix{ws: ws, summary: s, drafts: rr.Drafts})
	}
	return out, nil
}

// deletedName は、プロジェクト別のルールが指す削除された値の名前。ステータスで見つからなければテンプレート。
func deletedName(log []change.Record, rr repair.RefRepair) string {
	if n := workspace.NameOfDeleted(log, "statuses/"+string(rr.Value)); n != "" {
		return n
	}
	return workspace.NameOfDeleted(log, workspace.TemplateField(rr.Value))
}

// readInitialFixes は、完了・中止の初期ステータスを戻す修復。
func readInitialFixes(_ port.Reader, ws workspace.ID, log []change.Record) ([]wsFix, error) {
	d, ok := repair.FinishedInitial(log)
	if !ok {
		return nil, nil
	}
	st, _ := workspace.SettingsFromLog(log)
	var name string
	for _, v := range st.Statuses {
		if v.ID == st.CategoryDefaults[workspace.CategoryTodo] {
			name = v.Name
		}
	}
	return []wsFix{{ws: ws, summary: "初期ステータスを「" + oneLine(name) + "」に戻し", drafts: []change.Draft{d}}}, nil
}

// restoreReferenced は、設定が指している値(初期ステータス・分類の既定・初期優先度)や、
// 0 個になる分類のステータスの削除を取り消す(決定 C5b2-3・C5b2-4)。
// 削除の op ごとに 1 回保存し、その op の行があるログごとに change.Undo の行を書く。
func restoreReferenced(ctx context.Context, w port.RepairWriter) error {
	s, err := readSnapshot(w)
	if err != nil {
		return err
	}
	for _, f := range valueRestores(s, w.Facts()) {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := undo(w, s, f); err != nil {
			return err
		}
	}
	return nil
}

// valueRestores は、すべてのワークスペースの取り消す値の削除を、削除の op の辞書順に返す。
// 同じ op は、最初に見つけたログのもの 1 つ。
func valueRestores(s snapshot, facts repair.Facts) []found {
	byOp := map[change.ID]found{}
	for i, e := range s.entities {
		if e.ref.Kind != change.KindWorkspace {
			continue
		}
		for _, o := range repair.ValueRestores(e.log, facts) {
			if _, ok := byOp[o.Op]; !ok {
				byOp[o.Op] = found{Overlap: o, at: i}
			}
		}
	}
	return sortedByOp(byOp)
}
