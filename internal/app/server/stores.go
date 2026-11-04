package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/stores"
)

// storesChanged is the event that the stores of config.yaml changed: one came
// or went, or its path, color, visibility or account did, or the accounts or
// the default store did. It has no body; the GUI reads GET /api/stores again.
type storesChanged struct{}

// storesWritten is what a write to the stores or the accounts of config.yaml
// through the API does afterwards: it tells the clients, makes the poller
// look now (so that a store that came is opened, and one that went is let
// go), and wakes the sync loop (a store that came is synced). The poller
// tells the clients too when it finds the change itself, so a client may get
// the event twice; reading the stores again is idempotent.
func (s *Server) storesWritten() {
	s.hub.Publish(storesChanged{})
	s.poller.Kick()
	s.loop.Wake()
}

type storeAddInput struct {
	Body struct {
		Name    string   `json:"name" minLength:"1"`
		Remotes []string `json:"remotes" minItems:"1" doc:"Where to clone from; the first one, and the others are mirrors"`
		Account string   `json:"account,omitempty" doc:"The name of the account that clones and writes; the only account without it"`
	}
}

type storeViewOutput struct{ Body stores.StoreView }

type storeRemoveInput struct {
	Name string `path:"name" doc:"The store"`
	Keep bool   `query:"keep" default:"false" doc:"Take the store out of the config and leave the clone; without it the clone is deleted, unless commits are not pushed (409 store.unpushed)"`
}

type storeRemoveOutput struct{ Body stores.StoreRemoveResult }

type storeDefaultInput struct {
	Body struct {
		Name string `json:"name" minLength:"1"`
	}
}

type storeDefaultOutput struct {
	Body struct {
		Name string `json:"name"`
	}
}

type storePatchInput struct {
	Name string `path:"name" doc:"The store"`
	Body struct {
		Color  *string `json:"color,omitempty" doc:"One of the colors of the palette; an empty one puts it back to automatic"`
		Hidden *bool   `json:"hidden,omitempty" doc:"Whether the GUI leaves the store out of what it shows"`
	}
}

type storeDiscoverInput struct {
	Account string `query:"account" doc:"Look with this account only; with from, the account of the candidates"`
	From    string `query:"from" doc:"A directory to look for bare repositories that hold a store under; without it, GitHub"`
}

type candidatesOutput struct{ Body []stores.Candidate }

// mountCandidate is a candidate as the page sends it back: with the name it
// changed. The permission is the page's to show and is not needed here.
type mountCandidate struct {
	Name    string `json:"name" minLength:"1" doc:"The name to mount the store under"`
	URL     string `json:"url" minLength:"1"`
	Account string `json:"account,omitempty"`
}

type mountInput struct{ Body []mountCandidate }

type mountFailed struct {
	Name    string `json:"name"`
	Code    string `json:"code" doc:"What went wrong, as the type of a problem"`
	Message string `json:"message" doc:"The message in English, as the CLI prints it"`
}

type mountOutput struct {
	Body struct {
		Mounted []string      `json:"mounted" doc:"The names of the stores that were added"`
		Failed  []mountFailed `json:"failed" doc:"The candidates that could not be added; the others were"`
	}
}

// storeViewOf is the row of the store name of StoreList.
func storeViewOf(ss *usecase.Session, name string) (stores.StoreView, error) {
	list, err := stores.StoreList(ss)
	if err != nil {
		return stores.StoreView{}, err
	}
	for _, v := range list {
		if v.Name == name {
			return v, nil
		}
	}
	return stores.StoreView{}, fmt.Errorf("store %s is not listed after it was added", name)
}

