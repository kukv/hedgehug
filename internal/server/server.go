// Package server is the HTTP server of hh ui: the REST API under /api, its
// events, the sync loop, and the built GUI.
package server

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	"github.com/kukv/hedgehug/internal/auth"
	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/ops"
	"github.com/kukv/hedgehug/internal/store"
)

// APIVersion is the version of the API in api/openapi.yaml. It is not the
// version of hedgehug, so that the file does not change with each build.
const APIVersion = "1"

// Options are what the server is made with.
type Options struct {
	// The fields of each request's ops.Session.
	Getenv     func(string) string
	Now        func() time.Time
	Keyring    auth.Keyring
	HTTPClient *http.Client
	Run        func(name string, args ...string) (stdout, stderr []byte, err error)

	Token   string    // the bearer token every /api request needs
	Version string    // of hedgehug, for GET /api/health
	Static  fs.FS     // the built GUI (web.Dist()); nil serves the page that says to build it
	Log     io.Writer // warnings and the errors of the loop, a line each; nil discards them
	Clock   Clock     // what the sync loop waits on; nil is the real clock
}

// Server is the server of hh ui. Make it with New.
type Server struct {
	opt  Options
	port atomic.Int32 // the port the Host check allows
	mux  *http.ServeMux
	api  huma.API

	logMu sync.Mutex

	presence *Presence
	loop     *Loop
	hub      *Hub
	poller   *Poller
	stopOnce sync.Once
	cancel   context.CancelFunc
	runDone  chan struct{} // closed when the loop and the poller have returned; nil before Start
}

// New makes the server. Call SetPort with the port it listens on before
// serving.
func New(opt Options) *Server {
	if opt.Log == nil {
		opt.Log = io.Discard
	}
	if opt.Now == nil {
		opt.Now = time.Now
	}
	if opt.Clock == nil {
		opt.Clock = realClock{}
	}
	s := &Server{opt: opt, mux: http.NewServeMux()}
	s.presence = newPresence(func() { s.loop.Wake() })
	s.loop = newLoop(s, opt.Clock, s.presence.Visible)
	s.hub = newHub()
	s.poller = newPoller(s, s.hub)
	s.loop.onState = func(string) { s.poller.Kick() } // a sync ended: read its state now
	s.api = humago.New(s.mux, apiConfig())
	s.api.UseMiddleware(s.requireToken)
	s.register()
	s.mux.Handle("/", staticHandler(opt.Static))
	return s
}

// apiConfig is the huma config: OpenAPI 3.1, bearer auth, no docs or
// schema routes, no $schema links in the bodies.
func apiConfig() huma.Config {
	c := huma.DefaultConfig("Hedgehug", APIVersion)
	c.OpenAPIPath, c.DocsPath, c.SchemasPath = "", "", ""
	c.CreateHooks = nil
	c.Components.Schemas = huma.NewMapRegistry("#/components/schemas/", schemaName)
	c.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		"bearer": {Type: "http", Scheme: "bearer"},
	}
	c.Security = []map[string][]string{{"bearer": {}}}
	return c
}

// schemaName names the schemas of the OpenAPI document as huma does, except
// that ops.Problem (a finding of the index about a task) is IndexProblem, as
// Problem is the body of every error.
func schemaName(t reflect.Type, hint string) string {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.PkgPath() == reflect.TypeFor[ops.Problem]().PkgPath() && t.Name() == "Problem" {
		return "IndexProblem"
	}
	return huma.DefaultSchemaNamer(t, hint)
}

// register adds every operation. Each later file adds its own register
// function here.
func (s *Server) register() {
	s.registerHealth()
	s.registerTaskReads()
	s.registerTaskWrites()
	s.registerComments()
	s.registerActivity()
	s.registerAdmin()
	s.registerViews()
	s.registerSync()
	s.registerEvents()
}

// SetPort sets the port the Host check allows.
func (s *Server) SetPort(port int) { s.port.Store(int32(port)) } //nolint:gosec // a port fits in an int32

// Handler is the handler to serve: the Host check, then the API and the
// static files.
func (s *Server) Handler() http.Handler { return s.guardHost(s.mux) }

// API is the huma API, for the OpenAPI document.
func (s *Server) API() huma.API { return s.api }

// session is the ops.Session of one request, on the store named store ("" as
// config.Select picks). Its warnings go to the log; its writes tell the sync
// loop.
func (s *Server) session(storeName string) *ops.Session {
	return &ops.Session{
		Getenv:     s.opt.Getenv,
		Now:        s.opt.Now,
		Keyring:    s.opt.Keyring,
		HTTPClient: s.opt.HTTPClient,
		Run:        s.opt.Run,
		StoreName:  storeName,
		Warn:       func(m string) { s.logf("warning: %s", m) },
		AfterWrite: func(*store.Store, config.StoreEntry) { s.loop.Wake() },
	}
}

// logf writes a line to the log.
func (s *Server) logf(format string, args ...any) {
	s.logMu.Lock()
	defer s.logMu.Unlock()
	_, _ = fmt.Fprintf(s.opt.Log, format+"\n", args...)
}

// StoreParam is the store a request is on: ?store=<name>, or as config.Select
// picks without it. The inputs of per-store operations embed it. It is
// exported because huma skips an embedded struct whose type is not.
type StoreParam struct {
	Store string `query:"store" doc:"The store; without it, the one hedgehug picks as the CLI does"`
}

// OpenAPIYAML is api/openapi.yaml: the OpenAPI document of the API.
func OpenAPIYAML() ([]byte, error) {
	return New(Options{}).API().OpenAPI().YAML()
}

// Start runs the sync loop and the poller of the events until ctx is done
// or Shutdown is called.
func (s *Server) Start(ctx context.Context) {
	ctx, s.cancel = context.WithCancel(ctx)
	s.runDone = make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() { s.loop.Run(ctx) })
	wg.Go(func() { s.poller.Run(ctx) })
	go func() {
		wg.Wait()
		close(s.runDone)
	}()
}

// Shutdown ends the event streams, stops the loop and the poller, and waits
// for the pushes of the writes not pushed, 10 seconds at most. A second call
// does nothing.
func (s *Server) Shutdown(context.Context) error {
	s.stopOnce.Do(func() {
		if s.cancel != nil {
			s.cancel()
		}
		s.hub.Close() // http.Server.Shutdown does not end a stream by itself
		if s.runDone != nil {
			<-s.runDone // a step in progress may still start a sync
		}
		s.loop.Drain(10 * time.Second)
	})
	return nil
}
