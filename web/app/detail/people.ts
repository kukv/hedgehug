import { PeopleCache } from "../../core/people";
import type { AppCore } from "../context";

const caches = new WeakMap<object, PeopleCache>();

/**
 * The PeopleCache of the app. It is not one of the objects of AppCore, so it is made once per
 * connection, here.
 */
export function peopleOf(c: Pick<AppCore, "api" | "conn">): PeopleCache {
  let p = caches.get(c.conn);
  if (!p) {
    p = new PeopleCache({ api: c.api, conn: c.conn });
    caches.set(c.conn, p);
  }
  return p;
}
