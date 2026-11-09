package merge

import (
	"slices"
	"time"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/app/domain/model"
)

// taskMerge is the merge of one task file.
type taskMerge = fileMerge[model.Task]

// taskKind merges deleted_at last: whether a side edited the task decides
// it.
var taskKind = fileKind[model.Task]{
	decode:  model.DecodeTask,
	updated: func(x *model.Task) *time.Time { return &x.Updated },
	fields:  taskFields,
	encode:  (*model.Task).Encode,
	after:   func(m *taskMerge) { deletedAt(m, edited(m.b, m.o), edited(m.b, m.t)) },
}

// taskFields are the fields of a task file other than ulid, updated and
// deleted_at, which have rules of their own.
var taskFields = []field[model.Task]{
	scalarField("id", func(x *model.Task) *string { return &x.ID }),
	formerIDsField(),
	scalarField("workspace", func(x *model.Task) *model.ULID { return &x.Workspace }),
	scalarField("kind", func(x *model.Task) *string { return &x.Kind }),
	scalarField("title", func(x *model.Task) *string { return &x.Title }),
	scalarField("parent", func(x *model.Task) **model.ULID { return &x.Parent }),
	scalarField("rank", func(x *model.Task) *model.Rank { return &x.Rank }),
	scalarField("status", func(x *model.Task) *string { return &x.Status }),
	setField("assignees", func(x *model.Task) *[]string { return &x.Assignees }, ident),
	setField("watchers", func(x *model.Task) *[]string { return &x.Watchers }, ident),
	setField("labels", func(x *model.Task) *[]string { return &x.Labels }, ident),
	setField("repos", func(x *model.Task) *[]string { return &x.Repos }, ident),
	setField("blocked_by", func(x *model.Task) *[]model.ULID { return &x.BlockedBy }, model.ULID.String),
	setField("causes", func(x *model.Task) *[]model.ULID { return &x.Causes }, model.ULID.String),
	setField("relates", func(x *model.Task) *[]model.ULID { return &x.Relates }, model.ULID.String),
	setField("duplicates", func(x *model.Task) *[]model.ULID { return &x.Duplicates }, model.ULID.String),
	setField("clones", func(x *model.Task) *[]model.ULID { return &x.Clones }, model.ULID.String),
	scalarField("sprint", func(x *model.Task) **model.ULID { return &x.Sprint }),
	setField("releases", func(x *model.Task) *[]model.ULID { return &x.Releases }, model.ULID.String),
	setField("links", func(x *model.Task) *[]model.Link { return &x.Links }, func(l model.Link) string { return l.Ref }),
	mapField("props.", func(x *model.Task) *model.Fields { return &x.Props }, true),
	{
		same:    func(a, b *model.Task) bool { return model.NodeEqual(a.GH, b.GH) },
		names:   one[model.Task]("gh"),
		merge:   gh,
		machine: true,
	},
	scalarField("created", func(x *model.Task) *time.Time { return &x.Created }),
	scalarField("created_by", func(x *model.Task) *string { return &x.CreatedBy }),
	mapField("", func(x *model.Task) *model.Fields { return &x.Extra }, false),
	bodyField(func(x *model.Task) *string { return &x.Body }),
}

// edited reports whether x changed a field of b other than updated,
// deleted_at and gh, which a machine writes: a fetch must not bring back a
// task someone deleted.
func edited(b, x *model.Task) bool {
	return slices.ContainsFunc(taskFields, func(f field[model.Task]) bool { return !f.machine && !f.same(b, x) })
}

// TaskChanges names the fields a person changes that differ between a and b,
// in the order of the file: title, status, props.priority... It leaves out
// updated, which every change moves, and id and former_ids, which undo does
// not put back.
func TaskChanges(a, b *model.Task) []string {
	var out []string
	for _, f := range taskFields {
		if f.same(a, b) {
			continue
		}
		for _, n := range f.names(a, b) {
			if n != "id" && n != "former_ids" {
				out = append(out, n)
			}
		}
	}
	if !jsonEqual(a.DeletedAt, b.DeletedAt) {
		out = append(out, "deleted_at")
	}
	return out
}

// formerIDsField is a set that also takes the id that lost to the other
// side's, so that the number is not given again. It follows id, whose
// winner is already in dst. Under OursWins the caller keeps id and
// former_ids as they are.
func formerIDsField() field[model.Task] {
	f := setField("former_ids", func(x *model.Task) *[]string { return &x.FormerIDs }, ident)
	inner := f.merge
	f.merge = func(m *taskMerge) {
		inner(m)
		b, o, t := m.b.ID, m.o.ID, m.t.ID
		if m.p != LWW || o == b || t == b || o == t {
			return
		}
		lost := o
		if m.dst.ID == o {
			lost = t
		}
		if !slices.Contains(m.dst.FormerIDs, lost) {
			m.dst.FormerIDs = append(slices.Clone(m.dst.FormerIDs), lost)
		}
	}
	return f
}

