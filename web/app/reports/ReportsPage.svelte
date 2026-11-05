<script lang="ts">
  import { unwrap } from "../../core/api/client";
  import {
    cliReportBurndown,
    cliReportCfd,
    cliReportCycle,
    cliReportEstimate,
    cliReportVelocity,
  } from "../../core/cli";
  import { dateOf, shortDate, todayOf } from "../../core/gantt";
  import { errorMessage } from "../../core/messages";
  import { sprintDates, sprintsInOrder } from "../../core/planning";
  import {
    amountLabel,
    type BurndownView,
    burndownFacts,
    type CFDView,
    type CycleView,
    cfdPlot,
    daysLabel,
    type EstimateView,
    estimateLines,
    type VelocityRow,
    velocityPlot,
  } from "../../core/reports";
  import type { WorkspaceEntry } from "../../core/stores";
  import { core } from "../context";
  import { usePlans } from "../planning/plans.svelte";
  import { onLinkClick, router } from "../router";
  import CliLine from "../ui/CliLine.svelte";
  import BurndownPlot from "./BurndownPlot.svelte";
  import Plot from "./Plot.svelte";
  import { useReport } from "./report.svelte";

  let { entry }: { entry: WorkspaceEntry } = $props();

  const c = core();
  const plans = usePlans(() => ({ store: entry.store, name: entry.name }));
  const where = $derived({ store: entry.store, workspace: entry.name });
  const storeName = () => entry.store;

  const LASTS = [5, 10, 20];
  const today = todayOf(new Date());

  const started = $derived(sprintsInOrder(plans.sprints).filter((s) => s.state !== "planned"));
  let picked = $state("");
  const sprint = $derived(started.find((s) => s.name === picked) ?? started[0]);
  let by = $state<"points" | "count">("points");
  let last = $state(5);
  let since = $state(dateOf(today - 30));
  let until = $state(dateOf(today));
  let flowBy = $state<"category" | "status">("category");
  let estimateSprint = $state("");

  const burndown = useReport<BurndownView>(storeName, () => {
    const name = sprint?.name;
    if (!name) return null;
    const query = { ...where, sprint: name, by };
    return unwrap(c.api.GET("/api/reports/burndown", { params: { query } }));
  });
  const velocity = useReport<VelocityRow[] | null>(storeName, () =>
    unwrap(c.api.GET("/api/reports/velocity", { params: { query: { ...where, last } } })),
  );
  const flow = useReport<CFDView>(storeName, () =>
    unwrap(
      c.api.GET("/api/reports/cfd", {
        params: { query: { ...where, since, until, by: flowBy } },
      }),
    ),
  );
  const cycle = useReport<CycleView>(storeName, () =>
    unwrap(c.api.GET("/api/reports/cycle", { params: { query: { ...where, since, until } } })),
  );
  const estimate = useReport<EstimateView>(storeName, () => {
    const query = estimateSprint === "" ? where : { ...where, sprint: estimateSprint };
    return unwrap(c.api.GET("/api/reports/estimate", { params: { query } }));
  });

  const burnFacts = $derived(burndown.value ? burndownFacts(burndown.value) : null);
  const vel = $derived(velocityPlot(velocity.value ?? []));
  const velTable = $derived({
    head: ["スプリント", "完了 points", "完了件数", "points なし"],
    rows: (velocity.value ?? []).map((r) => [
      r.sprint.name,
      String(r.points),
      String(r.tasks),
      String(r.unpointed),
    ]),
  });
  const cfd = $derived(flow.value ? cfdPlot(flow.value) : null);
  const COLORS = ["amber", "accent", "success", "muted-2", "merged", "pending", "danger", "icon"];
  const cfdTable = $derived({
    head: ["日付", ...(cfd?.stacks.map((s) => s.name) ?? [])],
    rows: (cfd?.xs ?? []).map((x, i) => [
      shortDate(x),
      ...(cfd?.stacks.map((s) => String(s.values[i] ?? 0)) ?? []),
    ]),
  });
  const lines = $derived(estimate.value ? estimateLines(estimate.value) : []);
  const taskHref = (ulid: string) => router.href({ name: "task", ulid });
