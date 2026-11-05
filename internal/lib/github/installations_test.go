package github

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAppMissing(t *testing.T) {
	t.Parallel()
	all := map[string]any{"id": 1, "account": map[string]any{"login": "Org-A"}, "repository_selection": "all"}
	sel := map[string]any{"id": 2, "account": map[string]any{"login": "org-b"}, "repository_selection": "selected"}
	cases := []struct {
		name, owner, repo string
		refused           bool
		want              bool
	}{
		{name: "installed on every repository, in any case", owner: "org-a", repo: "web"},
		{name: "the repository is selected, on the second page", owner: "org-b", repo: "web"},
		{name: "the repository is not selected", owner: "org-b", repo: "ops", want: true},
		{name: "not installed on the owner", owner: "org-c", repo: "web", want: true},
		{name: "a token of no GitHub App says nothing", owner: "org-c", repo: "web", refused: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mux := http.NewServeMux()
			mux.HandleFunc("GET /user/installations", func(w http.ResponseWriter, _ *http.Request) {
				if tc.refused {
					w.WriteHeader(http.StatusForbidden)
					_, _ = w.Write([]byte(`{"message":"You must authenticate with an access token authorized to a GitHub App in order to list installations"}`))
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"installations": []any{all, sel}})
			})
			mux.HandleFunc("GET /user/installations/2/repositories", func(w http.ResponseWriter, r *http.Request) {
				repos := []any{}
				if r.URL.Query().Get("page") == "1" {
					for i := range installationPage {
						repos = append(repos, map[string]any{"name": fmt.Sprintf("r%d", i)})
					}
				} else {
					repos = append(repos, map[string]any{"name": "Web"})
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"repositories": repos})
			})
			srv := httptest.NewServer(mux)
			defer srv.Close()
			if got := New(Options{API: srv.URL}, "t", nil).AppMissing(tc.owner, tc.repo); got != tc.want {
				t.Errorf("AppMissing = %v, want %v", got, tc.want)
			}
		})
	}
}
