// Package notes は、詳細パネルとプロジェクトの概要の付録(説明・コメント・作業時間・関連・履歴)の API を持つ。
// 区画ごとに 1 ファイルで、Register に 1 行ずつ足す。
package notes

import (
	"github.com/danielgtaylor/huma/v2"

	appdescriptions "github.com/kukv/hedgehug/internal/app/descriptions"
)

// Services は、付録の API が使う usecase。区画のタスクが、自分のものを足す。
type Services struct {
	Descriptions appdescriptions.Service
}

// Register は、付録の API を登録する。
func Register(api huma.API, s Services) {
	registerDescriptions(api, s.Descriptions)
}
