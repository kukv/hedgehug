package workspaces

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// edit は、設定の 1 回の変更(01「書き込みの単位」)。アクティビティの文は保存の前に決めるので、読んだ設定から
// summary で作る。行は、書く Tx の中で読み直した設定から rows で作る(検査もここで行う)。行がなければ何も
// 保存しない。rows はアイテムにも書いてよい(同じ保存に入る)。
func (s Service) edit(ctx context.Context, store port.StoreID, ws workspace.ID,
	summary func(workspace.Settings) (string, error),
	rows func(tx port.Tx, st workspace.Settings) ([]change.Draft, error),
) error {
	return s.editAt(ctx, store, ws, nil, summary, rows)
}

// editAt は edit と同じで、targets(ワークスペースのログから、取り消してよい項目名の頭を返す)があれば、
// 別の画面で削除されたそれらへの保存として、削除を取り消して変更を残す(06「他の書き手との重なり」)。
// 保存の前の読みは、取り消す削除の行がないものとして作った設定(決定 R10)。取り消したのに行がなければ
// ErrNotFound(決定 R3)、取り消したものの名前が削除されていないものと重なれば断る(決定 R4)。どちらも何も残さない。
// targets は、保存の前の読みで 1 回、書く Tx の中で 1 回呼ぶ。Tx の中では rows より先に呼ぶ。
// ワークスペースが別の画面で削除されていれば、その削除も取り消す(決定 R11)。
func (s Service) editAt(ctx context.Context, store port.StoreID, ws workspace.ID,
	targets func(log []change.Record) []string,
	summary func(workspace.Settings) (string, error),
	rows func(tx port.Tx, st workspace.Settings) ([]change.Draft, error),
) error {
	return s.save(ctx, store, ws, true, targets, summary, rows)
}

// remove は edit と同じで、削除の操作(値・プロパティ・テンプレートの削除)が使う。削除済みのワークスペースは
// 取り消さずに ErrNotFound(決定 R11)。
func (s Service) remove(ctx context.Context, store port.StoreID, ws workspace.ID,
	summary func(workspace.Settings) (string, error),
	rows func(tx port.Tx, st workspace.Settings) ([]change.Draft, error),
) error {
	return s.save(ctx, store, ws, false, nil, summary, rows)
}

// save は、editAt・remove の中身。revive が真なら、削除済みのワークスペースの削除を取り消す。
func (s Service) save(ctx context.Context, store port.StoreID, ws workspace.ID, revive bool,
	targets func(log []change.Record) []string,
	summary func(workspace.Settings) (string, error),
	rows func(tx port.Tx, st workspace.Settings) ([]change.Draft, error),
) error {
	actor, err := s.me(ctx, store)
	if err != nil {
		return err
	}
	var st workspace.Settings
	if err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		if _, err := workspaceToSave(r, ws, revive); err != nil {
			return err
		}
		var err error
		st, err = settingsReviving(r, ws, targets)
		return err
	}); err != nil {
		return err
	}
	text, err := summary(st)
	if err != nil {
		return err
	}
	return s.Stores.Write(ctx, store, port.Operation{Actor: actor, Summary: text}, func(tx port.Tx) error {
		return writeReviving(ctx, tx, ws, revive, targets, rows)
	})
}

// workspaceToSave は、保存の前の読みで、ワークスペース ws を返す。revive なら削除済みも返す。
func workspaceToSave(r port.Reader, ws workspace.ID, revive bool) (workspace.Workspace, error) {
	if revive {
		return port.EditableWorkspace(r, ws)
	}
	return port.LiveWorkspace(r, ws)
}

// reviveWorkspace は、書く Tx の中で、revive ならワークスペース ws の削除を取り消し(取り消したら true)、
// そうでなければ削除済みを ErrNotFound で断る。
func reviveWorkspace(ctx context.Context, tx port.Tx, ws workspace.ID, revive bool) (bool, error) {
	if revive {
		return port.ReviveWorkspace(ctx, tx, ws, "", "")
	}
	_, err := port.LiveWorkspace(tx, ws)
	return false, err
}

// writeReviving は、save の書く Tx の中身。ワークスペース(revive のとき)と targets のものの削除を取り消し、
// 読み直した設定から rows で行を書く。
func writeReviving(ctx context.Context, tx port.Tx, ws workspace.ID, revive bool,
	targets func([]change.Record) []string,
	rows func(tx port.Tx, st workspace.Settings) ([]change.Draft, error),
) error {
	wsRevived, err := reviveWorkspace(ctx, tx, ws, revive)
	if err != nil {
		return err
	}
	revived, err := reviveTargets(ctx, tx, ws, targets)
	if err != nil {
		return err
	}
	st, err := tx.Workspaces().Settings(ws)
	if err != nil {
		return err
	}
	ds, err := rows(tx, st)
	if err != nil {
		return err
	}
	if (wsRevived || len(revived) > 0) && len(ds) == 0 {
		return port.ErrNotFound
	}
	if err := tx.WriteWorkspaces().Append(ws, ds...); err != nil {
		return err
	}
	if len(revived) > 0 {
		return checkRevivedNames(tx, ws, revived)
	}
	return nil
}

// settingsReviving は、targets の項目名を覆う削除の行がないものとして読んだ設定。targets がなければ、いまの設定。
func settingsReviving(r port.Reader, ws workspace.ID, targets func([]change.Record) []string) (workspace.Settings, error) {
	if targets == nil {
		return r.Workspaces().Settings(ws)
	}
	log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: ws})
	if err != nil {
		return workspace.Settings{}, err
	}
	st, ok := workspace.SettingsReviving(log, targets(log)...)
	if !ok {
		return workspace.Settings{}, port.ErrNotFound
	}
	return st, nil
}

// reviveTargets は、tx の中で読んだログから targets の項目名を作り、そのそれぞれに port.ReviveSetting を呼ぶ。
// 削除を取り消した項目名を返す。
func reviveTargets(ctx context.Context, tx port.Tx, ws workspace.ID, targets func([]change.Record) []string) ([]string, error) {
	if targets == nil {
		return nil, nil
	}
	log, err := tx.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: ws})
	if err != nil {
		return nil, err
	}
	var revived []string
	for _, f := range targets(log) {
		ok, err := port.ReviveSetting(ctx, tx, ws, f)
		if err != nil {
			return nil, err
		}
		if ok {
			revived = append(revived, f)
		}
	}
	return revived, nil
}

// checkRevivedNames は、行を書いたあとの設定で、削除を取り消した fs のそれぞれの名前が重ならないかを確かめる(決定 R4)。
func checkRevivedNames(tx port.Tx, ws workspace.ID, fs []string) error {
	st, err := tx.Workspaces().Settings(ws)
	if err != nil {
		return err
	}
	for _, f := range fs {
		if err := workspace.CheckRevivedName(st, f); err != nil {
			return err
		}
	}
	return nil
}
