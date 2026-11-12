// Package sysclock は、このマシンの時計と、内部 ID(ULID)の採番を持つ。
package sysclock

import (
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// Clock はこのマシンの時計。時刻帯はこのマシンのもの(記録の at に書く)。
type Clock struct{}

func (Clock) Now() time.Time { return time.Now() }

// IDs は ULID の採番。同じミリ秒の中でも増えていく。
type IDs struct{}

func (IDs) New() change.ID { return change.ID(ulid.Make().String()) }
