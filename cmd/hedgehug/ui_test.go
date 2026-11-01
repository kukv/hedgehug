package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// syncBuffer is a bytes.Buffer safe to read while the command writes.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

var uiURL = regexp.MustCompile(`http://127\.0\.0\.1:(\d+)/#token=([A-Za-z0-9_-]+)`)

// runningUI is hh ui running in the background.
type runningUI struct {
	stop   context.CancelFunc
	done   chan int
	stdout *syncBuffer
	port   int
	token  string
	mu     sync.Mutex
	opened []string
}

func (u *runningUI) openedURLs() []string {
	u.mu.Lock()
	defer u.mu.Unlock()
	return append([]string(nil), u.opened...)
}

func (te *testEnv) startUI(args ...string) *runningUI {
	te.t.Helper()
	ctx, stop := context.WithCancel(context.Background())
	u := &runningUI{stop: stop, done: make(chan int, 1), stdout: &syncBuffer{}}
	e := &env{
		stdin: strings.NewReader(""), stdout: u.stdout, stderr: &syncBuffer{},
		getenv: te.getenv, isTerminal: func(any) bool { return false },
		now:       time.Now,
		openURL:   func(s string) error { u.mu.Lock(); u.opened = append(u.opened, s); u.mu.Unlock(); return nil },
		uiContext: func() (context.Context, context.CancelFunc) { return ctx, stop },
	}
	go func() { u.done <- run(append([]string{"ui"}, args...), e) }()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if m := uiURL.FindStringSubmatch(u.stdout.String()); m != nil {
			u.port, _ = strconv.Atoi(m[1])
			u.token = m[2]
			break
		}
		select {
		case code := <-u.done:
			te.t.Fatalf("hh ui exited %d: %s", code, u.stdout.String())
		default:
		}
		if time.Now().After(deadline) {
			te.t.Fatalf("hh ui printed no URL: %q", u.stdout.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
	te.t.Cleanup(func() { u.stop(); <-u.done })
	return u
}

func health(t *testing.T, port int, token string) int {
	t.Helper()
	req, _ := http.NewRequest("GET", "http://127.0.0.1:"+strconv.Itoa(port)+"/api/health", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0
	}
	_ = resp.Body.Close()
	return resp.StatusCode
}

func (te *testEnv) uiJSON() (map[string]any, os.FileInfo) {
	te.t.Helper()
	p := filepath.Join(te.hhHome, "ui.json")
	st, err := os.Stat(p)
	if err != nil {
		return nil, nil
	}
	b, err := os.ReadFile(p)
	if err != nil {
		te.t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		te.t.Fatal(err)
	}
	return m, st
}

func TestUIStartsAndStops(t *testing.T) {
	te := newTestEnv(t)
	te.ok("store", "init", "me", "--prefix", "ME")
	u := te.startUI("--no-open")
	if code := health(t, u.port, u.token); code != 200 {
		t.Fatalf("health = %d", code)
	}
	if len(u.token) != 43 {
		t.Errorf("token %q, want 43 characters (32 bytes)", u.token)
	}
	if len(u.openedURLs()) != 0 {
		t.Errorf("--no-open opened %v", u.openedURLs())
	}
	info, st := te.uiJSON()
	if info == nil || int(info["port"].(float64)) != u.port || info["token"] != u.token || int(info["pid"].(float64)) != os.Getpid() {
		t.Fatalf("ui.json = %v", info)
	}
	if runtime.GOOS != "windows" && st.Mode().Perm() != 0o600 {
		t.Errorf("ui.json mode %v, want 0600", st.Mode().Perm())
	}
	u.stop()
	if code := <-u.done; code != 0 {
		t.Errorf("exit %d", code)
	}
	u.done <- 0 // for the Cleanup
	if info, _ := te.uiJSON(); info != nil {
		t.Errorf("ui.json left: %v", info)
	}
}

func TestUIOpensTheBrowser(t *testing.T) {
	te := newTestEnv(t)
	te.ok("store", "init", "me", "--prefix", "ME")
	u := te.startUI()
	// The URL is printed, then opened: wait for the second.
	for i := 0; i < 500 && len(u.openedURLs()) == 0; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	if len(u.openedURLs()) != 1 || !strings.Contains(u.openedURLs()[0], "#token="+u.token) {
		t.Errorf("opened %v", u.openedURLs())
	}
}

func TestUIReusesTheRunningServer(t *testing.T) {
	te := newTestEnv(t)
	te.ok("store", "init", "me", "--prefix", "ME")
	u := te.startUI("--no-open")
	var opened []string
	te.openURL = func(s string) error { opened = append(opened, s); return nil }
	code, out, errs := te.run("ui")
	if code != 0 || !strings.Contains(out, "Hedgehug UI is running: ") || !strings.Contains(out, "#token="+u.token) {
		t.Fatalf("second hh ui: %d %q %q", code, out, errs)
	}
	if len(opened) != 1 {
		t.Errorf("opened %v", opened)
	}
}

func TestUIOverwritesAStaleUIJSON(t *testing.T) {
	te := newTestEnv(t)
	te.ok("store", "init", "me", "--prefix", "ME")
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	dead := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	stale := `{"port":` + strconv.Itoa(dead) + `,"token":"old","pid":1,"version":"x"}`
	if err := os.WriteFile(filepath.Join(te.hhHome, "ui.json"), []byte(stale), 0o600); err != nil {
		t.Fatal(err)
	}
	u := te.startUI("--no-open")
	info, _ := te.uiJSON()
	if info["token"] != u.token {
		t.Errorf("ui.json = %v, want the new server", info)
	}
}

func TestUIPortInUse(t *testing.T) {
	te := newTestEnv(t)
	te.ok("store", "init", "me", "--prefix", "ME")
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
	code, _, errs := te.run("ui", "--no-open", "--port", port)
	if code != 1 || !strings.Contains(errs, "listen tcp") {
		t.Errorf("exit %d, stderr %q", code, errs)
	}
}