// ghLists are the lists of gh whose items are told apart by ref.
var ghLists = []string{"prs", "issues"}

// gh merges the values from GitHub key by key, or whole when one side's is
// not a mapping. prs and issues merge item by item. Neither side's value is
// recorded as lost: the next fetch brings GitHub's.
func gh(m *taskMerge) {
	lost := len(m.res.Overwritten)
	defer func() { m.res.Overwritten = m.res.Overwritten[:lost] }()
	b, o, t := m.b.GH, m.o.GH, m.t.GH
	if model.NodeEqual(o, t) || !isMapping(o) || !isMapping(t) || b != nil && !isMapping(b) {
		m.dst.GH = m.node("gh", b, o, t, m.dst.GH, false)
		return
	}
	bf, of, tf := fieldsOf(b), fieldsOf(o), fieldsOf(t)
	f := m.fields("gh.", bf, of, tf, fieldsOf(m.dst.GH), false)
	for i := range f {
		if slices.Contains(ghLists, f[i].Key) {
			if v := ghList(m, "gh."+f[i].Key, bf.Get(f[i].Key), of.Get(f[i].Key), tf.Get(f[i].Key)); v != nil {
				f[i].Value = v
			}
		}
	}
	m.dst.GH = mappingLike(m.dst.GH, f)
}

// ghList merges a list of gh that both sides changed, item by item, the
// items told apart by ref. It is nil when the node merge already has the
// answer: a side did not change the list, or an item has no ref.
func ghList(m *taskMerge, name string, b, o, t *yaml.Node) *yaml.Node {
	isList := func(n *yaml.Node) bool { return n != nil && n.Kind == yaml.SequenceNode }
	if !isList(o) || !isList(t) || model.NodeEqual(b, o) || model.NodeEqual(b, t) || model.NodeEqual(o, t) {
		return nil
	}
	var bs []*yaml.Node
	if isList(b) {
		bs = b.Content
	}
	ref := func(n *yaml.Node) string {
		if v := fieldsOf(n).Get("ref"); v != nil {
			return v.Value
		}
		return ""
	}
	if slices.ContainsFunc(slices.Concat(bs, o.Content, t.Content), func(n *yaml.Node) bool { return ref(n) == "" }) {
		return nil
	}
	// The keys of an item keep the order of the side the result is built on.
	top, other := t, o
	if m.oursTop {
		top, other = o, t
	}
	items := mergeSet(bs, o.Content, t.Content, ref)
	for i, x := range items {
		r := ref(x)
		bi, oi, ti := find(bs, r, ref), find(o.Content, r, ref), find(t.Content, r, ref)
		if oi < 0 || ti < 0 {
			continue
		}
		var bx *yaml.Node
		if bi >= 0 {
			bx = bs[bi]
		}
		d, from := top.Content[find(top.Content, r, ref)], other.Content[find(other.Content, r, ref)]
		f := m.fields(name+".", fieldsOf(bx), fieldsOf(o.Content[oi]), fieldsOf(t.Content[ti]), fieldsOf(d), false)
		items[i] = mappingLike(d, f, from)
	}
	out := *top
	out.Content = items
	return &out
}

// deletedAt merges deleted_at. A side that deleted a live task while the
// other edited it loses the deletion, and is told; otherwise deleted_at is
// a scalar.
func deletedAt(m *taskMerge, oEdited, tEdited bool) {
	b, o, t := m.b.DeletedAt, m.o.DeletedAt, m.t.DeletedAt
	oc, tc := !jsonEqual(b, o), !jsonEqual(b, t)
	var v *time.Time
	switch {
	case b == nil && oc && !tc && o != nil && tEdited:
		m.res.Notices = append(m.res.Notices, Notice{Kind: Revived, Path: m.path, Author: m.ours.Author})
	case b == nil && tc && !oc && t != nil && oEdited:
		m.res.Notices = append(m.res.Notices, Notice{Kind: Revived, Path: m.path, Author: m.theirs.Author})
	default:
		v = pick(m.sides, "deleted_at", b, o, t, jsonEqual)
	}
	if !jsonEqual(v, m.dst.DeletedAt) {
		m.dst.DeletedAt = v
	}
}
