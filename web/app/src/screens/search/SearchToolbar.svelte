<script lang="ts">
  import { layoutChoices, type Search } from "@hedgehug/core";
  import { LayoutSwitch } from "@hedgehug/ui";
  import type { Snippet } from "svelte";
  import TableToolbar from "../table/TableToolbar.svelte";

  // 検索の画面のツールバー(04「検索」)。左端にレイアウトの切り替えを置く(E3a-1)。テーブルなら列とグループの設定と
  // 件数、ツリーなら件数だけを出す(ツリーは設定を持たない。E3a-7)。actions は、どちらでも右端に置く。
  interface Props {
    search: Search;
    // treeCount は、ツリーの件数(E3a-6。削除済みの案内のときは 0)。木を組むまでは null で、出さない。
    treeCount: number | null;
    // actions は、画面ごとの操作。
    actions?: Snippet;
  }

  let { search, treeCount, actions }: Props = $props();

  const view = $derived(search.state);
  const rows = $derived($view.result?.rows ?? []);

  function choose(id: string) {
    const next = layoutChoices.find((c) => c.id === id);
    if (next) search.setLayout(next.id);
  }
</script>

{#snippet lead()}
  <LayoutSwitch choices={layoutChoices} value={$view.layout} onchange={choose} />
{/snippet}

{#snippet end()}
  {#if actions}
    <div class="actions">{@render actions()}</div>
  {/if}
{/snippet}

{#if $view.layout === "tree"}
  <div class="toolbar">
    {@render lead()}
    {#if treeCount !== null}
      <span class="count">{treeCount} 件</span>
    {/if}
    {@render end()}
  </div>
{:else}
  <TableToolbar
    {lead}
    count={$view.result ? rows.length : null}
    columns={$view.result?.columns ?? []}
    table={$view.table}
    onchange={(next) => search.setTable(next)}
  >
    {@render end()}
  </TableToolbar>
{/if}

<style>
  .toolbar {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 8px 24px;
    min-height: 44px;
  }
  .count {
    color: var(--muted);
    font-size: 12.5px;
    font-variant-numeric: tabular-nums;
  }
  .actions {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-left: auto;
  }
  @container frame (max-width: 860px) {
    .toolbar {
      padding-left: max(16px, env(safe-area-inset-left, 0px));
      padding-right: max(16px, env(safe-area-inset-right, 0px));
    }
  }
</style>
