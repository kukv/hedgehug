package gitstore

import (
	"encoding/json"
	"sort"
	"strings"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/domain/change"
	commentlog "github.com/kukv/hedgehug/internal/domain/comment"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
	"github.com/kukv/hedgehug/internal/domain/worklog"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// itemToYAML は、item.yaml の中身。status・priority・systems・fields の行末に、値の名前を注釈で書く(B2-19)。
// 名前が分からない値(消した値、設定のログがないワークスペース)には注釈を付けない。
func itemToYAML(it item.Item, log []change.Record, settings settingsOf) itemYAML {
	links := itemlink.FromLog(log)
	var st workspace.Settings
	if settings != nil {
		st, _ = settings(it.Place.Workspace)
	}
	y := itemYAML{
		ID: string(it.ID), Workspace: string(it.Place.Workspace), Project: string(it.Place.Project),
		Number: it.Place.Number, Kind: string(it.Kind), Rank: it.Rank, Title: it.Title,
		Created:  createdYAML{At: it.Created.At.Format(change.TimeLayout), By: it.Created.By},
		Status:   idNode(it.Status),
		Priority: idNode(it.Priority),
		Assignee: optString(it.Assignee), Start: optString(it.Start), End: optString(it.End),
		Points: it.Points, Estimate: it.Estimate,
		Systems:   systemsNode(it.Systems, st),
		Fields:    fieldsNode(it, log, st),
		Relations: relationsNode(it.Relations),
		Links:     linksNode(links.Links),
		LinksOff:  offNode(links.Off),
		Watchers:  it.Watchers,
		Comments:  commentsNode(commentlog.FromLog(log)),
		Worklogs:  worklogsNode(worklog.FromLog(log)),
	}
	y.Status.LineComment = comment(valueName(st.Statuses, it.Status))
	y.Priority.LineComment = comment(valueName(st.Priorities, it.Priority))
	if it.Place.Parent != nil {
		p := string(*it.Place.Parent)
		y.Parent = &p
	}
	return y
}

// optString は、空の文字列を nil(null)にする。
func optString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func systemsNode(ids []change.ID, st workspace.Settings) *yaml.Node {
	vs := make([]string, len(ids))
	for i, id := range ids {
		vs[i] = string(id)
	}
	n := flowSeq(vs...)
	n.LineComment = comment(strings.Join(namesOf(st.Systems, ids), ", "))
	return n
}

// namesOf は、ids のうち一覧にあるものの名前(ids の並び)。
func namesOf(vs []workspace.Value, ids []change.ID) []string {
	var out []string
	for _, id := range ids {
		if name := valueName(vs, id); name != "" {
			out = append(out, name)
		}
	}
	return out
}

// fieldsNode は、追加プロパティの値(プロパティの ID の辞書順)。複数選択は [..]、単一選択は 1 つの値、
// 選択式でない値はそのままの値。行末の注釈は選択式だけに付け、「プロパティ名: 値の名前」。
func fieldsNode(it item.Item, log []change.Record, st workspace.Settings) *yaml.Node {
	n := &yaml.Node{Kind: yaml.MappingNode}
	props := make([]string, 0, len(it.Choices)+len(it.Fields))
	for p := range it.Choices {
		props = append(props, string(p))
	}
	for p := range it.Fields {
		if _, ok := it.Choices[p]; !ok {
			props = append(props, string(p))
		}
	}
	if len(props) == 0 {
		n.Style = yaml.FlowStyle
		return n
	}
	sort.Strings(props)
	for _, p := range props {
		vs, ok := it.Choices[change.ID(p)]
		if !ok {
			n.Content = append(n.Content, key(p), rawNode(it.Fields[change.ID(p)]))
			continue
		}
		val := strNode(string(vs[0]))
		if item.MultiChoice(log, change.ID(p)) {
			ids := make([]string, len(vs))
			for i, v := range vs {
				ids[i] = string(v)
			}
			val = flowSeq(ids...)
		}
		val.LineComment = fieldComment(st, change.ID(p), vs)
		n.Content = append(n.Content, key(p), val)
	}
	return n
}

// rawNode は、ログに書いた JSON の値を、そのままの値のノードにする。
func rawNode(raw json.RawMessage) *yaml.Node {
	var v any
	_ = json.Unmarshal(raw, &v)
	n := &yaml.Node{}
	_ = n.Encode(v)
	return n
}

// fieldComment は「プロパティ名: 値の名前, …」。プロパティか値の名前が分からなければ空。
func fieldComment(st workspace.Settings, p change.ID, vs []change.ID) string {
	prop, ok := st.Property(p)
	if !ok {
		return ""
	}
	names := namesOf(prop.Choices, vs)
	if len(names) == 0 {
		return ""
	}
	return comment(prop.Name + ": " + strings.Join(names, ", "))
}

// relationsNode は、このアイテムが持つ関連を 1 件 1 行で書く(足した側だけが持つ)。行末の注釈は付けない
// (相手は別のワークスペースのこともあり、表示の ID はそのワークスペースの接頭辞で決まるため)。
func relationsNode(rs []item.Relation) *yaml.Node {
	rows := make([]*yaml.Node, len(rs))
	for i, r := range rs {
		rows[i] = mapping(true, key("type"), strNode(r.Type), key("to"), strNode(string(r.To)))
	}
	return seqOf(rows)
}

// commentsNode は、いまのコメントを投稿の古い順に 1 件 1 行で書く。by・at は最初の行のもの。
func commentsNode(cs []commentlog.Comment) *yaml.Node {
	rows := make([]*yaml.Node, len(cs))
	for i, c := range cs {
		rows[i] = mapping(true,
			key("id"), strNode(string(c.ID)),
			key("by"), strNode(c.By),
			key("at"), strNode(c.At.Format(change.TimeLayout)),
			key("body"), strNode(c.Body),
			key("edited"), boolNode(c.Edited),
		)
	}
	return seqOf(rows)
}

// worklogsNode は、いまの作業時間を記録の古い順に 1 件 1 行で書く。時間は小数(1.5・2)。
func worklogsNode(es []worklog.Entry) *yaml.Node {
	rows := make([]*yaml.Node, len(es))
	for i, e := range es {
		hours := &yaml.Node{}
		_ = hours.Encode(float64(e.Hundredths) / 100)
		rows[i] = mapping(true,
			key("id"), strNode(string(e.ID)),
			key("by"), strNode(e.By),
			key("date"), strNode(e.Date),
			key("hours"), hours,
			key("memo"), strNode(e.Memo),
		)
	}
	return seqOf(rows)
}

// linksNode は、PR・Issue のリンクを、リンクした順に 1 件 1 行で書く(保存形式 103〜106 行)。Issue は review を書かない。
func linksNode(ls []itemlink.Link) *yaml.Node {
	rows := make([]*yaml.Node, len(ls))
	for i, l := range ls {
		kv := []*yaml.Node{
			key("kind"), strNode(string(l.Kind)), key("repo"), strNode(l.Repo), key("number"), intNode(l.Number),
			key("title"), strNode(l.Title), key("state"), strNode(string(l.State)),
		}
		if l.Kind == itemlink.PR {
			kv = append(kv, key("review"), strNode(string(l.Review)))
		}
		rows[i] = mapping(true, kv...)
	}
	return seqOf(rows)
}

// offNode は、解除した PR の記録を 1 件 1 行で書く。
func offNode(rs []itemlink.Ref) *yaml.Node {
	rows := make([]*yaml.Node, len(rs))
	for i, r := range rs {
		rows[i] = mapping(true, key("kind"), strNode(string(r.Kind)), key("repo"), strNode(r.Repo), key("number"), intNode(r.Number))
	}
	return seqOf(rows)
}

func intNode(n int) *yaml.Node {
	v := &yaml.Node{}
	_ = v.Encode(n)
	return v
}
