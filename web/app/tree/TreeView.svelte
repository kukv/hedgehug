<script lang="ts">
  import { createVirtualizer } from "@tanstack/svelte-virtual";
  import { onDestroy, type Snippet, tick, untrack } from "svelte";
  import type { Attachment } from "svelte/attachments";
  import type { TaskView } from "../../core/api/client";
  import { cliLs } from "../../core/cli";
  import { KINDS, kindLabel } from "../../core/kinds";
  import { errorMessage } from "../../core/messages";
  import { core } from "../context";
  import { openCreate } from "../create-state.svelte";
  import { storeShown } from "../hidden";
  import { kbdLabel, modKey } from "../keys";
  import ListBar from "../list/ListBar.svelte";
  import type { ListScope } from "../list/scope";
  import { useListScope } from "../list/scope.svelte";
  import { readOnlyNow, readOnlyToast } from "../read-only";
  import { router } from "../router";
  import { selection } from "../selection.svelte";
  import Empty from "../states/Empty.svelte";
  import { useSubscribe } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import Menu from "../ui/Menu.svelte";
  import { openStoreMove, openWorkspaceMove } from "../workspace/dialogs-state.svelte";
  import {
    MOVE_STORE_VALUE,
    MOVE_VALUE,
    moveStoreItem,
    moveWorkspaceItem,
  } from "../workspace/move-menu";
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
  import { combine, hasTerm, NO_CANCELED, withoutTerm } from "./filter";
  import {
    countLabel,
    countMatched,
    type FlatRow,
    flatten,
    ROW_HEIGHT,
    VIRTUALIZE_AT,
  } from "./flatten";
  import { type KeyAction, type KeyInput, keyAction, needsSchema, writeTargetOf } from "./keyboard";
  import { openBlocker } from "./marks";
  import { checkParent, pickKind } from "./placement";
  import { orderRootsByStore, storesOf } from "./stores-order";
  import TreeRow from "./TreeRow.svelte";

  type Props = {
    view: ListScope;
    // The filter, when the screen that holds this view owns it (a saved view, the query screen).
    // Without it the view keeps the filter itself, from -status:canceled.
    query?: string;
    onquery?: (q: string) => void;
    // The line at the foot, in place of the hh ls of the filter.
    cli?: string;
    // At the right of the filter bar.
    actions?: Snippet;
  };
  // The prop is not called scope: Biome reads that name as the HTML attribute.
  let { view: scope, query, onquery, cli: cliLine, actions }: Props = $props();

  const c = core();
  // The selection is one for the whole app: a row of another screen is not one of this.
  selection.set(null);
  // And a row of this screen is not one of the next: a task detail that opens a create must not find it.
  onDestroy(() => selection.set(null));
  const cacheRev = useSubscribe(c.cache, () => ({}));
  const storesRev = useSubscribe(c.stores, () => ({}));
  const schemaRev = useSubscribe(c.schema, () => ({}));

  // --- what the view shows -------------------------------------------------
  // Where the view is (a workspace, a project, or every store) is resolved once, by useListScope.
  const ls = useListScope(() => scope);
  const all = $derived(scope.kind === "all");
  const viewRoot = $derived(scope.kind === "project" ? scope.ulid : null);
  // A view over every store lists the tasks of several stores, so each row has the schema of its
  // own store: the kinds, the statuses and the keys are read from that one, never from a single schema.
  function schemaOf(store: string | undefined) {
    schemaRev.current;
    return store ? c.schema.get(store) : undefined;
  }
  function toneOf(store: string) {
    storesRev.current;
    return c.stores.tone(store);
  }
  function workspaceOf(store: string | undefined, ulid: string | undefined) {
    storesRev.current;
    return store && ulid ? c.stores.workspace(store, ulid) : undefined;
  }
  // The stores in the order of GET /api/stores: the order a view over every store lists them in.
  const storeOrder = $derived.by(() => {
    storesRev.current;
    return all ? c.stores.stores().map((s) => s.name) : [];
  });
  // Each row shows its store when the view can hold the tasks of more than one.
  const multi = $derived(all && storeOrder.length >= 2);

  // Keys that need the schema, pressed before it is read, wait here with the store of the task they
  // act on, and run once the schema of that store is read, in the order pressed. A store whose
  // schema could not be read is in failed; failed is read only while the schema is missing.
  // ulid is the row that was selected when the key was pressed: the key runs only if it still is.
  type Waiting = { key: KeyInput; store: string; ulid: string | null };
  let waiting: Waiting[] = [];
  const failed = new Set<string>();
  // The stores of the rows. A child is in the store of its parent, so the roots hold them all. The
  // value is a string, so the schemas are asked for again only when the set of stores changes.
  const storesKey = $derived.by(() => {
    cacheRev.current;
    return (ls.store ? [ls.store] : storesOf(roots, taskOf)).join("\n");
  });
  $effect(() => {
    for (const store of storesKey === "" ? [] : storesKey.split("\n")) {
      failed.delete(store);
      c.schema.load(store).catch((err) => {
        failed.add(store);
        if (waiting.some((w) => w.store === store)) toast({ message: errorMessage(err) });
        waiting = waiting.filter((w) => w.store !== store);
      });
    }
  });
  $effect(() => {
    schemaRev.current;
    if (waiting.length === 0) return;
    // The keys go in order: one that waits for a schema holds back the ones after it.
    const n = waiting.findIndex((w) => !c.schema.get(w.store));
    const keys = n < 0 ? waiting : waiting.slice(0, n);
    if (keys.length === 0) return;
    waiting = n < 0 ? [] : waiting.slice(n);
    void (async () => {
      for (const w of keys) {
        if (selection.ulid !== w.ulid) continue;
        await dispatch(w.key);
      }
    })();
  });
  // The keys act on the selected task, so on the schema of its store.
  const keyStore = $derived.by(() => {
    cacheRev.current;
    const selected = selection.ulid ? taskOf(selection.ulid) : undefined;
    return selected?.store ?? ls.store;
  });
  const keySchema = $derived(schemaOf(keyStore));

  // --- the query -----------------------------------------------------------
  // The default filter leaves the canceled tasks out; clearing the field shows them.
  let own = $state<string>(NO_CANCELED);
  const applied = $derived(query ?? own);
  function setApplied(q: string) {
    if (query === undefined) own = q;
    onquery?.(q);
  }

  type TreeHandle = ReturnType<typeof c.cache.watchTree>;
  type ListHandle = ReturnType<typeof c.cache.watchList>;
  let tree = $state<TreeHandle | null>(null);
  let blockedList = $state<ListHandle | null>(null);
  let canceledList = $state<ListHandle | null>(null);

  // A view over every store asks without a store: the answer lists the tasks of all of them.
  $effect(() => {
    if (!ls.ready) {
      tree = null;
      return;
    }
    const h = c.cache.watchTree({ q: combine(ls.apiBase, applied), tree: true, store: ls.store });
    tree = h;
    return () => h.dispose();
  });
  $effect(() => {
    if (!ls.ready) {
      blockedList = null;
      return;
    }
    const h = c.cache.watchList({
      q: combine(ls.apiBase, "is:blocked"),
      tree: false,
      store: ls.store,
    });
    blockedList = h;
    return () => h.dispose();
  });
  $effect(() => {
    if (!ls.ready || !hasTerm(applied, NO_CANCELED)) {
      canceledList = null;
      return;
    }
    const q = combine(ls.apiBase, `${withoutTerm(applied, NO_CANCELED)} status:canceled`);
    const h = c.cache.watchList({ q, tree: false, store: ls.store });
    canceledList = h;
    return () => h.dispose();
  });

  // The roots of a view over every store come in one answer. The order by store is derived from the
  // tree, the tasks and the stores as they are each time this is computed, and kept nowhere: when
  // any of them changes later, the order is made again, so no moment of arrival can leave it wrong.
  const roots = $derived.by(() => {
    cacheRev.current;
    storesRev.current;
    const r = tree?.roots() ?? [];
    return all
      ? orderRootsByStore(
          r.filter((n) => inScope(taskOf(n.ulid)?.store)),
          storeOrder,
          taskOf,
        )
      : r;
  });
  const loading = $derived.by(() => {
    cacheRev.current;
    return !ls.ready || (tree?.loading() ?? true);
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
    storesRev.current;
    return canceledList
      ? canceledList.ulids().filter((u) => inScope(taskOf(u)?.store)).length
      : null;
  });

  function isClosed(t: TaskView): boolean {
    const category = c.schema.get(t.store)?.statuses?.find((s) => s.name === t.status)?.category;
    return category === "done" || category === "canceled";
  }

  function taskOf(ulid: string): TaskView | undefined {
    return c.cache.task(ulid);
  }

  // A view over every store leaves out the tasks of a hidden store (the page of a workspace or a
  // project is of its own store, so it is not left out).
  // (Read storesRev where this is used, so the result follows the directory.)
  function inScope(store: string | undefined): boolean {
    return !all || storeShown(c.stores, store);
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
  // is shown, and moves below the new task once that is. store and workspace (its ULID) are where
  // the task is made: those of the row it goes below, or of the view for the input at the foot.
  type Adding = {
    after: string | null;
    parent: string | null;
    depth: number;
    store: string | undefined;
    workspace: string | undefined;
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
  function open(ulid: string) {
    router.go(router.href({ name: "task", ulid }));
  }
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
  // "row" is the "…" button of a row; S and K open the other two.
  let menu = $state<{ which: "status" | "kind" | "row"; ulid: string } | null>(null);

  // Why the Store of a task cannot be written (in Japanese), or null.
  function readOnlyOf(store: string | undefined): string | null {
    storesRev.current;
    return store === undefined ? null : readOnlyNow(c.stores, store);
  }

  function actionOf(k: KeyInput): KeyAction {
    return keyAction(k, {
      rows,
      selected: selection.ulid,
      schema: keySchema,
      viewRoot,
      taskOf,
      idOf: (t) => c.stores.displayId(t.store, t.id),
    });
  }

  function onkeydown(e: KeyboardEvent) {
    if ((e.target as HTMLElement).closest("input, textarea, select, [contenteditable='true']"))
      return;
    const k = { key: e.key, shift: e.shiftKey, alt: e.altKey, mod: modKey(e) };
    if (keyStore && !keySchema && needsSchema(k)) {
      e.preventDefault();
      if (failed.has(keyStore))
        toast({ message: `${keyStore} の schema.yaml を読み込めていません` });
      else waiting.push({ key: k, store: keyStore, ulid: selection.ulid });
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
    // A Store that cannot be written refuses before anything else, even the refusals of the schema.
    const target = writeTargetOf(a, selection.ulid);
    const why = target === null ? null : readOnlyOf(taskOf(target)?.store);
    if (why !== null) {
      readOnlyToast(why);
      return false;
    }
    try {
      switch (a.type) {
        case "select":
          select(a.ulid);
          await focusRow(a.ulid);
          break;
        case "toggle":
          toggle(a.ulid, a.open);
          break;
        case "open-detail":
          open(a.ulid);
          break;
        case "add-below": {
          const row = rows.find((r) => r.ulid === a.after);
          // The task goes into the store and the workspace of the row it goes below.
          adding = {
            after: a.after,
            parent: a.parent,
            depth: row?.depth ?? 0,
            store: row?.task.store,
            workspace: row?.task.workspace.ulid,
          };
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
    const menuSchema = schemaOf(menuTask?.store);
    if (!menu || !menuTask || !menuSchema) return [];
    if (menu.which === "row") {
      storesRev.current;
      return [
        moveWorkspaceItem(menuTask, c.stores.workspaces(), readOnlyOf(menuTask.store)),
        moveStoreItem(c.stores.stores(), readOnlyOf(menuTask.store)),
      ].filter((i) => i !== null);
    }
    if (menu.which === "status") {
      return (menuSchema.statuses ?? []).map((s) => ({ value: s.name, label: s.label || s.name }));
    }
    const parent = menuTask.parent ? taskOf(menuTask.parent) : undefined;
    return KINDS.map((k) => {
      const why = checkParent({ ...menuTask, kind: k.name }, parent ?? null, (t) =>
        c.stores.displayId(t.store, t.id),
      );
      return { value: k.name, label: k.label, disabled: why !== null, hint: why?.title };
    });
  });
  async function pickFromMenu(value: string) {
    const t = menuTask;
    const which = menu?.which;
    menu = null;
    if (!t || !which) return;
    if (which === "row") {
      if (value === MOVE_VALUE) openWorkspaceMove(t);
      else if (value === MOVE_STORE_VALUE) openStoreMove(t);
      return;
    }
    try {
      await patchTask(c, t, which === "status" ? { status: value } : { kind: value });
    } catch (err) {
      toast({ message: errorMessage(err) });
    }
  }

  // --- adding ------------------------------------------------------------------------
  // Where a task made now would go: the row it goes below, else the view's own workspace.
  // An input at the foot of a view of one store carries neither (see startFootAdd): they are read
  // from the view at the moment they are used, so the input works once the workspace is loaded.
  const addStore = $derived(adding?.store ?? ls.store);
  const addWorkspace = $derived(
    adding?.workspace ? workspaceOf(adding.store, adding.workspace) : ls.workspace,
  );
  const addKind = $derived.by(() => {
    const parentId = adding ? adding.parent : viewRoot;
    const parent = parentId ? taskOf(parentId) : undefined;
    return pickKind(parent?.kind ?? null);
  });
  const whereLabel = $derived.by(() => {
    if (all) return addWorkspace ? `${addWorkspace.store} / ${addWorkspace.name}` : "";
    return scope.kind === "project" && ls.project
      ? `${ls.project.workspace.name} / ${ls.project.title}`
      : (ls.workspace?.name ?? "");
  });

  // The place a task would be added to cannot be written: the input is not shown (and goes if it was).
  $effect(() => {
    if (adding && readOnlyOf(addStore) !== null) adding = null;
  });

  function startAdd(a: Adding) {
    adding = a;
  }
  // The input at the foot: a view over every store has no workspace of its own to make a task in, so
  // the create dialog, which asks for one, opens instead.
  function startFootAdd() {
    if (all) openCreate({ kind: "task" });
    else
      startAdd({
        after: null,
        parent: viewRoot,
        depth: 0,
        store: undefined,
        workspace: undefined,
      });
  }

  // submitAdd says whether the task was made: the title typed stays when it was not.
  async function submitAdd(title: string): Promise<boolean> {
    const a = adding;
    const store = addStore;
    const workspace = addWorkspace;
    if (!a || !store || !workspace) return false;
    const parent = a.parent ? taskOf(a.parent) : undefined;
    const kind = pickKind(parent?.kind ?? null);
    if (!kind) {
      toast({
        message: `${parent ? `${kindLabel(parent.kind)} の下` : "最上位"}に置ける種類がありません`,
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
      adding = {
        after: created.ulid,
        parent: a.parent,
        depth: a.depth,
        store: a.store,
        workspace: a.workspace,
        trail,
      };
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
    return dropDecision({
      dragged,
      target,
      place,
      taskOf,
      readOnly: refusingStore(target.store, dragged.store),
      idOf: (t) => c.stores.displayId(t.store, t.id),
    });
  }

  // The first of the Stores that cannot be written, with the reason.
  function refusingStore(...stores: string[]): { store: string; why: string } | null {
    for (const store of stores) {
      const why = readOnlyOf(store);
      if (why !== null) return { store, why };
    }
    return null;
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
  const footCli = $derived(
    cliLine ??
      cliLs(combine(ls.cliBase, applied), true, scope.kind === "workspace" ? ls.store : undefined),
  );
  // One row takes the Tab key: the selected one when it is shown, else the first.
  const rowFocus = $derived(
    rows.find((r) => r.ulid === selection.ulid)?.ulid ?? rows[0]?.ulid ?? null,
  );
  const noTasks = $derived(
    withoutTerm(applied, NO_CANCELED).trim() === "" && !(canceledCount && canceledCount > 0),
  );
</script>

<section class="tree" aria-label="ツリー">
  <!-- The heading and the tabs of a workspace (WorkspaceHeader) and of a project (ProjectHeader)
       sit above this view, in their pages. -->
  <ListBar
    base={ls.cliBase}
    {applied}
    count={countLabel(matched, canceledCount)}
    cli={footCli}
    error={treeError}
    onchange={setApplied}
    {actions}
  />

  <div class="body">
    <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
    <div
      class="scroller"
      class:refusing={roomForRefusal}
      style:--refusal-h={refusalHeight > 0 ? `${refusalHeight}px` : undefined}
      role="tree"
      aria-label={all ? "タスク" : `${ls.crumbTitle} のタスク`}
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
            schema={schemaOf(item.row.task.store)}
            storeTone={multi ? toneOf(item.row.task.store) : undefined}
            selected={selection.ulid === item.row.ulid}
            focusable={rowFocus === item.row.ulid}
            live={c.cache.live(item.row.ulid)}
            blockedBy={openBlocker(item.row.task, taskOf, isClosed, blocked.has(item.row.ulid))}
            onselect={() => {
              // A click on a row closes the input row, and what was typed with it.
              adding = null;
              select(item.row.ulid);
            }}
            ontoggle={() => toggle(item.row.ulid, !item.row.expanded)}
            onopen={() => open(item.row.ulid)}
            onmore={schemaOf(item.row.task.store)
              ? () => (menu = { which: "row", ulid: item.row.ulid })
              : undefined}
            dragging={dragging === item.row.ulid}
            dropMark={dropMarkOf(item.row.ulid)}
            dnd={rowDnd(() => ({
              ulid: item.row.ulid,
              depth: item.row.depth,
              openChildren: item.row.hasChildren && item.row.expanded,
              onDragStart: () => (dragging = item.row.ulid),
              locked: readOnlyOf(item.row.task.store) !== null,
              onOver: (place, source) => setOver(item.row.ulid, place, source),
              onDrop: (source, place) => void dropOn(source, item.row.ulid, place),
            }))}
          />
        {:else}
          <AddRow
            editing={true}
            depth={item.depth}
            where={whereLabel}
            kindLabel={addKind ? kindLabel(addKind) : ""}
            ready={!!(addStore && addWorkspace)}
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

      {#if ls.notFound}
        <Empty
          variant="notfound"
          {...ls.notFoundProps}
          onHome={() => router.go(router.href({ name: "home" }))}
        />
      {:else if ls.error}
        <p class="note" role="alert">{errorMessage(ls.error)}</p>
      {:else if loading && rows.length === 0}
        <p class="note">読み込み中…</p>
      {:else if rows.length === 0 && !adding && !treeError}
        {#if noTasks && !all}
          <Empty
            variant="empty"
            {...ls.emptyProps}
            onCreate={() => openCreate({ kind: "task", parent: viewRoot ?? undefined })}
          />
        {:else}
          <Empty
            variant="nomatch"
            note={hasTerm(applied, NO_CANCELED) ? "中止を除外しています" : undefined}
            onClear={applied.trim() === "" ? undefined : () => setApplied("")}
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

      {#if !adding && readOnlyOf(ls.store) === null}
        <div class="foot-add">
          <AddRow
            editing={false}
            where={all ? "" : whereLabel}
            kindLabel={addKind ? kindLabel(addKind) : ""}
            onstart={startFootAdd}
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
    <span><kbd>{kbdLabel("mod+enter")}</kbd> 詳細を開く</span>
    <span class="dnd"
      >ドラッグで並べ替え・親の付け替え（どの下に置けるかに合う場所だけ受け付けます）</span
    >
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
