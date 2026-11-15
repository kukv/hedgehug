<script lang="ts">
  import {
    cellView,
    deletedNote,
    type SearchColumn,
    type SearchRow,
    type TableCells,
  } from "@hedgehug/core";
  import { IconButton, KindIcon, type KindName, RowCheck } from "@hedgehug/ui";
  import { detail, links } from "../../wire";
  import TableCell from "./TableCell.svelte";
  import TableEditCell from "./TableEditCell.svelte";
  import TableIdCell from "./TableIdCell.svelte";

  // テーブルの 1 行(04「テーブル」「セル」、E1-8・E2a-12・E2b)。ID・種類・タイトル・親はここで描き、
  // 編集できる列(ステータス・担当者・優先度・日付・数・関連システム・スプリント・リリース・追加プロパティ)は TableEditCell、
  // ほかの列は TableCell が cellView で描く。削除済みの行は薄く出し、タイトルを押せない文字にする。
  // 先頭は選択のチェック、末尾は 02 のアイテムのメニューを開く「⋯」(E2a-16・17)。削除済みの行はどちらも押せない。
  interface Props {
    row: SearchRow;
    columns: SearchColumn[];
    now: Date;
    checked: boolean;
    oncheck: (on: boolean) => void;
    // onmenu の rect は「⋯」の getBoundingClientRect()。
    onmenu: (rect: DOMRect) => void;
    cells: TableCells;
    reload: () => Promise<void>;
  }

  let { row, columns, now, checked, oncheck, onmenu, cells, reload }: Props = $props();

  const editKeys = new Set([
    "status",
    "assignee",
    "priority",
    "start",
    "end",
    "points",
    "estimate",
    "system",
    "sprint",
    "release",
  ]);
  const editable = (column: SearchColumn) =>
    editKeys.has(column.key) || column.key.startsWith("p:");
  const route = $derived({ name: "item" as const, store: row.storeName, item: row.display });
</script>

<tr class="row" class:deleted={row.deleted !== null}>
  <td class="edge">
    <RowCheck
      label={`${row.display} を選択`}
      {checked}
      onchange={oncheck}
      disabled={row.deleted !== null}
    />
  </td>
  {#each columns as column (column.key)}
    {#if column.key === "id"}
      <TableIdCell {row} />
    {:else if column.key === "kind"}
      <td>
        <span class="with-icon"><KindIcon kind={row.kind as KindName} />{row.kind}</span>
      </td>
    {:else if column.key === "title"}
      <td class="title-cell">
        {#if row.deleted}
          <span class="title">{row.title}</span>
          <span class="pill">削除済み</span>
          <span class="note">{deletedNote(row.deleted.daysLeft)}</span>
        {:else}
          <a
            class="title"
            href={links.href(route)}
            onclick={(e) => links.click(e, route, () => void detail.open(row.storeId, row.itemId))}
            onauxclick={(e) => links.click(e, route)}
            >{row.title}</a
          >
        {/if}
      </td>
    {:else if column.key === "parent"}
      <td>
        {#if row.parent}
          {@const parent = row.parent}
          <button
            type="button"
            class="parent"
            title={parent.title}
            disabled={row.deleted !== null}
            onclick={() => void detail.open(row.storeId, parent.itemId)}
          >
            {parent.display}
          </button>
        {/if}
      </td>
    {:else if editable(column)}
      <TableEditCell {row} {column} {now} {cells} {reload} />
    {:else}
      <TableCell view={cellView(row, column, now)} {column} />
    {/if}
  {/each}
  <td class="edge">
    <IconButton
      label="{row.display} のメニュー"
      size="xs"
      disabled={row.deleted !== null}
      onclick={(e) => onmenu((e.currentTarget as HTMLElement).getBoundingClientRect())}
      >⋯</IconButton
    >
  </td>
</tr>

<style>
  td {
    padding: 6px 8px;
    border-bottom: 1px solid var(--line);
    vertical-align: middle;
    white-space: nowrap;
  }
  .deleted :global(td) {
    opacity: 0.55;
  }
  .edge {
    width: 1%;
  }
  .parent {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--muted);
    border: 0;
    background: none;
    padding: 0;
    cursor: pointer;
  }
  .parent:disabled {
    cursor: default;
  }
  .parent:enabled:hover {
    text-decoration: underline;
  }
  .title-cell {
    white-space: normal;
    min-width: 200px;
  }
  .title {
    color: var(--ink);
    text-decoration: none;
  }
  a.title:hover {
    text-decoration: underline;
  }
  .pill {
    margin-left: 8px;
    font-size: 11px;
    padding: 1px 7px;
    border-radius: 999px;
    background: var(--c-red-bg);
    color: var(--c-red-fg);
  }
  .note {
    margin-left: 6px;
    font-size: 12px;
    color: var(--muted);
  }
  .with-icon {
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }
</style>
