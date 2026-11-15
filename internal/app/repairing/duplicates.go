package repairing

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/repair"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// renameFix は、重なった名前・接頭辞・キーの直し 1 つ。ref は set を書くログ、kind は件名の {種類}(決定 C5c1-7)。
type renameFix struct {
	ref    change.Ref
	kind   string
	rename repair.Rename
}

// renameDuplicates は、重なったワークスペースの名前・接頭辞、プロジェクトの名前、設定の名前・キーを直す
// (決定 C5c1-4〜C5c1-8)。順は C5c1-6 のとおり。直し 1 つを 1 回の保存にする。
func renameDuplicates(ctx context.Context, w port.RepairWriter) error {
	var fixes []renameFix
	err := w.Read(func(r port.Reader) error {
		var err error
		fixes, err = collectRenames(r, w.Facts())
		return err
	})
	if err != nil {
		return err
	}
	for _, f := range fixes {
		if err := ctx.Err(); err != nil {
			return err
		}
		n := f.rename
		op := port.Operation{Actor: syncActor, Summary: f.kind + "「" + oneLine(n.Value) + "」を「" + oneLine(n.To) + "」に変更"}
		if err := w.Write(op, func(tx port.Tx) error { return appendTo(tx, f.ref, change.NewSet(n.Field, n.Value, n.To)) }); err != nil {
			return err
		}
	}
	return nil
}

// collectRenames は、すべての直しを、書く順に集める。ワークスペースの名前 → 接頭辞 →
// ワークスペースの内部 ID 順に、プロジェクト → 設定。
func collectRenames(r port.Reader, facts repair.Facts) ([]renameFix, error) {
	wss, err := port.LiveWorkspaces(r)
	if err != nil {
		return nil, err
	}
	var names, prefixes []repair.Named
	logs := map[workspace.ID][]change.Record{}
	for _, ws := range wss {
		ref := change.Ref{Kind: change.KindWorkspace, ID: ws.ID}
		log, err := r.Changes().Of(ref)
		if err != nil {
			return nil, err
		}
		logs[ws.ID] = log
		names = append(names, named(ref, "name", workspace.NormalizeName(ws.Name), log))
		prefixes = append(prefixes, named(ref, "prefix", workspace.NormalizePrefix(ws.Prefix), log))
	}
	out := fixesOf("ワークスペース", repair.NameRenames(names, facts, false))
	out = append(out, fixesOf("接頭辞", repair.PrefixRenames(prefixes, facts))...)
	for _, ws := range wss {
		ps, err := projectRenames(r, ws.ID, facts)
		if err != nil {
			return nil, err
		}
		out = append(append(out, ps...), settingsRenames(ws.ID, logs[ws.ID], facts)...)
	}
	return out, nil
}

// projectRenames は、ワークスペース ws の、重なったプロジェクトの名前の直し。
func projectRenames(r port.Reader, ws workspace.ID, facts repair.Facts) ([]renameFix, error) {
	ps, err := port.LiveProjects(r, ws)
	if err != nil {
		return nil, err
	}
	var all []repair.Named
	for _, p := range ps {
		ref := change.Ref{Kind: change.KindProject, ID: p.ID}
		log, err := r.Changes().Of(ref)
		if err != nil {
			return nil, err
		}
		all = append(all, named(ref, "title", workspace.NormalizeName(p.Title), log))
	}
	return fixesOf("プロジェクト", repair.NameRenames(all, facts, false)), nil
}

// named は、ref のログ log の項目 f の値 v を、重なりを数える 1 つにする。
func named(ref change.Ref, f, v string, log []change.Record) repair.Named {
	row, _ := repair.NamingRow(log, f)
	return repair.Named{Ref: ref, Field: f, Value: v, Naming: row}
}

// fixesOf は、エンティティ自身のログに書く直しにする。
func fixesOf(kind string, rs []repair.Rename) []renameFix {
	out := make([]renameFix, 0, len(rs))
	for _, n := range rs {
		out = append(out, renameFix{ref: n.Ref, kind: kind, rename: n})
	}
	return out
}
