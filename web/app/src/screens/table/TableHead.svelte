<script lang="ts">
  import { type SearchColumn, type TableSettings, updatedHint } from "@hedgehug/core";

  // テーブルの見出しの行(04「並べ替え」、E2a-12)。並べ替えられる列はボタンで、押すと onsort(昇順 → 降順 → なし)。
  interface Props {
    columns: SearchColumn[];
    sort: TableSettings["sort"];
    onsort: (key: string) => void;
  }

  let { columns, sort, onsort }: Props = $props();

  function ariaSort(key: string) {
    if (sort?.key !== key) return "none";
    return sort.desc ? "descending" : "ascending";
  }
</script>

<thead>
  <tr>
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
