package port

import "github.com/kukv/hedgehug/internal/domain/change"

// IDs は内部 ID(ULID)の採番。
type IDs interface {
	New() change.ID
}
