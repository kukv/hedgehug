package gitstore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"syscall"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/client"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"

	"github.com/kukv/hedgehug/internal/app/port"
)

// stallAfter は、git の通信がまったく進まないとき、中断するまでの時間(06「同期のエラー」)。テストで縮める。
var stallAfter = 60 * time.Second

// errStalled は、通信が stallAfter のあいだ進まなかったこと。
var errStalled = errors.New("git: no progress")

// go-git は remote ごとの HTTP クライアントを受け取らないので、http と https の通信に見張りを入れる。
// 見張りは要求の context にあるときだけ働く(watch の外の通信は、そのまま通す)。
func init() {
	t := githttp.NewClient(&http.Client{Transport: watchedTransport{base: http.DefaultTransport}})
	client.InstallProtocol("https", t)
	client.InstallProtocol("http", t)
}

type watchKey struct{}

type watchdog struct {
	timer *time.Timer
	after time.Duration
}

func (w *watchdog) touch() { w.timer.Reset(w.after) }

// watch は、通信が stallAfter のあいだ進まなければ errStalled で取り消される ctx を返す。done で見張りを止める。
// 合わせる計算の時間は数えないよう、通信の呼び出しだけを包む。
func watch(ctx context.Context) (context.Context, func()) {
	ctx, cancel := context.WithCancelCause(ctx)
	w := &watchdog{after: stallAfter}
	w.timer = time.AfterFunc(stallAfter, func() { cancel(errStalled) })
	return context.WithValue(ctx, watchKey{}, w), func() {
		w.timer.Stop()
		cancel(nil)
	}
}

// watchedTransport は、要求と応答の本文を読み書きするたびに、見張りの時計を戻す。
type watchedTransport struct{ base http.RoundTripper }

func (t watchedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	w, _ := req.Context().Value(watchKey{}).(*watchdog)
	if w == nil {
		return t.base.RoundTrip(req)
	}
	w.touch()
	if req.Body != nil {
		req = req.Clone(req.Context())
		req.Body = touchingBody{ReadCloser: req.Body, w: w}
	}
	res, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	w.touch()
	res.Body = touchingBody{ReadCloser: res.Body, w: w}
	return res, nil
}

type touchingBody struct {
	io.ReadCloser
	w *watchdog
}

func (b touchingBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if n > 0 {
		b.w.touch()
	}
	return n, err
}

// classify は、remote との通信の失敗を、06「同期のエラー」の理由に当たる port のエラーに直す(決定 C2-7)。
// ctx は watch の ctx、sending は送信の途中か。
func classify(ctx context.Context, err error, sending bool) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(context.Cause(ctx), errStalled):
		return fmt.Errorf("%w: %w", port.ErrUnreachable, errStalled)
	case errors.Is(err, transport.ErrAuthenticationRequired):
		return fmt.Errorf("%w: %w", port.ErrAuthFailed, err)
	case errors.Is(err, transport.ErrAuthorizationFailed) && sending:
		return fmt.Errorf("%w: %w", port.ErrWriteDenied, err)
	case errors.Is(err, transport.ErrAuthorizationFailed), errors.Is(err, transport.ErrRepositoryNotFound):
		return fmt.Errorf("%w: %w", port.ErrRemoteUnreadable, err)
	}
	return fmt.Errorf("%w: %w", port.ErrUnreachable, err)
}

// refused は、送信をサーバーが断り、remote の先端が動いていないときの理由。直前に先端を読めているので、
// 401 と止まったとき以外は「書き込みが許可されていない」(フック・保護されたブランチを含む)。
func refused(ctx context.Context, err error) error {
	switch {
	case errors.Is(context.Cause(ctx), errStalled):
		return fmt.Errorf("%w: %w", port.ErrUnreachable, errStalled)
	case errors.Is(err, transport.ErrAuthenticationRequired):
		return fmt.Errorf("%w: %w", port.ErrAuthFailed, err)
	case brokenConnection(err):
		return fmt.Errorf("%w: %w", port.ErrUnreachable, err)
	}
	return fmt.Errorf("%w: %w", port.ErrWriteDenied, err)
}

// brokenConnection は、err がサーバーの断りでなく、通信が途中で切れたことか(サーバーが応答を返していれば false)。
// go-git は通信の失敗を *plumbing.UnexpectedError に包み、Unwrap を持たないので、中身を取り出して調べる。
func brokenConnection(err error) bool {
	var ue *plumbing.UnexpectedError
	if errors.As(err, &ue) {
		err = ue.Err
	}
	var ne net.Error
	var uerr *url.Error
	return errors.As(err, &ne) || errors.As(err, &uerr) ||
		errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.EPIPE)
}
