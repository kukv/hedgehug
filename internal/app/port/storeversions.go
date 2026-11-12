package port

import "context"

// StoreVersions は、Store ごとのいまの版の印の口。保存のたびに、書き手を問わず変わる(06「画面への反映」)。
type StoreVersions interface {
	// Versions は、このマシンで開いているすべての Store の版の印。読めない Store は含めない。
	Versions(ctx context.Context) (map[StoreID]string, error)
}
