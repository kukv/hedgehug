<script lang="ts">
  import { dropPlacement, type Sprint, startDisabledReason, type Totals } from "@hedgehug/core";
  import type { DropTarget } from "@hedgehug/ui";
  import { backlog } from "../../planning";
  import { itemDialog, planning, syncNow } from "../../wire";
  import BacklogFrame from "./BacklogFrame.svelte";

  // ワークスペースの「バックログ」のタブ(05「バックログ」)。上にスプリントの枠、下にバックログの枠を並べる。
  // スプリントの作成と開始のダイアログは親が出す(F2a-15)。
  interface Props {
    storeId: string;
    storeName: string;
    workspaceId: string;
    readOnly: boolean;
    oncreate: () => void;
    onstart: (frame: { sprint: Sprint; total: Totals }) => void;
  }

  let { storeId, storeName, workspaceId, readOnly, oncreate, onstart }: Props = $props();

  // タブを開いているあいだだけ読む。Store の出来事で読み直す(planning.ts)。親が Store の読み直しで作り直した
  // 値を渡しても、ID が同じなら開き直さない(開き直すと枠が消え、開いているダイアログが閉じる)。
  const sid = $derived(storeId);
  const wid = $derived(workspaceId);
  $effect(() => {
    void backlog.open(sid, wid);
    return () => backlog.close();
  });
  const loaded = backlog.state;
  const view = $derived(
    $loaded && $loaded.storeId === storeId && $loaded.workspaceId === workspaceId ? $loaded : null,
  );
  // スプリントの選択肢と「開始」の理由は、見出しの「実行中」と同じ概要から決める(WorkspaceScreen が開いている)。
  // 概要をまだ読めていないあいだは、見えている枠で決める。
  const overview = planning.state;
  const ov = $derived(
    $overview && $overview.storeId === storeId && $overview.workspaceId === workspaceId
      ? $overview.overview
      : null,
  );
  const startReason = $derived(
    startDisabledReason(
      ov,
      view?.frames.flatMap((f) => (f.sprint ? [{ sprint: f.sprint }] : [])),
    ),
  );
  const thisYear = $derived($syncNow.getFullYear());

  // hover は、ドラッグ中のドロップ先。同じ位置など、保存しない先では印を出さない(F2a-12)。
  let hover = $state<DropTarget | null>(null);
  function onhover(itemId: string, target: DropTarget | null) {
    hover = target && view && dropPlacement(view.frames, itemId, target) ? target : null;
  }
  function ondrop(itemId: string, target: DropTarget) {
    hover = null;
    const row = view?.frames.flatMap((f) => f.rows).find((r) => r.id === itemId);
    const drop = view && dropPlacement(view.frames, itemId, target);
    if (row && drop) void backlog.place(row, drop);
  }
</script>

{#if view}
  {#each view.frames as frame (frame.sprint?.id ?? "")}
    <BacklogFrame
      {frame}
      frames={view.frames}
      {storeName}
      settings={view.settings}
      members={view.members}
      overview={ov}
      {readOnly}
      {startReason}
      {thisYear}
      {hover}
      {onhover}
      {ondrop}
      {onstart}
      {oncreate}
      onitem={() => void itemDialog.open({ workspaceId, kind: "Story" })}
    />
  {/each}
{/if}
