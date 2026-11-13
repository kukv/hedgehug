<script lang="ts">
  import type { Snippet } from "svelte";

  interface Props {
    variant?: "default" | "primary" | "danger" | "ghost";
    size?: "md" | "sm" | "xs";
    type?: "button" | "submit";
    disabled?: boolean;
    title?: string;
    pressed?: boolean;
    onclick?: (event: MouseEvent) => void;
    children: Snippet;
  }

  let {
    variant = "default",
    size = "md",
    type = "button",
    disabled = false,
    title,
    pressed,
    onclick,
    children,
  }: Props = $props();

  // 押せない間は onclick を呼ばない(合成イベントでも)。
  function guarded(event: MouseEvent) {
    if (!disabled) onclick?.(event);
  }
</script>

<button
  {type}
  class="btn {variant} {size}"
  {disabled}
  {title}
  aria-pressed={pressed}
  onclick={guarded}
>
  {@render children()}
</button>

<style>
  .btn {
    height: var(--control-h);
    padding: 0 12px;
    border: 1px solid var(--line-input);
    background: var(--surface);
    border-radius: 7px;
    display: inline-flex;
    align-items: center;
    gap: 6px;
    white-space: nowrap;
    color: var(--ink);
  }
  .btn:hover {
    background: var(--hover);
  }
  .btn:disabled {
    opacity: 0.45;
    cursor: not-allowed;
  }
  .primary {
    background: var(--accent);
    border-color: var(--accent);
    color: var(--on-accent);
  }
  .primary:hover {
    filter: brightness(1.08);
    background: var(--accent);
  }
  .danger {
    color: var(--danger);
    border-color: color-mix(in srgb, var(--danger) 40%, var(--line-input));
  }
  .ghost {
    border-color: transparent;
    background: transparent;
    color: var(--muted);
  }
  .ghost:hover {
    color: var(--ink);
    background: var(--hover);
  }
  .sm {
    height: 28px;
    padding: 0 10px;
    font-size: 12.5px;
  }
  .xs {
    height: 24px;
    padding: 0 8px;
    font-size: 12px;
  }
</style>
