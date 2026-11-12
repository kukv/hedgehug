package port

import "context"

// Hosts は、config.yaml の hosts の口。
type Hosts interface {
	// ClientID は、ホスト github.com のデバイスフローの client_id。書かれていなければ空。
	ClientID(ctx context.Context) (string, error)
}
