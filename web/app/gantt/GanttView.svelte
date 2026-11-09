<script lang="ts">
  import type { Snippet } from "svelte";
  import type { TaskView } from "../../core/api/client";
  import { CATEGORY_LABEL, categoryOf } from "../../core/categories";
  import { cliLs } from "../../core/cli";
  import {
    chartRange,
    type DragMode,
    dayOf,
    dragged,
    dropProps,
    type GanttRow,
    ganttLinks,
    ganttRows,
    ganttWarnings,
    PX_PER_DAY,
    type Scale,
    type Span,
    shortDate,
    spanOf,
    ticks,
    todayOf,
  } from "../../core/gantt";
  import { errorMessage } from "../../core/messages";
  import { propNames } from "../../core/schema-edit";
  import type { GanttSettings } from "../../core/views";
  import { core } from "../context";
  import { openCreate } from "../create-state.svelte";
  import KindIcon from "../icons/KindIcon.svelte";
  import ListBar from "../list/ListBar.svelte";
  import type { ListScope } from "../list/scope";
  import { useListScope } from "../list/scope.svelte";
  import { usePlans } from "../planning/plans.svelte";
  import { useReadOnlyOf } from "../read-only";
  import { onLinkClick, router } from "../router";
  import Empty from "../states/Empty.svelte";
  import { useDisplayId, useSubscribe } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import { patchTask } from "../tree/actions";
  import { combine, hasTerm, NO_CANCELED } from "../tree/filter";
  import { countLabel, countMatched, VIRTUALIZE_AT, viewRoots } from "../tree/flatten";

  // The prop is not called scope: Biome reads that name as the HTML attribute.
  let {
    view: scope,
    query,
    onquery,
    settings,
    onsettings,
    cli,
    actions,
  }: {
    view: ListScope;
    query: string;
    onquery: (q: string) => void;
    settings: GanttSettings;
    onsettings: (s: GanttSettings) => void;
    cli?: string;
    actions?: Snippet;
  } = $props();

  const ROW = 36;
  const AXIS = 44;
  const BAND = 20;

  const c = core();
  const uid = $props.id();
  const did = useDisplayId(c.stores);
  const ls = useListScope(() => scope);
  const cacheRev = useSubscribe(c.cache, () => ({}));
  const schemaRev = useSubscribe(c.schema, () => ({}));
  const roOf = useReadOnlyOf(c.stores);

  const taskOf = (ulid: string): TaskView | undefined => {
    cacheRev.current;
    return c.cache.task(ulid);
  };

  // --- what the chart lists ---------------------------------------------------------------------
  type TreeHandle = ReturnType<typeof c.cache.watchTree>;
  let tree = $state<TreeHandle | null>(null);
  $effect(() => {
    if (!ls.ready) {
      tree = null;
      return;
    }
    const h = c.cache.watchTree({ q: combine(ls.apiBase, query), tree: true, store: ls.store });
    tree = h;
    return () => h.dispose();
  });

  const viewRoot = $derived(scope.kind === "project" ? scope.ulid : null);
  const roots = $derived.by(() => {
    cacheRev.current;
    return viewRoots(tree?.roots() ?? [], viewRoot);
  });
  const loading = $derived.by(() => {
    cacheRev.current;
    return !ls.ready || (tree?.loading() ?? true);
  });
  const treeError = $derived.by(() => {
    cacheRev.current;
    return tree?.error() ?? null;
  });
  const matched = $derived(countMatched(roots, null, taskOf));

  // Every task under the roots, for what does not depend on what is folded.
  const everyTask = $derived.by(() => {
    const out: TaskView[] = [];
    const walk = (nodes: typeof roots) => {
      for (const n of nodes) {
        const t = taskOf(n.ulid);
        if (t) out.push(t);
        walk(n.children);
      }
    };
    walk(roots);
    return out;
  });

  const storesNeeded = $derived([...new Set(everyTask.map((t) => t.store))]);
  const schemas = $derived(ls.schemas(storesNeeded));
  const schemaOf = (store: string) => {
    schemaRev.current;
    return c.schema.get(store);
  };

  // The sprints and the releases belong to a workspace: with several, there is no one set to draw.
  const places = $derived([
    ...new Set(everyTask.map((t) => JSON.stringify([t.store, t.workspace.name]))),
  ]);
  const place = $derived(places.length === 1 ? (places[0] ?? "") : "");
  const plans = usePlans(() => {
    if (place === "") return null;
    const [store = "", name = ""] = JSON.parse(place) as string[];
    return { store, name };
  });
  const bands = $derived(
    plans.sprints.flatMap((s) => {
      const span = s.start && s.end ? spanOf(s.start, s.end) : null;
      return span ? [{ sprint: s, span }] : [];
    }),
  );
  const lines = $derived(
    plans.releases.flatMap((r) => {
      const day = dayOf(r.date);
      return day === null ? [] : [{ release: r, day }];
    }),
  );

  let collapsed = $state.raw<ReadonlySet<string>>(new Set());
  const toggle = (ulid: string) => {
    const next = new Set(collapsed);
    if (!next.delete(ulid)) next.add(ulid);
    collapsed = next;
  };

  const rows = $derived(
    ganttRows({
      roots,
      taskOf,
      settings,
      collapsed,
      categoryOf: (t) => categoryOf(schemaOf(t.store), t.status),
      sprintSpan: (t) => {
        const u = t.sprint?.ulid;
        return bands.find((b) => b.sprint.ulid === u)?.span ?? null;
      },
    }),
  );
  const links = $derived(ganttLinks(rows));
  const warnings = $derived(ganttWarnings(rows, links));

  // --- the settings of the chart ----------------------------------------------------------------
  let scale = $state<Scale>("day");
  const dateProps = $derived([
    ...new Set(
      schemas.flatMap((s) => propNames(s).filter((name) => s.props[name]?.type === "date")),
    ),
  ]);
  const propChoices = (now: string) => (dateProps.includes(now) ? dateProps : [now, ...dateProps]);

  // --- the geometry -----------------------------------------------------------------------------
  const px = $derived(PX_PER_DAY[scale]);
  const today = todayOf(new Date());
  const range = $derived.by<Span>(() =>
    chartRange(
      rows,
      [
        ...bands.filter((b) => b.sprint.state !== "closed").flatMap((b) => [b.span.s, b.span.e]),
        ...lines.filter((l) => l.release.state === "unreleased").map((l) => l.day),
      ],
      today,
      scale,
    ),
  );
  const axis = $derived(ticks(range, scale));
  const xOf = (day: number) => (day - range.s) * px;
  const width = $derived(xOf(range.e + 1));
  const height = $derived(Math.max(rows.length, 1) * ROW);
  const yOf = (i: number) => i * ROW;
  const indexOf = $derived(new Map(rows.map((r, i) => [r.ulid, i])));

  // A long list draws the rows near the window. The rows are placed by their index in any case.
  let scroller = $state<HTMLElement>();
  let scrollTop = $state(0);
  let viewHeight = $state(800);
  const shown = $derived.by(() => {
    if (rows.length <= VIRTUALIZE_AT) return rows;
    const first = Math.max(0, Math.floor(scrollTop / ROW) - 5);
    return rows.slice(first, first + Math.ceil(viewHeight / ROW) + 10);
  });

  // The chart opens at today, and comes back to it with a new scale. The sprints are read after the
  // tasks and may widen the chart to the left: what is on the screen stays there.
  let anchor: { first: number; px: number } | null = null;
  $effect(() => {
    if (!scroller || rows.length === 0) return;
    const first = range.s;
    if (anchor?.px !== px) scroller.scrollLeft = Math.max(0, xOf(today) - 200);
    else if (anchor.first !== first) scroller.scrollLeft += (anchor.first - first) * px;
    anchor = { first, px };
  });

  // --- moving a bar -----------------------------------------------------------------------------
  type Drag = { ulid: string; mode: DragMode; delta: number; from: number };
  let drag = $state.raw<Drag | null>(null);

  const movable = (r: GanttRow) => r.kind !== "rollup" && r.bar !== null;
  const offFor = (r: GanttRow) => roOf.current(r.task.store);
  const barOf = (r: GanttRow, d: Drag | null): Span | null =>
    d?.ulid === r.ulid && r.bar ? dragged(r.bar, d.mode, d.delta) : r.bar;

  async function drop(r: GanttRow, d: Drag) {
    if (!r.bar) return;
    const props = dropProps(r, dragged(r.bar, d.mode, d.delta), settings);
    if (d.delta === 0 || Object.keys(props).length === 0) {
      drag = null;
      return;
    }
    try {
      await patchTask(c, r.task, { props });
    } catch (e) {
      toast({ message: errorMessage(e) });
    } finally {
      drag = null;
    }
  }

  function press(e: PointerEvent, r: GanttRow) {
    if (e.button !== 0 || !movable(r)) return;
    const edge = (e.target as Element).getAttribute("data-edge");
    (e.currentTarget as Element).setPointerCapture(e.pointerId);
    drag = {
      ulid: r.ulid,
      mode: edge === "start" || edge === "end" ? edge : "move",
      delta: 0,
      from: e.clientX,
    };
  }
  function move(e: PointerEvent, r: GanttRow) {
    if (drag?.ulid !== r.ulid) return;
    drag = { ...drag, delta: Math.round((e.clientX - drag.from) / px) };
  }
  function release(r: GanttRow) {
    if (drag?.ulid === r.ulid) void drop(r, drag);
  }

  // The keyboard: the arrows move the bar (with Shift, its end), Enter keeps it and Esc takes it back.
  function key(e: KeyboardEvent, r: GanttRow) {
    if (e.key === "ArrowLeft" || e.key === "ArrowRight") {
      e.preventDefault();
      const step = e.key === "ArrowLeft" ? -1 : 1;
      const mode: DragMode = e.shiftKey ? "end" : "move";
      const delta = drag?.ulid === r.ulid && drag.mode === mode ? drag.delta : 0;
      drag = { ulid: r.ulid, mode, delta: delta + step, from: 0 };
    } else if (e.key === "Enter" && drag?.ulid === r.ulid) {
      e.preventDefault();
      void drop(r, drag);
    } else if (e.key === "Escape" && drag?.ulid === r.ulid) {
      e.stopPropagation();
      drag = null;
    }
  }

  const spanWords = (s: Span) => `${shortDate(s.s)} – ${shortDate(s.e)}`;
  const taskHref = (t: TaskView) => router.href({ name: "task", ulid: t.ulid });
  const fill = (r: GanttRow) => (r.over ? "warn" : r.kind === "rollup" ? "rollup" : r.category);
  const barTop = (r: GanttRow) => (r.kind === "rollup" ? 13 : 10);
  const barHeight = (r: GanttRow) => (r.kind === "rollup" ? 10 : 16);

  function linkPath(from: Span, to: Span, y1: number, y2: number): string {
    const x1 = xOf(from.e + 1);
    const x2 = xOf(to.s);
    if (x2 - x1 >= 16) return `M${x1} ${y1}H${x2 - 8}V${y2}H${x2 - 2}`;
    const mid = y1 < y2 ? y2 - ROW / 2 : y2 + ROW / 2;
    return `M${x1} ${y1}h8V${mid}H${x2 - 10}V${y2}H${x2 - 2}`;
  }

  const lsLine = $derived(
    cli ??
      cliLs(
        combine(ls.cliBase, query).trim(),
        false,
        scope.kind === "workspace" ? ls.store : undefined,
      ),
  );
  const draggedTip = $derived.by(() => {
    if (!drag) return "";
    const r = rows.find((x) => x.ulid === drag?.ulid);
    const to = r ? barOf(r, drag) : null;
    if (!r?.bar || !to) return "";
    return `${spanWords(r.bar)} → ${spanWords(to)} · ${drag.from === 0 ? "Enter で保存" : "離すと保存"}`;
  });
