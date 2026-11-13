package machineconfig

import (
	"context"
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

// SyncSettings は、config.yaml の sync を読む port.SyncSettings。
func (f *File) SyncSettings() port.SyncSettings { return syncSettings{f} }

type syncSettings struct{ f *File }

func (s syncSettings) Timing(context.Context) (port.SyncTiming, error) {
	c, err := s.f.read()
	if err != nil {
		return port.SyncTiming{}, err
	}
	t := defaultTiming
	if c.Sync.Auto != nil {
		t.Auto = *c.Sync.Auto
	}
	if c.Sync.Interval > 0 {
		t.Interval = time.Duration(c.Sync.Interval) * time.Second
	}
	if c.Sync.PushInterval > 0 {
		t.AfterWrite = time.Duration(c.Sync.PushInterval) * time.Second
	}
	if c.Sync.GitHubInterval > 0 {
		t.GitHubEvery = time.Duration(c.Sync.GitHubInterval) * time.Second
	}
	return t, nil
}
