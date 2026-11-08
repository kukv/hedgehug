<script lang="ts">
  import Icon from "../icons/Icon.svelte";

  // The handle of a row: drag it, or press Alt+Up / Alt+Down on it. id names the handle for the
  // focus to come back to; hint is the element that tells the keys; off is why it cannot be used.
  let {
    id,
    subject,
    hint,
    disabled,
    off,
    onstep,
  }: {
    id: string;
    subject: string;
    hint: string;
    disabled: boolean;
    off: string | null;
    onstep: (by: -1 | 1) => void;
  } = $props();

  function onkeydown(e: KeyboardEvent): void {
    if (!e.altKey || e.shiftKey || e.ctrlKey || e.metaKey) return;
    if (e.key !== "ArrowUp" && e.key !== "ArrowDown") return;
    e.preventDefault();
    onstep(e.key === "ArrowUp" ? -1 : 1);
  }
</script>

<button
  type="button"
  class="handle"
  data-handle={id}
  aria-label="並べ替え（{subject}）"
  aria-describedby={hint}
  title={off ?? "ドラッグ、または ⌥↑ / ⌥↓ で移動"}
  disabled={disabled || off !== null}
  {onkeydown}
>
  <Icon name="grip" size={14} />
</button>

<style>
  .handle {
    display: inline-flex;
    padding: var(--space-4);
    border: 0;
    border-radius: var(--radius-4);
    background: none;
    color: var(--muted);
    cursor: grab;
  }
  .handle:hover:not(:disabled) {
    color: var(--ink);
  }
  .handle:focus-visible {
    outline: 2px solid var(--accent);
  }
  .handle:disabled {
    cursor: default;
    opacity: 0.4;
  }
</style>
