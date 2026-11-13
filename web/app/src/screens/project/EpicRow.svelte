<script lang="ts">
  import {
    categoryOf,
    dueLabel,
    epicProgress,
    type Item,
    type Member,
    progressPercent,
    type WorkspaceSettings,
  } from "@hedgehug/core";
  import { Avatar, KindIcon, ProgressBar } from "@hedgehug/ui";
  import StatusButton from "../item/StatusButton.svelte";

  // 概要の Epic の 1 行(02「概要の編集」の Epic の一覧)。進捗は Epic の下の Story・Task・Bug(Step と中止を除く)。
  interface Props {
    epic: Item;
    displayId: string;
    href: string;
    items: Item[];
    settings: WorkspaceSettings;
    members: Member[];
    today: Date;
    readOnly: boolean;
    onopen: (event: MouseEvent) => void;
    onauxopen: (event: MouseEvent) => void;
    onstatus: (statusId: string) => void;
  }

  let {
    epic,
    displayId,
    href,
    items,
    settings,
    members,
    today,
    readOnly,
    onopen,
    onauxopen,
    onstatus,
  }: Props = $props();

  const progress = $derived(epicProgress(items, epic.id, settings));
  const due = $derived(dueLabel(epic.end, categoryOf(settings, epic.status), today));
  const member = $derived(
    members.find((m) => m.login.toLowerCase() === epic.assignee.toLowerCase()),
  );
</script>

<div class="list-row">
  <KindIcon kind="Epic" />
  <code class="id">{displayId}</code>
  <a class="row-title" {href} onclick={onopen} onauxclick={onauxopen}>{epic.title}</a>
  {#if due}
    <span
      class="due"
      class:overdue={due.emphasis === "overdue"}
      class:today={due.emphasis === "today"}
      >{due.text}</span
    >
  {/if}
  <ProgressBar percent={progressPercent(progress)} detail="{progress.done} / {progress.total}" />
  <StatusButton status={epic.status} {settings} disabled={readOnly} onchange={onstatus} />
  {#if epic.assignee}
    <Avatar name={member?.name ?? ""} title={member?.name ?? "不明なユーザー"} />
  {/if}
</div>

<style>
  .list-row {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
    min-height: 34px;
    padding: 2px 0;
    border-bottom: 1px solid var(--line);
  }
  .id {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--muted);
  }
  .row-title {
    color: var(--ink);
    text-decoration: none;
    flex: 1 1 10em;
    min-width: 0;
  }
  .due {
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
</style>
