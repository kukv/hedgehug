<script lang="ts">
  import type {
    RowSelection,
    SearchColumn,
    SearchGroup,
    SearchRow,
    TableSettings,
  } from "@hedgehug/core";
  import TableHead from "./TableHead.svelte";
  import TableRow from "./TableRow.svelte";

  // アイテムのテーブル(04「テーブル」、E2a-12〜15)。検索と、ワークスペース・プロジェクトのタブで使う。
  // グループがあれば、グループごとに見出しの行(名前と件数)と、その行を描く。
  interface Props {
    rows: SearchRow[];
    // columns は出す列(並びどおり)。
    columns: SearchColumn[];
    groups: SearchGroup[];
    sort: TableSettings["sort"];
    onsort: (key: string) => void;
    // selection と onmenu は、行のチェックと「…」のメニューでつなぐ(E2a Task 9)。
    selection: RowSelection;
    onmenu: (row: SearchRow, rect: DOMRect) => void;
  }

  let { rows, columns, groups, sort, onsort }: Props = $props();
  // 終了日の「期限切れ」「今日」と相対時間は、画面を開いたときの時刻で決める(ツリーと同じ)。
  const now = new Date();
</script>

<table class="table">
  <TableHead {columns} {sort} {onsort} />
  {#if groups.length > 0}
    <!-- 同じアイテムが複数のグループに出るので、グループの位置と行の位置で見分ける -->
    {#each groups as group, g (g)}
      <tbody>
        <tr class="group">
          <th colspan={columns.length} scope="rowgroup">
            {group.label}
            <span class="count">{group.rows.length}</span>
          </th>
        </tr>
        {#each group.rows as index (`${g}/${index}`)}
          {@const row = rows[index]}
          {#if row}
            <TableRow {row} {columns} {now} />
          {/if}
        {/each}
      </tbody>
    {/each}
  {:else}
    <tbody>
      <!-- 同じ ID が別の Store にもあるので、Store とアイテムの内部 ID で見分ける -->
      {#each rows as row (`${row.storeId}/${row.itemId}`)}
        <TableRow {row} {columns} {now} />
      {/each}
    </tbody>
  {/if}
</table>

<style>
  .table {
    width: 100%;
    border-collapse: collapse;
    font-size: 13px;
  }
  .group th {
    text-align: left;
    font-size: 12.5px;
    font-weight: 600;
    padding: 12px 8px 6px;
    border-bottom: 1px solid var(--line);
  }
  .count {
    margin-left: 6px;
    color: var(--muted);
    font-weight: 400;
    font-variant-numeric: tabular-nums;
  }
</style>
