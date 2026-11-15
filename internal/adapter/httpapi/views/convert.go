package views

import (
	"github.com/kukv/hedgehug/internal/app/port"
	appviews "github.com/kukv/hedgehug/internal/app/views"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

func portStore(s string) port.StoreID { return port.StoreID(s) }

func workspaceID(s string) workspace.ID { return workspace.ID(s) }

func toView(v appviews.View) View {
	c := v.Content
	out := View{ID: v.ID, Name: v.Name, Content: ViewContent{
		Query: c.Query, Layout: c.Layout,
		Table: ViewTable{Columns: append([]string{}, c.Table.Columns...), Group: c.Table.Group},
	}}
	if c.Table.Sort != nil {
		out.Content.Table.Sort = &ViewSort{Key: c.Table.Sort.Key, Desc: c.Table.Sort.Desc}
	}
	return out
}

func toContent(c ViewContent) appviews.Content {
	out := appviews.Content{
		Query: c.Query, Layout: c.Layout,
		Table: appviews.Table{Columns: c.Table.Columns, Group: c.Table.Group},
	}
	if c.Table.Sort != nil {
		out.Table.Sort = &appviews.Sort{Key: c.Table.Sort.Key, Desc: c.Table.Sort.Desc}
	}
	return out
}
