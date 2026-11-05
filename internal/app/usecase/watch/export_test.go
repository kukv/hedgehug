package watch

import (
	"github.com/kukv/hedgehug/internal/app/storage/history"
	"github.com/kukv/hedgehug/internal/app/storage/store"
)

// SplitMax is splitMax for the tests.
const SplitMax = splitMax

// ChildTask is childTask for the tests.
var ChildTask = childTask

// SetOpenHistory replaces the build of the history index of w.
func SetOpenHistory(w *Watcher, f func(*store.Store, history.Options) (*history.History, error)) {
	w.openHistory = f
}

// Held is whether w holds the store named name.
func Held(w *Watcher, name string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.stores[name] != nil
}

// HeldStore is the store of that name w holds; nil when none.
func HeldStore(w *Watcher, name string) *store.Store {
	w.mu.Lock()
	defer w.mu.Unlock()
	if st := w.stores[name]; st != nil {
		return st.s
	}
	return nil
}

// HeldCount is how many stores w holds.
func HeldCount(w *Watcher) int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.stores)
}

// Released is whether w marks the store named name as rebuilt.
func Released(w *Watcher, name string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.released[name]
}

// Looked is whether a poll of w has run.
func Looked(w *Watcher) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.looked
}

// Scanned is whether a poll of w has read the stores to the end.
func Scanned(w *Watcher) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.scanned
}
