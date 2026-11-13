// Package apierr は、usecase のエラーを HTTP の状態に直す。機能の API のパッケージが使う。
package apierr

import (
	"errors"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

// From は、断り(refusal)を 422(detail に仕様の文言)、見つからないものを 404、書けない操作を 400 にする。
// ほかのエラーはそのまま返し、huma が 500 にする。
func From(err error) error {
	var r *refusal.Refusal
	switch {
	case err == nil:
		return nil
	case errors.As(err, &r):
		return huma.Error422UnprocessableEntity(r.Message)
	case errors.Is(err, port.ErrNotFound), errors.Is(err, port.ErrStoreNotFound):
		return huma.Error404NotFound("not found")
	case errors.Is(err, port.ErrInvalidOperation):
		return huma.Error400BadRequest("invalid operation")
	}
	return err
}
