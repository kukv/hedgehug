package machineconfig

import (
	"context"
	"slices"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
)

// syncYAML は config.yaml の sync(決定 C2-4。前回の実装と同じ鍵。秒で書く)。書かれていない項目は初期値。
type syncYAML struct {
	Auto         *bool `yaml:"auto,omitempty"`
	Interval     int   `yaml:"interval,omitempty"`      // 間隔(秒)
	PushInterval int   `yaml:"push_interval,omitempty"` // 書き込みのあと同期するまで(秒)

	GitHubInterval int `yaml:"github_interval,omitempty"` // GitHub を読む間隔(秒。D4a-7)
}

// 06「自動で同期する」の初期値。
var defaultTiming = port.SyncTiming{Auto: true, Interval: 5 * time.Minute, AfterWrite: 30 * time.Second, GitHubEvery: 5 * time.Minute}

// SyncSettings は、config.yaml の sync を読み書きする port.SyncSettings。
func (f *File) SyncSettings() port.SyncSettings { return syncSettings{f} }

type syncSettings struct{ f *File }

// choice は、秒 sec が choices(08 の選択肢。G1-18)のどれかならその長さ、そうでなければ def(書かれていない 0 も def)。
func choice(sec int, choices []time.Duration, def time.Duration) time.Duration {
	d := time.Duration(sec) * time.Second
	if slices.Contains(choices, d) {
		return d
	}
	return def
}

func (s syncSettings) Timing(context.Context) (port.SyncTiming, error) {
	c, err := s.f.read()
	if err != nil {
		return port.SyncTiming{}, err
	}
	t := defaultTiming
	if c.Sync.Auto != nil {
		t.Auto = *c.Sync.Auto
	}
	t.Interval = choice(c.Sync.Interval, port.IntervalChoices, t.Interval)
	t.AfterWrite = choice(c.Sync.PushInterval, port.AfterWriteChoices, t.AfterWrite)
	t.GitHubEvery = choice(c.Sync.GitHubInterval, port.GitHubChoices, t.GitHubEvery)
	return t, nil
}

func (s syncSettings) SetTiming(_ context.Context, t port.SyncTiming) error {
	return s.f.update(func(c *config) error {
		auto := t.Auto
		c.Sync = syncYAML{
			Auto:           &auto,
			Interval:       int(t.Interval / time.Second),
			PushInterval:   int(t.AfterWrite / time.Second),
			GitHubInterval: int(t.GitHubEvery / time.Second),
		}
		return nil
	})
}
