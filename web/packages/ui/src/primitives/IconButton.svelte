<script lang="ts">
  import type { Snippet } from "svelte";

  interface Props {
    label: string;
    size?: "md" | "sm" | "xs";
    active?: boolean;
    disabled?: boolean;
    title?: string;
    onclick?: (event: MouseEvent) => void;
    children: Snippet;
  }

  let {
    label,
    size = "md",
    active = false,
    disabled = false,
    title,
    onclick,
    children,
  }: Props = $props();

  // 押せない間は onclick を呼ばない(合成イベントでも)。
  function guarded(event: MouseEvent) {
    if (!disabled) onclick?.(event);
  }
</script>

<button
  type="button"
  class="icon-btn {size}"
  class:on={active}
  aria-label={label}
  {disabled}
  {title}
  onclick={guarded}
>
  {@render children()}
</button>

<style>
  .icon-btn {
    width: 30px;
    height: 30px;
    border: 0;
    background: transparent;
    border-radius: 6px;
    display: inline-grid;
    place-items: center;
    color: var(--muted);
    flex: none;
  }
  .icon-btn:hover,
  .on {
    background: var(--hover);
    color: var(--ink);
  }
  .icon-btn:disabled {
    opacity: 0.45;
    cursor: not-allowed;
  }
  .sm {
    width: 26px;
    height: 26px;
  }
  .xs {
    width: 22px;
    height: 22px;
  }
</style>
