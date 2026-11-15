package noticing

import (
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/repair"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// restoreText は、取り消した削除 d の通知の本文(決定 C5b2-9)。d がワークスペースの値の削除で、修復の直前のログ before で
// 設定の項目が指している(既定にした人を添える)か、分類の最後の値なら 06 の本文、ほかは重なりの本文(変更した人を添える)。
func restoreText(r port.Reader, ref change.Ref, before []change.Record, d change.Record, target string, sides repair.Facts) (string, error) {
	if ref.Kind == change.KindWorkspace {
		if reason, ok := repair.ReasonOf(before, d.F); ok {
			return reasonText(r, before, d.F, target, reason)
		}
	}
	who, err := memberName(r, repair.Changer(before, d, sides))
	if err != nil {
		return "", err
	}
	return withParticle(target, "は") + " " + who + " が同時に変更したため、削除を取り消しました", nil
}

// reasonText は、値 f の削除を、reason のために取り消した通知の本文。
func reasonText(r port.Reader, before []change.Record, f, target string, reason repair.RestoreReason) (string, error) {
	if reason.Field == "" {
		return withParticle(target, "は") + workspace.CategoryName(reason.Category) + "の最後のステータスのため、削除を取り消しました", nil
	}
	// ReasonOf が true を返したので、f は値の項目(statuses/<ID>・priorities/<ID>)。
	_, id, _ := strings.Cut(f, "/")
	who, err := memberName(r, setter(before, reason.Field, id))
	if err != nil {
		return "", err
	}
	return withParticle(target, "は") + " " + who + " が同時に既定にしたため、削除を取り消しました", nil
}

// setter は、log の項目 field を値 id にした人(人の set の行の最後の行の by)。なければ空。
func setter(log []change.Record, field, id string) string {
	rows := rowsWhere(log, func(c change.Record) bool {
		return c.T == change.Set && c.F == field && sameValue(c.To, change.Value(id))
	})
	return lastSaver(rows, "")
}
