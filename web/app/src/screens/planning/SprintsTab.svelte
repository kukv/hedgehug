<script lang="ts">
  import {
    type SprintCard as Card,
    dayOf,
    readOnlyNote,
    sections,
    startDisabledReason,
  } from "@hedgehug/core";
  import { Button } from "@hedgehug/ui";
  import { tick } from "svelte";
  import { planRequest } from "../../palette";
  import { sprints } from "../../planning";
  import { planning, syncNow } from "../../wire";
  import SprintCard from "./SprintCard.svelte";

  // ワークスペースの「スプリント」のタブ(05「スプリントの画面」)。作成・開始・完了・編集・削除の
  // ダイアログは親が渡す(Task 16)。
  interface Props {
    storeId: string;
    storeName: string;
    workspaceId: string;
    readOnly: boolean;
    oncreate: () => void;
    onstart: (card: Card) => void;
    onclose: (card: Card) => void;
    onedit: (card: Card) => void;
    ondelete: (card: Card) => void;
  }

  let {
    storeId,
    storeName,
    workspaceId,
    readOnly,
    oncreate,
    onstart,
    onclose,
    onedit,
    ondelete,
  }: Props = $props();

  // タブを開いているあいだだけ読む。Store の出来事で読み直す(planning.ts)。
  // 親が Store の読み直しで作り直した値を渡しても、ID が同じなら開き直さない(開き直すとカードが消え、
  // 開いているダイアログが閉じる)。
  const sid = $derived(storeId);
  const wid = $derived(workspaceId);
  $effect(() => {
    void sprints.open(sid, wid);
    return () => sprints.close();
  });
  // パレットの「スプリントを作成」の要求が、このワークスペースのものなら作成ダイアログを開く(A5c-11)。
  // 画面を移って出たばかりのときは、親(WorkspaceScreen)が概要を開く前なので、描き終わってから開く。
  const request = planRequest.request;
  $effect(() => {
    if ($request && planRequest.take(wid, "sprint")) void tick().then(oncreate);
  });
  const loaded = sprints.state;
  const overview = planning.state;
  const view = $derived(
    $loaded && $loaded.storeId === storeId && $loaded.workspaceId === workspaceId ? $loaded : null,
  );
  // 「開始」の理由は、見出しの「実行中」と同じ概要から決める(WorkspaceScreen が開いている)。
  // 概要をまだ読めていないあいだは、見えているカードで決める。
  const startReason = $derived(
    startDisabledReason(
      $overview && $overview.storeId === storeId && $overview.workspaceId === workspaceId
        ? $overview.overview
        : null,
      view?.cards,
    ),
  );
  // 日付は、時刻の更新(30 秒ごと)で読み直す。日付が変わると「残り N 日」が変わる(01「日付と時刻」)。
  const today = $derived(dayOf($syncNow));
  const thisYear = $derived($syncNow.getFullYear());
  const note = $derived(readOnly ? readOnlyNote : undefined);
</script>

<div class="top">
  <Button variant="primary" disabled={readOnly} title={note} onclick={oncreate}
    >スプリントを作成</Button
  >
</div>
{#if view}
  {#if view.cards.length === 0}
    <div class="empty">
      <p class="title">スプリントはまだありません</p>
      <p>期間を決めて Story・Task・Bug をまとめると、バーンダウンとベロシティが見られます。</p>
      <Button variant="primary" disabled={readOnly} title={note} onclick={oncreate}
        >スプリントを作成</Button
      >
    </div>
  {:else}
    {#each sections(view.cards) as sec (sec.state)}
      <section class="section" aria-label={sec.label}>
        <h2>{sec.label}</h2>
        {#each sec.cards as card (card.sprint.id)}
          <SprintCard
            {card}
            {storeId}
            {storeName}
            settings={view.settings}
            members={view.members}
            {readOnly}
            {startReason}
            {today}
            {thisYear}
            {onstart}
            {onclose}
            {onedit}
            {ondelete}
          />
        {/each}
      </section>
    {/each}
  {/if}
{/if}

<style>
  .top {
    display: flex;
    justify-content: flex-end;
    margin-bottom: 8px;
  }
  .section {
    display: flex;
    flex-direction: column;
    gap: 10px;
    margin-bottom: 20px;
  }
  h2 {
    font-size: 13px;
    color: var(--muted);
    margin: 0;
  }
  .empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 8px;
    padding: 48px 16px;
    text-align: center;
    color: var(--muted);
  }
  .empty p {
    margin: 0;
  }
  .empty .title {
    font-size: 15px;
    color: var(--ink);
  }
</style>
