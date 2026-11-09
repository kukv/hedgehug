package automation

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
)

// What a status is on an issue, as synced holds it.
const (
	IssueOpen       = "open"
	IssueCompleted  = "completed"
	IssueNotPlanned = "not_planned"
)

// FieldComments is the field that syncs the comments of an issue.
const FieldComments = "comments"

// The side whose value a sync of an issue overwrote.
const (
	SideTask   = "task"
	SideGitHub = "github"
)

// The rule and the event that the commits of mirror carry, in the trailers of
// the rules.
const (
	MirrorRule  = "mirror"
	MirrorEvent = "issue.mirror"
)

// TrailerIssueOverwritten is a value of a side that a sync of an issue
// overwrote.
const TrailerIssueOverwritten = "Hedgehug-Issue-Overwritten"

// For lists the fields that the link l syncs: those of the link, or else
// those of issue_fields for its mode. It leaves out the names it does not
// know, and the milestone when props does not say where it goes. A link that
// does not sync has none.
func (f IssueFields) For(l model.Link) []string {
	names := l.Fields
	switch {
	case l.Sync != model.SyncMirror && l.Sync != model.SyncPush:
		return nil
	case len(names) > 0:
	case l.Sync == model.SyncMirror:
		names = f.Mirror
	default:
		names = f.Push
	}
	var out []string
	for _, n := range names {
		if !slices.Contains(IssueFieldNames, n) || slices.Contains(out, n) {
			continue
		}
		if n == "milestone" && f.Props["milestone"] == "" {
			continue
		}
		out = append(out, n)
	}
	return out
}

// Scalars lists the fields that For lists but comments: the ones the sync of
// the values of an issue handles. The comments have a sync of their own.
func (f IssueFields) Scalars(l model.Link) []string {
	return slices.DeleteFunc(f.For(l), func(n string) bool { return n == FieldComments })
}

// StatusToken is what a status of category c is on an issue.
func StatusToken(c schema.Category) string {
	switch c {
	case schema.CategoryDone:
		return IssueCompleted
	case schema.CategoryCanceled:
		return IssueNotPlanned
	}
	return IssueOpen
}

// TokenCategory is the category that an issue in the state token asks of a
// task.
func TokenCategory(token string) schema.Category {
	switch token {
	case IssueCompleted:
		return schema.CategoryDone
	case IssueNotPlanned:
		return schema.CategoryCanceled
	}
	return schema.CategoryTodo
}

// Overwrite is a value that a sync of an issue wrote over: the field, the
// side it was on, and the value.
type Overwrite struct {
	Field, Side string
	Lost        []string
}

// Reflection is what a sync of one issue does: the values to write to the
// task and to GitHub, the values the two have in common after it, and the
// edits it writes over.
type Reflection struct {
	ToTask, ToGitHub, Synced model.IssueValues
	Overwritten              []Overwrite
}

// Reflect compares the fields of a link in mode across the task, GitHub and
// base, the values as they were last made the same. The side the mode reads
// from wins: a change on the other side is written over and noted. A field
// base lacks has no change to note.
func Reflect(mode string, fields []string, task, gh, base model.IssueValues) Reflection {
	r := Reflection{ToTask: model.IssueValues{}, ToGitHub: model.IssueValues{}, Synced: model.IssueValues{}}
	for _, f := range fields {
		t, g := task[f], gh[f]
		b, known := base[f]
		switch mode {
		case model.SyncMirror:
			r.Synced[f] = g
			if slices.Equal(t, g) {
				continue
			}
			r.ToTask[f] = g
			if known && !slices.Equal(t, b) {
				r.Overwritten = append(r.Overwritten, Overwrite{f, SideTask, t})
			}
		case model.SyncPush:
			r.Synced[f] = t
			if slices.Equal(t, g) {
				continue
			}
			r.ToGitHub[f] = t
			if known && !slices.Equal(g, b) {
				r.Overwritten = append(r.Overwritten, Overwrite{f, SideGitHub, g})
			}
		}
	}
	return r
}

// IssueOverwrittenTrailer is the value of TrailerIssueOverwritten:
// "<ULID> <field> <ref> <side> <JSON value>".
func IssueOverwrittenTrailer(u model.ULID, ref string, o Overwrite) string {
	lost := o.Lost
	if lost == nil {
		lost = []string{}
	}
	b, err := json.Marshal(lost) // a list of strings always encodes
	if err != nil {
		panic(err)
	}
	return strings.Join([]string{u.String(), o.Field, ref, o.Side, string(b)}, " ")
}

// ParseIssueOverwritten reads the value of TrailerIssueOverwritten.
func ParseIssueOverwritten(v string) (u model.ULID, ref string, o Overwrite, ok bool) {
	parts := strings.SplitN(v, " ", 5)
	if len(parts) != 5 {
		return model.ULID{}, "", Overwrite{}, false
	}
	u, err := model.ParseULID(parts[0])
	if err != nil {
		return model.ULID{}, "", Overwrite{}, false
	}
	var lost []string
	if err := json.Unmarshal([]byte(parts[4]), &lost); err != nil {
		return model.ULID{}, "", Overwrite{}, false
	}
	return u, parts[2], Overwrite{Field: parts[1], Side: parts[3], Lost: lost}, true
}
