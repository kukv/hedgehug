// Package server is the HTTP server of hh ui: the REST API under /api, its
// events, the sync loop, and the built GUI.
package server

import (
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
}

// Server is the server of hh ui. Make it with New.
type Server struct {
	opt  Options
	port atomic.Int32 // the port the Host check allows
	mux  *http.ServeMux
	api  huma.API

	logMu sync.Mutex
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
	s := &Server{opt: opt, mux: http.NewServeMux()}
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
// loop (Task 7).
func (s *Server) session(storeName string) *ops.Session {
	return &ops.Session{
		Getenv:     s.opt.Getenv,
		Now:        s.opt.Now,
		Keyring:    s.opt.Keyring,
		HTTPClient: s.opt.HTTPClient,
		Run:        s.opt.Run,
		StoreName:  storeName,
		Warn:       func(m string) { s.logf("warning: %s", m) },
		AfterWrite: func(*store.Store, config.StoreEntry) {},
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
