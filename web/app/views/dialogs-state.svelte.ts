import type { ViewDraft, ViewEntry } from "../../core/views";

/** The view dialog that is asked for; ViewDialogs draws it. */
export type ViewDialog =
  | { kind: "save"; draft: ViewDraft; store?: string }
  | { kind: "rename"; entry: ViewEntry }
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

export function openViewRename(entry: ViewEntry): void {
  current = { kind: "rename", entry };
}

export function openViewDelete(entry: ViewEntry): void {
  current = { kind: "delete", entry };
}

export function closeViewDialog(): void {
  current = null;
}
