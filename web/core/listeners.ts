// callAll runs every listener in order. A listener that throws must not stop
// the others or break the caller's state, so its error is only reported.
// The list is copied first so a listener that subscribes or unsubscribes
// while it runs does not change the round in progress.
export function callAll(fns: Iterable<() => void>): void {
  for (const fn of [...fns]) {
    try {
      fn();
    } catch (e) {
      console.error(e);
    }
  }
}
