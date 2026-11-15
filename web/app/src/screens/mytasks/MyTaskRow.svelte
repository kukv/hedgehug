<script lang="ts">
  import {
    type Category,
    deletedNote,
    dueLabel,
    type SearchRow,
    setRowStatus,
  } from "@hedgehug/core";
  import { KindIcon, type KindName, StatusDot } from "@hedgehug/ui";
  import { today } from "../../clock";
  import { myTaskBase, myTaskSettings, myTaskSettingsLoaded, myTasks } from "../../myTasks";
  import { detail, itemsPort, links, toasts } from "../../wire";
  import StatusButton from "../item/StatusButton.svelte";

  // マイタスクの 1 行(04「マイタスク」)。種類・ID・タイトル・Store とプロジェクト・期限・優先度・ステータスを出す。
  // ステータスは、行のワークスペースの設定を読めたらボタン(閲覧のみは無効)、読めるまでと削除済みの行は名前の文字にする
  // (削除済みの行は削除の注記を添えるので、ボタンの「閲覧のみの Store です」を出さない)。
  interface Props {
    row: SearchRow;
  }

  let { row }: Props = $props();

  const category = $derived(row.status?.category as Category | undefined);
  const due = $derived(dueLabel(row.end, category, $today));
  const route = $derived({ name: "item" as const, store: row.storeName, item: row.display });
  // 設定を読み終えるたびに引き直す(myTaskSettings 自体は描き直しを知らせない)。
  const settings = $derived.by(() => {
    void $myTaskSettingsLoaded;
    return myTaskSettings(row.storeId, row.workspaceId);
  });

  function onstatus(statusId: string) {
    void setRowStatus(
      {
        items: itemsPort,
        toasts,
        reload: async () => {
          await Promise.all([myTasks.reload(), myTaskBase.reload()]);
        },
      },
      row.storeId,
      row.itemId,
      statusId,
    );
  }
</script>

<li class="row" class:deleted={row.deleted !== null}>
  <KindIcon kind={row.kind as KindName} />
  <code class="id">{row.display}</code>
  <span class="title-cell">
    {#if row.deleted}
      <span class="title">{row.title}</span>
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
  </span>
  <span class="project">
    <i
      class="sdot"
      style:background="var(--t-{row.storeColor}-fg)"
      role="img"
      aria-label={row.storeName}
      title={row.storeName}
    ></i>
    {row.project.title}
  </span>
  {#if due}
    <span
      class="meta"
      class:overdue={due.emphasis === "overdue"}
      class:today={due.emphasis === "today"}
      >{due.text}</span
    >
  {/if}
  {#if row.priority}
    <span class="meta">{row.priority.name}</span>
  {/if}
  {#if settings && row.deleted === null}
    <StatusButton
      status={row.statusId}
      {settings}
      project={row.projectId}
      disabled={row.readOnly}
      onchange={onstatus}
    />
  {:else if row.status}
    <span class="meta with-icon">
      <StatusDot category={category ?? "todo"} label={row.status.name} />{row.status.name}
    </span>
  {/if}
</li>

<style>
  .row {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 6px 10px;
    padding: 6px 0;
    border-bottom: 1px solid var(--line);
  }
  .deleted {
    opacity: 0.55;
  }
  .id {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--muted);
  }
  .title-cell {
    flex: 1;
    min-width: 160px;
  }
  .title {
    color: var(--ink);
    text-decoration: none;
  }
  a.title:hover {
    text-decoration: underline;
  }
  .note {
    margin-left: 6px;
    font-size: 12px;
    color: var(--muted);
  }
  .project {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
    color: var(--muted);
  }
  .sdot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    flex: none;
  }
  .meta {
    font-size: 12px;
    color: var(--muted);
    white-space: nowrap;
  }
  .overdue {
    color: var(--danger);
  }
  .today {
    font-weight: 600;
  }
  .with-icon {
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }
</style>
