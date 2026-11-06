import { type Api, ApiError, type ShowView } from "../../core/api/client";

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

// forgetTask takes a task that no longer exists out of the list.
export function forgetTask(
  ulid: string,
  storage: Pick<Storage, "getItem" | "setItem"> = localStorage,
): void {
  try {
    storage.setItem(KEY, JSON.stringify(readRecent(storage).filter((u) => u !== ulid)));
  } catch {
    // The list is a convenience; losing it is fine.
  }
}

export type RecentDeps = {
  api: Api;
  has: (ulid: string) => boolean;
  // The point a read begins at, and what puts what was fetched in the cache as a read of that point.
  epoch: () => number;
  prime: (tasks: ShowView[], since: number) => void;
};

// fetchMissing gets the tasks of the list that the cache does not have. One that is gone
// (404) is forgotten; any other failure leaves the list as it is.
export async function fetchMissing(
  d: RecentDeps,
  ulids: string[],
  storage?: Pick<Storage, "getItem" | "setItem">,
): Promise<void> {
  await Promise.all(
    ulids
      .filter((u) => !d.has(u))
      .map(async (u) => {
        const since = d.epoch();
        try {
          const show = await d.api.getTask(u);
          d.prime([show], since);
        } catch (e) {
          if (e instanceof ApiError && e.status === 404) forgetTask(u, storage);
        }
      }),
  );
}
