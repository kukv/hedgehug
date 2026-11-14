<script lang="ts">
  import {
    assigneeText,
    type Category,
    cellView,
    deletedNote,
    deletedValue,
    type SearchColumn,
    type SearchRow,
  } from "@hedgehug/core";
  import { Avatar, KindIcon, type KindName, StatusDot } from "@hedgehug/ui";
  import { detail, links } from "../../wire";
  import TableCell from "./TableCell.svelte";

  // テーブルの 1 行(04「テーブル」「セル」、E1-8・E2a-12)。ID・種類・タイトル・ステータス・担当者・親はここで描き、
  // ほかの列は TableCell が cellView で描く。削除済みの行は薄く出し、タイトルを押せない文字にする。
  interface Props {
    row: SearchRow;
    columns: SearchColumn[];
    now: Date;
  }

  let { row, columns, now }: Props = $props();

  const category = $derived(row.status?.category as Category | undefined);
  const who = $derived(assigneeText(row.assignee));
  const route = $derived({ name: "item" as const, store: row.storeName, item: row.display });
</script>

<tr class="row" class:deleted={row.deleted !== null}>
  {#each columns as column (column.key)}
    {#if column.key === "id"}
      <td>
        <span class="with-icon">
          <i
            class="sdot"
            style:background="var(--t-{row.storeColor}-fg)"
            role="img"
            aria-label={row.storeName}
            title={row.storeName}
          ></i>
          <code class="id">{row.display}</code>
          {#if row.oldIdHit}
            <span class="old">旧 ID</span>
          {/if}
        </span>
      </td>
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
    {:else if column.key === "status"}
      <td>
        {#if row.removed.includes("status")}
          <span class="faint">{deletedValue}</span>
        {:else if row.status}
          <span class="with-icon">
            <StatusDot category={category ?? "todo"} label={row.status.name} />{row.status.name}
          </span>
        {/if}
      </td>
    {:else if column.key === "assignee"}
      <td>
        {#if row.unused.includes("assignee")}
          <span class="faint">—</span>
        {:else if who !== null}
          <span class="with-icon"><Avatar name={row.assignee?.name ?? ""} title={who} />{who}</span>
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
            onclick={() => void detail.open(row.storeId, parent.itemId)}
          >
            {parent.display}
          </button>
        {/if}
      </td>
    {:else}
      <TableCell view={cellView(row, column, now)} {column} />
    {/if}
  {/each}
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
  .sdot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    flex: none;
  }
  .id,
  .parent {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--muted);
  }
  .parent {
    border: 0;
    background: none;
    padding: 0;
    cursor: pointer;
  }
  .parent:hover {
    text-decoration: underline;
  }
  .old {
    font-size: 11px;
    color: var(--muted);
  }
  .faint {
    color: var(--muted);
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
