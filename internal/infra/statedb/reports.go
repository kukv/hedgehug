package statedb

import (
	"context"
	"database/sql"
	"errors"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

func init() {
	register("reports/001", `CREATE TABLE report_selections (
		store     TEXT NOT NULL,
		workspace TEXT NOT NULL,
		unit      TEXT NOT NULL,
		sprint    TEXT NOT NULL,
		layers    TEXT NOT NULL,
		estimate  TEXT NOT NULL,
		PRIMARY KEY (store, workspace)
	)`)
}

// ReportSelections は、レポートの選択(F3-5)。Store ID の列を持つので、Store ごとの行の削除がこの表にも効く。
func (d *DB) ReportSelections() ReportSelections { return ReportSelections{d.db} }

// ReportSelections は、ワークスペースごとのレポートの選択を state.db に置く。
// 渡された値をそのまま保ち、既定への直しは読む側(app/reports)が行う。
type ReportSelections struct{ db *sql.DB }

// Get は、保存した選択を返す。なければ 2 つ目の値が false。
func (r ReportSelections) Get(ctx context.Context, store port.StoreID, ws workspace.ID) (planning.Selection, bool, error) {
	var unit, sprint, layers, estimate string
	err := r.db.QueryRowContext(ctx,
		`SELECT unit, sprint, layers, estimate FROM report_selections WHERE store = ? AND workspace = ?`,
		string(store), string(ws)).Scan(&unit, &sprint, &layers, &estimate)
	if errors.Is(err, sql.ErrNoRows) {
		return planning.Selection{}, false, nil
	}
	if err != nil {
		return planning.Selection{}, false, err
	}
	return planning.Selection{
		Unit:     planning.Unit(unit),
		Sprint:   change.ID(sprint),
		Layers:   planning.Layers(layers),
		Estimate: change.ID(estimate),
	}, true, nil
}

// Put は、選択を置く(あれば上書きする)。
func (r ReportSelections) Put(ctx context.Context, store port.StoreID, ws workspace.ID, sel planning.Selection) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO report_selections (store, workspace, unit, sprint, layers, estimate)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (store, workspace) DO UPDATE SET
			unit = excluded.unit, sprint = excluded.sprint, layers = excluded.layers, estimate = excluded.estimate`,
		string(store), string(ws), string(sel.Unit), string(sel.Sprint), string(sel.Layers), string(sel.Estimate))
	return err
}
