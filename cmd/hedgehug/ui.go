package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/atomicfile"
	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/server"
	"github.com/kukv/hedgehug/web"
)

// uiInfo is ui.json: how to reach the server that runs, for the next hh ui.
type uiInfo struct {
	Port    int    `json:"port"`
	Token   string `json:"token"`
	PID     int    `json:"pid"`
	Version string `json:"version"`
}

func (u uiInfo) url() string {
	return "http://127.0.0.1:" + strconv.Itoa(u.Port) + "/#token=" + u.Token
}

func (a *app) uiCmd() *cobra.Command {
	var port int
	var noOpen bool
	cmd := &cobra.Command{
		Use:   "ui",
		Short: "Open the GUI in the browser",
		Long:  "Ui serves the GUI on 127.0.0.1 and opens it in the browser. When one runs already, it opens that one. Stop it with Ctrl-C.",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return a.ui(port, noOpen)
		},
	}
	cmd.Flags().IntVar(&port, "port", 0, "the port to listen on; 0 picks a free one")
	cmd.Flags().BoolVar(&noOpen, "no-open", false, "print the URL and do not open the browser")
	return cmd
}

func (a *app) ui(port int, noOpen bool) error {
	paths, err := config.Resolve(a.getenv)
	if err != nil {
		return err
	}
	file := filepath.Join(paths.DataDir, "ui.json")
	if info, ok := readRunningUI(file); ok {
		_, _ = fmt.Fprintln(a.stdout, "Hedgehug UI is running: "+info.url())
		if !noOpen && a.openURL != nil {
			a.warnBrowser(a.openURL(info.url())) // the command ends with this process otherwise
		}
		return nil
	}
	ln, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
	if err != nil {
		return err
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		_ = ln.Close()
		return err
	}
	info := uiInfo{Port: ln.Addr().(*net.TCPAddr).Port, Token: base64.RawURLEncoding.EncodeToString(b), PID: os.Getpid(), Version: version}
	srv := server.New(server.Options{
		Getenv: a.getenv, Now: a.now, Keyring: a.keyring, HTTPClient: a.httpClient, Run: a.run,
		Token: info.Token, Version: version, Static: web.Dist(), Log: a.stderr,
	})
	srv.SetPort(info.Port)
	data, err := json.Marshal(info)
	if err == nil {
		err = atomicfile.WriteFile(file, append(data, '\n'))
	}
	if err != nil {
		_ = ln.Close()
		return err
	}
	defer removeOwnUIJSON(file, info.Token)

	ctx, stop := a.uiContextOrSignals()
	defer stop()
	srv.Start(ctx)
	hs := &http.Server{Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second}
	served := make(chan error, 1)
	go func() { served <- hs.Serve(ln) }()

	_, _ = fmt.Fprintln(a.stdout, "Hedgehug UI: "+info.url())
	if !noOpen && a.openURL != nil {
		go func() { a.warnBrowser(a.openURL(info.url())) }() // do not wait for the opener
	}

	select {
	case <-ctx.Done():
		stop() // a second Ctrl-C ends the process while it shuts down
	case err := <-served:
		if !errors.Is(err, http.ErrServerClosed) {
			_ = srv.Shutdown(context.Background())
			return err
		}
	}
	srv.CloseEvents()
	shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = hs.Shutdown(shutCtx)
	return srv.Shutdown(context.Background())
}

// warnBrowser tells that the browser did not open, if err says so.
func (a *app) warnBrowser(err error) {
	if err != nil {
		_, _ = fmt.Fprintln(a.stderr, "warning: the browser did not open:", err)
	}
}

// uiContextOrSignals is the context hh ui runs until: the test's, or one
// that Ctrl-C (Ctrl-Break on Windows) and SIGTERM end.
func (a *app) uiContextOrSignals() (context.Context, context.CancelFunc) {
	if a.uiContext != nil {
		return a.uiContext()
	}
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

// readRunningUI reads ui.json and reports whether the server it names answers.
func readRunningUI(file string) (uiInfo, bool) {
	var info uiInfo
	b, err := os.ReadFile(file) //nolint:gosec // our own file in the data directory
	if err != nil || json.Unmarshal(b, &info) != nil || info.Port == 0 {
		return info, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:"+strconv.Itoa(info.Port)+"/api/health", nil)
	if err != nil {
		return info, false
	}
	req.Header.Set("Authorization", "Bearer "+info.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return info, false
	}
	_ = resp.Body.Close()
	return info, resp.StatusCode == http.StatusOK
}

// removeOwnUIJSON removes ui.json when it is still this server's (its pid and
// token): a later hh ui may have written its own.
func removeOwnUIJSON(file, token string) {
	var info uiInfo
	b, err := os.ReadFile(file) //nolint:gosec // our own file in the data directory
	if err != nil || json.Unmarshal(b, &info) != nil || info.PID != os.Getpid() || info.Token != token {
		return
	}
	_ = os.Remove(file)
}
