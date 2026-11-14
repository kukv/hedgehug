<script lang="ts">
  import {
    asOptions,
    type BacklogFrame,
    type BacklogRow,
    chosen,
    deletedValue,
    type Member,
    moveDown,
    moveUp,
    type PlanningOverview,
    readOnlyNote,
    sprintChoices,
    type WorkspaceSettings,
  } from "@hedgehug/core";
  import { Avatar, type DropTarget, dragRows, IconButton, KindIcon, Select } from "@hedgehug/ui";
  import { backlog } from "../../planning";
  import { links } from "../../wire";
  import StatusButton from "../item/StatusButton.svelte";
  import PointsInput from "./PointsInput.svelte";

  // バックログの画面の行(05「行」、F2a-8・F2a-16)。持ち手・種類・ID・タイトル・優先度・ステータス・ポイント・
  // 担当者・スプリント・「上へ」「下へ」。閲覧のみの Store では、タイトル以外の操作を無効にする。
  interface Props {
    row: BacklogRow;
    // sprint は、行のある枠のスプリントの ID(バックログは空)。
    sprint: string;
    frames: BacklogFrame[];
    storeName: string;
    settings: WorkspaceSettings;
    members: Member[];
    overview: PlanningOverview | null;
    readOnly: boolean;
    // marked は、ドラッグのドロップ先がこの行の直前のとき(行の上に線を出す)。
    marked: boolean;
    onhover: (target: DropTarget | null) => void;
    ondrop: (target: DropTarget) => void;
  }

  let {
    row,
    sprint,
    frames,
    storeName,
    settings,
    members,
    overview,
    readOnly,
    marked,
    onhover,
    ondrop,
  }: Props = $props();

  const note = $derived(readOnly ? readOnlyNote : undefined);
  const route = $derived({ name: "item" as const, store: storeName, item: row.display });
  const priority = $derived(
    settings.priorities.find((p) => p.id === row.priority)?.name ?? deletedValue,
  );
  const member = $derived(
    row.assignee
      ? members.find((m) => m.login.toLowerCase() === row.assignee.toLowerCase())
      : undefined,
  );
  // スプリントの選択肢は、「バックログ」と完了していないスプリント(概要を読むまでは出さない)。
  const choices = $derived(overview ? sprintChoices(overview, sprint) : []);
  const current = $derived(chosen(choices, sprint));
  let selected = $state("");
  $effect(() => {
    selected = current;
  });
  // 保存のあとは、いまの値に戻す(断られて読み直しても値が変わらないと、上の $effect は走り直さない)。
  async function pickSprint(id: string) {
    await backlog.setSprint(row, id);
    selected = current;
  }
  const up = $derived(moveUp(frames, row.id));
  const down = $derived(moveDown(frames, row.id));
</script>

<li class="row" class:marked data-row={row.id}>
  <button
    type="button"
    class="handle"
    aria-label="ドラッグの持ち手"
    disabled={readOnly}
    title={note}
    use:dragRows={{ itemId: row.id, disabled: readOnly, ondrop, onhover }}
  >
    ⠿
  </button>
  <KindIcon kind={row.kind} />
  <code class="id">{row.display}</code>
  <a
    class="row-title"
    href={links.href(route)}
    onclick={(e) => links.click(e, route, () => backlog.openDetail(row))}
    onauxclick={(e) => links.click(e, route)}
    >{row.title}</a
  >
  <span class="priority" class:deleted={priority === deletedValue}>{priority}</span>
  <StatusButton
    status={row.status}
    {settings}
    project={row.project}
    disabled={readOnly}
    onchange={(id) => void backlog.setStatus(row, id)}
  />
  <PointsInput {row} disabled={readOnly} />
  <span class="assignee">
    {#if row.assignee}
      <Avatar name={member?.name ?? ""} title={member?.name ?? "不明なユーザー"} />
    {/if}
  </span>
  <Select
    label="スプリント"
    size="sm"
    disabled={readOnly || !overview}
    title={note}
    options={asOptions(choices)}
    bind:value={selected}
    onchange={(id) => void pickSprint(id)}
  />
  <IconButton
    label="上へ"
    size="sm"
    disabled={readOnly || !up}
    title={note ?? "上へ"}
    onclick={() => up && void backlog.place(row, up)}
    >↑</IconButton
  >
  <IconButton
    label="下へ"
    size="sm"
    disabled={readOnly || !down}
    title={note ?? "下へ"}
    onclick={() => down && void backlog.place(row, down)}
    >↓</IconButton
  >
</li>

<style>
  .row {
    display: flex;
    align-items: center;
    gap: 8px;
    min-height: 36px;
    border-top: 2px solid transparent;
    border-bottom: 1px solid var(--line);
  }
  .marked {
    border-top-color: var(--accent);
  }
  .handle {
    touch-action: none;
    cursor: grab;
    color: var(--muted);
    background: none;
    border: none;
    padding: 0 2px;
  }
  .handle:disabled {
    cursor: not-allowed;
    opacity: 0.45;
  }
  .id {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--muted);
  }
  .row-title {
    flex: 1;
    min-width: 0;
    color: var(--ink);
    text-decoration: none;
    overflow-wrap: anywhere;
  }
  .priority {
    font-size: 12px;
    white-space: nowrap;
  }
  .deleted {
    color: var(--muted);
  }
  .assignee {
    display: inline-flex;
    width: 24px;
  }
</style>
