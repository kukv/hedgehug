// Package planning は、スプリントとリリースをワークスペースのログから読み書きする(F1-1〜F1-3)。
package planning

import (
	"encoding/json"
	"slices"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// SprintState は、スプリントの状態。
type SprintState string

// スプリントの状態。
const (
	Planned SprintState = "planned"
	Active  SprintState = "active"
	Closed  SprintState = "closed"
)

// ReleaseState は、リリースの状態。
type ReleaseState string

// リリースの状態。
const (
	Unreleased ReleaseState = "unreleased"
	Released   ReleaseState = "released"
	Archived   ReleaseState = "archived"
)

// CloseSprint は、スプリントの完了の確定値の行の種類(F1-3)。
const CloseSprint change.Type = "close_sprint"

const (
	sprintPrefix  = "sprints/"
	releasePrefix = "releases/"
)

// IsPlanRow は、ワークスペースのログの、スプリント・リリースの行(完了の確定値の行を含む)か。
func IsPlanRow(r change.Record) bool {
	return r.T == CloseSprint || strings.HasPrefix(r.F, sprintPrefix) || strings.HasPrefix(r.F, releasePrefix)
}

// Totals は、ポイントと件数の合計。
type Totals struct {
	Points float64 `json:"points"`
	Count  int     `json:"count"`
}

// Confirmed は、完了で確定した計画(Committed)と実績(Completed)。
type Confirmed struct {
	Committed Totals `json:"committed"`
	Completed Totals `json:"completed"`
}

// Sprint は、スプリント。Deleted は、削除済みのときの削除の印。
type Sprint struct {
	ID         change.ID
	Name       string
	State      SprintState
	Start, End string
	Goal       string
	Confirmed  *Confirmed
	Closing    *Closing // 完了の記録(完了していなければ nil)
	Deleted    *change.Deletion
}

// Closing は、完了の記録の行の時刻と op(F3-2)。完了による持ち越しの行(同じ op の set sprint)を見つけるのに使う。
type Closing struct {
	At time.Time
	Op change.ID
}

// Release は、リリース。
type Release struct {
	ID          change.ID
	Name        string
	State       ReleaseState
	Date        string
	Description string
	Deleted     *change.Deletion
}

// Plan は、ワークスペースのスプリントとリリース。削除済みを含み、内部 ID の順。
type Plan struct {
	Sprints  []Sprint
	Releases []Release
}

// FromLog は、ワークスペースのログのスプリントとリリース。名前のないものは除く。
func FromLog(log []change.Record) Plan {
	values := change.Resolve(log, change.Options{}).Values
	sprints := map[change.ID]*Sprint{}
	releases := map[change.ID]*Release{}
	for f, raw := range values {
		parts := strings.Split(f, "/")
		if len(parts) != 3 {
			continue
		}
		id := change.ID(parts[1])
		switch parts[0] + "/" {
		case sprintPrefix:
			s := entry(sprints, id, &Sprint{ID: id})
			setSprint(s, parts[2], raw)
		case releasePrefix:
			r := entry(releases, id, &Release{ID: id})
			setRelease(r, parts[2], raw)
		}
	}
	var p Plan
	for id, s := range sprints {
		if s.Name == "" {
			continue
		}
		s.Deleted = deletionOf(log, sprintPrefix+string(id))
		s.Confirmed, s.Closing = confirmedOf(log, id)
		p.Sprints = append(p.Sprints, *s)
	}
	for id, r := range releases {
		if r.Name == "" {
			continue
		}
		r.Deleted = deletionOf(log, releasePrefix+string(id))
		p.Releases = append(p.Releases, *r)
	}
	slices.SortFunc(p.Sprints, func(a, b Sprint) int { return strings.Compare(string(a.ID), string(b.ID)) })
	slices.SortFunc(p.Releases, func(a, b Release) int { return strings.Compare(string(a.ID), string(b.ID)) })
	return p
}

func entry[T any](m map[change.ID]*T, id change.ID, fresh *T) *T {
	if v, ok := m[id]; ok {
		return v
	}
	m[id] = fresh
	return fresh
}

func setSprint(s *Sprint, field string, raw json.RawMessage) {
	switch field {
	case "name":
		_ = json.Unmarshal(raw, &s.Name)
	case "state":
		_ = json.Unmarshal(raw, &s.State)
	case "start":
		_ = json.Unmarshal(raw, &s.Start)
	case "end":
		_ = json.Unmarshal(raw, &s.End)
	case "goal":
		_ = json.Unmarshal(raw, &s.Goal)
	}
}

func setRelease(r *Release, field string, raw json.RawMessage) {
	switch field {
	case "name":
		_ = json.Unmarshal(raw, &r.Name)
	case "state":
		_ = json.Unmarshal(raw, &r.State)
	case "date":
		_ = json.Unmarshal(raw, &r.Date)
	case "description":
		_ = json.Unmarshal(raw, &r.Description)
	}
}

// deletionOf は、f(sprints/<ID>・releases/<ID>)の削除の印。
// NewDelete は root を書かないが、スプリント・リリースの削除はどれも利用者が選んだもの(F1-14)なので Root は true。
func deletionOf(log []change.Record, f string) *change.Deletion {
	d := change.DeletionOfField(log, f)
	if d != nil {
		d.Root = true
	}
	return d
}

// confirmedOf は、スプリント id の完了の確定値と、その行の時刻と op。同時に 2 回完了していれば、change.Current が 1 つに決める。
func confirmedOf(log []change.Record, id change.ID) (*Confirmed, *Closing) {
	r, ok := change.Current(log, CloseSprint, sprintPrefix+string(id)+"/close", func(change.Record) bool { return true })
	if !ok {
		return nil, nil
	}
	var c Confirmed
	if json.Unmarshal(r.To, &c) != nil {
		return nil, nil
	}
	return &c, &Closing{At: r.At, Op: r.Op}
}

// LiveSprints は、削除済みでないスプリント(内部 ID の順)。
func (p Plan) LiveSprints() []Sprint {
	var out []Sprint
	for _, s := range p.Sprints {
		if s.Deleted == nil {
			out = append(out, s)
		}
	}
	return out
}

// LiveReleases は、削除済みでないリリース(内部 ID の順)。
func (p Plan) LiveReleases() []Release {
	var out []Release
	for _, r := range p.Releases {
		if r.Deleted == nil {
			out = append(out, r)
		}
	}
	return out
}

// UsableSprint は、削除済みでないスプリント id(F1-5)。
func (p Plan) UsableSprint(id change.ID) (Sprint, bool) {
	for _, s := range p.Sprints {
		if s.ID == id && s.Deleted == nil {
			return s, true
		}
	}
	return Sprint{}, false
}

// UsableRelease は、削除済みでないリリース id(F1-5)。
func (p Plan) UsableRelease(id change.ID) (Release, bool) {
	for _, r := range p.Releases {
		if r.ID == id && r.Deleted == nil {
			return r, true
		}
	}
	return Release{}, false
}
