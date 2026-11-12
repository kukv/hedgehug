package port

import "context"

// GitHub は GitHub の API の口。失敗は *ghcall.Failure で返す。トラック D が、使う操作だけを足していく。
type GitHub interface {
	// Viewer は、token の持ち主のログイン名を返す(07「認証の確認」)。
	Viewer(ctx context.Context, token string) (login string, err error)
}
