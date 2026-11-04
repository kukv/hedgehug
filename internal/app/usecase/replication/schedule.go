package replication

import (
	"time"

	"github.com/kukv/hedgehug/internal/app/remote/replicate"
	"github.com/kukv/hedgehug/internal/app/usecase"
)

// Schedule is what a loop that syncs in the background reads from
// config.yaml: the intervals, and the stores it syncs.
type Schedule struct {
	usecase.SyncIntervals
	Stores []string // the stores synced in the background (those with a remote)
}

// LoadSchedule reads the schedule as config.yaml has it now.
func LoadSchedule(ss *usecase.Session) (Schedule, error) {
	c, _, err := ss.Config()
	if err != nil {
		return Schedule{}, err
	}
	all, err := ss.SyncTargets(c, true)
	if err != nil {
		return Schedule{}, err
	}
	sch := Schedule{SyncIntervals: usecase.SyncIntervals{Interval: c.Sync.Interval, PushInterval: c.Sync.PushInterval}}
	for _, e := range all {
		if SyncsInBackground(e) {
			sch.Stores = append(sch.Stores, e.Name)
		}
	}
	return sch, nil
}

// PendingSince is the first write of the session's store not pushed; zero
// when there is none or sync.json does not read.
func PendingSince(ss *usecase.Session) time.Time {
	s, err := ss.OpenStore()
	if err != nil {
		return time.Time{}
	}
	st, err := replicate.ReadState(s)
	if err != nil {
		return time.Time{}
	}
	return st.PendingSince
}

// SyncOne is the sync of the session's store, as the background sync of the
// CLI does it. err is the failure of Config, Select or SyncStore; a sync that
// failed is in rep.Err. The duplicates across stores are added to the
// warnings, as a sync of all the stores adds them, also when the sync failed.
func SyncOne(ss *usecase.Session) (rep usecase.SyncReport, err error) {
	c, _, err := ss.Config()
	if err != nil {
		return rep, err
	}
	e, err := c.Select(ss.StoreName)
	if err != nil {
		return rep, err
	}
	rep, err = SyncStore(ss, c, e, false)
	reps := []usecase.SyncReport{rep}
	ss.AddDuplicateWarnings(reps)
	return reps[0], err
}
