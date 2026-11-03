import type { ViewDraft, ViewEntry } from "../../core/views";

/** The view dialog that is asked for; ViewDialogs draws it. */
export type ViewDialog =
  | { kind: "save"; draft: ViewDraft; store?: string }
  | { kind: "rename"; entry: ViewEntry; carry: ViewDraft | null }
  | { kind: "delete"; entry: ViewEntry };

// raw: the entry is the one the directory holds, not a copy of it.
let current = $state.raw<ViewDialog | null>(null);

export const viewDialog = {
  get current(): ViewDialog | null {
    return current;
  },
};

/** store is the Store the dialog starts with; without it, the default Store. */
export function openViewSave(i: { draft: ViewDraft; store?: string }): void {
  current = { kind: "save", draft: i.draft, store: i.store };
}

/** carry is the unsaved change of the screen of the view, which the screen of the new name takes up. */
export function openViewRename(entry: ViewEntry, carry: ViewDraft | null = null): void {
  current = { kind: "rename", entry, carry };
}

export function openViewDelete(entry: ViewEntry): void {
  current = { kind: "delete", entry };
}

export function closeViewDialog(): void {
  current = null;
}

// The unsaved change of a view that was renamed, for the screen of the new name to start with.
let carried: { store: string; name: string; draft: ViewDraft } | null = null;

export function carryDraft(store: string, name: string, draft: ViewDraft): void {
  carried = { store, name, draft };
}

/** Takes the carried change once, if it is for this view. */
export function takeCarriedDraft(store: string, name: string): ViewDraft | null {
  const c = carried;
  carried = null;
  return c && c.store === store && c.name === name ? c.draft : null;
}
