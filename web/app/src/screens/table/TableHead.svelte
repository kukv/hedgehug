<script lang="ts">
  import {
    type RowSelection,
    type SearchColumn,
    type SearchRow,
    type TableSettings,
    updatedHint,
  } from "@hedgehug/core";
  import SelectAllCheck from "./SelectAllCheck.svelte";

  // テーブルの見出しの行(04「並べ替え」、E2a-12)。並べ替えられる列はボタンで、押すと onsort(昇順 → 降順 → なし)。
  // 先頭は「すべて選択」、末尾は行の「⋯」の列の見出し(読み上げだけの「メニュー」)(E2a-16・17)。
  interface Props {
    rows: SearchRow[];
    selection: RowSelection;
    columns: SearchColumn[];
    sort: TableSettings["sort"];
    onsort: (key: string) => void;
  }

  let { rows, selection, columns, sort, onsort }: Props = $props();

  function ariaSort(key: string) {
    if (sort?.key !== key) return "none";
    return sort.desc ? "descending" : "ascending";
  }
</script>

<thead>
  <tr>
    <th scope="col" class="edge"><SelectAllCheck {rows} {selection} /></th>
    {#each columns as column (column.key)}
      <th
        scope="col"
        aria-sort={column.sortable ? ariaSort(column.key) : undefined}
        title={column.key === "updated" ? updatedHint : undefined}
      >
        {#if column.sortable}
          <button type="button" class="sort" onclick={() => onsort(column.key)}>
            {column.name}
            {#if sort?.key === column.key}
              <span class="arrow" aria-hidden="true">{sort.desc ? "↓" : "↑"}</span>
            {/if}
          </button>
        {:else}
          {column.name}
        {/if}
      </th>
    {/each}
    <th scope="col" class="edge"><span class="sr-only">メニュー</span></th>
  </tr>
</thead>

<style>
  th {
    position: sticky;
    top: 0;
    z-index: 1;
    background: var(--surface);
    color: var(--muted);
    font-size: 12px;
    font-weight: 600;
    text-align: left;
    white-space: nowrap;
    padding: 6px 8px;
    border-bottom: 1px solid var(--line);
  }
  .edge {
    width: 1%;
  }
  .sr-only {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip-path: inset(50%);
    white-space: nowrap;
  }
  .sort {
    border: 0;
    background: none;
    padding: 0;
    color: inherit;
    font: inherit;
    cursor: pointer;
  }
  .sort:hover {
    color: var(--ink);
  }
  .arrow {
    margin-left: 2px;
  }
</style>
