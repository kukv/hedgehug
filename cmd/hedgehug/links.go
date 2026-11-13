package main

import (
	"github.com/kukv/hedgehug/internal/adapter/httpapi"
	"github.com/kukv/hedgehug/internal/app/githublink"
	"github.com/kukv/hedgehug/internal/app/port"
)

// linksOf は、d の口で、リンクの操作を組む。API と、GitHub を自動で読む Reader が同じものを使う(D4a)。
func linksOf(d httpapi.Deps, tokens port.GitHubTokens) githublink.Links {
	return githublink.Links{
		Stores: d.Stores, Identity: d.Identity, Tokens: tokens, GitHub: d.GitHub, Accounts: d.Accounts,
		StoreGH: d.StoreGitHub, Checks: d.Checks, Reads: d.GitHubReads, Scans: d.PRScans, Missing: d.AppMissing,
		Inbox: d.Inbox, Events: d.Events, Clock: d.Clock,
	}
}
