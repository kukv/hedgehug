/**
 * isWatching tells whether the account is among the watchers of a task. GET /stores gives the
 * account of a store as host/login (the ID of the account that writes it), and the watchers of
 * a task are written the same way, so the two are compared as they are.
 */
export function isWatching(watchers: readonly string[], account: string): boolean {
  return account !== "" && watchers.includes(account);
}
