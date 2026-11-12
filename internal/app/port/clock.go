package port

import (
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// Clock は現在時刻(このマシンの時計。補正しない)。
type Clock interface {
	Now() time.Time
}

// IDs は内部 ID(ULID)の採番。
type IDs interface {
	New() change.ID
}
