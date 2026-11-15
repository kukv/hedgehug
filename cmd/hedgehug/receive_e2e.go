//go:build e2e

package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
)

// receiveHandler は、POST /e2e/receive を受け、本文 {store, item} のアイテムの最後の行の op を、このマシンで作っていない
// 保存として判定の記録に書く(C4d-17)。同期は e2e のビルドで再現できないので、通しのテストは履歴の「同期で受信」を
// ここから作る。版の印は書くたびに新しい ID にする。ほかの要求は next に渡す。API と同じトークンを求める。
func receiveHandler(token string, stores port.Stores, judged port.JudgedSaves, ids port.IDs, clock port.Clock, next http.Handler) http.Handler {
	want := []byte("Bearer " + token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/e2e/receive" {
			next.ServeHTTP(w, r)
			return
		}
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), want) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var b struct {
			Store port.StoreID `json:"store"`
			Item  item.ID      `json:"item"`
		}
		if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		op, err := lastOp(r.Context(), stores, b.Store, b.Item)
		if errors.Is(err, port.ErrNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		if err == nil {
			key := port.SaveKey{Version: "e2e-receive-" + string(ids.New()), Op: op, Author: "e2e", At: clock.Now()}
			err = judged.Mark(r.Context(), b.Store, []port.SaveKey{key})
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

// lastOp は、store のアイテム id のログの最後の行の op。ログがなければ port.ErrNotFound。
func lastOp(ctx context.Context, stores port.Stores, store port.StoreID, id item.ID) (change.ID, error) {
	var op change.ID
	err := stores.Read(ctx, store, func(r port.Reader) error {
		log, err := r.Changes().Of(change.Ref{Kind: change.KindItem, ID: id})
		if err != nil {
			return err
		}
		if len(log) == 0 {
			return port.ErrNotFound
		}
		op = log[len(log)-1].Op
		return nil
	})
	return op, err
}
