<script lang="ts">
  import { onDestroy, type Snippet } from "svelte";
  import { type ShowView, type TaskView, unwrap } from "../../core/api/client";
  import { cliLs, cliViewSave, Placeholder } from "../../core/cli";
  import { errorMessage } from "../../core/messages";
  import type { SchemaView } from "../../core/schema";
  import type { KanbanSettings } from "../../core/views";
  import { core } from "../context";
  import { openCreate } from "../create-state.svelte";
  import Icon from "../icons/Icon.svelte";
  import StatusIcon from "../icons/StatusIcon.svelte";
  import ListBar from "../list/ListBar.svelte";
  import type { ListScope } from "../list/scope";
  import { useListScope } from "../list/scope.svelte";
  import { router } from "../router";
  import { selection } from "../selection.svelte";
  import Empty from "../states/Empty.svelte";
  import { useSubscribe } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import { patchTask } from "../tree/actions";
  import { combine, hasTerm, NO_CANCELED, withoutTerm } from "../tree/filter";
  import { countLabel } from "../tree/flatten";
  import { openBlocker } from "../tree/marks";
  import Avatar from "../ui/Avatar.svelte";
  import CliLine from "../ui/CliLine.svelte";
  import Menu from "../ui/Menu.svelte";
  import { buildBoard, type Column, NO_LANE } from "./board";
  import { boardAutoScroll, boardDragEnd, cardDnd, cellDnd, type DragSource } from "./dnd";
  import { type DropDecision, decideDrop, scopedStatusesOf } from "./drop";
  import KanbanCard from "./KanbanCard.svelte";
  import RequiredDialog from "./RequiredDialog.svelte";

  type Props = {
    view: ListScope;
    query: string;
    onquery: (q: string) => void;
    settings: KanbanSettings;
    onsettings: (s: KanbanSettings) => void;
    cli?: string;
    actions?: Snippet;
  };
  // The prop is not called scope: Biome reads that name as the HTML attribute.
  let { view: scope, query, onquery, settings, onsettings, cli, actions }: Props = $props();

  const c = core();
  const ls = useListScope(() => scope);
  // The selection is one for the whole app: a row of another screen is not a card of this.
  selection.set(null);
  onDestroy(() => selection.set(null));
  const cacheRev = useSubscribe(c.cache, () => ({}));
  const storesRev = useSubscribe(c.stores, () => ({}));
  const schemaRev = useSubscribe(c.schema, () => ({}));

  // --- the tasks -------------------------------------------------------------
  const q = $derived(combine(ls.apiBase, query).trim());
  type ListHandle = ReturnType<typeof c.cache.watchList>;
  let list = $state<ListHandle | null>(null);
  let blockedList = $state<ListHandle | null>(null);
  let canceledList = $state<ListHandle | null>(null);

  $effect(() => {
    if (!ls.ready) {
      list = null;
      return;
    }
    const h = c.cache.watchList({ q, tree: false, store: ls.store });
    list = h;
    return () => h.dispose();
  });
  $effect(() => {
    if (!ls.ready) {
      blockedList = null;
      return;
    }
    const h = c.cache.watchList({
      q: combine(ls.apiBase, "is:blocked").trim(),
      tree: false,
      store: ls.store,
    });
    blockedList = h;
    return () => h.dispose();
  });
  // The default filter leaves the canceled tasks out; the count says how many that is.
  $effect(() => {
    if (!ls.ready || !hasTerm(query, NO_CANCELED)) {
      canceledList = null;
      return;
    }
    const h = c.cache.watchList({
      q: combine(ls.apiBase, `${withoutTerm(query, NO_CANCELED)} status:canceled`).trim(),
      tree: false,
      store: ls.store,
    });
    canceledList = h;
    return () => h.dispose();
  });

  const tasks = $derived.by(() => {
    cacheRev.current;
    return (list?.ulids() ?? []).flatMap((u) => {
      const t = c.cache.task(u);
      return t ? [t] : [];
    });
  });
  const loading = $derived.by(() => {
    cacheRev.current;
    return !ls.ready || (list?.loading() ?? true);
  });
  const listError = $derived.by(() => {
    cacheRev.current;
    return list?.error() ?? null;
  });
  const blocked = $derived.by(() => {
    cacheRev.current;
    return new Set(blockedList?.ulids() ?? []);
  });
  const canceledCount = $derived.by(() => {
    cacheRev.current;
    return canceledList ? canceledList.ulids().length : null;
  });

  // --- the schemas: the store of each task (and of the screen) -------------------
  const storesNeeded = $derived([
    ...new Set([...tasks.map((t) => t.store), ...(ls.store ? [ls.store] : [])]),
  ]);
  let failed = $state<ReadonlySet<string>>(new Set());
  $effect(() => {
    for (const store of storesNeeded) {
      c.schema.load(store).catch(() => {
        failed = new Set([...failed, store]);
      });
    }
  });
  const schemas = $derived.by(() => {
    schemaRev.current;
    const m = new Map<string, SchemaView>();
    for (const s of storesNeeded) {
      const v = c.schema.get(s);
      if (v) m.set(s, v);
    }
    return m;
  });
  // A store whose schema could not be read and has been read since is not a failure any more.
  const unread = $derived([...failed].filter((s) => !schemas.has(s)));
  const schemaWaiting = $derived(storesNeeded.some((s) => !schemas.has(s) && !failed.has(s)));
  const storeOrder = $derived.by(() => {
    storesRev.current;
    return c.stores.stores().map((s) => s.name);
  });
  // With two stores or more, each card says which store it is of.
  const manyStores = $derived(storeOrder.length >= 2);

  // --- the board -------------------------------------------------------------------
  const scopedStatuses = $derived.by(() => {
    if (scope.kind !== "project" || !ls.store) return undefined;
    const sc = schemas.get(ls.store);
    return sc ? scopedStatusesOf(sc, scope.ulid) : undefined;
  });
  const board = $derived(
    buildBoard(tasks, {
      mode: settings.columns,
      swimlane: settings.swimlane,
      schemas,
      scopedStatuses,
      storeOrder,
      store: ls.store,
    }),
  );
  // How many tasks a column has: a task with two assignees is in two rows but counts once.
  const counts = $derived(
    board.columns.map(
      (_, i) => new Set(board.lanes.flatMap((l) => l.cells[i] ?? []).map((t) => t.ulid)).size,
    ),
  );

  function isClosed(t: TaskView): boolean {
    const category = schemas.get(t.store)?.statuses?.find((s) => s.name === t.status)?.category;
    return category === "done" || category === "canceled";
  }
  const taskOf = (ulid: string) => c.cache.task(ulid);

  function open(ulid: string) {
    router.go(router.href({ name: "task", ulid }));
  }

  // --- drag and drop ---------------------------------------------------------------------
  let dragging = $state<DragSource | null>(null);
  let over = $state<{ column: string; lane: string; decision: DropDecision } | null>(null);
  let required = $state<{
    task: TaskView;
    status: string;
    fields: string[];
    elsewhere: string[];
    schema: SchemaView;
  } | null>(null);

  function decide(
    source: DragSource,
    column: Column,
    lane: string,
    detail?: ShowView,
  ): DropDecision | null {
    const t = taskOf(source.ulid);
    if (!t) return null;
    return decideDrop({
      task: t,
      column,
      sameLane: source.lane === lane,
      swimlane: settings.swimlane,
      mode: settings.columns,
      schema: schemas.get(t.store),
      detail,
    });
  }

  // The columns the dragged card cannot go to, in its own row: they show a red frame at once.
  const forbidden = $derived.by(() => {
    const out = new Set<string>();
    const d = dragging;
    if (!d) return out;
    for (const col of board.columns) {
      const r = decide(d, col, d.lane);
      if (r && !r.ok && !r.silent) out.add(col.key);
    }
    return out;
  });

  function markOf(col: Column, lane: string): "ok" | "reject" | null {
    if (over && over.column === col.key && over.lane === lane) {
      if (over.decision.ok) return "ok";
      return over.decision.silent ? null : "reject";
    }
    return dragging && dragging.lane === lane && forbidden.has(col.key) ? "reject" : null;
  }

  function setOver(col: Column, lane: string, source: DragSource | null) {
    if (!source) {
      if (over?.column === col.key && over.lane === lane) over = null;
      return;
    }
    const decision = decide(source, col, lane);
    over = decision ? { column: col.key, lane, decision } : null;
  }

  // The full task, for the required fields the list does not carry (watchers and repos).
  async function detailOf(t: TaskView): Promise<ShowView> {
    const have = c.cache.detail(t.ulid);
    if (have) return have;
    return unwrap(
      c.api.GET("/api/tasks/{id}", { params: { path: { id: t.ulid }, query: { store: t.store } } }),
    );
  }

  // The drops go one after another: a drop is decided on the card the drop before it left, so a card
  // dragged again while its write is in flight is not judged on its old status. A drop another one
  // has come after gives up its dialog, and a board that is gone shows and sends nothing more.
  let dropChain: Promise<void> = Promise.resolve();
  let dropSeq = 0;
  let gone = false;
  $effect(() => () => {
    gone = true;
  });

  function dropOn(col: Column, lane: string, source: DragSource) {
    over = null;
    dragging = null;
    required = null;
    const seq = ++dropSeq;
    dropChain = dropChain.then(() => runDrop(col, lane, source, seq));
  }

  async function runDrop(col: Column, lane: string, source: DragSource, seq: number) {
    if (gone) return;
    const t = taskOf(source.ulid);
    let d = decide(source, col, lane);
    if (!t || !d) return;
    try {
      if (d.ok && d.elsewhere.some((f) => f === "watchers" || f === "repos")) {
        const detail = await detailOf(t);
        if (gone || seq !== dropSeq) return;
        d = decide(source, col, lane, detail) ?? d;
      }
    } catch (err) {
      if (!gone) toast({ message: errorMessage(err) });
      return;
    }
    if (!d.ok) {
      if (!d.silent) toast({ message: `${d.title}。${d.reason}` });
      return;
    }
    const schema = schemas.get(t.store);
    if ((d.fill.length > 0 || d.elsewhere.length > 0) && schema) {
      required = { task: t, status: d.status, fields: d.fill, elsewhere: d.elsewhere, schema };
      return;
    }
    try {
      await patchTask(c, t, { status: d.status });
    } catch (err) {
      if (!gone) toast({ message: errorMessage(err) });
    }
  }

  const refusal = $derived(
    over && !over.decision.ok && !over.decision.silent ? over.decision : null,
  );

  // --- the buttons ---------------------------------------------------------------------------
  // A column's "+" opens the create dialog in that status (in that category, by category).
  function addTo(col: Column) {
    openCreate(
      col.status !== null
        ? { kind: "task", status: col.status, category: col.category }
        : { kind: "task", category: col.key },
    );
  }

  let menu = $state<"columns" | "swimlane" | null>(null);
  let columnsButton = $state<HTMLButtonElement | null>(null);
  let swimlaneButton = $state<HTMLButtonElement | null>(null);
  const COLUMN_ITEMS = [
    { value: "status", label: "ステータス" },
    { value: "category", label: "カテゴリ" },
  ];
  const SWIMLANE_ITEMS = [
    { value: "none", label: "なし" },
    { value: "assignee", label: "担当者" },
    { value: "project", label: "プロジェクト" },
  ];
  const labelIn = (items: { value: string; label: string }[], v: string) =>
    items.find((i) => i.value === v)?.label ?? v;

  function pick(value: string) {
    const which = menu;
    menu = null;
    if (which === "columns")
      onsettings({ ...settings, columns: value as KanbanSettings["columns"] });
    if (which === "swimlane") {
      onsettings({ ...settings, swimlane: value as KanbanSettings["swimlane"] });
    }
  }

  // --- reading the screen ------------------------------------------------------------------------
  const lsLine = $derived(
    cli ??
      cliLs(
        combine(ls.cliBase, query).trim(),
        false,
        scope.kind === "workspace" ? ls.store : undefined,
      ),
  );
  // The settings of this screen as the command that saves them in a view.
  const saveLine = $derived(
    cliViewSave({
      name: new Placeholder("名前"),
      body: {
        layout: "kanban",
        kanban_columns: settings.columns,
        swimlane: settings.swimlane,
      },
    }),
  );
  const noTasks = $derived(
    withoutTerm(query, NO_CANCELED).trim() === "" && !(canceledCount && canceledCount > 0),
  );
  const crumbTitle = $derived(
    scope.kind === "project" ? (ls.project?.title ?? "") : (ls.workspace?.name ?? ""),
  );
  const emptyScope = $derived<"project" | "workspace">(
    scope.kind === "project" ? "project" : "workspace",
  );
  // Spread, because Biome reads a prop called scope as the HTML attribute.
  const emptyProps = $derived({ scope: emptyScope, name: crumbTitle });
  const notFoundProps = $derived({
    scope: emptyScope,
    name: scope.kind === "workspace" ? scope.ulid : "",
  });
  const showBoard = $derived(!loading && tasks.length > 0 && !schemaWaiting);
