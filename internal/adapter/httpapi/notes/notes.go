// Package notes は、詳細パネルとプロジェクトの概要の付録(説明・コメント・作業時間・関連・履歴)の API を持つ。
// 区画ごとに 1 ファイルで、Register に 1 行ずつ足す。
package notes

import (
	"github.com/danielgtaylor/huma/v2"

	appcomments "github.com/kukv/hedgehug/internal/app/comments"
	appdescriptions "github.com/kukv/hedgehug/internal/app/descriptions"
	apphistory "github.com/kukv/hedgehug/internal/app/history"
	apprelations "github.com/kukv/hedgehug/internal/app/relations"
	appworklogs "github.com/kukv/hedgehug/internal/app/worklogs"
)

// Services は、付録の API が使う usecase。区画のタスクが、自分のものを足す。
type Services struct {
	Descriptions appdescriptions.Service
	Comments     appcomments.Service
	Worklogs     appworklogs.Service
	Relations    apprelations.Service
	History      apphistory.Service
}

// Register は、付録の API を登録する。
func Register(api huma.API, s Services) {
	registerDescriptions(api, s.Descriptions)
	registerComments(api, s.Comments)
	registerWorklogs(api, s.Worklogs)
	registerRelations(api, s.Relations)
	registerHistory(api, s.History)
}
