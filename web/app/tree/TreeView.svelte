<script lang="ts">
  import { createVirtualizer } from "@tanstack/svelte-virtual";
  import { tick, untrack } from "svelte";
  import type { Attachment } from "svelte/attachments";
  import type { TaskView } from "../../core/api/client";
  import { cliLs } from "../../core/cli";
  import { errorMessage } from "../../core/messages";
  import { core } from "../context";
  import Icon from "../icons/Icon.svelte";
  import KindIcon from "../icons/KindIcon.svelte";
  import { kbdLabel, modKey } from "../keys";
  import { onLinkClick, router } from "../router";
  import { selection } from "../selection.svelte";
  import Empty from "../states/Empty.svelte";
  import { useSubscribe } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import Menu from "../ui/Menu.svelte";
  import StoreBadge from "../ui/StoreBadge.svelte";
  import AddRow from "./AddRow.svelte";
  import { createBelow, moveTask, patchTask } from "./actions";
  import {
    type DropDecision,
    type DropPlace,
    dropDecision,
    rowDnd,
    treeAutoScroll,
    treeDragEnd,
  } from "./dnd";
  import FilterBar from "./FilterBar.svelte";
  import { baseQuery, combine, hasTerm, NO_CANCELED, type QueryBase, withoutTerm } from "./filter";
  import {
    countLabel,
    countMatched,
    type FlatRow,
    flatten,
    ROW_HEIGHT,
    VIRTUALIZE_AT,
  } from "./flatten";
  import { type KeyAction, type KeyInput, keyAction, needsSchema } from "./keyboard";
  import { openBlocker } from "./marks";
  import { checkParent, kindLabel, pickKind } from "./placement";
  import TreeRow from "./TreeRow.svelte";

  type Scope =
    | { kind: "workspace"; store: string; ulid: string }
    | { kind: "project"; ulid: string };
  // The prop is not called scope: Biome reads that name as the HTML attribute.
  let { view: scope }: { view: Scope } = $props();

  const c = core();
  // The selection is one for the whole app: a row of another screen is not one of this.
  selection.set(null);
  const cacheRev = useSubscribe(c.cache, () => ({}));
  const storesRev = useSubscribe(c.stores, () => ({}));
  const schemaRev = useSubscribe(c.schema, () => ({}));

  // --- what the view shows -------------------------------------------------
  const project = $derived.by(() => {
    cacheRev.current;
    return scope.kind === "project" ? c.cache.task(scope.ulid) : undefined;
  });
  const store = $derived(scope.kind === "workspace" ? scope.store : project?.store);
  const workspace = $derived.by(() => {
    storesRev.current;
    if (scope.kind === "workspace") return c.stores.workspace(scope.store, scope.ulid);
    if (!project) return undefined;
    return c.stores
      .workspaces()
      .find((w) => w.store === project.store && w.ulid === project.workspace.ulid);
  });
  const schema = $derived.by(() => {
    schemaRev.current;
    return store ? c.schema.get(store) : undefined;
  });
  const viewRoot = $derived(scope.kind === "project" ? scope.ulid : null);
  // The workspace the address names is not in the stores (removed, or never there).
  const workspaceMissing = $derived.by(() => {
    storesRev.current;
    return scope.kind === "workspace" && !workspace && c.stores.loaded();
  });
  const base = $derived<QueryBase | null>(
    scope.kind === "workspace"
      ? workspace
        ? { kind: "workspace", name: workspace.name }
        : null
      : project
        ? { kind: "project", ulid: project.ulid, id: project.id }
        : null,
  );
  const apiBase = $derived(base ? baseQuery(base, "api") : null);
  const cliBase = $derived(base ? baseQuery(base, "cli") : "");

  type DetailHandle = ReturnType<typeof c.cache.watchDetail>;
  let detail = $state<DetailHandle | null>(null);
  $effect(() => {
    if (scope.kind !== "project") return;
    const h = c.cache.watchDetail(scope.ulid);
    detail = h;
    return () => {
      h.dispose();
      detail = null;
    };
  });
  // Why the project cannot be shown (deleted, not found, not fetched).
  const projectError = $derived.by(() => {
    cacheRev.current;
    return project ? null : (detail?.error() ?? null);
  });
  const notFound = $derived(workspaceMissing || projectError?.status === 404);
  // Keys that need the schema, pressed before it is read, wait here and run once it is.
  let waiting: KeyInput[] = [];
  let schemaFailed = false;
  $effect(() => {
    if (!store) return;
    schemaFailed = false;
    c.schema.load(store).catch((err) => {
      schemaFailed = true;
      if (waiting.length > 0) toast({ message: errorMessage(err) });
      waiting = [];
    });
  });
  $effect(() => {
    if (!schema || waiting.length === 0) return;
    const keys = waiting;
    waiting = [];
    void (async () => {
      for (const k of keys) {
        await dispatch(k);
      }
    })();
  });

  // --- the query -----------------------------------------------------------
  // The default filter leaves the canceled tasks out; clearing the field shows them.
  let applied = $state<string>(NO_CANCELED);

  type TreeHandle = ReturnType<typeof c.cache.watchTree>;
  type ListHandle = ReturnType<typeof c.cache.watchList>;
  let tree = $state<TreeHandle | null>(null);
  let blockedList = $state<ListHandle | null>(null);
  let canceledList = $state<ListHandle | null>(null);

  $effect(() => {
    if (!apiBase || !store) {
      tree = null;
      return;
    }
    const h = c.cache.watchTree({ q: combine(apiBase, applied), tree: true, store });
    tree = h;
    return () => h.dispose();
  });
  $effect(() => {
    if (!apiBase || !store) {
      blockedList = null;
      return;
    }
    const h = c.cache.watchList({ q: combine(apiBase, "is:blocked"), tree: false, store });
    blockedList = h;
    return () => h.dispose();
  });
  $effect(() => {
    if (!apiBase || !store || !hasTerm(applied, NO_CANCELED)) {
      canceledList = null;
      return;
    }
    const q = combine(apiBase, `${withoutTerm(applied, NO_CANCELED)} status:canceled`);
    const h = c.cache.watchList({ q, tree: false, store });
    canceledList = h;
    return () => h.dispose();
  });

  const roots = $derived.by(() => {
    cacheRev.current;
    return tree?.roots() ?? [];
  });
  const loading = $derived.by(() => {
    cacheRev.current;
    return base === null || (tree?.loading() ?? true);
  });
  const treeError = $derived.by(() => {
    cacheRev.current;
    return tree?.error() ?? null;
  });
  const blocked = $derived.by(() => {
    cacheRev.current;
    return new Set(blockedList?.ulids() ?? []);
  });
  const canceledCount = $derived.by(() => {
    cacheRev.current;
    return canceledList ? canceledList.ulids().length : null;
  });

  function isClosed(t: TaskView): boolean {
    const category = c.schema.get(t.store)?.statuses?.find((s) => s.name === t.status)?.category;
    return category === "done" || category === "canceled";
  }

  function taskOf(ulid: string): TaskView | undefined {
    return c.cache.task(ulid);
  }

  // --- the rows --------------------------------------------------------------
  let collapsed = $state<ReadonlySet<string>>(new Set());
  const rows = $derived.by(() => {
    cacheRev.current;
    return flatten(roots, viewRoot, taskOf, collapsed);
  });
  const matched = $derived.by(() => {
    cacheRev.current;
    return countMatched(roots, viewRoot, taskOf);
  });

  // after: the row the input goes below. trail: the rows it went below before, oldest first. The
  // tree reads a task made only after the write, so the input waits at the newest of them that
  // is shown, and moves below the new task once that is.
  type Adding = {
    after: string | null;
    parent: string | null;
    depth: number;
    trail?: string[];
  };
  let adding = $state<Adding | null>(null);
  const MAX_TRAIL = 20;
  type Item = { type: "row"; row: FlatRow } | { type: "add"; depth: number };
  const items = $derived.by<Item[]>(() => {
    const list: Item[] = rows.map((row) => ({ type: "row", row }));
    if (!adding) return list;
    let at = list.length;
    if (adding.after !== null) {
      const anchors = [...(adding.trail ?? []), adding.after];
      let i = -1;
      for (let k = anchors.length - 1; k >= 0 && i < 0; k--) {
        i = rows.findIndex((r) => r.ulid === anchors[k]);
      }
      if (i >= 0) {
        const depth = rows[i]?.depth ?? 0;
        at = i + 1;
        while (at < rows.length && (rows[at]?.depth ?? 0) > depth) at++;
      }
    }
    list.splice(at, 0, { type: "add", depth: adding.depth });
    return list;
  });

  function select(ulid: string | null) {
    selection.set(ulid);
  }
  const FOCUS_FRAMES = 20;
  // Each call takes a number; a call that is no longer the latest gives up its wait.
  let focusSeq = 0;
  async function focusRow(ulid: string) {
    const seq = ++focusSeq;
    const i = items.findIndex((it) => it.type === "row" && it.row.ulid === ulid);
    if (i >= 0) vinst?.scrollToIndex(i);
    await tick();
    // A long list draws the row after the window moves: wait a few frames for it.
    for (let frame = 0; ; frame++) {
      // Another row was asked for, or the add row opened and took the focus: leave it be.
      if (seq !== focusSeq || adding) return;
      const el = scroller?.querySelector<HTMLElement>(`[data-ulid="${ulid}"]`);
      if (el) {
        el.focus();
        return;
      }
      if (!virtualized || frame >= FOCUS_FRAMES) return;
      await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()));
    }
  }
  // The page may move the element of a row when the rows change (the tree is read again after a
  // move, or someone else reorders), and the browser then drops its focus. The row that has the
  // focus before the update gets it back after, if the focus fell to the body; a focus the user
  // put on something else is left alone.
  const ADD_INPUT = "#add"; // the input row, which is not a task
  let heldRow: string | null = null;
  $effect.pre(() => {
    items;
    const el = document.activeElement;
    heldRow =
      el instanceof HTMLElement
        ? (el.getAttribute("data-ulid") ?? (el.closest(".add") ? ADD_INPUT : null))
        : null;
  });
  $effect(() => {
    items;
    const held = heldRow;
    heldRow = null;
    if (!held) return;
    const active = document.activeElement;
    if (active && active !== document.body) return;
    if (held === ADD_INPUT) {
      void tick().then(() => scroller?.querySelector<HTMLElement>(".add input")?.focus());
      return;
    }
    if (rows.some((r) => r.ulid === held)) void focusRow(held);
  });
  function toggle(ulid: string, openIt: boolean) {
    const next = new Set(collapsed);
    if (openIt) next.delete(ulid);
    else next.add(ulid);
    collapsed = next;
  }

  // --- virtual scrolling -------------------------------------------------------
  const virtualized = $derived(items.length > VIRTUALIZE_AT);
  let scroller = $state<HTMLDivElement | null>(null);
  type Virtualizer = {
    getVirtualItems(): { index: number; start: number }[];
    getTotalSize(): number;
    scrollToIndex(i: number): void;
    setOptions(o: { count: number; getScrollElement: () => HTMLDivElement | null }): void;
  };
  let vinst = $state.raw<Virtualizer | null>(null);
  let windowed = $state<{ index: number; start: number }[]>([]);
  let totalSize = $state(0);
  const virtualizer = createVirtualizer<HTMLDivElement, HTMLDivElement>({
    count: 0,
    getScrollElement: () => scroller,
    estimateSize: () => ROW_HEIGHT,
    overscan: 12,
  });
  $effect(() =>
    virtualizer.subscribe((v) => {
      vinst = v as unknown as Virtualizer;
      windowed = v.getVirtualItems();
      totalSize = v.getTotalSize();
    }),
  );
  // A long list draws the rows in view only: the input row is made to be in view.
  const addIndex = $derived(items.findIndex((it) => it.type === "add"));
  $effect(() => {
    if (!virtualized || addIndex < 0) return;
    untrack(() => vinst)?.scrollToIndex(addIndex);
  });
  // The key of a row is its task, so the input row and the rows keep their elements (and the
  // title typed) when the window moves or the list changes above them.
  function itemKey(i: number): string {
    const it = items[i];
    return it ? (it.type === "row" ? it.row.ulid : "add") : `#${i}`;
  }
  // The window is only asked for while the list is long.
  $effect(() => {
    vinst?.setOptions({
      count: virtualized ? items.length : 0,
      getScrollElement: () => scroller,
    });
  });

  // --- keys ------------------------------------------------------------------------
  let menu = $state<{ which: "status" | "kind"; ulid: string } | null>(null);

  function actionOf(k: KeyInput): KeyAction {
    return keyAction(k, { rows, selected: selection.ulid, schema, viewRoot, taskOf });
  }

  function onkeydown(e: KeyboardEvent) {
    if ((e.target as HTMLElement).closest("input, textarea, select, [contenteditable='true']"))
      return;
    const k = { key: e.key, shift: e.shiftKey, alt: e.altKey, mod: modKey(e) };
    if (!schema && needsSchema(k)) {
      e.preventDefault();
      if (schemaFailed) toast({ message: `${store} の schema.yaml を読み込めていません` });
      else waiting.push(k);
      return;
    }
    // A move waits for the moves before it, and the key may act on the rows they leave.
    if (isMoveKey(k) && moves > 0) {
      e.preventDefault();
      void dispatch(k);
      return;
    }
    const a = actionOf(k);
    if (a.type === "none") return;
    e.preventDefault();
    void dispatch(k, a);
  }

  // The keys that move a task: Alt+Up/Down, Tab, Shift+Tab.
  function isMoveKey(k: KeyInput): boolean {
    if (k.mod) return false;
    if (k.key === "Tab") return !k.alt;
    return k.alt && !k.shift && (k.key === "ArrowUp" || k.key === "ArrowDown");
  }

  // The moves (keys and drops) go one after another: a move is decided on the rows the move before
  // it left, so three presses move a task three places. A failure, a refusal, the selection
  // leaving the row, or the view going away drops the moves still waiting.
  let moveChain: Promise<void> = Promise.resolve();
  let moveEpoch = 0;
  let moves = 0;
  let gone = false;
  $effect(() => () => {
    gone = true;
    moveEpoch++;
  });

  // job says whether the move went through; at is the row the selection must stay on.
  function enqueueMove(job: () => Promise<boolean>, at?: string | null): Promise<void> {
    const epoch = moveEpoch;
    moves++;
    moveChain = moveChain.then(async () => {
      try {
        if (epoch !== moveEpoch || (at !== undefined && selection.ulid !== at)) return;
        if (!(await job())) moveEpoch++;
      } finally {
        moves--;
      }
    });
    return moveChain;
  }

  function dispatch(k: KeyInput, decided?: KeyAction): Promise<void> {
    if (!isMoveKey(k)) {
      const a = decided ?? actionOf(k);
      return a.type === "none" ? Promise.resolve() : run(a).then(() => {});
    }
    return enqueueMove(async () => {
      // The rows are those the moves before this one left.
      const a = actionOf(k);
      return a.type === "none" ? true : run(a);
    }, selection.ulid);
  }

  // run says whether the action went through: false for a refusal or a failed write.
  async function run(a: KeyAction): Promise<boolean> {
    try {
      switch (a.type) {
        case "select":
          select(a.ulid);
          await focusRow(a.ulid);
          break;
        case "toggle":
          toggle(a.ulid, a.open);
          break;
        case "add-below": {
          const depth = rows.find((r) => r.ulid === a.after)?.depth ?? 0;
          adding = { after: a.after, parent: a.parent, depth };
          break;
        }
        case "move": {
          const t = taskOf(a.ulid);
          if (!t) break;
          const { commit } = await moveTask(c, t, a.to);
          // Show the new parent's children only once the task is there.
          if (a.to.parent) toggle(a.to.parent, true);
          await focusRow(a.ulid);
          // The next move is decided on the rows this one left: read the tree again.
          if (commit) await c.cache.refreshTrees();
          break;
        }
        case "set-status": {
          const t = taskOf(a.ulid);
          if (t) await patchTask(c, t, { status: a.status });
          break;
        }
        case "menu":
          menu = { which: a.which, ulid: a.ulid };
          break;
        case "reject":
          toast({ message: `${a.rejection.title}。${a.rejection.reason}` });
          return false;
      }
      return true;
    } catch (err) {
      if (!gone) toast({ message: errorMessage(err) });
      return false;
    }
  }

  const menuTask = $derived(menu ? taskOf(menu.ulid) : undefined);
  const menuItems = $derived.by(() => {
    if (!menu || !menuTask || !schema) return [];
    if (menu.which === "status") {
      return (schema.statuses ?? []).map((s) => ({ value: s.name, label: s.label || s.name }));
    }
    const parent = menuTask.parent ? taskOf(menuTask.parent) : undefined;
    return (schema.kinds ?? []).map((k) => {
      const why = checkParent(schema, { ...menuTask, kind: k.name }, parent ?? null);
      return { value: k.name, label: k.label || k.name, disabled: why !== null, hint: why?.title };
    });
  });
  async function pickFromMenu(value: string) {
    const t = menuTask;
    const which = menu?.which;
    menu = null;
    if (!t || !which) return;
    try {
      await patchTask(c, t, which === "status" ? { status: value } : { kind: value });
    } catch (err) {
      toast({ message: errorMessage(err) });
    }
  }

  // --- adding ------------------------------------------------------------------------
  const addKind = $derived.by(() => {
    if (!schema) return null;
    const parentId = adding ? adding.parent : viewRoot;
    const parent = parentId ? taskOf(parentId) : undefined;
    return pickKind(schema, parent?.kind ?? null);
  });
  const whereLabel = $derived(
    scope.kind === "project" && project
      ? `${project.workspace.name} / ${project.title}`
      : (workspace?.name ?? ""),
  );

  function startAdd(a: Adding) {
    adding = a;
  }

  // submitAdd says whether the task was made: the title typed stays when it was not.
  async function submitAdd(title: string): Promise<boolean> {
    const a = adding;
    if (!a || !store || !workspace || !schema) return false;
    const parent = a.parent ? taskOf(a.parent) : undefined;
    const kind = pickKind(schema, parent?.kind ?? null);
    if (!kind) {
      toast({
        message: `${parent ? `${kindLabel(schema, parent.kind)}の下` : "ルート"}に置ける種類がありません（${store} の schema.yaml の kinds）`,
      });
      adding = null;
      return true;
    }
    try {
      const created = await createBelow(c, {
        title,
        kind,
        store,
        parent: a.parent,
        workspace: workspace.prefix,
        after: a.after,
      });
      // Esc (or a click elsewhere) closed the input while the task was being made: it stays closed,
      // and the focus stays where the user put it.
      if (adding !== a) return true;
      select(created.ulid);
      // keep going below the new row, as an outliner does
      const trail = [...(a.trail ?? []), ...(a.after ? [a.after] : [])].slice(-MAX_TRAIL);
      adding = { after: created.ulid, parent: a.parent, depth: a.depth, trail };
      return true;
    } catch (err) {
      toast({ message: errorMessage(err) });
      return false;
    }
  }

  // --- drag and drop ---------------------------------------------------------------------
  let dragging = $state<string | null>(null);
  let over = $state<{ ulid: string; place: DropPlace; decision: DropDecision } | null>(null);

  function decide(sourceUlid: string, targetUlid: string, place: DropPlace): DropDecision | null {
    const dragged = taskOf(sourceUlid);
    const target = taskOf(targetUlid);
    if (!dragged || !target) return null;
    return dropDecision({ dragged, target, place, schema, taskOf });
  }

  function setOver(targetUlid: string, place: DropPlace | null, sourceUlid: string | null) {
    if (!place || !sourceUlid) {
      if (over?.ulid === targetUlid) over = null;
      return;
    }
    if (over?.ulid === targetUlid && over.place === place) return;
    const decision = decide(sourceUlid, targetUlid, place);
    over = decision ? { ulid: targetUlid, place, decision } : null;
  }

  async function dropOn(sourceUlid: string, targetUlid: string, place: DropPlace) {
    over = null;
    dragging = null;
    roomForRefusal = false;
    refusalHeight = 0;
    void enqueueMove(async () => {
      const d = decide(sourceUlid, targetUlid, place);
      const t = taskOf(sourceUlid);
      if (!d || !t) return true;
      if (!d.ok) {
        if (!d.silent && !gone) toast({ message: `${d.title}。${d.reason}` });
        // A refusal drops the moves still waiting, as a refused key does; a drop that changes
        // nothing (silent) does not.
        return d.silent;
      }
      try {
        const { commit } = await moveTask(c, t, d.to);
        // Show the new parent's children only once the task is there.
        if (d.to.parent) toggle(d.to.parent, true);
        if (commit) await c.cache.refreshTrees();
        return true;
      } catch (err) {
        if (!gone) toast({ message: errorMessage(err) });
        return false;
      }
    });
  }

  function dropMarkOf(ulid: string): "before" | "after" | "child" | "reject" | null {
    if (over?.ulid !== ulid) return null;
    if (over.decision.ok) return over.place;
    return over.decision.silent ? null : "reject";
  }
  let refusalHeight = $state(0);
  // The panel's height, taken again when its words change.
  const measureRefusal: Attachment<HTMLElement> = (el) => {
    refusal;
    refusalHeight = Math.max(refusalHeight, el.offsetHeight);
  };
  // Once the panel has shown, the room for it stays until the drag ends: taking it
  // back would shorten the list under the pointer and move the rows.
  let roomForRefusal = $state(false);
  $effect(() => {
    if (refusal) roomForRefusal = true;
  });
  const refusal = $derived(
    over && !over.decision.ok && !over.decision.silent ? over.decision : null,
  );

  // --- reading the screen ---------------------------------------------------------------
  const cli = $derived(
    cliLs(combine(cliBase, applied), true, scope.kind === "workspace" ? store : undefined),
  );
  // One row takes the Tab key: the selected one when it is shown, else the first.
  const rowFocus = $derived(
    rows.find((r) => r.ulid === selection.ulid)?.ulid ?? rows[0]?.ulid ?? null,
  );
  const noTasks = $derived(
    withoutTerm(applied, NO_CANCELED).trim() === "" && !(canceledCount && canceledCount > 0),
  );
  const crumbTitle = $derived(
    scope.kind === "project" ? (project?.title ?? "") : (workspace?.name ?? ""),
  );
  // Spread, because Biome reads a prop called scope as the HTML attribute.
  const emptyProps = $derived({ scope: scope.kind, name: crumbTitle });
  const notFoundProps = $derived({
    scope: scope.kind,
    name: scope.kind === "workspace" ? scope.ulid : "",
  });
