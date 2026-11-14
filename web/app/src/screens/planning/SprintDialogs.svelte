<script lang="ts">
  import {
    currentSprintCard,
    newSprintForm,
    type PlanningOverview,
    type SprintCard,
    type SprintDialogKind,
    sprintForm,
  } from "@hedgehug/core";
  import { backlog, sprints } from "../../planning";
  import CloseSprintDialog from "./CloseSprintDialog.svelte";
  import DeleteSprintDialog from "./DeleteSprintDialog.svelte";
  import SprintDialog from "./SprintDialog.svelte";
  import StartSprintDialog from "./StartSprintDialog.svelte";

  // スプリントのダイアログ(05)。WorkspaceScreen が開いたものを出す。作成・完了・削除は概要
  // (既定・移動先・保持日数)を使うので、親が読んでから渡す。
  interface Props {
    storeId: string;
    workspaceId: string;
    kind: SprintDialogKind;
    // card は、開いたときのカード(作成は null)。バックログの画面からの開始は、スプリントの枠(F2a-15)。
    card: Pick<SprintCard, "sprint" | "total"> | null;
    overview: PlanningOverview | null;
    // today は、このマシンの時刻帯での今日(YYYY-MM-DD)。
    today: string;
    onclose: () => void;
  }

  let { storeId, workspaceId, kind, card, overview, today, onclose }: Props = $props();

  // 開始・完了・削除は、いまのカードで出す(保存の失敗のあとの読み直しなどで、件数や未完了が変わる)。
  // スプリントがなくなったら閉じる。編集は、開いたときの値から入力を始めるので、開いたときのカードのまま。
  // 開始は、バックログのタブから開いたときは、いまのスプリントの枠で出す(スプリントのカードは読んでいない)。
  const loaded = sprints.state;
  const frames = backlog.state;
  const full = $derived(
    card
      ? currentSprintCard(
          $loaded && $loaded.storeId === storeId && $loaded.workspaceId === workspaceId
            ? $loaded.cards
            : undefined,
          card.sprint.id,
        )
      : null,
  );
  const frame = $derived(
    card && $frames && $frames.storeId === storeId && $frames.workspaceId === workspaceId
      ? $frames.frames.find((f) => f.sprint?.id === card.sprint.id)
      : undefined,
  );
  const started = $derived(
    full ?? (frame?.sprint ? { sprint: frame.sprint, total: frame.total } : null),
  );
  const live = $derived(kind === "edit" ? card : kind === "start" ? started : full);
  $effect(() => {
    if (card && !live) onclose();
  });
</script>

{#if kind === "create" && overview}
  <SprintDialog {storeId} {workspaceId} sprint={null} initial={newSprintForm(overview)} {onclose} />
{:else if kind === "edit" && live}
  <SprintDialog
    {storeId}
    {workspaceId}
    sprint={live.sprint}
    initial={sprintForm(live.sprint)}
    {onclose}
  />
{:else if kind === "start" && started}
  <StartSprintDialog {storeId} {workspaceId} card={started} {today} {onclose} />
{:else if kind === "close" && full && overview}
  <CloseSprintDialog {storeId} {workspaceId} card={full} {overview} {onclose} />
{:else if kind === "delete" && full && overview}
  <DeleteSprintDialog
    {storeId}
    {workspaceId}
    card={full}
    retentionDays={overview.retentionDays}
    {onclose}
  />
{/if}
