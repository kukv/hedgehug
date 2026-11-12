package apptest

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
)

// Identity は、どの Store でも Login を「あなた」とする偽物。
type Identity struct {
	Login string
}

func (i Identity) Me(context.Context, port.StoreID) (string, error) { return i.Login, nil }
