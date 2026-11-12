package gitstore

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
)

// localMachine は、Store を作ってサンプルデータを保存し、まだどこにも送っていないマシン。
func localMachine(t *testing.T) machine {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "a")
	if err := Create(dir, "01STORE0000000000000000001", t0); err != nil {
		t.Fatal(err)
	}
	m := newMachine(t, dir, "A", t0)
	if err := apptest.Sample(t.Context(), m.stores, m.id); err != nil {
		t.Fatal(err)
	}
	return m
}

// statusServer は、どの要求にも code を返す HTTP の remote。
func statusServer(t *testing.T, code int) port.Remote {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if code == http.StatusUnauthorized {
			w.Header().Set("WWW-Authenticate", `Basic realm="x"`)
		}
		w.WriteHeader(code)
	}))
	t.Cleanup(srv.Close)
	return port.Remote{URL: srv.URL + "/o/r.git", Token: "tok-taro"}
}

// 06「同期のエラー」: 受け取りで断られたら、Store を変えずに理由を返す。
func TestSyncClassifiesRefusalsWhileReceiving(t *testing.T) {
	cases := []struct {
		code int
		want error
	}{
		{http.StatusUnauthorized, port.ErrAuthFailed},
		{http.StatusForbidden, port.ErrRemoteUnreadable},
		{http.StatusNotFound, port.ErrRemoteUnreadable},
		{http.StatusInternalServerError, port.ErrUnreachable},
	}
	for _, c := range cases {
		t.Run(http.StatusText(c.code), func(t *testing.T) {
			m := localMachine(t)
			before := snap(t, m)
			_, err := m.rep.Sync(t.Context(), m.id, statusServer(t, c.code))
			if !errors.Is(err, c.want) {
				t.Errorf("Sync = %v, want %v", err, c.want)
			}
			unchanged(t, m, before)
		})
	}
}

func TestSyncUnreachableWhenNothingListens(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()
	m := localMachine(t)
	if _, err := m.rep.Sync(t.Context(), m.id, port.Remote{URL: url + "/o/r.git"}); !errors.Is(err, port.ErrUnreachable) {
		t.Errorf("Sync = %v", err)
	}
}

// 06「同期のエラー」: git の通信が stallAfter のあいだまったく進まなければ、中断して「接続できません」。
func TestSyncGivesUpWhenTheRemoteStalls(t *testing.T) {
	saved := stallAfter
	stallAfter = 300 * time.Millisecond
	t.Cleanup(func() { stallAfter = saved })
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done() // 何も返さずに待ち続ける
	}))
	t.Cleanup(srv.Close)
	m := localMachine(t)
	start := time.Now()
	_, err := m.rep.Sync(t.Context(), m.id, port.Remote{URL: srv.URL + "/o/r.git"})
	if !errors.Is(err, port.ErrUnreachable) {
		t.Errorf("Sync = %v", err)
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Errorf("took %v", d)
	}
}

// 06「同期のエラー」: 送信の途中で接続が切れたら、サーバーが断ったのではないので「接続できません」。
func TestRealGitSyncUnreachableWhenConnectionBreaksWhileSending(t *testing.T) {
	var armed, broke atomic.Bool
	remote, _ := newRealRemote(t, func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/git-receive-pack") && armed.Load() {
				conn, _, err := w.(http.Hijacker).Hijack()
				if err == nil {
					broke.Store(true)
					_ = conn.Close()
				}
				return
			}
			next.ServeHTTP(w, r)
		})
	})
	a := firstMachine(t, remote)
	a.write(t, "a", set("title", "", "A"))
	armed.Store(true)
	_, err := a.rep.Sync(t.Context(), a.id, remote)
	if !broke.Load() {
		t.Fatal("the send did not reach the breaking server")
	}
	if !errors.Is(err, port.ErrUnreachable) || errors.Is(err, port.ErrWriteDenied) {
		t.Errorf("Sync = %v, want ErrUnreachable", err)
	}
}

// 06「同期のエラー」: 受け取れても、送信で 403 なら「書き込みが許可されていません」。
func TestRealGitSyncWriteDenied(t *testing.T) {
	remote, _ := newRealRemote(t, func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("service") == "git-receive-pack" || strings.HasSuffix(r.URL.Path, "/git-receive-pack") {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	})
	m := localMachine(t)
	if _, err := m.rep.Sync(t.Context(), m.id, remote); !errors.Is(err, port.ErrWriteDenied) {
		t.Errorf("Sync = %v", err)
	}
}
