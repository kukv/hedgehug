const KEY = "hedgehug.recent";
const MAX = 8;

// readRecent is the ULIDs of the tasks opened last, the latest first. It is a
// convenience of this browser: when the storage cannot be read it is empty.
export function readRecent(storage: Pick<Storage, "getItem"> = localStorage): string[] {
  try {
    const v: unknown = JSON.parse(storage.getItem(KEY) ?? "[]");
    return Array.isArray(v) ? v.filter((x): x is string => typeof x === "string") : [];
  } catch {
    return [];
  }
}

// rememberTask puts a task at the top of the list, once, and keeps the last eight.
export function rememberTask(
  ulid: string,
  storage: Pick<Storage, "getItem" | "setItem"> = localStorage,
): void {
  try {
    const next = [ulid, ...readRecent(storage).filter((u) => u !== ulid)].slice(0, MAX);
    storage.setItem(KEY, JSON.stringify(next));
  } catch {
    // The list is a convenience; losing it is fine.
  }
}
