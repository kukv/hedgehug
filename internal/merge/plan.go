package merge

import (
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/model"
)

// plan merges a sprint, a release, a template or a workspace field by
// field, as tasks are: one side's value whole, timed by updated, the
// unknown keys and props key by key, and the body with diff3.
func (m *merger) plan(base []byte, ours, theirs Side) (Result, error) {
	switch kindOf(m.path) {
	case kindSprint:
		return mergeFile(m, base, ours, theirs, fileKind[model.Sprint]{
			model.DecodeSprint, func(x *model.Sprint) *time.Time { return &x.Updated }, sprintFields, (*model.Sprint).Encode, nil,
		})
	case kindRelease:
		return mergeFile(m, base, ours, theirs, fileKind[model.Release]{
			model.DecodeRelease, func(x *model.Release) *time.Time { return &x.Updated }, releaseFields, (*model.Release).Encode, nil,
		})
	case kindWorkspace:
		return mergeFile(m, base, ours, theirs, fileKind[model.Workspace]{
			model.DecodeWorkspace, func(x *model.Workspace) *time.Time { return &x.Updated }, workspaceFields, (*model.Workspace).Encode, nil,
		})
	}
	name := strings.TrimSuffix(strings.TrimPrefix(m.path, "templates/"), ".md")
	return mergeFile(m, base, ours, theirs, fileKind[model.Template]{
		func(data []byte) (*model.Template, error) { return model.DecodeTemplate(name, data) },
		func(x *model.Template) *time.Time { return &x.Updated }, templateFields, (*model.Template).Encode, nil,
	})
}

var sprintFields = []field[model.Sprint]{
	scalarField("name", func(x *model.Sprint) *string { return &x.Name }),
	scalarField("start", func(x *model.Sprint) *model.Date { return &x.Start }),
	scalarField("end", func(x *model.Sprint) *model.Date { return &x.End }),
	scalarField("state", func(x *model.Sprint) *string { return &x.State }),
	scalarField("goal", func(x *model.Sprint) *string { return &x.Goal }),
	scalarField("created", func(x *model.Sprint) *time.Time { return &x.Created }),
	mapField("", func(x *model.Sprint) *model.Fields { return &x.Extra }, false),
	bodyField(func(x *model.Sprint) *string { return &x.Body }),
}

var releaseFields = []field[model.Release]{
	scalarField("name", func(x *model.Release) *string { return &x.Name }),
	scalarField("date", func(x *model.Release) *model.Date { return &x.Date }),
	scalarField("state", func(x *model.Release) *string { return &x.State }),
	scalarField("created", func(x *model.Release) *time.Time { return &x.Created }),
	mapField("", func(x *model.Release) *model.Fields { return &x.Extra }, false),
	bodyField(func(x *model.Release) *string { return &x.Body }),
}

// templateFields take each list whole, as any other field of a template:
// only the props are merged key by key, as they are in tasks.
var templateFields = []field[model.Template]{
	scalarField("kind", func(x *model.Template) *string { return &x.Item.Kind }),
	scalarField("title", func(x *model.Template) *string { return &x.Item.Title }),
	scalarField("status", func(x *model.Template) *string { return &x.Item.Status }),
	scalarField("assignees", func(x *model.Template) *model.Assignees { return &x.Item.Assignees }),
	scalarField("watchers", func(x *model.Template) *[]string { return &x.Item.Watchers }),
	scalarField("labels", func(x *model.Template) *[]string { return &x.Item.Labels }),
	scalarField("repos", func(x *model.Template) *[]string { return &x.Item.Repos }),
	scalarField("blocked_by", func(x *model.Template) *[]model.ULID { return &x.Item.BlockedBy }),
	scalarField("sprint", func(x *model.Template) **model.ULID { return &x.Item.Sprint }),
	scalarField("releases", func(x *model.Template) *[]model.ULID { return &x.Item.Releases }),
	scalarField("links", func(x *model.Template) *[]model.Link { return &x.Item.Links }),
	mapField("props.", func(x *model.Template) *model.Fields { return &x.Item.Props }, true),
	scalarField("children", func(x *model.Template) *[]model.TemplateItem { return &x.Item.Children }),
	scalarField("created", func(x *model.Template) *time.Time { return &x.Created }),
	mapField("", func(x *model.Template) *model.Fields { return &x.Extra }, false),
	bodyField(func(x *model.Template) *string { return &x.Item.Body }),
}

var workspaceFields = []field[model.Workspace]{
	scalarField("name", func(x *model.Workspace) *string { return &x.Name }),
	prefixField(),
	scalarField("created", func(x *model.Workspace) *time.Time { return &x.Created }),
	mapField("", func(x *model.Workspace) *model.Fields { return &x.Extra }, false),
	bodyField(func(x *model.Workspace) *string { return &x.Body }),
}

// prefixPair is a workspace's id prefix and when it was set, merged as one
// value. It is recorded by its prefix alone.
type prefixPair struct {
	IDPrefix  string
	PrefixSet time.Time
}

func (p prefixPair) MarshalYAML() (any, error) { return p.IDPrefix, nil }

func prefixOf(w *model.Workspace) prefixPair { return prefixPair{w.IDPrefix, w.PrefixSet} }

func samePrefix(a, b any) bool {
	x, y := a.(prefixPair), b.(prefixPair)
	return x.IDPrefix == y.IDPrefix && jsonEqual(x.PrefixSet, y.PrefixSet)
}

// prefixField is the pair of id_prefix and prefix_set, named id_prefix.
// When both sides set the prefix, the one set later wins.
func prefixField() field[model.Workspace] {
	return field[model.Workspace]{
		same: func(a, b *model.Workspace) bool { return samePrefix(prefixOf(a), prefixOf(b)) },
		merge: func(m *fileMerge[model.Workspace]) {
			v := pickAt(m.sides, "id_prefix", m.o.PrefixSet, m.t.PrefixSet, prefixOf(m.b), prefixOf(m.o), prefixOf(m.t), samePrefix)
			if !samePrefix(v, prefixOf(m.dst)) {
				m.dst.IDPrefix, m.dst.PrefixSet = v.IDPrefix, v.PrefixSet
			}
		},
	}
}
