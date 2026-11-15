<script lang="ts">
  import {
    allReadOnly,
    deletedTreeText,
    emptyText,
    nextSort,
    readOnlyNote,
    type Search,
    type SearchTreeView,
    selectedText,
    shownColumns,
    treeEmptyText,
  } from "@hedgehug/core";
  import { Button, SelectionBar } from "@hedgehug/ui";
  import type { Snippet } from "svelte";
  import { openRowMenu, searchSelection } from "../../search";
  import { itemDialog } from "../../wire";
  import { bulkChange, bulkDelete } from "../../wireActions";
  import ItemTable from "../table/ItemTable.svelte";
  import SearchTree from "../tree/SearchTree.svelte";
  import SearchToolbar from "./SearchToolbar.svelte";

  // 検索の画面の本体(04「検索」)。ツールバーと、レイアウトがツリーならツリー(E3a)、そうでなければテーブル(E2a)を出す。
  // どちらも、空のときは本体の代わりに文を出す。ツリーで is:deleted の検索をしたときは、ツリーの代わりに案内を出す(E3a-7)。
  interface Props {
    search: Search;
    // actions は、画面ごとの操作。ツールバーの右端に置く。
    actions?: Snippet;
  }

  let { search, actions }: Props = $props();

  const view = $derived(search.state);
  const rows = $derived($view.result?.rows ?? []);
  const errors = $derived($view.result?.errors ?? []);
  const columns = $derived($view.result?.columns ?? []);
  // deleted は、ツリーで削除済みの案内を出すとき(書いてあるかは応答から決める。画面は解析しない)。
  const deleted = $derived($view.layout === "tree" && ($view.result?.deleted ?? false));
  // 最初の検索が終わるまでは、空の表示を出さない(開いた直後に「まだ何もありません」がちらつかない)。
  const empty = $derived($view.result ? emptyText($view.query, rows.length, errors) : null);
  const picked = searchSelection.rows;
  // tree は、ツリーが描いている木。組むまでは件数も空の文も出さない。空の文の書き方の誤りは、いまの結果ではなく
  // 木を組んだ結果のもの(木を組み直すまでは、描いている木と文を食い違わせない)。
  let tree = $state<(SearchTreeView & { errors: string[] }) | null>(null);
  const treeEmpty = $derived(
    tree
      ? treeEmptyText({ hasProjects: tree.hasProjects, rows: tree.nodes.length }, tree.errors)
      : null,
  );
  // ツリーを出していないあいだは、描いた木を捨てる(また出したとき、組み直すまで前の件数や空の文を出さない)。
  $effect(() => {
    if ($view.layout !== "tree" || deleted) tree = null;
  });
</script>

<SearchToolbar {search} {actions} treeCount={deleted ? 0 : (tree?.matched ?? null)} />

{#if deleted}
  <div class="content">
    <div class="empty">
      <p class="empty-note">{deletedTreeText}</p>
      <Button size="sm" onclick={() => search.setLayout("table")}>テーブルで開く</Button>
    </div>
  </div>
{:else if $view.layout === "tree"}
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
    <SearchTree {search} bind:view={tree} />
  </div>
{:else}
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
    .content {
      padding-left: max(16px, env(safe-area-inset-left, 0px));
      padding-right: max(16px, env(safe-area-inset-right, 0px));
    }
  }
</style>
