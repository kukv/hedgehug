package syncing

import (
	"context"
	"errors"
	"sync"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/syncstatus"
)

// AllOutcome は、「すべて同期」の結果。Lines は 1 つのトーストの行(06「同期の結果」)。
type AllOutcome struct {
	Nothing  bool // 表示中で remote がある Store がなく、何もしなかった
	Outcomes []Outcome
	Lines    []string
}

// SyncAll は、表示中で remote がある Store をすべて同期する(サイドバーの「すべて同期」ほか)。
// remote がない Store と非表示の Store は数えない。
func (s *Service) SyncAll(ctx context.Context) (AllOutcome, error) {
	targets, err := s.Targets.List(ctx)
	if err != nil {
		return AllOutcome{}, err
	}
	var picked []port.SyncTarget
	for _, t := range targets {
		if t.Remote != "" && !t.Hidden {
			picked = append(picked, t)
		}
	}
	if len(picked) == 0 {
		return AllOutcome{Nothing: true, Lines: []string{syncstatus.NothingToSync}}, nil
	}
	outs := make([]Outcome, len(picked))
	errs := make([]error, len(picked))
	var wg sync.WaitGroup
	for i, t := range picked {
		wg.Go(func() { outs[i], errs[i] = s.once(ctx, t) })
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return AllOutcome{}, err
	}
	ok := 0
	var failures []syncstatus.Failure
	for _, o := range outs {
		if o.OK() {
			ok++
			continue
		}
		failures = append(failures, syncstatus.Failure{Store: o.Name, Reason: o.Reason})
	}
	return AllOutcome{Outcomes: outs, Lines: syncstatus.All(ok, failures)}, nil
}
