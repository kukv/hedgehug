<script lang="ts">
  import {
    type Member,
    periodText,
    pill,
    progressRatio,
    progressText,
    type ReportActive,
    readOnlyNote,
    type SprintCard,
    type WorkspaceSettings,
  } from "@hedgehug/core";
  import { Button } from "@hedgehug/ui";
  import PlanItems from "./PlanItems.svelte";
  import SprintBurndown from "./SprintBurndown.svelte";
  import SprintMenu from "./SprintMenu.svelte";

  // スプリントの画面のカード(05「スプリントの画面」の表)。ボードは出さない(F1-19)。
  // ボタンとメニューの動きは親が渡す。
  interface Props {
    card: SprintCard;
    // active は、実行中のスプリントの数字(F3-15)。まだ読めていなければ null。
    active: ReportActive | null;
    storeId: string;
    storeName: string;
    settings: WorkspaceSettings;
    members: Member[];
    readOnly: boolean;
    // startReason は、「開始」を押せない理由(実行中のスプリントがあるとき)。
    startReason: string | null;
    // today は、このマシンの時刻帯での今日(YYYY-MM-DD)。日付が変わるとピルが変わる。
    today: string;
    thisYear: number;
    onstart: (card: SprintCard) => void;
    onclose: (card: SprintCard) => void;
    onedit: (card: SprintCard) => void;
    ondelete: (card: SprintCard) => void;
    onburndown: (card: SprintCard) => void;
  }

  let {
    card,
    active,
    storeId,
    storeName,
    settings,
    members,
    readOnly,
    startReason,
    today,
    thisYear,
    onstart,
    onclose,
    onedit,
    ondelete,
    onburndown,
  }: Props = $props();

  const s = $derived(card.sprint);
  const badge = $derived(pill(s, today));
  const progress = $derived(progressText(card));
  const percent = $derived(Math.round(progressRatio(card) * 100));
  const startWhy = $derived(readOnly ? readOnlyNote : startReason);
  // 小さいバーンダウンは、読めた数字が、このカードの実行中のスプリントのものであるときだけ出す。
  const burn = $derived(s.state === "active" && active?.sprint?.id === s.id ? active : null);
  // 実行中だけ開いておく。読み直しで開閉を戻さないよう、状態が変わったときだけ合わせる。
  const running = $derived(s.state === "active");
  let open = $state(false);
  $effect.pre(() => {
    open = running;
  });
</script>

<article class="card" aria-label={s.name}>
  <header class="head">
    <h3>{s.name}</h3>
    {#if badge}
      <span class="pill">{badge}</span>
    {/if}
    <span class="period">{periodText(s, thisYear)}</span>
    <span class="actions">
      {#if s.state === "planned"}
        <Button
          size="sm"
          disabled={startWhy !== null}
          title={startWhy ?? undefined}
          onclick={() => onstart(card)}
          >開始</Button
        >
      {:else if s.state === "active"}
        <Button
          size="sm"
          disabled={readOnly}
          title={readOnly ? readOnlyNote : undefined}
          onclick={() => onclose(card)}
          >完了…</Button
        >
      {/if}
      <SprintMenu
        sprint={s}
        {readOnly}
        onedit={() => onedit(card)}
        onburndown={() => onburndown(card)}
        ondelete={() => ondelete(card)}
      />
    </span>
  </header>
  {#if s.goal}
    <p class="goal">ゴール: {s.goal}</p>
  {/if}
  <div class="progress">
    <span
      class="bar"
      role="progressbar"
      aria-label="進捗"
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={percent}
      aria-valuetext={progress}
    >
      <span class="fill" style:width="{percent}%"></span>
    </span>
    <span class="text">{progress}</span>
  </div>
  {#if burn}
    <SprintBurndown active={burn} start={s.start} {thisYear} />
  {/if}
  <details bind:open>
    <summary>アイテム {card.items.length} 件</summary>
    <PlanItems rows={card.items} {storeId} {storeName} {settings} {members} />
  </details>
</article>

<style>
  .card {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 12px 14px;
    border: 1px solid var(--line);
    border-radius: 10px;
    background: var(--surface);
  }
  .head {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
  }
  h3 {
    font-size: 14px;
    margin: 0;
  }
  .pill {
    font-size: 11.5px;
    padding: 1px 8px;
    border-radius: 999px;
    background: var(--line);
  }
  .period,
  .goal,
  .text {
    font-size: 12px;
    color: var(--muted);
  }
  .goal {
    margin: 0;
  }
  .actions {
    margin-left: auto;
    display: inline-flex;
    gap: 4px;
  }
  .progress {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .bar {
    flex: 0 0 160px;
    height: 6px;
    border-radius: 3px;
    background: var(--line);
    overflow: hidden;
  }
  .fill {
    display: block;
    height: 100%;
    background: var(--accent);
  }
  summary {
    cursor: pointer;
    font-size: 12.5px;
  }
</style>
