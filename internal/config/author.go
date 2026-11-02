package config

import "github.com/kukv/hedgehug/internal/gitrepo"

// CommitAuthor returns the author of the commits of a store: the login of the
// account that writes it, with the noreply address of its host.
func CommitAuthor(acct Account) gitrepo.Signature {
	return gitrepo.Signature{Name: acct.Login, Email: acct.Login + "@users.noreply." + acct.Host}
}
