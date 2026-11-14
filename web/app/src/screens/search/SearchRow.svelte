<script lang="ts">
  import {
    assigneeText,
    type Category,
    deletedNote,
    dueLabel,
    type SearchRow,
  } from "@hedgehug/core";
  import { Avatar, StatusDot } from "@hedgehug/ui";
  import { detail, links } from "../../wire";

  // 検索のテーブルの 1 行(04「テーブル」、E1-8・E1-18)。削除済みの行は薄く出し、タイトルを押せない文字にする。
  interface Props {
    row: SearchRow;
    today: Date;
  }

  let { row, today }: Props = $props();

  const category = $derived(row.status?.category as Category | undefined);
  const due = $derived(dueLabel(row.end, category, today));
  const who = $derived(assigneeText(row.assignee));
  const route = $derived({ name: "item" as const, store: row.storeName, item: row.display });
</script>

<tr class="row" class:deleted={row.deleted !== null}>
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
  <td>
    {#if row.status}
      <span class="with-icon">
        <StatusDot category={category ?? "todo"} label={row.status.name} />{row.status.name}
      </span>
    {/if}
  </td>
  <td>
    {#if who !== null}
      <span class="with-icon">
        <Avatar name={row.assignee?.name ?? ""} title={who} />{who}
      </span>
    {/if}
  </td>
  <td>{row.priority?.name ?? ""}</td>
  <td>
    {#if due}
      <span
        class="due"
        class:overdue={due.emphasis === "overdue"}
        class:today={due.emphasis === "today"}
        >{due.text}</span
      >
    {/if}
  </td>
  <td class="num">{row.points ?? ""}</td>
</tr>

<style>
  td {
    padding: 6px 8px;
    border-bottom: 1px solid var(--line);
    vertical-align: middle;
    white-space: nowrap;
  }
  .deleted td {
    opacity: 0.55;
  }
  .sdot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    flex: none;
  }
  .id {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--muted);
  }
  .old {
    font-size: 11px;
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
  .due {
    color: var(--muted);
    font-size: 12px;
  }
  .overdue {
    color: var(--danger);
  }
  .today {
    font-weight: 600;
  }
  .num {
    text-align: right;
    font-variant-numeric: tabular-nums;
  }
</style>
