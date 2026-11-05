<script lang="ts">
  import { type Link, prChip } from "../../core/github";

  // The pull requests of a task in one chip (Foundations). tabbable: false in the tree, whose rows
  // take the Tab key one at a time.
  type Props = { links: readonly Link[] | undefined; tabbable?: boolean };
  let { links, tabbable = true }: Props = $props();

  const chip = $derived(prChip(links));
</script>

{#if chip}
  <a
    class="chip"
    data-tone={chip.tone}
    href={chip.href}
    target="_blank"
    rel="noreferrer"
    tabindex={tabbable ? undefined : -1}
    onclick={(e) => e.stopPropagation()}
    ondblclick={(e) => e.stopPropagation()}
  >
    {chip.label}
    {#if chip.text}
      <span class="state">{chip.text}</span>
    {/if}
    {#if chip.more > 0}
      <span class="more">+{chip.more}</span>
    {/if}
  </a>
{/if}

<style>
  .chip {
    position: relative;
    z-index: 1;
    flex: 0 0 auto;
    display: inline-flex;
    align-items: center;
    gap: var(--space-4);
    height: 1.375rem;
    box-sizing: border-box;
    padding: 0 var(--space-8);
    border: var(--hairline) solid var(--line);
    border-radius: var(--radius-pill);
    background: var(--surface);
    font-family: var(--font-mono);
    font-size: var(--fs-mono);
    text-decoration: none;
    white-space: nowrap;
  }
  .chip:hover {
    border-color: var(--line-input);
  }
  .state {
    font-family: var(--font-sans);
  }
  .more {
    color: var(--muted);
  }
  [data-tone="open"] {
    color: var(--success);
  }
  [data-tone="draft"],
  [data-tone="closed"],
  [data-tone="unread"] {
    color: var(--ink-2);
  }
  [data-tone="merged"] {
    color: var(--merged);
  }
  [data-tone="fail"] {
    color: var(--danger);
  }
  [data-tone="changes"] {
    color: var(--pending);
  }
</style>
