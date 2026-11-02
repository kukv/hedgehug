let ulid = $state<string | null>(null);

/** The row the tree has selected; the create dialog and the palette read it. */
export const selection = {
  get ulid(): string | null {
    return ulid;
  },
  set(u: string | null): void {
    ulid = u;
  },
};
