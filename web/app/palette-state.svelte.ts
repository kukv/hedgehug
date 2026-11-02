let open = $state(false);

/** Whether the command palette is open. The sidebar opens it; CommandPalette reads it. */
export const palette = {
  get open(): boolean {
    return open;
  },
};

export function openPalette(): void {
  open = true;
}

export function closePalette(): void {
  open = false;
}
