package github

import (
	"fmt"
	"strings"
)

const installationPage = 100

type installation struct {
	ID      int64 `json:"id"`
	Account struct {
		Login string `json:"login"`
	} `json:"account"`
	RepositorySelection string `json:"repository_selection"`
}

// pages reads a list GitHub wraps in an object: page is given each page, and
// says how many items it had.
func (c *Client) pages(url string, page func(url string) (int, error)) error {
	for n := 1; ; n++ {
		got, err := page(fmt.Sprintf("%s?per_page=%d&page=%d", url, installationPage, n))
		if err != nil {
			return err
		}
		if got < installationPage {
			return nil
		}
	}
}

// AppMissing reports whether GitHub says the GitHub App of the token does not
// reach owner/name. It is what is left to ask after a 404: GitHub hides a
// private repository from such a token instead of refusing it. A token of no
// GitHub App is refused the list, and the answer is then false: nothing is
// known.
func (c *Client) AppMissing(owner, name string) bool {
	var found *installation
	err := c.pages(c.api+"/user/installations", func(url string) (int, error) {
		var p struct {
			Installations []installation `json:"installations"`
		}
		if _, err := c.do("GET", url, nil, &p); err != nil {
			return 0, err
		}
		for i := range p.Installations {
			if strings.EqualFold(p.Installations[i].Account.Login, owner) {
				found = &p.Installations[i]
			}
		}
		return len(p.Installations), nil
	})
	if err != nil {
		return false
	}
	if found == nil {
		return true
	}
	if found.RepositorySelection != "selected" {
		return false
	}
	selected := false
	err = c.pages(fmt.Sprintf("%s/user/installations/%d/repositories", c.api, found.ID), func(url string) (int, error) {
		var p struct {
			Repositories []struct {
				Name string `json:"name"`
			} `json:"repositories"`
		}
		if _, err := c.do("GET", url, nil, &p); err != nil {
			return 0, err
		}
		for _, r := range p.Repositories {
			selected = selected || strings.EqualFold(r.Name, name)
		}
		return len(p.Repositories), nil
	})
	return err == nil && !selected
}
