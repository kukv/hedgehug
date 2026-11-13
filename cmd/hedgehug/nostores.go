//go:build !e2e

package main

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
)

// noSyncTargets は、同期する Store を返さない(決定 C2-2)。
type noSyncTargets struct{}

func (noSyncTargets) List(context.Context) ([]port.SyncTarget, error) { return nil, nil }

// noStoreGitHub は、Store を返さず、マシンのログイン名を未設定とする。
type noStoreGitHub struct{}

func (noStoreGitHub) List(context.Context) ([]port.StoreGitHubInfo, error) { return nil, nil }
func (noStoreGitHub) Unassign(context.Context, account.ID) error           { return nil }
func (noStoreGitHub) MachineLogin(context.Context) (string, error)         { return "", nil }
