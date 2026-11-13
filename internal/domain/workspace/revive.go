package workspace

import (
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// 別の画面で削除された設定への保存は、変更を残して削除を取り消す(06「他の書き手との重なり」)。
// 下の関数は、その取り消しで使う、ワークスペースのログの読み方。

// Covering は、項目名 f を覆う削除の行(f が同じか、行の f + "/" が f の頭に当たるもの)のうち、
// 取り消されていないものを、ログの順に返す。テンプレートの子なら、子とテンプレートの両方の行。
func Covering(log []change.Record, f string) []change.Record {
	restored := change.Restored(log)
	var out []change.Record
	for _, r := range log {
		if r.T == change.Delete && !restored(r) && (r.F == f || strings.HasPrefix(f, r.F+"/")) {
			out = append(out, r)
		}
	}
	return out
}

// SettingsReviving は、fs のそれぞれを覆う削除の行がないものとして、ログから設定を作る。
// 取り消す前の設定を、削除済みのものの名前を含めて読むのに使う。log は変えない。
func SettingsReviving(log []change.Record, fs ...string) (Settings, bool) {
	var drop []change.ID
	for _, f := range fs {
		for _, r := range Covering(log, f) {
			drop = append(drop, r.ID)
		}
	}
	kept := make([]change.Record, 0, len(log))
	for _, r := range log {
		if !slices.Contains(drop, r.ID) {
			kept = append(kept, r)
		}
	}
	return SettingsFromLog(kept)
}

// CheckRevivedName は、取り消した f(削除の行の f)のプロパティ・値の名前が、同じ一覧のほかのものと重ならないかを
// 確かめる(作成・名前の変更と同じ比べ方)。テンプレート・ルールなどほかのものと、見つからないものは nil。
func CheckRevivedName(st Settings, f string) error {
	parts := strings.Split(f, "/")
	if len(parts) == 2 && parts[0] == "properties" {
		p, ok := st.Property(change.ID(parts[1]))
		if !ok {
			return nil
		}
		return CheckPropertyName(st, p.ID, p.Name)
	}
	var l List
	switch {
	case len(parts) == 2 && slices.Contains(valueLists, parts[0]):
		l = List{Kind: ListKind(parts[0])}
	case len(parts) == 3 && parts[0] == string(ListChoices):
		l = List{Kind: ListChoices, Property: change.ID(parts[1])}
	default:
		return nil
	}
	vs, _ := st.Values(l)
	i := slices.IndexFunc(vs, func(v Value) bool { return v.ID == change.ID(parts[len(parts)-1]) })
	if i < 0 {
		return nil
	}
	return CheckValueName(vs, vs[i].ID, vs[i].Name)
}
