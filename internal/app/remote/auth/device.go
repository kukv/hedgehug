package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/app/storage/config"
	"github.com/kukv/hedgehug/internal/lib/github"
	"github.com/kukv/hedgehug/internal/lib/osfs"
)

// refreshBefore is how long before its end an access token is refreshed.
const refreshBefore = 5 * time.Minute

// defaultLockWait is how long a refresh waits for another one to end, when
// Options.LockWait is not set.
const defaultLockWait = 30 * time.Second

// Prompt shows the user the code and where to enter it.
type Prompt func(dc *github.DeviceCode)

var errCodeExpired = errors.New("the code expired before it was entered; run hedgehug auth login again")

// ErrCanceled: the device flow was stopped by Options.Context.
var ErrCanceled = errors.New("the login was canceled")

// canceledOr is ErrCanceled when ctx ended (the request failed because of it),
// and err otherwise.
func canceledOr(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ErrCanceled
	}
	return err
}

// DeviceLogin runs the device flow of the Hedgehug GitHub App on the host h
// and returns the tokens it got.
func DeviceLogin(h config.Host, opt Options, show Prompt) (DeviceToken, error) {
	id := ClientID(h)
	if id == "" {
		return DeviceToken{}, fmt.Errorf("the device flow needs hosts[].client_id for %s in config.yaml; or use token: gh, token: env:<VAR>, or hedgehug auth login <name> --with-token", h.Host)
	}
	ctx := opt.Context
	if ctx == nil {
		ctx = context.Background()
	}
	dc, err := github.RequestDeviceCode(ctx, opt.HTTP, h.Web, id)
	if err != nil {
		return DeviceToken{}, canceledOr(ctx, err)
	}
	if ctx.Err() != nil { // ended while the code was asked for: nobody is shown it
		return DeviceToken{}, ErrCanceled
	}
	show(dc)
	interval := time.Duration(dc.Interval) * time.Second
	deadline := opt.now().Add(time.Duration(dc.ExpiresIn) * time.Second)
	for {
		if ctx.Err() != nil {
			return DeviceToken{}, ErrCanceled
		}
		tr, err := github.PollToken(ctx, opt.HTTP, h.Web, id, dc.DeviceCode)
		if err != nil {
			return DeviceToken{}, canceledOr(ctx, err)
		}
		switch tr.Error {
		case "":
			return deviceTokenOf(tr, opt.now()), nil
		case "authorization_pending":
		case "slow_down":
			interval += 5 * time.Second
		case "expired_token":
			return DeviceToken{}, errCodeExpired
		case "access_denied":
			return DeviceToken{}, errors.New("the login was denied")
		default:
			return DeviceToken{}, fmt.Errorf("the device flow failed: %s", tr.Error)
		}
		if opt.now().After(deadline) {
			return DeviceToken{}, errCodeExpired
		}
		if !opt.wait(ctx, interval) {
			return DeviceToken{}, ErrCanceled
		}
	}
}

// WithLock runs fn holding auth.lock (opt.LockFile). A login that keeps a
// token in the keychain and a logout that removes one take it, as a refresh
// does, so that they do not cross a refresh of the same entry.
func WithLock(opt Options, fn func() error) error {
	if opt.LockFile == "" {
		return errors.New("auth: LockFile is not set")
	}
	err := osfs.WithFileLock(opt.LockFile, opt.lockWait(), fn)
	if errors.Is(err, osfs.ErrLocked) {
		return fmt.Errorf("another hedgehug is refreshing or changing the token; wait a little and try again: %w", err)
	}
	return err
}

// deviceTokenOf makes the tokens of a response. An expires_in of 0 is a
// token with no end (the App has expiring tokens turned off): its time is
// left zero.
func deviceTokenOf(tr *github.TokenResponse, now time.Time) DeviceToken {
	t := DeviceToken{AccessToken: tr.AccessToken, RefreshToken: tr.RefreshToken}
	if tr.ExpiresIn > 0 {
		t.ExpiresAt = now.Add(time.Duration(tr.ExpiresIn) * time.Second)
	}
	if tr.RefreshTokenExpiresIn > 0 {
		t.RefreshExpiresAt = now.Add(time.Duration(tr.RefreshTokenExpiresIn) * time.Second)
	}
	return t
}

// fresh reports whether t needs no refresh yet: it has no end, or the end is
// more than refreshBefore away.
func fresh(t DeviceToken, now time.Time) bool {
	return t.ExpiresAt.IsZero() || now.Add(refreshBefore).Before(t.ExpiresAt)
}

// deviceToken returns the access token of a device login, refreshed first
// when it ends within refreshBefore. The refresh holds auth.lock and reads
// the keychain again under it, so of two processes only one refreshes.
func deviceToken(a config.Account, opt Options) (string, error) {
	k := opt.Keychain()
	t, err := LoadDeviceToken(a, k)
	if errors.Is(err, ErrNotFound) {
		return "", fmt.Errorf("%s is not logged in; run hedgehug auth login %s", Key(a), a.Name)
	}
	if err != nil {
		return "", err
	}
	if fresh(t, opt.now()) {
		return accessToken(a, t)
	}
	if opt.LockFile == "" {
		return "", errors.New("auth: LockFile is not set")
	}
	if opt.Host.Web == "" {
		return "", errors.New("auth: Host is not set")
	}
	if ClientID(opt.Host) == "" {
		return "", fmt.Errorf("refreshing the token of %s needs hosts[].client_id for %s in config.yaml; run hedgehug auth login %s", Key(a), a.Host, a.Name)
	}
	err = WithLock(opt, func() error {
		if t, err = LoadDeviceToken(a, k); err != nil {
			return err
		}
		if fresh(t, opt.now()) {
			return nil // another process refreshed it
		}
		if !t.RefreshExpiresAt.IsZero() && opt.now().After(t.RefreshExpiresAt) {
			return fmt.Errorf("the login of %s has expired; run hedgehug auth login %s", Key(a), a.Name)
		}
		tr, err := github.RefreshToken(opt.HTTP, opt.Host.Web, ClientID(opt.Host), t.RefreshToken)
		if err != nil {
			return err
		}
		if tr.Error != "" {
			return fmt.Errorf("refreshing the token of %s failed (%s); run hedgehug auth login %s", Key(a), tr.Error, a.Name)
		}
		t = deviceTokenOf(tr, opt.now())
		if err := SaveDeviceToken(a, t, k); err != nil {
			// The old refresh token is spent, so the new pair is lost.
			return fmt.Errorf("got a new token for %s but could not keep it in the keychain: %w; run hedgehug auth login %s", Key(a), err, a.Name)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return accessToken(a, t)
}

// accessToken is the access token of t, trimmed; an empty one is not a login.
func accessToken(a config.Account, t DeviceToken) (string, error) {
	tok := strings.TrimSpace(t.AccessToken)
	if tok == "" {
		return "", fmt.Errorf("the keychain has no access token of %s; run hedgehug auth login %s", Key(a), a.Name)
	}
	return tok, nil
}
