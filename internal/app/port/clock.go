package port

import (
	"time"
)

// Clock は現在時刻(このマシンの時計。補正しない)。
type Clock interface {
	Now() time.Time
}
