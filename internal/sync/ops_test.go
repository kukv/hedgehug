package sync

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
	"github.com/kukv/hedgehug/internal/store"
)

// write is one write a clone made, kept to check that no value is lost.
type write struct {
	clone  int
	commit plumbing.Hash
	task   model.ULID // the workspace for the ws.* fields; zero for the schema
	field  string     // title, status, parent, workspace, deleted_at, body, labels, id, comment, purge, ws.rm, ws.name, ws.id_prefix, schema.status, schema.prop
	value  string     // JSON; for body, the line written; for labels, the label added
}

type op struct {
	name string
	do   func(c *cluster, i int, rnd *rand.Rand) []write
}

// ops are the operations a clone does. Adds come three times so that there
// are tasks to change, and mv twice for the parent cycles.
var ops = []op{
	{"add", opAdd},
	{"add", opAdd},
	{"add", opAdd},
	{"set title", opSetTitle},
	{"status", opStatus},
	{"mv", opMove},
	{"mv", opMove},
	{"mv --workspace", opMoveWS},
	{"rm", opRemove},
	{"restore", opRestore},
	{"purge", opPurge},
	{"comment", opComment},
	{"body", opBody},
	{"label", opLabel},
	{"schema", opSchema},
	{"workspace add", opWSAdd},
	{"workspace prefix", opWSPrefix},
	{"workspace rename", opWSRename},
	{"workspace rm", opWSRm},
}

var (
	prefixPool = []string{"WEB", "API", "OPS", "DOC"}
	namePool   = []string{"web", "api", "ops", "docs"}
)

// view is clone i's tasks and workspaces as it has them now.
type view struct {
	tasks map[model.ULID]*model.Task
	fwd   map[model.ULID]*model.Forward // the forwards of tasks that moved to another store
	ws    map[model.ULID]*model.Workspace
}

func (c *cluster) view(i int) *view {
	c.t.Helper()
	v := &view{tasks: map[model.ULID]*model.Task{}, fwd: map[model.ULID]*model.Forward{}, ws: map[model.ULID]*model.Workspace{}}
	for t, err := range c.stores[i].Tasks() {
		if err != nil {
			c.t.Fatalf("clone %d: %v", i, err)
		}
		v.tasks[t.ULID] = t
	}
	for _, f := range c.forwards(i) {
		v.fwd[f.ULID] = f
	}
	for w, err := range c.stores[i].Workspaces() {
		if err != nil {
			c.t.Fatalf("clone %d: %v", i, err)
		}
		v.ws[w.ULID] = w
	}
	return v
}

func sortedULIDs[T any](m map[model.ULID]T) []model.ULID {
	out := make([]model.ULID, 0, len(m))
	for u := range m {
		out = append(out, u)
	}
	slices.SortFunc(out, func(a, b model.ULID) int { return strings.Compare(a.String(), b.String()) })
	return out
}

// pickTask returns a task of v that ok accepts, or nil.
func (v *view) pickTask(rnd *rand.Rand, ok func(*model.Task) bool) *model.Task {
	var cands []*model.Task
	for _, u := range sortedULIDs(v.tasks) {
		if t := v.tasks[u]; ok(t) {
			cands = append(cands, t)
		}
	}
	if len(cands) == 0 {
		return nil
	}
	return cands[rnd.IntN(len(cands))]
}

func (v *view) pickWS(rnd *rand.Rand, ok func(*model.Workspace) bool) *model.Workspace {
	var cands []*model.Workspace
	for _, u := range sortedULIDs(v.ws) {
		if w := v.ws[u]; ok(w) {
			cands = append(cands, w)
		}
	}
	if len(cands) == 0 {
		return nil
	}
	return cands[rnd.IntN(len(cands))]
}

// subtree returns u and every task under it, deleted ones too.
func (v *view) subtree(u model.ULID) []model.ULID {
	out := []model.ULID{u}
	for i := 0; i < len(out); i++ {
		for _, x := range sortedULIDs(v.tasks) {
			if p := v.tasks[x].Parent; p != nil && *p == out[i] && !slices.Contains(out, x) {
				out = append(out, x)
			}
		}
	}
	return out
}

