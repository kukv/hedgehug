<script lang="ts">
  import {
    isSelected,
    type RowSelection,
    type SearchColumn,
    type SearchGroup,
    type SearchRow,
    type TableSettings,
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
    // selection は行のチェックと「すべて選択」(E2a-16)、onmenu は行末の「⋯」(E2a-17)。
    selection: RowSelection;
    onmenu: (row: SearchRow, rect: DOMRect) => void;
  }

  let { rows, columns, groups, sort, onsort, selection, onmenu }: Props = $props();
  const picked = $derived(selection.rows);
  // 終了日の「期限切れ」「今日」と相対時間は、結果が届いたときの時刻で決める(読み直した行の「N 分前」がずれない)。
  const now = $derived.by(() => {
    void rows;
    return new Date();
  });
</script>

<!-- 同じアイテムが複数のグループに出ても、チェックは Store とアイテムの内部 ID で 1 つの選択を指す -->
{#snippet line(row: SearchRow)}
  <TableRow
    {row}
    {columns}
    {now}
    checked={isSelected($picked, row.storeId, row.itemId)}
    oncheck={(on) =>
      selection.toggle(
        { storeId: row.storeId, itemId: row.itemId, display: row.display, readOnly: row.readOnly },
        on,
      )}
    onmenu={(rect) => onmenu(row, rect)}
  />
{/snippet}

<table class="table">
  <TableHead {rows} {selection} {columns} {sort} {onsort} />
  {#if groups.length > 0}
    <!-- 同じアイテムが複数のグループに出るので、グループの位置と行の位置で見分ける -->
    {#each groups as group, g (g)}
      <tbody>
        <tr class="group">
          <!-- 先頭のチェックと末尾の「⋯」の列も含める -->
          <th colspan={columns.length + 2} scope="rowgroup">
            {group.label}
            <span class="count">{group.rows.length}</span>
          </th>
        </tr>
        {#each group.rows as index (`${g}/${index}`)}
          {@const row = rows[index]}
          {#if row}
            {@render line(row)}
          {/if}
        {/each}
      </tbody>
    {/each}
  {:else}
    <tbody>
      <!-- 同じ ID が別の Store にもあるので、Store とアイテムの内部 ID で見分ける -->
      {#each rows as row (`${row.storeId}/${row.itemId}`)}
        {@render line(row)}
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
