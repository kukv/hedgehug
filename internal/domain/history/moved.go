package history

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// Origin は、同じ Store の中で別のワークスペースから移ってきたアイテムの履歴の材料(A5a-17)。関数が nil なら
// その材料はない(引けない)ものとする。Transfers は、このアイテムを Target に持つ、この Store の転送記録。
type Origin struct {
	Settings      func(workspace.ID) (workspace.Settings, bool)
	WorkspaceName func(workspace.ID) (string, bool)
	ProjectTitle  func(project.ID) (string, bool)
	StoreName     string
	Transfers     []transfer.Record
}

func (o Origin) settings(id workspace.ID) (workspace.Settings, bool) {
	if o.Settings == nil {
		return workspace.Settings{}, false
	}
	return o.Settings(id)
}

// workspaceName・projectTitle は、引けなければ内部 ID をそのまま返す(手で書き換えたときだけ起きる)。
func (o Origin) workspaceName(id workspace.ID) string {
	if o.WorkspaceName != nil {
		if name, ok := o.WorkspaceName(id); ok {
			return name
		}
	}
	return string(id)
}

func (o Origin) projectTitle(id project.ID) string {
	if o.ProjectTitle != nil {
		if title, ok := o.ProjectTitle(id); ok {
			return title
		}
	}
	return string(id)
}

const movedFormat = "%s から移動(%s / %s / %s)"

// movedInText は、moved_in の行(別の Store から移ってきた)の文。
func movedInText(r change.Record) (string, bool) {
	ms := transfer.MovedInOf([]change.Record{r})
	if len(ms) != 1 {
		return "", false
	}
	m := ms[0]
	return fmt.Sprintf(movedFormat, m.Old, m.FromStoreName, m.FromWorkspaceName, m.FromProjectTitle), true
}

// movedText は、ワークスペースが変わる場所の set の行の文。同じ操作に moved_in の行があれば(別の Store から)そちらが
// 文を持つので出さない。同じ Store の中の移動は、同じ操作の転送記録の旧 ID で作り、記録がなければ出さない。
func movedText(log []change.Record, r change.Record, c Context) (string, bool) {
	if slices.ContainsFunc(log, func(x change.Record) bool { return x.Op == r.Op && x.T == change.MovedIn }) {
		return "", false
	}
	i := slices.IndexFunc(c.Origin.Transfers, func(t transfer.Record) bool { return t.Op == r.Op })
	from, _, ok := places(r)
	if i < 0 || !ok {
		return "", false
	}
	return fmt.Sprintf(movedFormat, c.Origin.Transfers[i].Old, c.Origin.StoreName,
		c.Origin.workspaceName(from.Workspace), c.Origin.projectTitle(from.Project)), true
}

func places(r change.Record) (item.Place, item.Place, bool) {
	var from, to item.Place
	if json.Unmarshal(r.From, &from) != nil || json.Unmarshal(r.To, &to) != nil {
		return from, to, false
	}
	return from, to, true
}

// changesWorkspace は、場所の set の行で、ワークスペースが変わるか。
func changesWorkspace(r change.Record) bool {
	from, to, ok := places(r)
	return r.T == change.Set && r.F == "place" && ok && from.Workspace != to.Workspace
}

// trail は、ログの各行を書いたときにアイテムがいたワークスペース(作成と場所の set の to から追う)。
type trail struct {
	before  []workspace.ID             // 行 i の直前のワークスペース
	opFrom  map[change.ID]workspace.ID // 操作の最初の場所の set の from
	opTo    map[change.ID]workspace.ID // 操作の最後の場所の set の to
	current workspace.ID               // 読み終えたら、いまのワークスペース(Context.Settings の)
	movedIn []movedInAt                // moved_in の行
}

type movedInAt struct {
	i     int
	op    change.ID
	names map[change.ID]string
}

func newTrail(log []change.Record) trail {
	t := trail{before: make([]workspace.ID, len(log)), opFrom: map[change.ID]workspace.ID{}, opTo: map[change.ID]workspace.ID{}}
	for i, r := range log {
		t.before[i] = t.current
		t.read(i, r)
	}
	return t
}

