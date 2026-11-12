// Command hedgehug は、このマシンで画面のサーバーを動かす。CLI(hh ui ほか)ができるまでは、
// 起動すると待ち受け、画面の URL を標準出力に出す(この計画の決定 7)。
package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	context.AfterFunc(ctx, stop) // 止め始めたら、2 回目の Ctrl-C ですぐに終わるようにする
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
	fmt.Printf("http://%s/#token=%s\n", ln.Addr(), token)
	return serve(ctx, ln, wrap(token, httpapi.New(token, web.Dist(), deps)))
}

// serve は、ctx が終わるまで ln で待ち受ける。終わったら、処理中の要求を(最大 5 秒)待ってから戻る。
func serve(ctx context.Context, ln net.Listener, h http.Handler) error {
	srv := &http.Server{Handler: h, ReadHeaderTimeout: 10 * time.Second}
	shutdown := make(chan error, 1)
	go func() {
		<-ctx.Done()
		wait, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second) // ctx はもう終わっているので、取り消しだけを外す
		defer cancel()
		shutdown <- srv.Shutdown(wait)
	}()
	if err := srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return <-shutdown
}
