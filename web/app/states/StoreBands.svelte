<script lang="ts">
  import { core } from "../context";
  import Icon from "../icons/Icon.svelte";
  import { readOnlySentence, useReadOnly } from "../read-only";

  // The bands above the page of a Store: it is read-only (and why).
  // spaced: the page has no gap of its own, so the band keeps its distance from what follows.
  let { store, spaced = false }: { store: string; spaced?: boolean } = $props();

  const { stores } = core();
  const reason = useReadOnly(stores, () => store);
</script>

{#if reason.current !== null}
  <div class="band" class:spaced role="status" aria-label="閲覧のみ">
    <Icon name="lock" size={16} />
    <span>{readOnlySentence(reason.current)}</span>
  </div>
{/if}

<style>
  .band {
    display: flex;
    align-items: center;
    gap: var(--space-10);
    padding: var(--space-10) 14px;
    border-radius: var(--radius-10);
    line-height: 1.55;
    background: var(--ink);
    color: var(--surface);
  }
  .spaced {
    margin-bottom: var(--space-12);
  }
  .band span {
    flex: 1;
    min-width: 0;
    overflow-wrap: anywhere;
  }
  .band :global(svg) {
    color: var(--alert-on-dark);
  }
</style>
