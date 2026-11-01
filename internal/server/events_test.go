package server

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/ops"
)

// eventStream reads /api/events.
type eventStream struct {
	cancel context.CancelFunc
	resp   *http.Response
	events chan [2]string // name, data
}

func (ts *testServer) events() *eventStream {
	ts.t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, "GET", ts.hs.URL+"/api/events", nil)
	req.Header.Set("Authorization", "Bearer "+testToken)
	resp, err := ts.hs.Client().Do(req) //nolint:bodyclose // closed by eventStream.Close
	if err != nil {
		cancel()
		ts.t.Fatal(err)
	}
	if resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		cancel()
		_ = resp.Body.Close()
		ts.t.Fatalf("events: %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	es := &eventStream{cancel: cancel, resp: resp, events: make(chan [2]string, 64)}
	go func() {
		defer close(es.events)
		sc := bufio.NewScanner(resp.Body)
		var name, data string
		for sc.Scan() {
			line := sc.Text()
			switch {
			case strings.HasPrefix(line, "event: "):
				name = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				data += strings.TrimPrefix(line, "data: ")
			case line == "":
				if data != "" {
					es.events <- [2]string{name, data}
				}
				name, data = "", ""
			}
		}
	}()
	ts.t.Cleanup(es.Close)
	return es
}

func (es *eventStream) Close() { es.cancel(); _ = es.resp.Body.Close() }

// next returns the data of the first event named name, skipping others.
func (es *eventStream) next(t *testing.T, name string) json.RawMessage {
	t.Helper()
	timeout := time.After(10 * time.Second)
	for {
		select {
		case e, ok := <-es.events:
			if !ok {
				t.Fatalf("the stream ended before %s", name)
			}
			if e[0] == name {
				return json.RawMessage(e[1])
			}
		case <-timeout:
			t.Fatalf("no %s event", name)
		}
	}
}

func startedServer(t *testing.T) *testServer {
	t.Helper()
	ts := newTestServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	ts.srv.Start(ctx)
	t.Cleanup(func() { cancel(); _ = ts.srv.Shutdown(context.Background()) })
	return ts
}

func TestEventsHelloFirst(t *testing.T) {
	ts := startedServer(t)
	es := ts.events()
	var hello helloEvent
	if err := json.Unmarshal(es.next(t, "hello"), &hello); err != nil {
		t.Fatal(err)
	}
	if hello.Client == "" || len(hello.Stores) != 1 || hello.Stores[0].Store != "me" || hello.Stores[0].Head == "" {
		t.Fatalf("hello = %+v", hello)
	}
	if !ts.srv.presence.Visible() {
		t.Error("the connection is not visible")
	}
	ts.call("PUT", "/api/presence", map[string]any{"client": hello.Client, "visible": false}, 204, nil)
	if ts.srv.presence.Visible() {
		t.Error("still visible")
	}
	es.Close()
	eventually(t, "the client to go", func() bool {
		ts.srv.presence.mu.Lock()
		defer ts.srv.presence.mu.Unlock()
		return len(ts.srv.presence.clients) == 0
	})
}

func TestEventsAPIWrite(t *testing.T) {
	ts := startedServer(t)
	es := ts.events()
	es.next(t, "hello")
	var out taskWriteBody
	ts.call("POST", "/api/tasks", map[string]any{"title": "one"}, 201, &out)
	var e tasksChanged
	_ = json.Unmarshal(es.next(t, "tasks.changed"), &e)
	if e.Commit != *out.Commit || len(e.ULIDs) != 1 || e.ULIDs[0] != out.Task.ULID {
		t.Errorf("event = %+v", e)
	}
}

var (
	hhOnce sync.Once
	hhBin  string
	hhErr  error
)

// hhBinary builds hedgehug once for the tests that run it as another process.
func hhBinary(t *testing.T) string {
	t.Helper()
	hhOnce.Do(func() {
		dir, err := os.MkdirTemp("", "hh-server-test")
		if err != nil {
			hhErr = err
			return
		}
		hhBin = filepath.Join(dir, "hedgehug")
		if runtime.GOOS == "windows" {
			hhBin += ".exe"
		}
		if out, err := exec.Command("go", "build", "-o", hhBin, "github.com/kukv/hedgehug/cmd/hedgehug").CombinedOutput(); err != nil { //nolint:gosec // our own package
			hhErr = fmt.Errorf("%v: %s", err, out)
		}
	})
	if hhErr != nil {
		t.Fatal(hhErr)
	}
	return hhBin
}

func TestEventsCLIWrite(t *testing.T) {
	ts := startedServer(t)
	es := ts.events()
	es.next(t, "hello")
	cmd := exec.Command(hhBinary(t), "add", "from the CLI") //nolint:gosec // the binary the test built
	cmd.Env = []string{"PATH=" + os.Getenv("PATH")}
	for k, v := range ts.env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("hedgehug add: %v: %s", err, out)
	}
	var e tasksChanged
	_ = json.Unmarshal(es.next(t, "tasks.changed"), &e)
	if e.Op != "add" || len(e.ULIDs) != 1 {
		t.Errorf("event = %+v", e)
	}
}

func TestEventsReconnect(t *testing.T) {
	ts := startedServer(t)
	es := ts.events()
	var h1 helloEvent
	_ = json.Unmarshal(es.next(t, "hello"), &h1)
	es.Close()
	if _, err := ts.session().Add(ops.AddInput{Title: "while away"}); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the poller to see HEAD", func() bool {
		st, _ := ts.srv.poller.Snapshot()
		return st[0].Head != h1.Stores[0].Head
	})
	es = ts.events()
	var h2 helloEvent
	_ = json.Unmarshal(es.next(t, "hello"), &h2)
	if h2.Stores[0].Head == h1.Stores[0].Head || h2.Client == h1.Client {
		t.Errorf("hello after reconnect = %+v (before %+v)", h2, h1)
	}
}

func TestEventsEndOnShutdown(t *testing.T) {
	ts := startedServer(t)
	es := ts.events()
	es.next(t, "hello")
	_ = ts.srv.Shutdown(context.Background())
	timeout := time.After(5 * time.Second)
	for {
		select {
		case _, ok := <-es.events:
			if !ok {
				return
			}
		case <-timeout:
			t.Fatal("the stream did not end on Shutdown")
		}
	}
}

func TestEventsNeedToken(t *testing.T) {
	ts := newTestServer(t)
	req, _ := http.NewRequest("GET", ts.hs.URL+"/api/events", nil)
	if resp := ts.raw(req); resp.Status != 401 {
		t.Errorf("status %d", resp.Status)
	}
}

func TestEventsAfterShutdownSendNoHello(t *testing.T) {
	ts := startedServer(t)
	_ = ts.srv.Shutdown(context.Background())
	es := ts.events()
	select {
	case e, ok := <-es.events:
		if ok {
			t.Errorf("event after Shutdown: %v", e)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the stream did not end")
	}
}
