// Package items は、アイテムの操作(02)を持つ。1 ファイル 1 操作。
package items

import "github.com/kukv/hedgehug/internal/app/port"

// Service は、アイテムの操作が使う口。
type Service struct {
	Stores   port.Stores
	IDs      port.IDs
	Identity port.Identity
}
