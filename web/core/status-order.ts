import { CATEGORIES, type Category } from "./categories";
import type { SchemaView } from "./schema";

// Where a status stands in sort:status: the place of its category, then its place in the schema
// (the index in the whole list of statuses), then the place of its store.
export type StatusPlace = { category: number; pos: number; store: number };

const UNKNOWN = Number.MAX_SAFE_INTEGER;

// statusPlace is the place of a status in the schema of a store. storeAt is the index of the store
// in the order of GET /api/stores (negative: not known). A status the schema does not list, or one
// of a category it does not know, stands after all the others, as the server puts it.
export function statusPlace(
  schema: SchemaView | undefined,
  status: string,
  storeAt: number,
): StatusPlace {
  const statuses = schema?.statuses ?? [];
  const pos = statuses.findIndex((s) => s.name === status);
  const category = CATEGORIES.indexOf((pos >= 0 ? statuses[pos]?.category : "") as Category);
  return {
    category: category < 0 ? CATEGORIES.length : category,
    pos: pos < 0 ? UNKNOWN : pos,
    store: storeAt < 0 ? UNKNOWN : storeAt,
  };
}

// compareStatusPlace orders two places: the category, then the place in the schema, then the store.
export function compareStatusPlace(a: StatusPlace, b: StatusPlace): number {
  return (
    a.category - b.category || (a.pos < b.pos ? -1 : a.pos > b.pos ? 1 : 0) || a.store - b.store
  );
}

// statusRank is a place as one number to compare, for a sort of many tasks: it orders as
// compareStatusPlace does while the schema has fewer than 1000 statuses and there are fewer than
// 1000 stores.
export function statusRank(
  schema: SchemaView | undefined,
  status: string,
  storeAt: number,
): number {
  const p = statusPlace(schema, status, storeAt);
  return p.category * 1_000_000 + Math.min(p.pos, 999) * 1_000 + Math.min(p.store, 999);
}
