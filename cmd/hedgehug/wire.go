//go:build !e2e

package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"

	"github.com/kukv/hedgehug/internal/adapter/httpapi"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/infra/github"
	"github.com/kukv/hedgehug/internal/infra/gitstore"
	"github.com/kukv/hedgehug/internal/infra/machineconfig"
	"github.com/kukv/hedgehug/internal/infra/secret"
	"github.com/kukv/hedgehug/internal/infra/sysclock"
)

// wire は、本物の部品を口に差し込む。Store の一覧の読み込みはトラック G が足す(この計画の決定 6)。
func wire(context.Context) (httpapi.Deps, error) {
	stores := gitstore.New(sysclock.Clock{}, sysclock.IDs{})
	cfg, err := configPath()
	if err != nil {
		return httpapi.Deps{}, err
	}
	file := machineconfig.Open(cfg)
	gh := github.New(nil) // 時計のずれはトラック C が受け取る(トラック D の計画の決定 3)
	return httpapi.Deps{
		Stores:      stores,
		IDs:         sysclock.IDs{},
		Identity:    noIdentity{},
		StoreList:   noStores{},
		Accounts:    file.Accounts(),
		Secrets:     secret.New(),
		GitHub:      gh,
		DeviceFlow:  gh,
		Hosts:       file.Hosts(),
		StoreGitHub: noStoreGitHub{},
		Clock:       sysclock.Clock{},
	}, nil
}

// noStores は、Store の一覧(トラック G)が入るまでの仮。表示する Store はない。
type noStores struct{}

func (noStores) List(context.Context) ([]port.StoreInfo, error) { return []port.StoreInfo{}, nil }

// noIdentity は、「あなた」を決める仕組み(トラック G)が入るまでの仮。保存する操作は断る。
type noIdentity struct{}

func (noIdentity) Me(context.Context, port.StoreID) (string, error) {
	return "", errors.New("このマシンのログイン名が未設定です")
}

// tokenAndAddr は、起動ごとのトークンと、127.0.0.1 の空いた番号を返す。
func tokenAndAddr() (token, addr string) { return newToken(), "127.0.0.1:0" }

func newToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b) // crypto/rand.Read は失敗しない(Go 1.24 から)
	return hex.EncodeToString(b)
}

// wrap は、本物のビルドでは何も足さない(e2e のビルドだけが、テストの入口を足す)。
func wrap(_ string, h http.Handler) http.Handler { return h }
