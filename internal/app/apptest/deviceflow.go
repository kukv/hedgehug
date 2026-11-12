package apptest

import (
	"context"
	"sync"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

// DeviceFlow は port.DeviceFlow の偽物。Polls は PollDevice が順に返す答え(使い切ったら最後のものを返し続ける)。
// StartErr・PollErr があれば、その問い合わせはそれを返す。Refreshed は RefreshDevice が返すトークンの組
// (Access が空なら TokenRejected)。Starts・Refreshes は呼ばれた回数。
type DeviceFlow struct {
	mu         sync.Mutex
	Code       port.DeviceCode
	Polls      []port.DeviceGrant
	StartErr   error
	PollErr    error
	Refreshed  account.DeviceToken
	RefreshErr error
	Starts     int
	Refreshes  int
}

// NewDeviceFlow は、コード WDJB-MJHT、期限 15 分、間隔 5 秒で始まり、ずっと待っている偽物を返す。
func NewDeviceFlow() *DeviceFlow {
	return &DeviceFlow{
		Code: port.DeviceCode{
			DeviceCode: "device-1", UserCode: "WDJB-MJHT", VerificationURI: "https://github.com/login/device",
			ExpiresIn: 15 * time.Minute, Interval: 5 * time.Second,
		},
		Polls: []port.DeviceGrant{{Status: port.DevicePending}},
	}
}

func (d *DeviceFlow) StartDevice(context.Context, string) (port.DeviceCode, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Starts++
	if d.StartErr != nil {
		return port.DeviceCode{}, d.StartErr
	}
	return d.Code, nil
}

func (d *DeviceFlow) PollDevice(context.Context, string, string) (port.DeviceGrant, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.PollErr != nil {
		return port.DeviceGrant{}, d.PollErr
	}
	g := d.Polls[0]
	if len(d.Polls) > 1 {
		d.Polls = d.Polls[1:]
	}
	return g, nil
}

func (d *DeviceFlow) RefreshDevice(context.Context, string, string) (account.DeviceToken, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Refreshes++
	if d.RefreshErr != nil {
		return account.DeviceToken{}, d.RefreshErr
	}
	if d.Refreshed.Access == "" {
		return account.DeviceToken{}, &ghcall.Failure{Kind: ghcall.TokenRejected}
	}
	return d.Refreshed, nil
}

// Hosts は port.Hosts の偽物。
type Hosts struct{ ID string }

func (h Hosts) ClientID(context.Context) (string, error) { return h.ID, nil }
