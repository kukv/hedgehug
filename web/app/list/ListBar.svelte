<script lang="ts">
  import type { Snippet } from "svelte";
  import FilterBar from "../tree/FilterBar.svelte";

  type Props = {
    base: string; // the fixed part of the query, as the user would type it; "" for every store
    applied: string; // what is applied now
    count: string; // "16 件 · 中止 1 件を除外"
    cli: string; // the hh line under the field
    error: unknown;
    onchange: (q: string) => void;
    controls?: Snippet; // the switches of the layout (columns, swimlane...)
    actions?: Snippet; // the buttons ("ビューを保存"...)
  };
  let { base, applied, count, cli, error, onchange, controls, actions }: Props = $props();
</script>

<!-- The filter of every list screen, with the switches of its layout and its buttons at the right. -->
<div class="row">
  <div class="filter">
    <FilterBar {base} {applied} {count} {cli} {error} {onchange} />
  </div>
  {#if controls || actions}
    <div class="side">
      {#if controls}
        <div class="controls">{@render controls()}</div>
      {/if}
      {#if actions}
        <div class="actions">{@render actions()}</div>
      {/if}
    </div>
  {/if}
</div>

<style>
  .row {
    display: flex;
    flex-wrap: wrap;
    align-items: flex-start;
  }
  /* Wide enough for the fixed part, the field, the count and some of the command; with less, the
     switches go to the next line. */
  .filter {
    flex: 1 1 36rem;
    min-width: 0;
  }
  /* The bar has its own gutter at both ends; the side takes the right one. */
  .filter :global(.bar) {
    padding-right: 0.75rem;
  }
  .side {
    flex: 0 1 auto;
    min-width: 0;
    display: flex;
    align-items: center;
    gap: 0.75rem;
    min-height: 2.25rem;
    margin: 0.75rem 1.75rem 0;
  }
  .controls,
  .actions {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    min-width: 0;
  }
</style>
