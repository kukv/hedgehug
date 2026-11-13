// Package projects は、プロジェクトの操作(02「プロジェクト」)を持つ。1 ファイル 1 操作。
package projects

import "github.com/kukv/hedgehug/internal/app/port"

// Service は、プロジェクトの操作が使う口。Clock はアクティビティの日付の年を決める。
type Service struct {
	Stores   port.Stores
	IDs      port.IDs
	Identity port.Identity
	Clock    port.Clock
}
