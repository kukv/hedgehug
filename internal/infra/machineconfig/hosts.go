package machineconfig

import (
	"context"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
)

// hostYAML は hosts の 1 行(前回と同じ形。host と client_id だけを読む。この小計画の決定 D2-14)。
type hostYAML struct {
	Host     string `yaml:"host"`
	ClientID string `yaml:"client_id,omitempty"`
}

// Hosts は、config.yaml の hosts を読む port.Hosts。
func (f *File) Hosts() port.Hosts { return hosts{f} }

type hosts struct{ f *File }

func (h hosts) ClientID(context.Context) (string, error) {
	c, err := h.f.read()
	if err != nil {
		return "", err
	}
	for _, x := range c.Hosts {
		if strings.TrimSpace(x.Host) == account.Host {
			return strings.TrimSpace(x.ClientID), nil
		}
	}
	return "", nil
}
