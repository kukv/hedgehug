package ops

// beforeUpdate lets tests write to the store after an operation read its
// index and before it writes, as another process could.
var beforeUpdate = func() {}

// SetBeforeUpdate sets the hook for a test and returns what restores it.
func SetBeforeUpdate(f func()) (restore func()) {
	old := beforeUpdate
	beforeUpdate = f
	return func() { beforeUpdate = old }
}
