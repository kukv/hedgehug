import { createRawSnippet } from "svelte";

export function text(value: string) {
  return createRawSnippet(() => ({ render: () => `<span>${value}</span>` }));
}