func live(t *model.Task) bool { return t.DeletedAt == nil }

func js(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func parentJSON(p *model.ULID) string {
	if p == nil {
		return "null"
	}
	return js(p.String())
}

func deletedJSON(t *time.Time) string {
	if t == nil {
		return "null"
	}
	return js(t.UTC().Format(time.RFC3339))
}

func word(rnd *rand.Rand, prefix string) string {
	return fmt.Sprintf("%s%06d", prefix, rnd.IntN(1000000))
}

func (c *cluster) nextRank() model.Rank {
	c.t.Helper()
	r, err := model.Between(c.last, "")
	if err != nil {
		c.t.Fatal(err)
	}
	c.last = r
	return r
}

// update runs fn as one operation of clone i. A write the store refuses for
// the rules of the schema is a correct outcome, and gives no commit; any
// other error fails the test.
func (c *cluster) update(i int, o store.Op, fn func(tx *store.Tx) error) (plumbing.Hash, bool) {
	c.t.Helper()
	ok, err := c.stores[i].Update(o, fn)
	var ve *store.ValidationError
	var se *store.SchemaError
	if err != nil && !errors.As(err, &ve) && !errors.As(err, &se) {
		c.t.Fatalf("clone %d: %s: %v", i, o.Name, err)
	}
	if err != nil || !ok {
		return plumbing.ZeroHash, false
	}
	return mustHead(c.t, c.stores[i]), true
}

func (c *cluster) taskOp(i int, name string, u model.ULID) store.Op { return c.op(i, name, u) }

func (c *cluster) wsOp(i int, w model.ULID) store.Op {
	o := c.op(i, "workspace", model.ULID{})
	o.Workspace = w
	return o
}

func opAdd(c *cluster, i int, rnd *rand.Rand) []write {
	v := c.view(i)
	ws := v.pickWS(rnd, func(*model.Workspace) bool { return true })
	if ws == nil {
		return nil
	}
	x := &model.Task{ULID: model.NewULID(), Workspace: ws.ULID, Kind: "task", Title: word(rnd, "t"), Status: "todo", Rank: c.nextRank()}
	if p := v.pickTask(rnd, func(t *model.Task) bool { return live(t) && t.Workspace == ws.ULID && rnd.IntN(2) == 0 }); p != nil {
		x.Parent = &p.ULID
	}
	h, ok := c.update(i, c.taskOp(i, "add", x.ULID), func(tx *store.Tx) error { return tx.PutTask(x) })
	if !ok {
		return nil
	}
	ws2 := []write{
		{i, h, x.ULID, "title", js(x.Title)},
		{i, h, x.ULID, "status", js(x.Status)},
		{i, h, x.ULID, "workspace", js(x.Workspace.String())},
		{i, h, x.ULID, "id", js(x.ID)},
	}
	return append(ws2, write{i, h, x.ULID, "parent", parentJSON(x.Parent)}, write{i, h, x.ULID, "deleted_at", "null"})
}

func opSetTitle(c *cluster, i int, rnd *rand.Rand) []write {
	t := c.view(i).pickTask(rnd, live)
	if t == nil {
		return nil
	}
	title := word(rnd, "t")
	h, ok := c.update(i, c.taskOp(i, "update", t.ULID), func(tx *store.Tx) error {
		x, err := tx.Task(t.ULID)
		if err != nil {
			return err
		}
		x.Title = title
		return tx.PutTask(x)
	})
	if !ok {
		return nil
	}
	return []write{{i, h, t.ULID, "title", js(title)}}
}

func opStatus(c *cluster, i int, rnd *rand.Rand) []write {
	t := c.view(i).pickTask(rnd, live)
	if t == nil {
		return nil
	}
	sts := c.stores[i].Schema().Statuses
	status := sts[rnd.IntN(len(sts))].Name
	h, ok := c.update(i, c.taskOp(i, "status", t.ULID), func(tx *store.Tx) error {
		x, err := tx.Task(t.ULID)
		if err != nil {
			return err
		}
		x.Status = status
		return tx.PutTask(x)
	})
	if !ok {
		return nil
	}
	return []write{{i, h, t.ULID, "status", js(status)}}
}

func opMove(c *cluster, i int, rnd *rand.Rand) []write {
	v := c.view(i)
	t := v.pickTask(rnd, live)
	if t == nil {
		return nil
	}
	under := v.subtree(t.ULID)
	var p *model.ULID
	if q := v.pickTask(rnd, func(x *model.Task) bool {
		return live(x) && x.Workspace == t.Workspace && !slices.Contains(under, x.ULID) && rnd.IntN(3) > 0
	}); q != nil {
		p = &q.ULID
	}
	rank := c.nextRank()
	h, ok := c.update(i, c.taskOp(i, "mv", t.ULID), func(tx *store.Tx) error {
		x, err := tx.Task(t.ULID)
		if err != nil {
			return err
		}
		x.Parent, x.Rank = p, rank
		return tx.PutTask(x)
	})
	if !ok {
		return nil
	}
	return []write{{i, h, t.ULID, "parent", parentJSON(p)}}
}

// opMoveWS moves a task and everything under it to another workspace, as
// mv --workspace does.
func opMoveWS(c *cluster, i int, rnd *rand.Rand) []write {
	v := c.view(i)
	t := v.pickTask(rnd, live)
	if t == nil {
		return nil
	}
	to := v.pickWS(rnd, func(w *model.Workspace) bool { return w.ULID != t.Workspace })
	if to == nil {
		return nil
	}
	var p *model.ULID
	if q := v.pickTask(rnd, func(x *model.Task) bool { return live(x) && x.Workspace == to.ULID && rnd.IntN(2) == 0 }); q != nil {
		p = &q.ULID
	}
	rank := c.nextRank()
	var moved []*model.Task
	h, ok := c.update(i, c.taskOp(i, "mv", t.ULID), func(tx *store.Tx) error {
		moved = nil
		for _, u := range v.subtree(t.ULID) {
			x, err := tx.Task(u)
			if err != nil {
				return err
			}
			x.Workspace = to.ULID
			if u == t.ULID {
				x.Parent, x.Rank = p, rank
			}
			if err := tx.PutTask(x); err != nil {
				return err
			}
			moved = append(moved, x)
		}
		return nil
	})
	if !ok {
		return nil
	}
	var out []write
	for _, x := range moved {
		out = append(out, write{i, h, x.ULID, "workspace", js(to.ULID.String())})
		if x.ID != v.tasks[x.ULID].ID { // a workspace with the same prefix keeps the id
			out = append(out, write{i, h, x.ULID, "id", js(x.ID)})
		}
	}
	return append(out, write{i, h, t.ULID, "parent", parentJSON(p)})
}

func opRemove(c *cluster, i int, rnd *rand.Rand) []write {
	v := c.view(i)
	t := v.pickTask(rnd, live)
	if t == nil {
		return nil
	}
	now := time.Now().Truncate(time.Second)
	var hit []model.ULID
	h, ok := c.update(i, c.taskOp(i, "rm", t.ULID), func(tx *store.Tx) error {
		hit = nil
		for _, u := range v.subtree(t.ULID) {
			x, err := tx.Task(u)
			if err != nil {
				return err
			}
			if x.DeletedAt != nil {
				continue
			}
			x.DeletedAt = &now
			if err := tx.PutTask(x); err != nil {
				return err
			}
			hit = append(hit, u)
		}
		return nil
	})
	if !ok {
		return nil
	}
	var out []write
	for _, u := range hit {
		out = append(out, write{i, h, u, "deleted_at", deletedJSON(&now)})
	}
	return out
}

// opRestore restores a deleted task whose parent is not deleted, with what
// the same rm deleted under it.
func opRestore(c *cluster, i int, rnd *rand.Rand) []write {
	v := c.view(i)
	t := v.pickTask(rnd, func(x *model.Task) bool {
		if live(x) {
			return false
		}
		p := x.Parent
		return p == nil || v.tasks[*p] == nil || live(v.tasks[*p])
	})
	if t == nil {
		return nil
	}
	var hit []model.ULID
	h, ok := c.update(i, c.taskOp(i, "restore", t.ULID), func(tx *store.Tx) error {
		hit = nil
		for _, u := range v.subtree(t.ULID) {
			if d := v.tasks[u].DeletedAt; d == nil || !d.Equal(*t.DeletedAt) {
				continue
			}
			x, err := tx.Task(u)
			if err != nil {
				return err
			}
			x.DeletedAt = nil
			if err := tx.PutTask(x); err != nil {
				return err
			}
			hit = append(hit, u)
		}
		return nil
	})
	if !ok {
		return nil
	}
	var out []write
	for _, u := range hit {
		out = append(out, write{i, h, u, "deleted_at", "null"})
	}
	return out
}

// opPurge purges a deleted task with nothing live under it.
func opPurge(c *cluster, i int, rnd *rand.Rand) []write {
	v := c.view(i)
	t := v.pickTask(rnd, func(x *model.Task) bool {
		return !live(x) && !slices.ContainsFunc(v.subtree(x.ULID), func(u model.ULID) bool { return live(v.tasks[u]) })
	})
	if t == nil {
		return nil
	}
	h, ok := c.update(i, c.taskOp(i, "purge", t.ULID), func(tx *store.Tx) error {
		for _, u := range v.subtree(t.ULID) {
			if err := tx.RemoveTask(u); err != nil {
				return err
			}
		}
		return nil
	})
	if !ok {
		return nil
	}
	var out []write
	for _, u := range v.subtree(t.ULID) {
		out = append(out, write{i, h, u, "purge", "null"})
	}
	return out
}

func opComment(c *cluster, i int, rnd *rand.Rand) []write {
	t := c.view(i).pickTask(rnd, live)
	if t == nil {
		return nil
	}
	cm := &model.Comment{ULID: model.NewULID(), Author: c.opts[i].Account, Body: word(rnd, "c") + "\n"}
	h, ok := c.update(i, c.taskOp(i, "comment", t.ULID), func(tx *store.Tx) error { return tx.PutComment(t.ULID, cm) })
	if !ok {
		return nil
	}
	return []write{{i, h, t.ULID, "comment", js(cm.ULID.String())}}
}

// opBody appends a line to the body, or replaces one of its lines.
func opBody(c *cluster, i int, rnd *rand.Rand) []write {
	t := c.view(i).pickTask(rnd, live)
	if t == nil {
		return nil
	}
	line := word(rnd, "b")
	h, ok := c.update(i, c.taskOp(i, "edit", t.ULID), func(tx *store.Tx) error {
		x, err := tx.Task(t.ULID)
		if err != nil {
			return err
		}
		var lines []string
		if x.Body != "" {
			lines = strings.Split(strings.TrimSuffix(x.Body, "\n"), "\n")
		}
		if len(lines) > 0 && rnd.IntN(2) == 0 {
			lines[rnd.IntN(len(lines))] = line
		} else {
			lines = append(lines, line)
		}
		x.Body = strings.Join(lines, "\n") + "\n"
		return tx.PutTask(x)
	})
	if !ok {
		return nil
	}
	return []write{{i, h, t.ULID, "body", line}}
}

func opLabel(c *cluster, i int, rnd *rand.Rand) []write {
	t := c.view(i).pickTask(rnd, live)
	if t == nil {
		return nil
	}
	label := word(rnd, "l")
	h, ok := c.update(i, c.taskOp(i, "label", t.ULID), func(tx *store.Tx) error {
		x, err := tx.Task(t.ULID)
		if err != nil {
			return err
		}
		x.Labels = append(x.Labels, label)
		return tx.PutTask(x)
	})
	if !ok {
		return nil
	}
	return []write{{i, h, t.ULID, "labels", label}}
}

// opSchema adds a status or a prop to schema.yaml.
func opSchema(c *cluster, i int, rnd *rand.Rand) []write {
	name := word(rnd, "x")
	field := "schema.prop"
	h, ok := c.update(i, c.op(i, "schema", model.ULID{}), func(tx *store.Tx) error {
		b, err := c.stores[i].Schema().Encode()
		if err != nil {
			return err
		}
		sc, err := schema.Decode(b)
		if err != nil {
			return err
		}
		if rnd.IntN(2) == 0 {
			field = "schema.status"
			cats := schema.Categories
			sc.Statuses = append(sc.Statuses, schema.Status{Name: name, Category: cats[rnd.IntN(len(cats))]})
		} else {
			field = "schema.prop"
			sc.Props = append(sc.Props, schema.Prop{Name: name, Type: schema.TypeNumber})
		}
		return tx.PutSchema(sc)
	})
	if !ok {
		return nil
	}
	return []write{{i, h, model.ULID{}, field, js(name)}}
}

// clashes reports whether the store would refuse a workspace with this name
// and prefix because another workspace has the same: the name in any case,
// the prefix exactly.
func (v *view) clashes(self model.ULID, name, prefix string) bool {
	for u, o := range v.ws {
		if u != self && (o.IDPrefix == prefix || strings.EqualFold(o.Name, name)) {
			return true
		}
	}
	return false
}

func opWSAdd(c *cluster, i int, rnd *rand.Rand) []write {
	w := &model.Workspace{ULID: model.NewULID(), Name: namePool[rnd.IntN(len(namePool))], IDPrefix: prefixPool[rnd.IntN(len(prefixPool))], PrefixSet: time.Now()}
	if c.view(i).clashes(w.ULID, w.Name, w.IDPrefix) {
		return nil
	}
	h, ok := c.update(i, c.wsOp(i, w.ULID), func(tx *store.Tx) error { return tx.PutWorkspace(w) })
	if !ok {
		return nil
	}
	return []write{{i, h, w.ULID, "ws.name", js(w.Name)}, {i, h, w.ULID, "ws.id_prefix", js(w.IDPrefix)}}
}

// opWSPrefix changes the prefix of a workspace and renumbers its tasks, as
// workspace prefix does.
func opWSPrefix(c *cluster, i int, rnd *rand.Rand) []write {
	v := c.view(i)
	ws := v.pickWS(rnd, func(*model.Workspace) bool { return true })
	if ws == nil {
		return nil
	}
	prefix := prefixPool[rnd.IntN(len(prefixPool))]
	if prefix == ws.IDPrefix || v.clashes(ws.ULID, ws.Name+"\x00", prefix) {
		return nil
	}
	var tasks []*model.Task
	h, ok := c.update(i, c.wsOp(i, ws.ULID), func(tx *store.Tx) error {
		tasks = nil
		w, err := tx.Workspace(ws.ULID)
		if err != nil {
			return err
		}
		w.IDPrefix, w.PrefixSet = prefix, time.Now()
		if err := tx.PutWorkspace(w); err != nil {
			return err
		}
		var ulids []model.ULID
		for _, u := range sortedULIDs(v.tasks) {
			if v.tasks[u].Workspace == ws.ULID {
				ulids = append(ulids, u)
			}
		}
		number := func(u model.ULID) int {
			_, n, _ := model.ParseTaskID(v.tasks[u].ID)
			return n
		}
		slices.SortStableFunc(ulids, func(a, b model.ULID) int { return number(a) - number(b) })
		for _, u := range ulids {
			x, err := tx.Task(u)
			if err != nil {
				return err
			}
			tasks = append(tasks, x)
			if err := tx.PutTask(x); err != nil {
				return err
			}
		}
		return nil
	})
	if !ok {
		return nil
	}
	out := []write{{i, h, ws.ULID, "ws.id_prefix", js(prefix)}}
	for _, x := range tasks {
		if x.ID != v.tasks[x.ULID].ID {
			out = append(out, write{i, h, x.ULID, "id", js(x.ID)})
		}
	}
	return out
}

func opWSRename(c *cluster, i int, rnd *rand.Rand) []write {
	v := c.view(i)
	ws := v.pickWS(rnd, func(*model.Workspace) bool { return true })
	if ws == nil {
		return nil
	}
	name := strings.ToLower(namePool[rnd.IntN(len(namePool))])
	if name == ws.Name || v.clashes(ws.ULID, name, ws.IDPrefix+"\x00") {
		return nil
	}
	h, ok := c.update(i, c.wsOp(i, ws.ULID), func(tx *store.Tx) error {
		w, err := tx.Workspace(ws.ULID)
		if err != nil {
			return err
		}
		w.Name = name
		return tx.PutWorkspace(w)
	})
	if !ok {
		return nil
	}
	return []write{{i, h, ws.ULID, "ws.name", js(name)}}
}

// opWSRm removes a workspace that has no task, deleted ones included, unless
// it is the last one.
func opWSRm(c *cluster, i int, rnd *rand.Rand) []write {
	v := c.view(i)
	if len(v.ws) < 2 {
		return nil // the last workspace stays, or nothing else can be written
	}
	ws := v.pickWS(rnd, func(w *model.Workspace) bool {
		for _, t := range v.tasks {
			if t.Workspace == w.ULID {
				return false
			}
		}
		return true
	})
	if ws == nil {
		return nil
	}
	h, ok := c.update(i, c.wsOp(i, ws.ULID), func(tx *store.Tx) error { return tx.RemoveWorkspace(ws.ULID) })
	if !ok {
		return nil
	}
	return []write{{i, h, ws.ULID, "ws.rm", "null"}}
}

// moveOps are the operations of the tests with two stores: the ones above
// and blocked_by, which the moves between stores cut.
var moveOps = append(slices.Clone(ops), op{"block", opBlock}, op{"block", opBlock}, op{"ref", opRef}, op{"ref", opRef})

// refProp is the task_ref property of the schema of the clusters.
const refProp = "ref"

// opRef makes the task_ref property of a task point at another task of the
// same store. Like opBlock it keeps no write: the invariant is that nothing
// points at a forward.
func opRef(c *cluster, i int, rnd *rand.Rand) []write {
	v := c.view(i)
	t := v.pickTask(rnd, live)
	if t == nil {
		return nil
	}
	b := v.pickTask(rnd, func(x *model.Task) bool { return live(x) && x.ULID != t.ULID })
	if b == nil {
		return nil
	}
	c.update(i, c.taskOp(i, "update", t.ULID), func(tx *store.Tx) error {
		x, err := tx.Task(t.ULID)
		if err != nil {
			return err
		}
		x.Props.Set(refProp, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: b.ULID.String()})
		return tx.PutTask(x)
	})
	return nil
}