</script>

{#snippet failed(e: unknown)}
  <p class="msg" role="alert">{errorMessage(e)}</p>
{/snippet}

<div class="page">
  <div class="bar">
    <span class="note">git 履歴から集計 · 移動してきたタスクは移動元の履歴も含みます</span>
  </div>

  <div class="grid">
    <section aria-labelledby="bd-h">
      <div class="head">
        <h2 id="bd-h">バーンダウン{sprint ? ` · ${sprint.name}` : ""}</h2>
        {#if sprint}
          <span class="note">{sprintDates(sprint)} · {sprint.state}</span>
        {/if}
        <div class="right">
          <label class="pick">
            スプリント
            <select
              value={sprint?.name ?? ""}
              onchange={(e) => (picked = e.currentTarget.value)}
              disabled={started.length === 0}
            >
              {#each started as s (s.ulid)}
                <option value={s.name}>{s.name}</option>
              {/each}
            </select>
          </label>
          <fieldset class="seg">
            <legend class="sr">単位</legend>
            <button type="button" aria-pressed={by === "points"} onclick={() => (by = "points")}>
              points
            </button>
            <button type="button" aria-pressed={by === "count"} onclick={() => (by = "count")}>
              件数
            </button>
          </fieldset>
        </div>
      </div>
      {#if !sprint}
        <p class="note">{plans.loaded ? "開始したスプリントはありません" : "読み込み中…"}</p>
      {:else if burndown.error}
        {@render failed(burndown.error)}
      {:else if burndown.value && burnFacts}
        {#if by === "points" && burndown.value.by === "count"}
          <p class="note">points のあるタスクがないため、件数で数えています</p>
        {/if}
        <p class="note">
          残り {amountLabel(burnFacts.remaining, burndown.value.by)} / 範囲
          {amountLabel(burnFacts.scope, burndown.value.by)}（途中で追加
          {amountLabel(burnFacts.added, burndown.value.by)}）
        </p>
        <BurndownPlot view={burndown.value} />
        <CliLine command={cliReportBurndown({ ...where, sprint: sprint.name, by })} />
      {:else}
        <p class="note">読み込み中…</p>
      {/if}
    </section>

    <section aria-labelledby="vel-h">
      <div class="head">
        <h2 id="vel-h">ベロシティ</h2>
        {#if vel.average !== null}
          <span class="note">直近 3 スプリントの平均 <b>{vel.average} pt</b></span>
        {/if}
        <div class="right">
          <label class="pick">
            期間
            <select bind:value={last}>
              {#each LASTS as n (n)}
                <option value={n}>直近 {n} スプリント</option>
              {/each}
            </select>
          </label>
        </div>
      </div>
      {#if velocity.error}
        {@render failed(velocity.error)}
      {:else if vel.names.length > 0}
        <Plot
          label="ベロシティ"
          xs={vel.names.map((_, i) => i)}
          xLabel={(x) => vel.names[x] ?? ""}
          table={velTable}
          legend={false}
          lines={[{ label: "完了", values: vel.points, color: "accent", bars: true }]}
        />
        <CliLine command={cliReportVelocity({ ...where, last })} />
      {:else}
        <p class="note">{velocity.loading ? "読み込み中…" : "閉じたスプリントはありません"}</p>
      {/if}
    </section>

    <section aria-labelledby="cfd-h">
      <div class="head">
        <h2 id="cfd-h">累積フロー図</h2>
        <div class="right">
          <label class="pick">開始 <input type="date" bind:value={since}></label>
          <label class="pick">終了 <input type="date" bind:value={until}></label>
          <fieldset class="seg">
            <legend class="sr">累積フロー図の列</legend>
            <button
              type="button"
              aria-pressed={flowBy === "category"}
              onclick={() => (flowBy = "category")}
            >
              カテゴリ
            </button>
            <button
              type="button"
              aria-pressed={flowBy === "status"}
              onclick={() => (flowBy = "status")}
            >
              ステータス
            </button>
          </fieldset>
        </div>
      </div>
      {#if flow.error}
        {@render failed(flow.error)}
      {:else if cfd}
        <Plot
          label="累積フロー図"
          xs={cfd.xs}
          xLabel={shortDate}
          table={cfdTable}
          lines={cfd.stacks.map((s, i) => ({
            label: s.name,
            values: s.top,
            color: COLORS[i % COLORS.length] ?? "accent",
            fill: true,
          }))}
        />
        <CliLine command={cliReportCfd({ ...where, since, until, by: flowBy })} />
      {:else}
        <p class="note">読み込み中…</p>
      {/if}
    </section>

    <section aria-labelledby="ct-h">
      <div class="head">
        <h2 id="ct-h">サイクルタイム / リードタイム</h2>
        {#if cycle.value}
          <span class="note">{since} – {until} に完了した {cycle.value.lead.count} 件</span>
        {/if}
      </div>
      {#if cycle.error}
        {@render failed(cycle.error)}
      {:else if cycle.value}
        <div class="cards">
          <div class="card">
            <span>サイクルタイム 中央値</span>
            <b>{daysLabel(cycle.value.cycle.median)}</b>
            <small
              >doing に入ってから done まで · 85% は {daysLabel(cycle.value.cycle.p85)} 以内</small
            >
          </div>
          <div class="card">
            <span>リードタイム 中央値</span>
            <b>{daysLabel(cycle.value.lead.median)}</b>
            <small>作成から done まで · 85% は {daysLabel(cycle.value.lead.p85)} 以内</small>
          </div>
        </div>
        {#if (cycle.value.warnings?.length ?? 0) > 0}
          <p class="note">
            移動元の Store がマウントされていないタスクが
            {cycle.value.warnings?.length}
            件あります。移動の時点から数えています
          </p>
        {/if}
        <CliLine command={cliReportCycle({ ...where, since, until })} />
      {:else}
        <p class="note">読み込み中…</p>
      {/if}
    </section>

    <section class="wide" aria-labelledby="ea-h">
      <div class="head">
        <h2 id="ea-h">見積もりと実績</h2>
        {#if estimate.value}
          <span class="note"
            >合計 見積 {estimate.value.estimate} / 実績 {estimate.value.actual}</span
          >
        {/if}
        <div class="right">
          <label class="pick">
            対象
            <select bind:value={estimateSprint}>
              <option value="">すべてのタスク</option>
              {#each sprintsInOrder(plans.sprints) as s (s.ulid)}
                <option value={s.name}>{s.name}</option>
              {/each}
            </select>
          </label>
        </div>
      </div>
      {#if estimate.error}
        {@render failed(estimate.error)}
      {:else if lines.length > 0}
        <ul class="ea" aria-label="見積もりと実績のタスク">
          {#each lines as l (l.ulid)}
            <li>
              <span class="who">
                <span class="id">{l.id}</span>
                <a href={taskHref(l.ulid)} onclick={(e) => onLinkClick(e, taskHref(l.ulid))}
                  >{l.title}</a
                >
              </span>
              <span class="track" aria-hidden="true">
                {#if l.estimateAt !== null}
                  <i class="est" style:left="{l.estimateAt}%"></i>
                {/if}
                {#if l.actualAt !== null}
                  <i class="act" data-over={l.over || undefined} style:left="{l.actualAt}%"></i>
                {/if}
              </span>
              <span class="nums" data-over={l.over || undefined}>
                {l.estimate}
                → {l.actual}{l.over ? "（超過）" : ""}
              </span>
            </li>
          {/each}
        </ul>
      {:else}
        <p class="note">{estimate.loading ? "読み込み中…" : "見積もりも作業時間もありません"}</p>
      {/if}
      {#if estimate.value}
        <CliLine command={cliReportEstimate({ ...where, sprint: estimateSprint || undefined })} />
      {/if}
    </section>
  </div>
</div>

<style>
  .page {
    box-sizing: border-box;
    height: 100%;
    overflow-y: auto;
    padding-bottom: var(--space-16);
  }
  .bar {
    padding: var(--space-12) var(--gutter) 0;
  }
  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(min(440px, 100%), 1fr));
    gap: var(--space-16);
    padding: var(--space-8) var(--gutter);
  }
  section {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
    min-width: 0;
    padding: var(--space-16) 18px;
    border: 1px solid var(--line);
    border-radius: 12px;
  }
  .wide {
    grid-column: 1 / -1;
  }
  .head {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-8);
  }
  .right {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-8);
    margin-left: auto;
  }
  h2 {
    margin: 0;
    font-size: 14px;
    font-weight: 600;
  }
  .note {
    margin: 0;
    color: var(--muted);
    font-size: var(--fs-12);
  }
  .note b {
    color: var(--ink);
  }
  .msg {
    margin: 0;
    color: var(--danger);
  }
  .pick {
    display: inline-flex;
    align-items: center;
    gap: var(--space-6);
    color: var(--muted);
    font-size: var(--fs-12);
  }
  .pick select,
  .pick input {
    min-height: 30px;
    border: 1px solid var(--line-input);
    border-radius: 6px;
    background: var(--surface);
    color: var(--ink);
    font: inherit;
  }
  .sr {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip-path: inset(50%);
  }
  .seg {
    display: inline-flex;
    margin: 0;
    padding: 2px;
    border: 0;
    border-radius: 6px;
    background: var(--hover);
  }
  .seg button {
    min-height: 26px;
    padding: 0 var(--space-8);
    border: 0;
    border-radius: 5px;
    background: transparent;
    color: var(--ink-2);
    font: inherit;
    font-size: var(--fs-12);
    cursor: pointer;
  }
  .seg button[aria-pressed="true"] {
    background: var(--surface);
    color: var(--ink);
    font-weight: 600;
  }
  .cards {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-10);
  }
  .card {
    flex: 1 1 120px;
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: var(--space-10) var(--space-12);
    border-radius: 8px;
    background: var(--sunken);
  }
  .card span {
    color: var(--ink-2);
    font-size: var(--fs-12);
  }
  .card b {
    font-size: 20px;
    font-weight: 600;
  }
  .card small {
    color: var(--muted);
    font-size: 11.5px;
  }
  .ea {
    display: flex;
    flex-direction: column;
    gap: 2px;
    margin: 0;
    padding: 0;
    list-style: none;
  }
  .ea li {
    display: grid;
    grid-template-columns: minmax(0, 260px) minmax(0, 1fr) minmax(110px, auto);
    align-items: center;
    gap: var(--space-8);
    min-height: 30px;
  }
  .who {
    display: flex;
    align-items: center;
    gap: var(--space-8);
    min-width: 0;
  }
  .who a {
    overflow: hidden;
    color: var(--ink);
    text-decoration: none;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .id,
  .nums {
    color: var(--muted);
    font-family: var(--font-mono);
    font-size: 11.5px;
    white-space: nowrap;
  }
  .nums[data-over] {
    color: var(--danger);
    font-weight: 500;
  }
  .track {
    position: relative;
    height: 12px;
    border-bottom: 1px solid var(--line-soft);
  }
  .track i {
    position: absolute;
    top: 3px;
    box-sizing: border-box;
    width: 11px;
    height: 11px;
    margin-left: -6px;
    border-radius: 999px;
  }
  .est {
    border: 1.5px solid var(--ink);
    background: var(--surface);
  }
  .act {
    background: var(--accent);
  }
  .act[data-over] {
    background: var(--danger);
  }
</style>
