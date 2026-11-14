<script lang="ts">
  import {
    type BacklogFrame,
    type Member,
    type PlanningOverview,
    periodText,
    pt,
    readOnlyNote,
    type Sprint,
    type Totals,
    type WorkspaceSettings,
  } from "@hedgehug/core";
  import { Button, type DropTarget } from "@hedgehug/ui";
  import BacklogRow from "./BacklogRow.svelte";

  // バックログの画面の枠(05「並びと表示」、F2a-14)。スプリントの枠は、名前・状態・期間・件数とポイント・ゴールと、
  // 計画中なら「開始」。バックログの枠は、見出しに「スプリントを作成」と「作成」を置く。
  interface Props {
    frame: BacklogFrame;
    frames: BacklogFrame[];
    storeName: string;
    settings: WorkspaceSettings;
    members: Member[];
    overview: PlanningOverview | null;
    readOnly: boolean;
    // startReason は、「開始」を押せない理由(実行中のスプリントがあるとき)。
    startReason: string | null;
    thisYear: number;
    // hover は、ドラッグのドロップ先(この枠でなければ印を出さない)。
    hover: DropTarget | null;
    onhover: (itemId: string, target: DropTarget | null) => void;
    ondrop: (itemId: string, target: DropTarget) => void;
    onstart: (frame: { sprint: Sprint; total: Totals }) => void;
    oncreate: () => void;
    onitem: () => void;
  }

  let {
    frame,
    frames,
    storeName,
    settings,
    members,
    overview,
    readOnly,
    startReason,
    thisYear,
    hover,
    onhover,
    ondrop,
    onstart,
    oncreate,
    onitem,
  }: Props = $props();

  const s = $derived(frame.sprint);
  const key = $derived(s?.id ?? "");
  const here = $derived(hover !== null && (hover.frame ?? "") === key ? hover : null);
  const totals = $derived(`${frame.total.count} 件 · ${pt(frame.total.points)} pt`);
  const note = $derived(readOnly ? readOnlyNote : undefined);
  const startWhy = $derived(readOnly ? readOnlyNote : startReason);
</script>

<section
  class="frame"
  class:tail={here !== null && here.row === null}
  aria-label={s?.name ?? "バックログ"}
  data-frame={key}
>
  {#if s}
    <header class="head">
      <h3>{s.name}</h3>
      <span class="meta">{s.state === "active" ? "実行中" : "計画中"}</span>
      <span class="meta">{periodText(s, thisYear)}</span>
      <span class="meta">{totals}</span>
      {#if s.state === "planned"}
        <span class="actions">
          <Button
            size="sm"
            disabled={startWhy !== null}
            title={startWhy ?? undefined}
            onclick={() => onstart({ sprint: s, total: frame.total })}
            >開始</Button
          >
        </span>
      {/if}
    </header>
    {#if s.goal}
      <p class="goal">ゴール: {s.goal}</p>
    {/if}
  {:else}
    <header class="head">
      <h3>バックログ</h3>
      <span class="meta">{totals} · スプリントに入っていない未完了の Story・Task・Bug</span>
      <span class="actions">
        <Button size="sm" disabled={readOnly} title={note} onclick={oncreate}
          >スプリントを作成</Button
        >
        <Button size="sm" variant="primary" disabled={readOnly} title={note} onclick={onitem}
          >作成</Button
        >
      </span>
    </header>
  {/if}
  {#if frame.rows.length === 0}
    <p class="empty">
      {s ? "バックログからここにドラッグ、または行の「スプリント」で選択" : "バックログは空です"}
    </p>
  {:else}
    <ul class="rows">
      {#each frame.rows as row (row.id)}
        <BacklogRow
          {row}
          sprint={key}
          {frames}
          {storeName}
          {settings}
          {members}
          {overview}
          {readOnly}
          marked={here?.row === row.id}
          onhover={(t) => onhover(row.id, t)}
          ondrop={(t) => ondrop(row.id, t)}
        />
      {/each}
    </ul>
  {/if}
</section>

<style>
  .frame {
    display: flex;
    flex-direction: column;
    gap: 6px;
    padding: 12px 14px;
    margin-bottom: 14px;
    border: 1px solid var(--line);
    border-bottom-width: 2px;
    border-radius: var(--radius);
    background: var(--surface);
  }
  .tail {
    border-bottom-color: var(--accent);
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
  .meta {
    font-size: 12px;
    color: var(--muted);
  }
  .actions {
    display: flex;
    gap: 6px;
    margin-left: auto;
  }
  .goal,
  .empty {
    margin: 0;
    font-size: 12.5px;
    color: var(--muted);
  }
  .empty {
    padding: 8px 0;
  }
  .rows {
    list-style: none;
    margin: 0;
    padding: 0;
  }
</style>