// opBlock makes a task wait for another of the same store. It keeps no write:
// the invariant it feeds is that nothing blocks on a forward.
func opBlock(c *cluster, i int, rnd *rand.Rand) []write {
	v := c.view(i)
	t := v.pickTask(rnd, live)
	if t == nil {
		return nil
	}
	b := v.pickTask(rnd, func(x *model.Task) bool { return live(x) && x.ULID != t.ULID })
	if b == nil {
		return nil
	}
	c.update(i, c.taskOp(i, "update", t.ULID), func(tx *store.Tx) error {
		x, err := tx.Task(t.ULID)
		if err != nil {
			return err
		}
		if !slices.Contains(x.BlockedBy, b.ULID) {
			x.BlockedBy = append(x.BlockedBy, b.ULID)
		}
		return tx.PutTask(x)
	})
	return nil
}

// opMoveStore moves a task and everything under it, deleted ones too, from
// clone i of src to clone i of dst, as hedgehug mv --store does: the two
// stores are locked in the order of their ULIDs, dst gets the move-in commit,
// then src the move-out. It returns the writes that dst has made of the
// tasks, for the checks; nil when nothing moved.
func opMoveStore(src, dst *cluster, i int, rnd *rand.Rand) []write {
	sv, dv := src.view(i), dst.view(i)
	// A task may come back to a store that holds its forward: the move-in
	// replaces the forward. One that dst holds as a task cannot move in.
	t := sv.pickTask(rnd, func(x *model.Task) bool {
		return live(x) && !slices.ContainsFunc(sv.subtree(x.ULID), func(u model.ULID) bool { return dv.tasks[u] != nil })
	})
	to := dv.pickWS(rnd, func(*model.Workspace) bool { return true })
	if t == nil || to == nil {
		return nil
	}
	under := sv.subtree(t.ULID)
	var moved, rest []*model.Task
	for _, u := range sortedULIDs(sv.tasks) {
		if !slices.Contains(under, u) {
			rest = append(rest, sv.tasks[u])
		}
	}
	for _, u := range under {
		moved = append(moved, sv.tasks[u])
	}
	notes := map[string][]byte{}
	for _, u := range under {
		n, err := src.stores[i].NoteFiles(u)
		if err != nil {
			src.t.Fatal(err)
		}
		for rel, b := range n {
			notes[rel] = b
		}
	}
	unlinked := store.Unlinks(src.stores[i].Schema(), moved, rest)

	l, err := store.LockInOrder(src.stores[i], dst.stores[i])
	if err != nil {
		src.t.Fatal(err)
	}
	defer l.Release()
	in := dst.op(i, "move-in", t.ULID)
	in.Locked = l
	h, ok := dst.update(i, in, func(tx *store.Tx) error {
		return tx.MoveIn(store.MoveIn{
			From: src.stores[i].Meta().ULID, Tasks: moved, Workspace: to.ULID,
			Notes: notes, Unlinked: unlinked,
		})
	})
	if !ok {
		return nil // the schema of dst refuses the tasks; nothing was written
	}
	out := src.op(i, "move-out", model.ULID{})
	out.Detail = t.ID + " " + t.Title
	out.Locked = l
	if _, ok := src.update(i, out, func(tx *store.Tx) error {
		return tx.MoveOut(store.MoveOut{To: dst.stores[i].Meta().ULID, Tasks: under, Unlinked: unlinked})
	}); !ok {
		src.t.Fatalf("clone %d: the move-out of %s was refused after its move-in", i, t.ULID)
	}

	var ws []write
	for _, x := range moved {
		nt, err := dst.stores[i].Task(x.ULID)
		if err != nil {
			dst.t.Fatal(err)
		}
		// The ids of the source are foreign here, except those this store gave the
		// task before: it may come back, and reusing its own old id is a reuse.
		for _, id := range append([]string{x.ID}, x.FormerIDs...) {
			if !slices.Contains(src.foreign[x.ULID], id) && !slices.Contains(src.held[x.ULID], id) {
				src.held[x.ULID] = append(src.held[x.ULID], id)
			}
			if !slices.Contains(dst.held[x.ULID], id) && !slices.Contains(dst.foreign[x.ULID], id) {
				dst.foreign[x.ULID] = append(dst.foreign[x.ULID], id)
			}
		}
		dst.moves[x.ULID] = src.moves[x.ULID] + 1
		if dst.moves[x.ULID] > 1 {
			dst.multi++
		}
		if dv.fwd[x.ULID] != nil {
			dst.backs++
		}
		// What the task carried is checked in dst from here on: its body, its
		// labels and its comments went with it.
		for _, line := range strings.Split(strings.TrimSuffix(nt.Body, "\n"), "\n") {
			if line != "" {
				ws = append(ws, write{i, h, x.ULID, "body", line})
			}
		}
		for _, l := range nt.Labels {
			ws = append(ws, write{i, h, x.ULID, "labels", l})
		}
		for rel := range notes {
			if dir := path.Dir(model.CommentPath(x.ULID, model.ULID{})) + "/"; strings.HasPrefix(rel, dir) {
				ws = append(ws, write{i, h, x.ULID, "comment", js(strings.TrimSuffix(path.Base(rel), ".md"))})
			}
		}
		ws = append(ws,
			write{i, h, x.ULID, "title", js(nt.Title)},
			write{i, h, x.ULID, "status", js(nt.Status)},
			write{i, h, x.ULID, "workspace", js(nt.Workspace.String())},
			write{i, h, x.ULID, "id", js(nt.ID)},
			write{i, h, x.ULID, "parent", parentJSON(nt.Parent)},
			write{i, h, x.ULID, "deleted_at", deletedJSON(nt.DeletedAt)},
		)
	}
	return ws
}
