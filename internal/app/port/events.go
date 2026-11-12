package port

// EventKind は、画面に知らせる出来事の種類(06「画面への反映」)。
type EventKind string

const (
	EventStore EventKind = "store" // Store の中身が変わった(書き手を問わない)
	EventSync  EventKind = "sync"  // Store の同期の状態が変わった
)

// Event は、画面に知らせる出来事。
type Event struct {
	Kind  EventKind
	Store StoreID
}

// Events は、出来事を画面に知らせる口。実装は adapter(Server-Sent Events。設計書 3 節)。
// Publish は待たない(遅い画面には届かないことがある)。
type Events interface {
	Publish(e Event)
}
