//go:build !e2e

package main

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
)

// noSyncTargets は、Store の一覧(トラック G)が入るまでの仮。同期する Store はない(決定 C2-2)。
type noSyncTargets struct{}

func (noSyncTargets) List(context.Context) ([]port.SyncTarget, error) { return nil, nil }

// noStoreGitHub は、Store の一覧(トラック G)が入るまでの仮。Store はなく、マシンのログイン名は未設定。
type noStoreGitHub struct{}

func (noStoreGitHub) List(context.Context) ([]port.StoreGitHubInfo, error) { return nil, nil }
func (noStoreGitHub) Unassign(context.Context, account.ID) error           { return nil }
func (noStoreGitHub) MachineLogin(context.Context) (string, error)         { return "", nil }
