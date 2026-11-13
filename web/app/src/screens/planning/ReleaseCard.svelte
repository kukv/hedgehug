<script lang="ts">
  import {
    type Member,
    menuAt,
    type ReleaseCard,
    type ReleaseState,
    readOnlyNote,
    releaseDateText,
    releaseMenu,
    releaseOverdue,
    releaseProgressRatio,
    releaseProgressText,
    releaseStateColor,
    releaseStateLabel,
    type WorkspaceSettings,
  } from "@hedgehug/core";
  import { Button, IconButton, Menu } from "@hedgehug/ui";
  import PlanItems from "./PlanItems.svelte";

  // リリースの画面のカード(05「並びと表示」)。ボタンとメニューの動きは親が渡す。
  interface Props {
    card: ReleaseCard;
    storeId: string;
    storeName: string;
    settings: WorkspaceSettings;
    members: Member[];
    readOnly: boolean;
    // today は、このマシンの時刻帯での今日(YYYY-MM-DD)。日付が変わると「予定日を過ぎています」が変わる。
    today: string;
    thisYear: number;
    onstate: (card: ReleaseCard, to: ReleaseState) => void;
    onedit: (card: ReleaseCard) => void;
    ondelete: (card: ReleaseCard) => void;
  }

  let {
    card,
    storeId,
    storeName,
    settings,
    members,
    readOnly,
    today,
    thisYear,
    onstate,
    onedit,
    ondelete,
  }: Props = $props();

  const r = $derived(card.release);
  // ピルの色は、ui の値の色のトークン(青・緑・灰)。
  const color = $derived(releaseStateColor(r.state));
  const late = $derived(releaseOverdue(r, today));
  const progress = $derived(releaseProgressText(card));
  const percent = $derived(Math.round(releaseProgressRatio(card) * 100));
  let menu = $state<{ x: number; y: number } | null>(null);

  function select(id: string) {
    menu = null;
    if (id === "edit") onedit(card);
    else if (id === "delete") ondelete(card);
    else onstate(card, id as ReleaseState);
  }
</script>

<article class="card" aria-label={r.name}>
  <header class="head">
    <h3>{r.name}</h3>
    <span class="pill" style:background="var(--c-{color}-bg)" style:color="var(--c-{color}-fg)"
      >{releaseStateLabel(r.state)}</span
    >
    <span class="date" class:late>
      {#if late}
        予定日を過ぎています
      {/if}
      {releaseDateText(r, thisYear)}
    </span>
    <span class="actions">
      {#if r.state === "unreleased"}
        <Button
          size="sm"
          disabled={readOnly}
          title={readOnly ? readOnlyNote : undefined}
          onclick={() => onstate(card, "released")}
          >リリース済みにする</Button
        >
      {/if}
      <IconButton
        label="{r.name} のメニュー"
        size="sm"
        onclick={(e) => (menu = menuAt((e.currentTarget as HTMLElement).getBoundingClientRect()))}
        >⋯</IconButton
      >
    </span>
  </header>
  {#if r.description}
    <p class="description">{r.description}</p>
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
  <details>
    <summary>アイテム {card.items.length} 件</summary>
    <PlanItems
      rows={card.items}
      {storeId}
      {storeName}
      {settings}
      {members}
      empty="ありません。アイテムの「リリース」で選べます"
    />
  </details>
</article>
<Menu
  open={menu !== null}
  x={menu?.x ?? 0}
  y={menu?.y ?? 0}
  items={releaseMenu(r, readOnly)}
  onselect={select}
  onclose={() => (menu = null)}
/>

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
  }
  .date,
  .description,
  .text {
    font-size: 12px;
    color: var(--muted);
  }
  .late {
    color: var(--danger);
    font-weight: 600;
  }
  .description {
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
