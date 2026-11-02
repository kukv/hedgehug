<script lang="ts">
  import type { Snippet } from "svelte";

  type Props = {
    tone?: "neutral" | "accent" | "success" | "warning" | "danger" | "orange";
    mono?: boolean;
    children: Snippet;
  };
  let { tone = "neutral", mono = false, children }: Props = $props();
</script>

<span class="chip" class:mono data-tone={tone}>{@render children()}</span>

<style>
  .chip {
    display: inline-flex;
    align-items: center;
    gap: var(--space-4);
    height: 20px;
    padding: 0 var(--space-6);
    border-radius: var(--radius-4);
    font-size: var(--fs-12);
    white-space: nowrap;
  }
  .mono {
    font-family: var(--font-mono);
    font-size: var(--fs-mono);
  }
  .chip[data-tone="neutral"] {
    background: var(--hover);
    color: var(--ink-2);
  }
  .chip[data-tone="accent"] {
    background: var(--accent-weak);
    color: var(--accent-ink);
  }
  .chip[data-tone="success"] {
    background: var(--success-weak);
    color: var(--success-ink);
  }
  .chip[data-tone="warning"] {
    background: var(--warning-weak);
    color: var(--warning);
  }
  .chip[data-tone="danger"] {
    background: var(--danger-weak);
    color: var(--danger);
  }
  .chip[data-tone="orange"] {
    background: var(--store-orange-bg);
    color: var(--store-orange-fg);
  }
</style>