func (s *Server) registerStores() {
	huma.Register(s.api, huma.Operation{
		OperationID: "add-store", Method: http.MethodPost, Path: "/api/stores/add", DefaultStatus: http.StatusCreated,
		Summary: "Set up a store that others made by cloning it, as hh store add", Tags: []string{"stores"},
	}, func(_ context.Context, in *storeAddInput) (*storeViewOutput, error) {
		b := in.Body
		ss := s.session("")
		defer ss.Close()
		r, err := stores.StoreAdd(ss, b.Name, b.Remotes, "", b.Account)
		if err != nil {
			return nil, problemOf(err)
		}
		v, err := storeViewOf(ss, r.Name)
		if err != nil {
			return nil, problemOf(err)
		}
		s.storesWritten()
		return &storeViewOutput{Body: v}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "remove-store", Method: http.MethodDelete, Path: "/api/stores/{name}",
		Summary: "Take a store out of the config and delete its clone, as hh store remove", Tags: []string{"stores"},
	}, func(_ context.Context, in *storeRemoveInput) (*storeRemoveOutput, error) {
		ss := s.session(in.Name)
		defer ss.Close()
		var res stores.StoreRemoveResult
		remove := func() (err error) {
			res, err = stores.StoreRemove(ss, in.Name, in.Keep)
			return err
		}
		var err error
		// The poller holds the store open; the clone is deleted, so it lets go
		// first, and no poll opens it again until the removal is over. With
		// keep the clone stays, and the poller lets go when the store is gone
		// from the config. A name that is no store is StoreRemove's to refuse.
		if e, eerr := ss.CurrentStore(); eerr == nil && !in.Keep {
			err = s.poller.ReleaseFor(e.Path, remove)
		} else {
			err = remove()
		}
		if err != nil {
			return nil, problemOf(err)
		}
		s.storesWritten()
		return &storeRemoveOutput{Body: res}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "set-default-store", Method: http.MethodPut, Path: "/api/stores/default",
		Summary: "Make a store the one hedgehug picks without --store, as hh store default", Tags: []string{"stores"},
	}, func(_ context.Context, in *storeDefaultInput) (*storeDefaultOutput, error) {
		ss := s.session("")
		defer ss.Close()
		name, err := stores.StoreDefault(ss, in.Body.Name)
		if err != nil {
			return nil, problemOf(err)
		}
		s.storesWritten()
		out := &storeDefaultOutput{}
		out.Body.Name = name
		return out, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "update-store", Method: http.MethodPatch, Path: "/api/stores/{name}",
		Summary: "Set the color or the visibility of a store on this machine, as hh store set", Tags: []string{"stores"},
	}, func(_ context.Context, in *storePatchInput) (*storeViewOutput, error) {
		b := in.Body
		if b.Color == nil && b.Hidden == nil {
			return nil, newProblem(http.StatusBadRequest, "store.set_args", "give color or hidden")
		}
		ss := s.session("")
		defer ss.Close()
		v, err := stores.StoreSet(ss, in.Name, b.Color, b.Hidden)
		if err != nil {
			return nil, problemOf(err)
		}
		s.storesWritten()
		return &storeViewOutput{Body: v}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "discover-stores", Method: http.MethodGet, Path: "/api/stores/discover",
		Summary: "List the stores that can be set up, as hh store discover", Tags: []string{"stores"},
	}, func(_ context.Context, in *storeDiscoverInput) (*candidatesOutput, error) {
		ss := s.session("")
		defer ss.Close()
		cands, err := stores.StoreDiscover(ss, stores.DiscoverInput{Account: in.Account, From: in.From})
		if err != nil {
			p := problemOf(err)
			if p.Type == "choice.required" {
				// The usecase calls it a usage error (the CLI asks); to the page the
				// request is fine and the answer needs a pick.
				p.Status, p.Title = http.StatusUnprocessableEntity, http.StatusText(http.StatusUnprocessableEntity)
			}
			return nil, p
		}
		if cands == nil {
			cands = []stores.Candidate{}
		}
		return &candidatesOutput{Body: cands}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "mount-stores", Method: http.MethodPost, Path: "/api/stores/mount",
		Summary: "Set up the stores discover listed, each as hh store add does", Tags: []string{"stores"},
	}, func(_ context.Context, in *mountInput) (*mountOutput, error) {
		if len(in.Body) == 0 {
			return nil, badRequest(errors.New("give at least one store"))
		}
		cands := make([]stores.Candidate, len(in.Body))
		for i, c := range in.Body {
			cands[i] = stores.Candidate{Name: c.Name, URL: c.URL, Account: c.Account}
		}
		ss := s.session("")
		defer ss.Close()
		res, err := stores.StoreMount(ss, cands)
		if err != nil && len(res.Failed) == 0 {
			return nil, problemOf(err) // StoreMount's own error is the failures, which are in res
		}
		out := &mountOutput{}
		out.Body.Mounted, out.Body.Failed = append([]string{}, res.Mounted...), []mountFailed{}
		for _, f := range res.Failed {
			p := problemOf(f.Err)
			out.Body.Failed = append(out.Body.Failed, mountFailed{Name: f.Name, Code: p.Type, Message: p.Detail})
		}
		if len(res.Mounted) > 0 {
			s.storesWritten()
		}
		return out, nil
	})
}
