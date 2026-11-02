<script lang="ts">
  import type { Snippet } from "svelte";
  import type { HTMLButtonAttributes } from "svelte/elements";

  type Props = HTMLButtonAttributes & {
    variant?: "primary" | "default" | "confirm" | "danger" | "ghost";
    size?: "md" | "sm";
    children: Snippet;
  };
  let { variant = "default", size = "md", type = "button", children, ...rest }: Props = $props();
</script>

<button {type} class="btn" data-variant={variant} data-size={size} {...rest}>
  {@render children()}
</button>

<style>
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
