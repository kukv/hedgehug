// Package events は、port.Events の実装と、画面に出来事を送る Server-Sent Events(06「画面への反映」)。
package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/app/port"
)

// buffer は、1 つの画面に溜めておける出来事の数。溢れた分は落とす(画面は次の出来事で読み直す)。
const buffer = 64

// Hub は、つないでいる画面ごとに出来事を配る port.Events。
type Hub struct {
	mu     sync.Mutex
	subs   map[chan port.Event]struct{}
	closed bool
}

func NewHub() *Hub { return &Hub{subs: map[chan port.Event]struct{}{}} }

// Publish は、つないでいるすべての画面に e を送る。待たない。
func (h *Hub) Publish(e port.Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs {
		select {
		case ch <- e:
		default:
		}
	}
}

// Listeners は、つないでいる画面の数。
func (h *Hub) Listeners() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subs)
}

// Close は、つないでいるすべての流れを終え、このあとつなぐ流れもすぐ終える(決定 C2-27)。
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closed = true
	for ch := range h.subs {
		close(ch)
		delete(h.subs, ch)
	}
}

// subscribe は、出来事を受ける口を作る。Close のあとなら、閉じた口を返す。
func (h *Hub) subscribe() (<-chan port.Event, func()) {
	ch := make(chan port.Event, buffer)
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		close(ch)
		return ch, func() {}
	}
	h.subs[ch] = struct{}{}
	return ch, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		delete(h.subs, ch)
	}
}

var errNoFlush = errors.New("flush できない応答")

// sseOutput は、Server-Sent Events の応答。本文は Body が直接書く。
type sseOutput struct {
	Body func(ctx huma.Context)
}

// Register は、GET /api/events を登録する。画面は fetch で読み続ける(EventSource はヘッダーを付けられない。
// 決定 C2-20)。huma の sse は OpenAPI 3.0 に書き下せない const を出すため、枠は手で書く。
func Register(api huma.API, h *Hub) {
	huma.Register(api, huma.Operation{
		OperationID: "watch-events", Method: http.MethodGet, Path: "/api/events",
		Summary: "Store の変化と同期の状態の変化を、Server-Sent Events で送り続ける",
		Description: "出来事ごとに `event: store` または `event: sync` の行と、" +
			"`data: {\"store\":\"<Store ID>\"}` の行を、空行で区切って送る。",
		Responses: map[string]*huma.Response{
			"200": {
				Description: "Server-Sent Events の流れ",
				Content:     map[string]*huma.MediaType{"text/event-stream": {Schema: &huma.Schema{Type: "string"}}},
			},
		},
	}, func(_ context.Context, _ *struct{}) (*sseOutput, error) {
		return &sseOutput{Body: func(ctx huma.Context) { stream(ctx, h) }}, nil
	})
}

func stream(ctx huma.Context, h *Hub) {
	ctx.SetHeader("Content-Type", "text/event-stream")
	ctx.SetHeader("Cache-Control", "no-cache")
	w := ctx.BodyWriter()
	flush := func() error {
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
			return nil
		}
		return errNoFlush
	}
	ch, stop := h.subscribe()
	defer stop()
	// つないだことを画面に伝える(ヘッダーを先に流す)
	if flush() != nil {
		return
	}
	for {
		select {
		case <-ctx.Context().Done():
			return
		case e, ok := <-ch:
			if !ok {
				return
			}
			if writeFrame(w, e) != nil || flush() != nil {
				return
			}
		}
	}
}

// writeFrame は、出来事を 1 つの Server-Sent Events のフレームとして書く。
func writeFrame(w io.Writer, e port.Event) error {
	kind := "store"
	if e.Kind == port.EventSync {
		kind = "sync"
	}
	data, _ := json.Marshal(map[string]string{"store": string(e.Store)})
	_, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", kind, data)
	return err
}
