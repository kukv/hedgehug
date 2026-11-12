// Package projects は、プロジェクトの操作(02「プロジェクト」)を持つ。1 ファイル 1 操作。
package projects

import "github.com/kukv/hedgehug/internal/app/port"

// Service は、プロジェクトの操作が使う口。
type Service struct {
	Stores   port.Stores
	IDs      port.IDs
	Identity port.Identity
}
