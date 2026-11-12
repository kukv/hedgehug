// Package member は、Store のメンバー(担当者・オーナー・メンションの候補)を持つ。
package member

import (
	"encoding/json"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// Member は、メンバーの 1 人。ログイン名は大文字と小文字を区別せずに比べる(01)。
type Member struct {
	Login string
	Name  string // 表示名。初期値はログイン名
}

var loginPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,39}$`)

// ValidLogin は、ログイン名に使えるか(英数字・ハイフン・アンダースコア、39 文字以内。07・08)。
func ValidLogin(s string) bool { return loginPattern.MatchString(s) }

// Init は、メンバーの一覧のログを始める作成の変更。Store を作った最初の保存で書く。
func Init() change.Draft { return change.NewCreate(map[string]any{"members": []string{}}) }

// Join は、メンバーの一覧への追加。
func Join(login string) change.Draft { return change.NewAdd("members", login) }

// Rename は、表示名の変更。
func Rename(login, from, to string) change.Draft {
	return change.NewSet("name/"+strings.ToLower(login), from, to)
}

var opts = change.Options{
	Multi: func(f string) bool { return f == "members" },
	Valid: func(r change.Record) bool {
		if r.F != "members" || r.T != change.Add {
			return true
		}
		var login string
		return json.Unmarshal(r.To, &login) == nil && ValidLogin(login)
	},
}

// FromLog は、メンバーの一覧のログから、ログイン名(小文字)の順の一覧を作る。
// 大文字と小文字だけが違うログイン名は同じメンバーとし、先に追加した綴りを残す。
func FromLog(log []change.Record) []Member {
	s := change.Resolve(log, opts)
	present := map[string]bool{}
	for _, raw := range s.Lists["members"] {
		var login string
		if json.Unmarshal(raw, &login) == nil {
			present[login] = true
		}
	}
	first := map[string]spelling{}
	for _, r := range log {
		var login string
		if r.F != "members" || r.T != change.Add || json.Unmarshal(r.To, &login) != nil || !present[login] {
			continue
		}
		key := strings.ToLower(login)
		if e, ok := first[key]; !ok || r.At.Before(e.at) || (r.At.Equal(e.at) && r.ID < e.id) {
			first[key] = spelling{login: login, at: r.At, id: r.ID}
		}
	}
	out := make([]Member, 0, len(first))
	for key, sp := range first {
		m := Member{Login: sp.login, Name: sp.login}
		if v, ok := s.Values["name/"+key]; ok {
			_ = json.Unmarshal(v, &m.Name)
		}
		out = append(out, m)
	}
	slices.SortFunc(out, func(a, b Member) int {
		return strings.Compare(strings.ToLower(a.Login), strings.ToLower(b.Login))
	})
	return out
}

// spelling は、ログイン名の綴りと、それを追加した行の時刻と ID。
type spelling struct {
	login string
	at    time.Time
	id    change.ID
}
