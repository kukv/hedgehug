package machineconfig

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
)

func writeConfig(t *testing.T, body string) *File {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return Open(path)
}

func openAll(port.StoreID) bool { return true }

func listIDs(t *testing.T, f *File, open func(port.StoreID) bool) []string {
	t.Helper()
	got, err := f.Stores(open).StoreList().List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, s := range got {
		ids = append(ids, string(s.ID))
	}
	return ids
}

const threeStores = `stores:
  - {id: s1, name: one, path: /a, color: sky}
  - {id: s2, name: two, path: /b, color: pink, hidden: true}
  - {id: s3, name: three, path: /c, color: olive, read_only: true}
`

func TestStoreListOrderAndHidden(t *testing.T) {
	f := writeConfig(t, threeStores)
	got, err := f.Stores(openAll).StoreList().List(context.Background())
	want := []port.StoreInfo{
		{ID: "s1", Name: "one", Color: "sky"},
		{ID: "s3", Name: "three", Color: "olive", ReadOnly: true},
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("List = %+v, %v", got, err)
	}
}

func TestStoreListSkipsUnopened(t *testing.T) {
	f := writeConfig(t, threeStores)
	open := func(id port.StoreID) bool { return id != "s1" }
	v := f.Stores(open)
	ctx := context.Background()
	if got := listIDs(t, f, open); !reflect.DeepEqual(got, []string{"s3"}) {
		t.Errorf("StoreList = %v", got)
	}
	targets, err := v.SyncTargets().List(ctx)
	if err != nil || len(targets) != 2 || targets[0].ID != "s2" || targets[1].ID != "s3" {
		t.Errorf("SyncTargets = %+v, %v", targets, err)
	}
	gh, err := v.StoreGitHub().List(ctx)
	if err != nil || len(gh) != 2 || gh[0].ID != "s2" || gh[1].ID != "s3" {
		t.Errorf("StoreGitHub = %+v, %v", gh, err)
	}
}

func TestStoreListColorFallback(t *testing.T) {
	f := writeConfig(t, `stores:
  - {id: s1, name: a, path: /a, color: teal}
  - {id: s2, name: b, path: /b}
  - {id: s3, name: c, path: /c, color: grey}
  - {id: s4, name: d, path: /d, color: orange}
  - {id: s5, name: e, path: /e}
`)
	got, err := f.Stores(openAll).StoreList().List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var colors []string
	for _, s := range got {
		colors = append(colors, s.Color)
	}
	if want := []string{"teal", "orange", "purple", "orange", "sky"}; !reflect.DeepEqual(colors, want) {
		t.Errorf("colors = %v, want %v", colors, want)
	}
}

func TestSyncTargetsSSHRemote(t *testing.T) {
	f := writeConfig(t, `stores:
  - id: s1
    name: a
    path: /a
    remote: git@github.com:org/tasks.git
    mirrors: [git@github.com:org/m1, "https://github.com/org/m2"]
  - {id: s2, name: b, path: /b, remote: "git@github.com:org/tasks"}
  - {id: s3, name: c, path: /c, remote: "https://github.com/org/tasks"}
`)
	got, err := f.Stores(openAll).SyncTargets().List(context.Background())
	if err != nil || len(got) != 3 {
		t.Fatalf("List = %+v, %v", got, err)
	}
	const url = "https://github.com/org/tasks.git"
	if got[0].Remote != url || got[1].Remote != url || got[2].Remote != "https://github.com/org/tasks" {
		t.Errorf("remotes = %q %q %q", got[0].Remote, got[1].Remote, got[2].Remote)
	}
	if want := []string{"https://github.com/org/m1.git", "https://github.com/org/m2"}; !reflect.DeepEqual(got[0].Mirrors, want) {
		t.Errorf("mirrors = %v", got[0].Mirrors)
	}
}

func TestStoreGitHubUnassign(t *testing.T) {
	f := writeConfig(t, `login: taro
accounts:
  - {id: a1, name: n, login: l, method: token}
hosts:
  - {host: example.com, client_id: abc}
sync:
  auto: false
stores:
  - {id: s1, name: a, path: /a, account: a1, remote: "https://github.com/o/r"}
  - {id: s2, name: b, path: /b, account: a1}
  - {id: s3, name: c, path: /c, account: a2}
`)
	gh := f.Stores(openAll).StoreGitHub()
	ctx := context.Background()
	if err := gh.Unassign(ctx, "a1"); err != nil {
		t.Fatal(err)
	}
	got, err := gh.List(ctx)
	if err != nil || len(got) != 3 || got[0].Account != "" || got[1].Account != "" || got[2].Account != "a2" || !got[0].Remote || got[1].Remote {
		t.Errorf("List = %+v, %v", got, err)
	}
	if login, err := gh.MachineLogin(ctx); err != nil || login != "taro" {
		t.Errorf("MachineLogin = %q, %v", login, err)
	}
	c, err := f.read()
	if err != nil || len(c.Accounts) != 1 || len(c.Hosts) != 1 || c.Sync.Auto == nil || *c.Sync.Auto {
		t.Errorf("other keys lost: %+v, %v", c, err)
	}
}

const oldShape = "stores:\n  - {name: a, path: /x, remotes: [u]}\n"

func TestStoreEntriesSkipsOldShape(t *testing.T) {
	f := writeConfig(t, oldShape)
	got, err := f.StoreEntries()
	if err != nil || len(got) != 0 {
		t.Errorf("StoreEntries = %+v, %v", got, err)
	}
	f = writeConfig(t, "stores:\n  - {id: s1, name: a, path: /a}\n  - {id: s2, name: b}\n")
	got, err = f.StoreEntries()
	if want := []StoreEntry{{ID: "s1", Path: "/a"}}; err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("StoreEntries = %+v, %v", got, err)
	}
}

func TestOldShapeSurvivesRewrite(t *testing.T) {
	f := writeConfig(t, oldShape)
	if err := f.Stores(openAll).StoreGitHub().Unassign(context.Background(), "a1"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(f.path)
	if err != nil || !strings.Contains(string(b), "path: /x") {
		t.Errorf("rewritten = %s, %v", b, err)
	}
	if _, err := f.read(); err != nil {
		t.Errorf("read after rewrite: %v", err)
	}
	if got, err := f.StoreEntries(); err != nil || len(got) != 0 {
		t.Errorf("StoreEntries = %+v, %v", got, err)
	}
}