</script>

<section class="tree" aria-label="ツリー">
  <header class="head">
    <nav class="crumb" aria-label="パンくず">
      {#if store}
        <StoreBadge {store} tone={c.stores.tone(store)} />
      {/if}
      {#if scope.kind === "project"}
        {#if workspace}
          {@const wsHref = router.href({
            name: "workspace",
            store: workspace.store,
            ulid: workspace.ulid,
          })}
          <a class="ws" href={wsHref} onclick={(e) => onLinkClick(e, wsHref)}>
            <Icon name="workspace" size={16} />{workspace.name}
          </a>
          <span class="sep" aria-hidden="true">›</span>
        {/if}
        <KindIcon kind="project" />
        <h1>{crumbTitle}</h1>
        {#if project}
          <span class="id">{project.id}</span>
        {/if}
      {:else}
        <Icon name="workspace" size={16} />
        <h1>{crumbTitle}</h1>
        {#if workspace}
          <span class="id">{workspace.prefix}</span>
        {/if}
      {/if}
    </nav>
  </header>

  <nav class="tabs" aria-label="ビューの切り替え">
    <span class="tab" aria-current="page">ツリー</span>
  </nav>

  <FilterBar
    base={cliBase}
    {applied}
    count={countLabel(matched, canceledCount)}
    {cli}
    error={treeError}
    onchange={(q) => (applied = q)}
  />

  <div class="body">
    <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
    <div
      class="scroller"
      class:refusing={roomForRefusal}
      style:--refusal-h={refusalHeight > 0 ? `${refusalHeight}px` : undefined}
      role="tree"
      aria-label="{crumbTitle} のタスク"
      tabindex="-1"
      bind:this={scroller}
      {onkeydown}
      {@attach treeAutoScroll()}
      {@attach treeDragEnd(() => {
        dragging = null;
        over = null;
        roomForRefusal = false;
        refusalHeight = 0;
      })}
    >
      {#snippet entry(item: Item)}
        {#if item.type === "row"}
          <TreeRow
            row={item.row}
            {schema}
            selected={selection.ulid === item.row.ulid}
            focusable={rowFocus === item.row.ulid}
            live={c.cache.live(item.row.ulid)}
            blockedBy={openBlocker(item.row.task, taskOf, isClosed, blocked.has(item.row.ulid))}
            onselect={() => select(item.row.ulid)}
            ontoggle={() => toggle(item.row.ulid, !item.row.expanded)}
            dragging={dragging === item.row.ulid}
            dropMark={dropMarkOf(item.row.ulid)}
            dnd={rowDnd(() => ({
              ulid: item.row.ulid,
              depth: item.row.depth,
              openChildren: item.row.hasChildren && item.row.expanded,
              onDragStart: () => (dragging = item.row.ulid),
              onOver: (place, source) => setOver(item.row.ulid, place, source),
              onDrop: (source, place) => void dropOn(source, item.row.ulid, place),
            }))}
          />
        {:else}
          <AddRow
            editing={true}
            depth={item.depth}
            where={whereLabel}
            kindLabel={schema && addKind ? kindLabel(schema, addKind) : ""}
            ready={!!(store && workspace && schema)}
            onstart={() => {}}
            onsubmit={submitAdd}
            oncancel={() => {
              const back = adding?.after ?? selection.ulid;
              adding = null;
              if (back) void focusRow(back);
            }}
          />
        {/if}
      {/snippet}

      {#if notFound}
        <Empty
          variant="notfound"
          {...notFoundProps}
          onHome={() => router.go(router.href({ name: "home" }))}
        />
      {:else if projectError}
        <p class="note" role="alert">{errorMessage(projectError)}</p>
      {:else if loading && rows.length === 0}
        <p class="note">読み込み中…</p>
      {:else if rows.length === 0 && !adding && !treeError}
        {#if noTasks}
          <Empty variant="empty" {...emptyProps} />
        {:else}
          <Empty
            variant="nomatch"
            note={hasTerm(applied, NO_CANCELED) ? "中止を除外しています" : undefined}
            onClear={() => (applied = "")}
          />
        {/if}
      {/if}

      {#if virtualized}
        <div class="vbox" style:height="{totalSize}px">
          {#each windowed as v (itemKey(v.index))}
            {@const item = items[v.index]}
            {#if item}
              <div class="vrow" style:transform="translateY({v.start}px)">
                {@render entry(item)}
              </div>
            {/if}
          {/each}
        </div>
      {:else}
        <div class="list">
          {#each items as item (item.type === "row" ? item.row.ulid : "add")}
            {@render entry(item)}
          {/each}
        </div>
      {/if}

      {#if !adding}
        <div class="foot-add">
          <AddRow
            editing={false}
            where={whereLabel}
            kindLabel={schema && addKind ? kindLabel(schema, addKind) : ""}
            onstart={() => startAdd({ after: null, parent: viewRoot, depth: 0 })}
            onsubmit={submitAdd}
            oncancel={() => {}}
          />
        </div>
      {/if}
    </div>

    {#if refusal && !refusal.ok}
      <aside class="refusal" role="status" {@attach measureRefusal}>
        <strong>{refusal.title}</strong>
        <p>{refusal.reason}</p>
      </aside>
    {/if}
  </div>

  <footer class="keys">
    <span><kbd>Enter</kbd> 下に追加</span>
    <span><kbd>{kbdLabel("tab")}</kbd> / <kbd>{kbdLabel("shift+tab")}</kbd> 字下げ・戻す</span>
    <span>
      <kbd>{kbdLabel("alt+arrowup")}</kbd> <kbd>{kbdLabel("alt+arrowdown")}</kbd> 並べ替え
    </span>
    <span><kbd>Space</kbd> 完了</span>
    <span><kbd>S</kbd> ステータス</span>
    <span><kbd>K</kbd> 種類</span>
    <span class="dnd">ドラッグで並べ替え・親の付け替え（置ける親の種類だけ受け付けます）</span>
  </footer>
</section>

{#if menu && menuItems.length > 0}
  <Menu
    anchor={scroller?.querySelector<HTMLElement>(`[data-ulid="${menu.ulid}"]`) ?? null}
    items={menuItems}
    onselect={(v) => void pickFromMenu(v)}
    onclose={() => {
      const u = menu?.ulid;
      menu = null;
      if (u) void focusRow(u);
    }}
  />
{/if}

<style>
  .tree {
    box-sizing: border-box;
    display: flex;
    flex-direction: column;
    height: 100%;
    min-width: 0;
    min-height: 0;
  }
  .head {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.75rem;
    padding: 1.125rem 1.75rem 0;
  }
  .crumb {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.625rem;
    min-width: 0;
  }
  .ws {
    display: inline-flex;
    align-items: center;
    gap: 0.375rem;
    color: var(--ink-2);
    text-decoration: none;
  }
  .sep {
    color: var(--muted-2);
  }
  h1 {
    margin: 0;
    font-size: 1.19rem;
    font-weight: 600;
  }
  .id {
    font-family: var(--font-mono);
    font-size: 0.75rem;
    color: var(--muted);
  }
  .tabs {
    display: flex;
    gap: 0.25rem;
    padding: 0.625rem 1.75rem 0;
    border-bottom: var(--hairline) solid var(--line);
  }
  .tab {
    display: inline-flex;
    align-items: center;
    min-height: 2.25rem;
    padding: 0 0.75rem;
    margin-bottom: calc(var(--hairline) * -1);
    border-bottom: 2px solid var(--ink);
    color: var(--ink);
    font-weight: 600;
  }
  .body {
    position: relative;
    flex: 1 1 auto;
    min-height: 0;
  }
  .scroller {
    box-sizing: border-box;
    height: 100%;
    overflow: auto;
    padding: 0 1rem;
    outline: none;
  }
  /* The refusal lies over the foot of the list: leave room to scroll the last rows above it. */
  .scroller.refusing {
    padding-bottom: calc(var(--refusal-h, 6rem) + 1rem);
  }
  .list {
    display: flex;
    flex-direction: column;
    gap: var(--hairline);
  }
  .vbox {
    position: relative;
    width: 100%;
  }
  .vrow {
    position: absolute;
    top: 0;
    left: 0;
    width: 100%;
  }
  .foot-add {
    margin-top: 0.375rem;
  }
  .note {
    margin: 1rem 0.75rem;
    color: var(--muted);
  }
  .keys {
    display: flex;
    flex-wrap: wrap;
    gap: 0.25rem 1.125rem;
    padding: 0.625rem 1.75rem;
    border-top: var(--hairline) solid var(--line);
    color: var(--muted);
    font-size: 0.75rem;
  }
  kbd {
    padding: 0 0.3rem;
    border: var(--hairline) solid var(--line);
    border-radius: var(--radius-4);
    background: var(--sunken);
    font-family: var(--font-mono);
    font-size: 0.7rem;
    color: var(--ink-2);
  }
  .refusal {
    /* Over the list, not in the flow: showing it must not resize the scroller. */
    position: absolute;
    right: 1rem;
    bottom: 0.5rem;
    left: 1rem;
    z-index: 1;
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
    font-size: 0.78rem;
    color: var(--ink-2);
  }
  .dnd {
    margin-left: auto;
  }
</style>
