<script lang="ts">
  import {
    currentSprintCard,
    newSprintForm,
    type PlanningOverview,
    type SprintCard,
    type SprintDialogKind,
    sprintForm,
  } from "@hedgehug/core";
  import { sprints } from "../../planning";
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
    // card は、開いたときのカード(作成は null)。
    card: SprintCard | null;
    overview: PlanningOverview | null;
    // today は、このマシンの時刻帯での今日(YYYY-MM-DD)。
    today: string;
    onclose: () => void;
  }

  let { storeId, workspaceId, kind, card, overview, today, onclose }: Props = $props();

  // 開始・完了・削除は、いまのカードで出す(保存の失敗のあとの読み直しなどで、件数や未完了が変わる)。
  // スプリントがなくなったら閉じる。編集は、開いたときの値から入力を始めるので、開いたときのカードのまま。
  const loaded = sprints.state;
  const live = $derived(
    card && kind !== "edit"
      ? currentSprintCard(
          $loaded && $loaded.storeId === storeId && $loaded.workspaceId === workspaceId
            ? $loaded.cards
            : undefined,
          card.sprint.id,
        )
      : card,
  );
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
{:else if kind === "start" && live}
  <StartSprintDialog {storeId} {workspaceId} card={live} {today} {onclose} />
{:else if kind === "close" && live && overview}
  <CloseSprintDialog {storeId} {workspaceId} card={live} {overview} {onclose} />
{:else if kind === "delete" && live && overview}
  <DeleteSprintDialog
    {storeId}
    {workspaceId}
    card={live}
    retentionDays={overview.retentionDays}
    {onclose}
  />
{/if}
