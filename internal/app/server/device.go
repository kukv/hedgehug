package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/app/usecase"
)

// deviceRun is the device flow that is running.
type deviceRun struct {
	id     string
	cancel context.CancelFunc
	done   chan struct{} // closed by end
	once   sync.Once     // end runs once: runDevice calls it early and defers it
}

// deviceFlow holds the one device flow the server runs at a time.
type deviceFlow struct {
	mu      sync.Mutex
	current *deviceRun
	wg      sync.WaitGroup // every goroutine of a run, until it has told how it ended
}

// begin starts a run on a context of parent, unless one is running.
func (d *deviceFlow) begin(parent context.Context) (*deviceRun, context.Context, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.current != nil || parent.Err() != nil { // none starts once Shutdown ended parent
		return nil, nil, false
	}
	d.wg.Add(1)
	ctx, cancel := context.WithCancel(parent)
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	r := &deviceRun{id: hex.EncodeToString(b), cancel: cancel, done: make(chan struct{})}
	d.current = r
	return r, ctx, true
}

// end clears r, before its event is published: a screen that starts the next
// flow on the event is not told 409.
func (d *deviceFlow) end(r *deviceRun) {
	r.once.Do(func() {
		d.mu.Lock()
		if d.current == r {
			d.current = nil
		}
		d.mu.Unlock()
		r.cancel()
		close(r.done)
	})
}

// get is the run with the id, or nil.
func (d *deviceFlow) get(id string) *deviceRun {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.current != nil && d.current.id == id {
		return d.current
	}
	return nil
}

// wait cancels the run, if any, and waits until its goroutine has returned
// (after its event). The caller has ended the parent context first, so that
// begin starts no new run: the lock orders this after any begin that did.
func (d *deviceFlow) wait() {
	d.mu.Lock()
	r := d.current
	d.mu.Unlock()
	if r != nil {
		r.cancel()
	}
	d.wg.Wait()
}

type deviceStartInput struct {
	Body struct {
		Name string `json:"name,omitempty" doc:"The account to log in (again), as hh auth login <name>; without it the account of the login that approves the code is added"`
		Host string `json:"host,omitempty" doc:"As hh auth login --host: the host of a new account (github.com without it); for an account that exists it must be its host"`
	}
}

type deviceStartBody struct {
	ID              string    `json:"id" doc:"For DELETE /api/auth/device/{id} and the event auth.device"`
	UserCode        string    `json:"user_code" doc:"The code to enter at verification_uri"`
	VerificationURI string    `json:"verification_uri"`
	ExpiresAt       time.Time `json:"expires_at" doc:"When the code ends; the flow ends with an error event then"`
}

type deviceStartOutput struct{ Body deviceStartBody }

type deviceCancelInput struct {
	ID string `path:"id" doc:"The id POST /api/auth/device answered"`
}

// authDeviceEvent is how a device flow ended.
type authDeviceEvent struct {
	ID      string `json:"id" doc:"The id POST /api/auth/device answered"`
	State   string `json:"state" enum:"done,error"`
	Account string `json:"account,omitempty" doc:"For done: the name of the account that got the token"`
	Code    string `json:"code,omitempty" doc:"For error: the Code of the error, such as auth.canceled"`
	Message string `json:"message,omitempty" doc:"For error: the message in English"`
}

// deviceStart is what the goroutine of a flow tells the request that started
// it: the code, or the problem that stopped the flow before there was one.
type deviceStart struct {
	code usecase.DeviceCode
	prob *Problem
}

func (s *Server) registerDevice() {
	huma.Register(s.api, huma.Operation{
		OperationID: "start-device-login", Method: http.MethodPost, Path: "/api/auth/device", DefaultStatus: http.StatusCreated,
		Summary: "Start the device flow, as hh auth login [<name>] [--host]; the server polls, and the event auth.device says how it ended", Tags: []string{"accounts"},
	}, func(ctx context.Context, in *deviceStartInput) (*deviceStartOutput, error) {
		run, rctx, ok := s.device.begin(s.base)
		if !ok {
			return nil, newProblem(http.StatusConflict, "auth.device_running",
				"a device flow is running; wait for it, or cancel it with DELETE /api/auth/device/{id}")
		}
		started := make(chan deviceStart, 1)
		go s.runDevice(rctx, run, in.Body.Name, in.Body.Host, started)
		select {
		case st := <-started:
			if st.prob != nil {
				return nil, st.prob
			}
			return &deviceStartOutput{Body: deviceStartBody{
				ID: run.id, UserCode: st.code.UserCode, VerificationURI: st.code.VerificationURI,
				ExpiresAt: s.opt.Now().Add(time.Duration(st.code.ExpiresIn) * time.Second),
			}}, nil
		case <-ctx.Done(): // the client went away before the code: nobody will enter it
			run.cancel()
			return nil, newProblem(http.StatusRequestTimeout, "request.canceled", "the request ended before the code was shown")
		}
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "cancel-device-login", Method: http.MethodDelete, Path: "/api/auth/device/{id}",
		Summary: "Stop the device flow; it ends with the event auth.device (error, auth.canceled)", Tags: []string{"accounts"},
	}, func(ctx context.Context, in *deviceCancelInput) (*struct{}, error) {
		run := s.device.get(in.ID)
		if run == nil {
			return nil, newProblem(http.StatusNotFound, "auth.device_not_found", "no device flow "+in.ID+" is running")
		}
		run.cancel()
		select {
		case <-run.done:
		case <-ctx.Done():
		}
		return nil, nil
	})
}

// runDevice runs the login of one device flow and tells how it ended: the
// request that started it before the code was shown, the event after.
func (s *Server) runDevice(ctx context.Context, run *deviceRun, name, host string, started chan<- deviceStart) {
	defer s.device.wg.Done()
	defer s.device.end(run) // whatever happens, the flow is not left running
	ss := s.session("")
	defer ss.Close()
	shown := false
	res, err := ss.AuthLogin(usecase.AuthLoginInput{
		Name: name, Host: host, HostGiven: host != "", Context: ctx,
		OnCode: func(c usecase.DeviceCode) {
			shown = true
			started <- deviceStart{code: c}
		},
	})
	s.device.end(run)
	switch {
	case err != nil && !shown:
		started <- deviceStart{prob: problemOf(err)}
	case err != nil:
		e := usecase.AsError(err)
		code := e.Code
		if code == "" {
			code = "internal"
		}
		s.hub.Publish(authDeviceEvent{ID: run.id, State: "error", Code: code, Message: err.Error()})
	case res.Uses != "": // an account of gh or env:VAR: no code was asked for
		p := newProblem(http.StatusUnprocessableEntity, "auth.no_device_flow",
			"account "+res.Account+" takes its token from "+res.Uses+"; Hedgehug has nothing to log in")
		p.Args = map[string]any{"account": res.Account, "uses": res.Uses}
		started <- deviceStart{prob: p}
	default:
		s.pinnedStores(res.Pinned)
		s.hub.Publish(authDeviceEvent{ID: run.id, State: "done", Account: res.Account})
	}
}