</script>

<section class="kanban" aria-label="カンバン">
  <ListBar
    base={ls.cliBase}
    applied={query}
    count={countLabel(tasks.length, canceledCount)}
    cli={lsLine}
    error={listError}
    onchange={onquery}
    {actions}
  >
    {#snippet controls()}
      <button
        type="button"
        class="ctl"
        bind:this={columnsButton}
        aria-haspopup="menu"
        aria-expanded={menu === "columns"}
        onclick={() => (menu = menu === "columns" ? null : "columns")}
      >
        列: {labelIn(COLUMN_ITEMS, settings.columns)}
        <Icon name="chevron-down" size={12} />
      </button>
      <button
        type="button"
        class="ctl"
        bind:this={swimlaneButton}
        aria-haspopup="menu"
        aria-expanded={menu === "swimlane"}
        onclick={() => (menu = menu === "swimlane" ? null : "swimlane")}
      >
        スイムレーン: {labelIn(SWIMLANE_ITEMS, settings.swimlane)}
        <Icon name="chevron-down" size={12} />
      </button>
    {/snippet}
  </ListBar>

  <div class="body">
    {#if ls.notFound}
      <Empty
        variant="notfound"
        {...notFoundProps}
        onHome={() => router.go(router.href({ name: "home" }))}
      />
    {:else if loading || (tasks.length > 0 && schemaWaiting)}
      <p class="note">読み込み中…</p>
    {:else if tasks.length === 0}
      {#if !listError}
        {#if scope.kind !== "all" && noTasks}
          <Empty
            variant="empty"
            {...emptyProps}
            onCreate={() =>
              openCreate({
                kind: "task",
                parent: scope.kind === "project" ? scope.ulid : undefined,
              })}
          />
        {:else}
          <Empty
            variant="nomatch"
            note={hasTerm(query, NO_CANCELED) ? "中止を除外しています" : undefined}
            onClear={query.trim() === "" ? undefined : () => onquery("")}
          />
        {/if}
      {/if}
    {/if}

    {#if unread.length > 0 && showBoard}
      <p class="note" role="alert">
        {unread.join("、")}
        の schema.yaml を読み込めていません。列は読み込めたものだけです
      </p>
    {/if}

    {#if showBoard}
      <section
        class="scroller"
        aria-label="カンバンの盤面"
        {@attach boardAutoScroll()}
        {@attach boardDragEnd(() => {
          dragging = null;
          over = null;
        })}
      >
        <div class="board" style:--cols={board.columns.length}>
          <div class="heads">
            {#each board.columns as col, i (col.key)}
              <div class="head" data-column={col.key}>
                <StatusIcon status={col.key} category={col.category} />
                <span class="name">{col.label}</span>
                <span class="count">{counts[i]}</span>
                {#if !col.unknown}
                  <button
                    type="button"
                    class="add"
                    aria-label="{col.label} にタスクを追加"
                    onclick={() => addTo(col)}
                  >
                    <Icon name="plus" size={14} />
                  </button>
                {/if}
              </div>
            {/each}
          </div>

          {#each board.lanes as lane (lane.key)}
            <section class="lane" aria-label={lane.label || undefined}>
              {#if settings.swimlane !== "none"}
                <h3 class="lane-head">
                  {#if settings.swimlane === "assignee" && lane.key !== NO_LANE}
                    <Avatar name={lane.key} />
                  {/if}
                  <span title={lane.key !== NO_LANE ? lane.key : undefined}>{lane.label}</span>
                  <span class="count">
                    {new Set(lane.cells.flat().map((t) => t.ulid)).size}
                  </span>
                </h3>
              {/if}
              <div class="row">
                {#each board.columns as col, i (col.key)}
                  <ul
                    class="cell"
                    aria-label="{col.label}{lane.label ? ` · ${lane.label}` : ""}"
                    data-column={col.key}
                    data-lane={lane.key}
                    data-drop={markOf(col, lane.key) ?? undefined}
                    {@attach cellDnd(() => ({
                      onOver: (s) => setOver(col, lane.key, s),
                      onDrop: (s) => dropOn(col, lane.key, s),
                    }))}
                  >
                    {#each lane.cells[i] ?? [] as t (t.ulid)}
                      <KanbanCard
                        task={t}
                        schema={schemas.get(t.store)}
                        blockedBy={openBlocker(t, taskOf, isClosed, blocked.has(t.ulid))}
                        tone={manyStores ? c.stores.tone(t.store) : undefined}
                        dragging={dragging?.ulid === t.ulid && dragging.lane === lane.key}
                        onopen={() => open(t.ulid)}
                        dnd={cardDnd(() => ({
                          ulid: t.ulid,
                          lane: lane.key,
                          onDragStart: () => (dragging = { ulid: t.ulid, lane: lane.key }),
                        }))}
                      />
                    {/each}
                  </ul>
                {/each}
              </div>
            </section>
          {/each}
        </div>
      </section>

      {#if refusal && !refusal.ok}
        <aside class="refusal" role="status">
          <strong>{refusal.title}</strong>
          <p>{refusal.reason}</p>
        </aside>
      {/if}
    {/if}
  </div>

  <footer class="keys">
    <span>ドラッグでステータスを変更（遷移の制約に合わない列は赤枠）</span>
    <span class="save"><CliLine command={saveLine} /></span>
  </footer>
</section>

{#if menu === "columns"}
  <Menu
    anchor={columnsButton}
    items={COLUMN_ITEMS}
    label="列"
    selected={settings.columns}
    onselect={pick}
    onclose={() => (menu = null)}
  />
{:else if menu === "swimlane"}
  <Menu
    anchor={swimlaneButton}
    items={SWIMLANE_ITEMS}
    label="スイムレーン"
    selected={settings.swimlane}
    onselect={pick}
    onclose={() => (menu = null)}
  />
{/if}

{#if required}
  <RequiredDialog
    task={required.task}
    status={required.status}
    fields={required.fields}
    elsewhere={required.elsewhere}
    schema={required.schema}
    onsubmit={async (body) => {
      if (required) await patchTask(c, required.task, body);
    }}
    onclose={() => (required = null)}
  />
{/if}

<style>
  .kanban {
    box-sizing: border-box;
    display: flex;
    flex-direction: column;
    height: 100%;
    min-width: 0;
    min-height: 0;
  }
  .ctl {
    display: inline-flex;
    align-items: center;
    gap: var(--space-6);
    height: 2.25rem;
    padding: 0 var(--space-10);
    border: var(--hairline) solid var(--line-input);
    border-radius: var(--radius-8);
    background: var(--surface);
    color: var(--ink-2);
    font-size: var(--fs-13);
    white-space: nowrap;
    cursor: pointer;
  }
  .ctl:hover {
    background: var(--hover);
  }
  .body {
    position: relative;
    flex: 1 1 auto;
    min-height: 0;
    display: flex;
    flex-direction: column;
  }
  .note {
    margin: 1rem 1.75rem;
    color: var(--muted);
  }
  .scroller {
    flex: 1 1 auto;
    min-height: 0;
    overflow: auto;
    padding: 0 1.75rem 1rem;
  }
  .board {
    display: flex;
    flex-direction: column;
    gap: var(--space-12);
    width: max-content;
    min-width: 100%;
  }
  .heads,
  .row {
    display: grid;
    grid-template-columns: repeat(var(--cols), 17rem);
    gap: var(--space-10);
  }
  .heads {
    position: sticky;
    top: 0;
    z-index: 2;
    padding: var(--space-4) 0;
    background: var(--surface);
  }
  .head {
    display: flex;
    align-items: center;
    gap: var(--space-8);
    min-width: 0;
    padding: 0 var(--space-4);
  }
  .name {
    font-size: var(--fs-13);
    font-weight: 600;
    color: var(--ink);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .count {
    font-size: var(--fs-12);
    color: var(--muted);
  }
  .add {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 24px;
    height: 24px;
    margin-left: auto;
    border: 0;
    border-radius: var(--radius-6);
    background: transparent;
    color: var(--muted);
    cursor: pointer;
  }
  .add:hover {
    background: var(--hover);
    color: var(--ink);
  }
  .lane {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
  }
  .lane-head {
    display: flex;
    align-items: center;
    gap: var(--space-8);
    margin: 0;
    font-size: var(--fs-13);
    font-weight: 600;
    color: var(--ink-2);
  }
  .cell {
    margin: 0;
    list-style: none;
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
    min-height: 5rem;
    padding: var(--space-8);
    border-radius: var(--radius-10);
    background: var(--sunken);
  }
  .cell[data-drop="ok"] {
    background: var(--accent-faint);
    box-shadow: inset 0 0 0 calc(var(--hairline) * 1.5) var(--accent);
  }
  .cell[data-drop="reject"] {
    background: var(--danger-weak);
    box-shadow: inset 0 0 0 calc(var(--hairline) * 1.5) var(--danger);
  }
  .refusal {
    /* Over the board, not in the flow: showing it must not resize the board. */
    position: absolute;
    right: 1.75rem;
    bottom: 0.5rem;
    left: 1.75rem;
    z-index: 3;
    pointer-events: none;
    padding: 0.625rem 0.875rem;
    border: var(--hairline) solid var(--danger);
    border-radius: var(--radius-8);
    background: var(--danger-weak);
    color: var(--ink);
  }
  .refusal strong {
    color: var(--danger);
  }
  .refusal p {
    margin: 0.25rem 0 0;
    font-size: var(--fs-12);
    color: var(--ink-2);
  }
  .keys {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.25rem 1.125rem;
    padding: 0.625rem 1.75rem;
    border-top: var(--hairline) solid var(--line);
    color: var(--muted);
    font-size: var(--fs-12);
  }
  .save {
    margin-left: auto;
    min-width: 0;
  }
</style>