</script>

<div class="gantt">
  <ListBar
    base={ls.cliBase}
    applied={query}
    count={countLabel(matched, null)}
    cli={lsLine}
    error={treeError}
    onchange={onquery}
    {actions}
  >
    {#snippet controls()}
      <fieldset class="seg">
        <legend class="sr">表示する種類</legend>
        <button
          type="button"
          aria-pressed={!settings.roadmap}
          onclick={() => onsettings({ ...settings, roadmap: false })}
        >
          すべての種類
        </button>
        <button
          type="button"
          aria-pressed={settings.roadmap}
          onclick={() => onsettings({ ...settings, roadmap: true })}
        >
          ロードマップ
        </button>
      </fieldset>
      <label class="pick">
        開始
        <select
          value={settings.start}
          onchange={(e) => onsettings({ ...settings, start: e.currentTarget.value })}
        >
          {#each propChoices(settings.start) as p (p)}
            <option value={p}>{p}</option>
          {/each}
        </select>
      </label>
      <label class="pick">
        終了
        <select
          value={settings.end}
          onchange={(e) => onsettings({ ...settings, end: e.currentTarget.value })}
        >
          {#each propChoices(settings.end) as p (p)}
            <option value={p}>{p}</option>
          {/each}
        </select>
      </label>
      <label class="pick">
        目盛り
        <select bind:value={scale}>
          <option value="day">日</option>
          <option value="week">週</option>
          <option value="month">月</option>
        </select>
      </label>
    {/snippet}
  </ListBar>

  {#if ls.notFound}
    <Empty
      variant="notfound"
      {...ls.notFoundProps}
      onHome={() => router.go(router.href({ name: "home" }))}
    />
  {:else if loading}
    {#if !treeError}
      <p class="note">読み込み中…</p>
    {/if}
  {:else if rows.length === 0}
    {#if !treeError}
      {#if scope.kind !== "all" && query.trim() === NO_CANCELED}
        <Empty
          variant="empty"
          {...ls.emptyProps}
          onCreate={() =>
            openCreate({ kind: "task", parent: scope.kind === "project" ? scope.ulid : undefined })}
        />
      {:else}
        <Empty
          variant="nomatch"
          note={hasTerm(query, NO_CANCELED) ? "中止を除外しています" : undefined}
          onClear={query.trim() === "" ? undefined : () => onquery("")}
        />
      {/if}
    {/if}
  {:else}
    <div class="legend">
      <span><i data-fill="doing"></i>{CATEGORY_LABEL.doing}</span>
      <span><i data-fill="todo"></i>{CATEGORY_LABEL.todo}</span>
      <span><i data-fill="done"></i>{CATEGORY_LABEL.done}</span>
      <span><i data-fill="sprint"></i>日付なし（スプリントの期間）</span>
      <span><i data-fill="warn"></i>警告</span>
      {#if places.length > 1}
        <span class="many">複数のワークスペースがあるため、スプリントとリリースは描きません</span>
      {/if}
      {#if draggedTip}
        <span class="tip" role="status">{draggedTip}</span>
      {/if}
    </div>

    <div
      class="chart"
      bind:this={scroller}
      bind:clientHeight={viewHeight}
      onscroll={(e) => (scrollTop = e.currentTarget.scrollTop)}
    >
      <div class="corner">タスク</div>
      <svg class="axis" {width} height={AXIS} style:width="{width}px">
        <title>ガントの目盛り</title>
        {#each bands as b (b.sprint.ulid)}
          <g>
            <title>{`${b.sprint.name} ${spanWords(b.span)}`}</title>
            <rect
              x={xOf(b.span.s)}
              y="0"
              width={xOf(b.span.e + 1) - xOf(b.span.s)}
              height={BAND}
              data-state={b.sprint.state}
            />
            <text x={xOf(b.span.s) + 8} y="14">{b.sprint.name} · {spanWords(b.span)}</text>
          </g>
        {/each}
        {#each axis as t (t.day)}
          <text
            class="day"
            data-today={t.day === today || undefined}
            x={xOf(t.day) + (scale === "day" ? px / 2 : 2)}
            y="37"
            text-anchor={scale === "day" ? "middle" : "start"}
          >
            {t.label}
          </text>
        {/each}
      </svg>

      <ul class="labels" aria-label="ガントのタスク" style:height="{height}px">
        {#each shown as r (r.ulid)}
          <li
            style:top="{yOf(indexOf.get(r.ulid) ?? 0)}px"
            style:padding-left="{12 + r.depth * 18}px"
          >
            {#if r.hasChildren}
              <button
                type="button"
                class="fold"
                aria-expanded={r.expanded}
                aria-label="{r.task.title} の子を{r.expanded ? "折りたたむ" : "展開する"}"
                onclick={() => toggle(r.ulid)}
              >
                {r.expanded ? "▾" : "▸"}
              </button>
            {:else}
              <span class="fold"></span>
            {/if}
            <KindIcon kind={r.task.kind} size={13} />
            <span class="id">{did.current(r.task.store, r.task.id)}</span>
            <a
              href={taskHref(r.task)}
              data-rollup={r.kind === "rollup" || undefined}
              onclick={(e) => onLinkClick(e, taskHref(r.task))}
              >{r.task.title}</a
            >
          </li>
        {/each}
      </ul>

      <svg class="plot" {width} {height} style:width="{width}px">
        <title>ガントのバー</title>
        <defs>
          <marker
            id="{uid}-ok"
            viewBox="0 0 8 8"
            refX="7"
            refY="4"
            markerWidth="7"
            markerHeight="7"
            orient="auto"
          >
            <path d="M0 0L8 4L0 8z" class="head" />
          </marker>
          <marker
            id="{uid}-late"
            viewBox="0 0 8 8"
            refX="7"
            refY="4"
            markerWidth="7"
            markerHeight="7"
            orient="auto"
          >
            <path d="M0 0L8 4L0 8z" class="head" data-late="true" />
          </marker>
        </defs>
        {#each axis as t (t.day)}
          {#if t.weekend}
            <rect class="weekend" x={xOf(t.day)} y="0" width={px} {height} />
          {/if}
          <path class="grid" d="M{xOf(t.day)} 0V{height}" />
        {/each}
        {#each bands as b (b.sprint.ulid)}
          {#if b.sprint.state === "active"}
            <rect
              class="current"
              x={xOf(b.span.s)}
              y="0"
              width={xOf(b.span.e + 1) - xOf(b.span.s)}
              {height}
            />
          {/if}
        {/each}
        {#each shown as r (r.ulid)}
          <path class="row" d="M0 {yOf((indexOf.get(r.ulid) ?? 0) + 1)}H{width}" />
        {/each}

        {#each links as l (`${l.from.ulid}-${l.to.ulid}`)}
          {@const a = barOf(l.from, drag)}
          {@const b = barOf(l.to, drag)}
          {#if a && b}
            <path
              class="link"
              data-late={l.late || undefined}
              marker-end="url(#{uid}-{l.late ? "late" : "ok"})"
              d={linkPath(
                a,
                b,
                yOf(indexOf.get(l.from.ulid) ?? 0) + ROW / 2,
                yOf(indexOf.get(l.to.ulid) ?? 0) + ROW / 2,
              )}
            />
          {/if}
        {/each}

        {#each shown as r (r.ulid)}
          {@const bar = barOf(r, drag)}
          {@const y = yOf(indexOf.get(r.ulid) ?? 0)}
          {#if bar}
            {@const left = xOf(bar.s) + 2}
            {@const w = xOf(bar.e + 1) - xOf(bar.s) - 4}
            {@const name = `${did.current(r.task.store, r.task.id)} ${r.task.title} ${spanWords(bar)}`}
            {#if movable(r) && offFor(r) === null}
              <!-- biome-ignore lint/a11y/useSemanticElements: an element of an SVG; <button> cannot be in one -->
              <g
                class="bar"
                role="button"
                tabindex="0"
                aria-label="{name}。矢印キーで動かし、Enter で確定"
                data-ulid={r.ulid}
                data-kind={r.kind}
                data-fill={fill(r)}
                data-moving={drag?.ulid === r.ulid || undefined}
                onkeydown={(e) => key(e, r)}
                onblur={() => {
                  if (drag?.ulid === r.ulid && drag.from === 0) drag = null;
                }}
                onpointerdown={(e) => press(e, r)}
                onpointermove={(e) => move(e, r)}
                onpointerup={() => release(r)}
                onpointercancel={() => (drag = null)}
              >
                <rect
                  class="body"
                  x={left}
                  y={y + barTop(r)}
                  width={w}
                  height={barHeight(r)}
                  rx="4"
                />
                <rect
                  class="edge"
                  data-edge="start"
                  x={left - 3}
                  y={y + barTop(r)}
                  width="8"
                  height={barHeight(r)}
                />
                <rect
                  class="edge"
                  data-edge="end"
                  x={left + w - 5}
                  y={y + barTop(r)}
                  width="8"
                  height={barHeight(r)}
                />
              </g>
            {:else}
              <g class="bar" data-ulid={r.ulid} data-kind={r.kind} data-fill={fill(r)}>
                <title>{r.kind === "rollup" ? `${name}（子から集計）` : name}</title>
                <rect
                  class="body"
                  x={left}
                  y={y + barTop(r)}
                  width={w}
                  height={barHeight(r)}
                  rx={r.kind === "rollup" ? 2 : 4}
                />
              </g>
            {/if}
            {#if r.over && r.own}
              <path class="own" d="M{xOf(r.own.e + 1)} {y + 5}V{y + 31}" />
            {/if}
          {/if}
        {/each}

        <path class="today" d="M{xOf(today) + px / 2} 0V{height}" />
        {#each lines as l (l.release.ulid)}
          <g>
            <title>{`${l.release.name} リリース予定 ${shortDate(l.day)}`}</title>
            <path class="release" d="M{xOf(l.day + 1)} 0V{height}" />
            <text class="release" x={xOf(l.day + 1) + 4} y={height - 6}>
              {l.release.name} {shortDate(l.day)}
            </text>
          </g>
        {/each}
      </svg>
    </div>

    {#if warnings.length > 0}
      <ul class="warnings" aria-label="ガントの警告">
        {#each warnings as w (w.key)}
          <li><b>{w.id}</b>：{w.text}</li>
        {/each}
      </ul>
    {/if}
  {/if}
</div>

<style>
  .gantt {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
  }
  .note {
    margin: var(--space-16) var(--gutter);
    color: var(--muted);
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
    padding: 3px;
    border: 0;
    border-radius: 8px;
    background: var(--hover);
  }
  .seg button {
    min-height: 28px;
    padding: 0 10px;
    border: 0;
    border-radius: 6px;
    background: transparent;
    color: var(--ink-2);
    font: inherit;
    cursor: pointer;
  }
  .seg button[aria-pressed="true"] {
    background: var(--surface);
    color: var(--ink);
    font-weight: 600;
  }
  .pick {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    color: var(--muted);
    font-size: 12px;
  }
  .pick select {
    min-height: 32px;
    border: 1px solid var(--line-input);
    border-radius: 6px;
    background: var(--surface);
    color: var(--ink);
    font: inherit;
  }
  .legend {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 6px 14px;
    padding: 0 var(--gutter) var(--space-8);
    color: var(--ink-2);
    font-size: 12px;
  }
  .legend span {
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }
  .legend i {
    box-sizing: border-box;
    width: 18px;
    height: 8px;
    border-radius: 3px;
    background: var(--fill);
  }
  .legend i[data-fill="sprint"] {
    border: 1.5px dashed var(--muted);
    background: transparent;
  }
  .legend .tip {
    margin-left: auto;
    padding: 2px 8px;
    border-radius: 6px;
    background: var(--ink);
    color: var(--surface);
  }
  [data-fill="doing"] {
    --fill: var(--accent);
  }
  [data-fill="todo"] {
    --fill: var(--muted-2);
  }
  [data-fill="done"] {
    --fill: var(--success);
  }
  [data-fill="canceled"] {
    --fill: var(--line-tree);
  }
  [data-fill="rollup"] {
    --fill: var(--ink-2);
  }
  [data-fill="warn"] {
    --fill: var(--amber);
  }
  .chart {
    flex: 1 1 auto;
    min-height: 0;
    display: grid;
    grid-template-columns: 300px max-content;
    grid-template-rows: 44px auto;
    align-content: start;
    margin: 0 var(--gutter) var(--space-12);
    border: 1px solid var(--line);
    border-radius: 10px;
    overflow: auto;
    background: var(--surface);
  }
  .corner {
    position: sticky;
    top: 0;
    left: 0;
    z-index: 3;
    display: flex;
    align-items: flex-end;
    padding: 0 12px 8px;
    box-sizing: border-box;
    border-right: 1px solid var(--line);
    border-bottom: 1px solid var(--line);
    background: var(--sunken);
    color: var(--muted);
    font-size: 12px;
  }
  .axis {
    position: sticky;
    top: 0;
    z-index: 2;
    display: block;
    border-bottom: 1px solid var(--line);
    background: var(--surface);
  }
  .axis rect {
    fill: var(--hover);
  }
  .axis rect[data-state="active"] {
    fill: var(--accent-weak);
  }
  .axis text {
    fill: var(--ink-2);
    font-size: 11px;
  }
  .axis text.day {
    font-family: var(--font-mono);
    font-size: 10.5px;
  }
  .axis text[data-today] {
    fill: var(--accent-ink);
    font-weight: 600;
  }
  .labels {
    position: sticky;
    left: 0;
    z-index: 1;
    margin: 0;
    padding: 0;
    list-style: none;
    border-right: 1px solid var(--line);
    background: var(--surface);
  }
  .labels li {
    position: absolute;
    left: 0;
    right: 0;
    box-sizing: border-box;
    height: 36px;
    display: flex;
    align-items: center;
    gap: 7px;
    padding-right: 10px;
    border-bottom: 1px solid var(--line-soft);
  }
  .labels a {
    min-width: 0;
    overflow: hidden;
    color: var(--ink);
    text-decoration: none;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .labels a[data-rollup] {
    font-weight: 600;
  }
  .id {
    flex: 0 0 auto;
    color: var(--muted);
    font-family: var(--font-mono);
    font-size: 11px;
  }
  .fold {
    flex: 0 0 auto;
    width: 16px;
    padding: 0;
    border: 0;
    background: none;
    color: var(--muted);
    font: inherit;
    cursor: pointer;
  }
  .plot {
    display: block;
  }
  .weekend {
    fill: var(--sunken);
  }
  .grid {
    stroke: var(--line-soft);
  }
  .current {
    fill: var(--accent);
    opacity: 0.04;
  }
  .row {
    stroke: var(--line-soft);
  }
  .edge {
    fill: transparent;
    cursor: ew-resize;
  }
  .bar .body {
    fill: var(--fill);
  }
  .bar[data-kind="sprint"] .body {
    fill: transparent;
    stroke: var(--fill);
    stroke-width: 1.5;
    stroke-dasharray: 4 3;
  }
  .bar[role="button"] {
    cursor: grab;
    outline: none;
  }
  .bar[data-moving] .body,
  .bar[role="button"]:focus-visible .body {
    stroke: var(--accent-ink);
    stroke-width: 2;
  }
  .own {
    stroke: var(--ink);
    stroke-width: 2;
  }
  .link {
    fill: none;
    stroke: var(--muted);
    stroke-width: 1.4;
  }
  .link[data-late] {
    stroke: var(--amber);
    stroke-width: 1.6;
  }
  .head {
    fill: var(--muted);
  }
  .head[data-late] {
    fill: var(--amber);
  }
  .today {
    stroke: var(--accent);
    stroke-width: 2;
  }
  path.release {
    stroke: var(--ink);
    stroke-width: 2;
    stroke-dasharray: 5 4;
  }
  text.release {
    fill: var(--ink);
    font-size: 10.5px;
    font-weight: 600;
  }
  .warnings {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-8);
    margin: 0 var(--gutter) var(--space-12);
    padding: 0;
    list-style: none;
  }
  .warnings li {
    flex: 1 1 320px;
    padding: 10px 14px;
    border-radius: 10px;
    background: var(--warning-weak);
    line-height: 1.6;
  }
</style>