// read は、行 i を読んで、いまのワークスペース(current)と、操作ごとの場所の変更と、moved_in を進める。
func (t *trail) read(i int, r change.Record) {
	switch r.T {
	case change.Create:
		var v struct{ Place item.Place }
		if json.Unmarshal(r.To, &v) == nil {
			t.current = v.Place.Workspace
		}
	case change.Set:
		from, to, ok := places(r)
		if !ok || r.F != "place" {
			return
		}
		if _, seen := t.opFrom[r.Op]; !seen {
			t.opFrom[r.Op] = from.Workspace
		}
		t.opTo[r.Op], t.current = to.Workspace, to.Workspace
	case change.MovedIn:
		if ms := transfer.MovedInOf([]change.Record{r}); len(ms) == 1 {
			t.movedIn = append(t.movedIn, movedInAt{i: i, op: r.Op, names: ms[0].Names})
		}
	}
}

// moving は、操作 op がワークスペースを変える(別のワークスペースへの移動)か。
func (t trail) moving(op change.ID) bool {
	from, ok := t.opFrom[op]
	return ok && from != t.opTo[op]
}

// vocabs は、行 i の from と to の値を引く定義。from は、その操作の最初の場所の set の from のワークスペース
// (場所の set がなければ行の直前のワークスペース)、to は、その操作の最後の場所の set の to のワークスペース。
func (t trail) vocabs(i int, r change.Record, c Context) (vocab, vocab) {
	from, ok := t.opFrom[r.Op]
	if !ok {
		from = t.before[i]
	}
	to, ok := t.opTo[r.Op]
	if !ok {
		to = t.before[i]
	}
	return t.vocab(from, i, r.Op, c), t.vocab(to, i, r.Op, c)
}

// vocab は、ワークスペース ws の定義。いまのワークスペース(追えないときも)は Context.Settings、同じ Store の
// ほかのワークスペースは Origin.Settings、どちらにもないもの(別の Store から来た)は、同じ操作か行 i より後で
// 最初の moved_in の names。
func (t trail) vocab(ws workspace.ID, i int, op change.ID, c Context) vocab {
	if ws == "" || ws == t.current {
		return vocab{settings: c.Settings}
	}
	if st, ok := c.Origin.settings(ws); ok {
		return vocab{settings: st}
	}
	v := vocab{settings: c.Settings, byNames: true}
	if j := slices.IndexFunc(t.movedIn, func(m movedInAt) bool { return m.op == op || m.i > i }); j >= 0 {
		v.names = t.movedIn[j].names
	}
	return v
}

// vocab は、値とプロパティの名前を引く定義。byNames なら、moved_in の names で引く(プロパティの型は settings、
// つまりいまのワークスペースの同じ内部 ID のプロパティから決める)。
type vocab struct {
	settings workspace.Settings
	names    map[change.ID]string
	byNames  bool
}

// property は、項目名 f が指すプロパティ。組み込みは項目名がキー(= 内部 ID)、追加は fields/<内部 ID>。
// names で引くとき、いまのワークスペースにないプロパティは、names に名前があれば型なしで返す。
func (v vocab) property(f string) (workspace.Property, bool) {
	id := change.ID(strings.TrimPrefix(f, item.FieldKey("")))
	p, ok := v.settings.Property(id)
	if !v.byNames {
		return p, ok
	}
	name, named := v.names[id]
	if !ok {
		return workspace.Property{ID: id, Name: name}, named
	}
	if named {
		p.Name = name
	}
	p.Choices = v.values()
	return p, true
}

// values は、names のすべての値(型を知らなくても、どの一覧で引いても名前が返る)。
func (v vocab) values() []workspace.Value {
	out := make([]workspace.Value, 0, len(v.names))
	for id, name := range v.names {
		out = append(out, workspace.Value{ID: id, Name: name})
	}
	return out
}

// display は、値 raw の表示。names にない値は「(削除された値)」。型なしのプロパティの値は、names にあればその名前、
// なければそのまま。
func (v vocab) display(p workspace.Property, raw json.RawMessage, c Context) string {
	if !v.byNames {
		return item.Display(p, raw, v.settings, c.Names, c.Today)
	}
	if p.Type == "" {
		var s change.ID
		if json.Unmarshal(raw, &s) == nil {
			if name, ok := v.names[s]; ok {
				return name
			}
		}
		return item.Display(p, raw, v.settings, c.Names, c.Today)
	}
	vs := v.values()
	st := workspace.Settings{Properties: []workspace.Property{p}, Statuses: vs, Priorities: vs, Systems: vs}
	return item.Display(p, raw, st, c.Names, c.Today)
}
