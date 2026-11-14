package search

import (
	"reflect"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/query"
)

func searchWith(t *testing.T, f fixture, text string, opt TableOptions) Result {
	t.Helper()
	res, err := f.svc.Search(t.Context(), text, opt)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func titlesOf(res Result) []string {
	var out []string
	for _, r := range res.Rows {
		out = append(out, r.Facts.Item.Title)
	}
	return out
}

// sort:-points のクエリでも、列見出しの並べ替え(タイトル)があればそちらで並べる。なければクエリの sort:。
func TestSearchHeaderSortOverridesQuery(t *testing.T) {
	f := newFixture(t)
	must(t, apptest.Sample(t.Context(), f.stores, store1))
	for i, c := range []struct {
		id    item.ID
		title string
		pt    float64
	}{{item3, "b", 3}, {item4, "c", 8}, {item5, "a", 5}} {
		it := task(c.id, 3+i, item.Bug, nil, c.title)
		it.Points = pts(c.pt)
		addItem(t, f.stores, it)
	}
	byTitle := titlesOf(searchWith(t, f, "kind:bug sort:-points", TableOptions{Sort: &query.Sort{Key: "title"}}))
	if want := []string{"a", "b", "c"}; !reflect.DeepEqual(byTitle, want) {
		t.Errorf("列見出し = %v, want %v", byTitle, want)
	}
	byQuery := titlesOf(searchWith(t, f, "kind:bug sort:-points", TableOptions{}))
	if want := []string{"c", "a", "b"}; !reflect.DeepEqual(byQuery, want) {
		t.Errorf("クエリ = %v, want %v", byQuery, want)
	}
}

// テーブルの「グループ」の設定が先で、「なし」ならクエリの group:。どちらも使えなければ分けない。
func TestSearchGroupSettingWins(t *testing.T) {
	f := newFixture(t)
	must(t, apptest.Sample(t.Context(), f.stores, store1))
	for _, c := range []struct{ text, setting, want string }{
		{"group:kind", "assignee", "assignee"}, {"group:kind", "none", "kind"}, {"group:kind", "", "kind"},
	} {
		if got := searchWith(t, f, c.text, TableOptions{Group: c.setting}).GroupBy; got != c.want {
			t.Errorf("%q + %q: GroupBy = %q, want %q", c.text, c.setting, got, c.want)
		}
	}
	res := searchWith(t, f, "group:foo", TableOptions{})
	if res.GroupBy != "" || res.Groups != nil {
		t.Errorf("GroupBy = %q, Groups = %v", res.GroupBy, res.Groups)
	}
}

// Groups の Rows は、並べ替えたあとの Rows の位置を指す。
func TestSearchGroupsUseSortedRows(t *testing.T) {
	f := newFixture(t)
	must(t, apptest.Sample(t.Context(), f.stores, store1))
	a := task(item3, 3, item.Task, nil, "a")
	a.Status = apptest.SampleStatusDone
	addItem(t, f.stores, a)
	addItem(t, f.stores, task(item4, 4, item.Task, nil, "b"))
	res := searchWith(t, f, "sort:title group:status", TableOptions{})
	if got, want := titlesOf(res), []string{"a", "b", "ログイン画面を作る", "認証"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("rows = %v, want %v", got, want)
	}
	if len(res.Groups) != 2 || res.Groups[0].Label != "Done" || !reflect.DeepEqual(res.Groups[0].Rows, []int{0}) ||
		res.Groups[1].Label != "Todo" || !reflect.DeepEqual(res.Groups[1].Rows, []int{1, 2, 3}) {
		t.Errorf("Groups = %+v", res.Groups)
	}
}

// クエリのエラーでも、「表示する列」を開けるように Columns は返す。
func TestSearchColumnsOnError(t *testing.T) {
	f := newFixture(t)
	must(t, apptest.Sample(t.Context(), f.stores, store1))
	res := searchOf(t, f, "status:")
	if len(res.Errors) != 1 || len(res.Columns) != 17 {
		t.Errorf("Errors = %v, Columns = %d", res.Errors, len(res.Columns))
	}
}
