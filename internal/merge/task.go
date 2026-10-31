package merge

import (
	"slices"
	"time"

	"github.com/kukv/hedgehug/internal/model"
)

// taskMerge is the merge of one task file.
type taskMerge = fileMerge[model.Task]

func taskUpdated(x *model.Task) *time.Time { return &x.Updated }

func (m *merger) task(base []byte, ours, theirs Side) (Result, error) {
	if ours.Data == nil || theirs.Data == nil {
		return m.revive(ours, theirs), nil
	}
	tm, ok := load(m, base, ours, theirs, model.DecodeTask, taskUpdated)
	if !ok {
		return m.other(ours, theirs)
	}
	oEdited, tEdited := edited(tm.b, tm.o), edited(tm.b, tm.t)
	for _, f := range taskFields {
		f.merge(tm)
	}
	tm.updated(taskUpdated)
	deletedAt(tm, oEdited, tEdited)
	return tm.encode((*model.Task).Encode)
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
	scalarField("sprint", func(x *model.Task) **model.ULID { return &x.Sprint }),
	setField("releases", func(x *model.Task) *[]model.ULID { return &x.Releases }, model.ULID.String),
	setField("links", func(x *model.Task) *[]model.Link { return &x.Links }, func(l model.Link) string { return l.Ref }),
	mapField("props.", func(x *model.Task) *model.Fields { return &x.Props }, true),
	{
		same:  func(a, b *model.Task) bool { return model.NodeEqual(a.GH, b.GH) },
		merge: gh,
	},
	scalarField("created", func(x *model.Task) *time.Time { return &x.Created }),
	scalarField("created_by", func(x *model.Task) *string { return &x.CreatedBy }),
	mapField("", func(x *model.Task) *model.Fields { return &x.Extra }, false),
	bodyField(func(x *model.Task) *string { return &x.Body }),
}

// edited reports whether x changed a field of b other than updated and
// deleted_at.
func edited(b, x *model.Task) bool {
	return slices.ContainsFunc(taskFields, func(f field[model.Task]) bool { return !f.same(b, x) })
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

// gh merges the values from GitHub key by key, or whole when one side's is
// not a mapping.
func gh(m *taskMerge) {
	b, o, t := m.b.GH, m.o.GH, m.t.GH
	if model.NodeEqual(o, t) || !isMapping(o) || !isMapping(t) || b != nil && !isMapping(b) {
		m.dst.GH = m.node("gh", b, o, t, m.dst.GH, false)
		return
	}
	f := m.fields("gh.", fieldsOf(b), fieldsOf(o), fieldsOf(t), fieldsOf(m.dst.GH), false)
	m.dst.GH = mappingLike(m.dst.GH, f)
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
