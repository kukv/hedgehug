<script lang="ts">
  import {
    allReadOnly,
    emptyText,
    nextSort,
    readOnlyNote,
    type SearchTreeView,
    selectedText,
    shownColumns,
    treeEmptyText,
  } from "@hedgehug/core";
  import { Button, SelectionBar } from "@hedgehug/ui";
  import { openRowMenu, search, searchSelection } from "../../search";
  import { itemDialog } from "../../wire";
  import { bulkChange, bulkDelete } from "../../wireActions";
  import ItemTable from "../table/ItemTable.svelte";
  import TableToolbar from "../table/TableToolbar.svelte";
  import SearchTree from "../tree/SearchTree.svelte";

  // 検索の画面の本体(04「検索」)。レイアウトがツリーならツリー(E3a)、そうでなければテーブル(E2a)を出す。
  // どちらも、空のときは本体の代わりに文を出す。
  const view = search.state;
  const rows = $derived($view.result?.rows ?? []);
  const errors = $derived($view.result?.errors ?? []);
  const columns = $derived($view.result?.columns ?? []);
  // 最初の検索が終わるまでは、空の表示を出さない(開いた直後に「まだ何もありません」がちらつかない)。
  const empty = $derived($view.result ? emptyText($view.query, rows.length, errors) : null);
  const picked = searchSelection.rows;
  // tree は、ツリーが描いている木。組むまでは件数も空の文も出さない。
  let tree = $state<SearchTreeView | null>(null);
  const treeEmpty = $derived(
    tree ? treeEmptyText({ hasProjects: tree.hasProjects, rows: tree.nodes.length }, errors) : null,
  );
</script>

{#if $view.layout === "tree"}
  <!-- ツリーには設定がないので、件数だけを出す(E3a-6・7) -->
  <div class="toolbar">
    {#if tree}
      <span class="count">{tree.matched} 件</span>
    {/if}
  </div>
  <div class="content">
    {#if treeEmpty}
      <div class="empty">
        <p class="empty-title">{treeEmpty.title}</p>
        {#if treeEmpty.body}
          <p class="empty-note">{treeEmpty.body}</p>
        {/if}
      </div>
    {/if}
    <!-- 空のあいだも組み直しを続けるので、ツリーは出したままにする(行がなければ何も描かない) -->
    <SearchTree bind:view={tree} />
  </div>
{:else}
  <TableToolbar
    count={$view.result ? rows.length : null}
    {columns}
    table={$view.table}
    onchange={(next) => search.setTable(next)}
  />
  {#if $picked.length > 0}
    <SelectionBar
      text={selectedText($picked.length)}
      readOnlyNote={allReadOnly($picked) ? readOnlyNote : undefined}
      onchange={() => void bulkChange.open(searchSelection, { after: () => search.reload() })}
      ondelete={() => void bulkDelete.open(searchSelection, () => search.reload())}
      onclear={() => searchSelection.clear()}
    />
  {/if}
  <div class="content">
    {#if empty}
      <div class="empty">
        <p class="empty-title">{empty.title}</p>
        {#if empty.body}
          <p class="empty-note">{empty.body}</p>
        {/if}
        {#if empty.create}
          <div class="create">
            <Button variant="primary" size="sm" onclick={() => void itemDialog.open()}>作成</Button>
          </div>
        {/if}
      </div>
    {:else if rows.length > 0}
      <ItemTable
        {rows}
        columns={shownColumns($view.table, columns)}
        groups={$view.result?.groups ?? []}
        sort={$view.table.sort}
        onsort={(key) => search.setTable(nextSort($view.table, key))}
        selection={searchSelection}
        onmenu={(row, rect) => void openRowMenu(row, rect)}
      />
    {/if}
  </div>
{/if}

<style>
  .toolbar {
    display: flex;
    align-items: center;
    padding: 8px 24px;
    min-height: 44px;
  }
  .count {
    color: var(--muted);
    font-size: 12.5px;
  }
  .content {
    flex: 1;
    overflow: auto;
    padding: 0 24px 40px;
    min-height: 0;
  }
  .empty {
    padding: 40px 0;
    text-align: center;
  }
  .empty-title {
    margin: 0;
    font-weight: 500;
  }
  .empty-note {
    margin: 6px auto 12px;
    max-width: 520px;
    color: var(--muted);
    font-size: 12.5px;
  }
  .create {
    margin-top: 12px;
  }
  @container frame (max-width: 860px) {
    .toolbar,
    .content {
      padding-left: max(16px, env(safe-area-inset-left, 0px));
      padding-right: max(16px, env(safe-area-inset-right, 0px));
    }
  }
</style>
