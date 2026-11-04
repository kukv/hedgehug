<script lang="ts">
  import type { Snippet } from "svelte";
  import type { HTMLButtonAttributes } from "svelte/elements";

  type Props = HTMLButtonAttributes & {
    variant?: "primary" | "default" | "confirm" | "danger" | "ghost";
    size?: "md" | "sm";
    children: Snippet;
    /** Why the button cannot be used: its tooltip, and read out as its description. */
    why?: string;
  };
  let {
    variant = "default",
    size = "md",
    type = "button",
    why,
    children,
    ...rest
  }: Props = $props();
  const uid = $props.id();
</script>

<button
  {type}
  class="btn"
  data-variant={variant}
  data-size={size}
  title={why}
  aria-describedby={why ? `${uid}-why` : undefined}
  {...rest}
>
  {@render children()}
</button>
{#if why}
  <span id="{uid}-why" class="sr">{why}</span>
{/if}

<style>
  .sr {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip: rect(0 0 0 0);
    white-space: nowrap;
  }
  .btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-6);
    border: 1px solid transparent;
    border-radius: var(--radius-6);
    font-weight: 500;
    cursor: pointer;
    white-space: nowrap;
  }
  .btn[data-size="md"] {
    min-height: var(--control-h);
    padding: 0 var(--space-16);
    font-size: var(--fs-13);
  }
  .btn[data-size="sm"] {
    min-height: var(--control-h-sm);
    padding: 0 var(--space-12);
    font-size: var(--fs-12);
  }
  .btn[data-variant="primary"] {
    background: var(--accent);
    color: var(--surface);
  }
  .btn[data-variant="default"] {
    background: var(--surface);
    border-color: var(--line-input);
    color: var(--ink);
  }
  .btn[data-variant="confirm"] {
    background: var(--ink);
    color: var(--surface);
  }
  .btn[data-variant="danger"] {
    background: var(--danger);
    color: var(--surface);
  }
  .btn[data-variant="ghost"] {
    background: transparent;
    color: var(--ink-2);
  }
  .btn[data-variant="ghost"]:hover:not(:disabled) {
    background: var(--hover);
  }
  .btn:disabled {
    background: var(--line);
    border-color: transparent;
    color: var(--muted);
    cursor: not-allowed;
  }
</style>
