// Command hedgehug は、このマシンで画面のサーバーを動かす。CLI(hh ui ほか)ができるまでは、
// 起動すると待ち受け、画面の URL を標準出力に出す(この計画の決定 7)。
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/kukv/hedgehug/internal/adapter/httpapi"
	"github.com/kukv/hedgehug/web"
)

// version は、リリースのビルドで -ldflags "-X main.version=…" から入る。
var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "hedgehug:", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	deps, err := wire(ctx)
	if err != nil {
		return err
	}
	deps.Version = version
	token, addr := tokenAndAddr()
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	srv := &http.Server{Handler: httpapi.New(token, web.Dist(), deps), ReadHeaderTimeout: 10 * time.Second}
	fmt.Printf("http://%s/#token=%s\n", ln.Addr(), token)
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	if err := srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func newToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b) // crypto/rand.Read は失敗しない(Go 1.24 から)
	return hex.EncodeToString(b)
}
