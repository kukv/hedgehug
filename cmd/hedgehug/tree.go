package main

import (
	"errors"
	"text/template"

	"github.com/kukv/hedgehug/internal/query"
)

func (a *app) lsTree(*query.Query, bool, *template.Template) error {
	return errors.New("--tree is not done yet")
}
