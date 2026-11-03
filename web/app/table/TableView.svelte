<script lang="ts">
  import {
    constructSortFn,
    createSortedRowModel,
    createTable,
    rowSortingFeature,
    type SortingState,
    tableFeatures,
  } from "@tanstack/svelte-table";
  import { createVirtualizer } from "@tanstack/svelte-virtual";
  import type { Snippet } from "svelte";
  import type { TaskView } from "../../core/api/client";
  import { cliLs, cliViewSave, Placeholder } from "../../core/cli";
  import type { PatchField } from "../../core/save";
  import type { TableSettings } from "../../core/views";
  import { core } from "../context";
  import { openCreate } from "../create-state.svelte";
  import Icon from "../icons/Icon.svelte";
  import ListBar from "../list/ListBar.svelte";
  import type { ListScope } from "../list/scope";
  import { useListScope } from "../list/scope.svelte";
  import { router } from "../router";
  import Empty from "../states/Empty.svelte";
  import { useSubscribe } from "../svelte-store";
  import { combine, hasTerm, NO_CANCELED, withoutTerm } from "../tree/filter";
  import { countLabel, VIRTUALIZE_AT } from "../tree/flatten";
  import Button from "../ui/Button.svelte";
  import CliLine from "../ui/CliLine.svelte";
  import Menu, { type MenuItem } from "../ui/Menu.svelte";
  import {
    type ColumnSpec,
    columnCandidates,
    compareKeys,
    patchFieldOf,
    type SortCtx,
    STORE_COLUMN,
    shownColumns,
    sortKey,
    toggleColumn,
  } from "./columns";
  import { GROUP_HEIGHT, type GroupBy, groupTasks, ROW_HEIGHT, type TableItem } from "./group";
  import TableCell from "./TableCell.svelte";

  let {
    view,
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
    settings: TableSettings;
    onsettings: (s: TableSettings) => void;
    cli?: string;
    actions?: Snippet;
  } = $props();

  const c = core();
  const scope = useListScope(() => view);
  const cacheRev = useSubscribe(c.cache, () => ({}));
  const storesRev = useSubscribe(c.stores, () => ({}));
  const schemaRev = useSubscribe(c.schema, () => ({}));
  // A fresh object on every change of the SaveController.
  const saves = useSubscribe(c.save, () => ({
    editing: (ulid: string, field: PatchField) => c.save.editing(ulid, field),
    state: (ulid: string) => c.save.state(ulid),
  }));

  // The cells read the cache and the schemas through these, so they follow a change.
  const taskOf = (ulid: string): TaskView | undefined => {
    cacheRev.current;
    return c.cache.task(ulid);
  };
  const schemaOf = (store: string) => {
    schemaRev.current;
    return c.schema.get(store);
  };

  // --- what the table lists ----------------------------------------------------------
  const joinQuery = (base: string, q: string) => combine(base, q).trim();

  type ListHandle = ReturnType<typeof c.cache.watchList>;
  let list = $state<ListHandle | null>(null);
  let canceledList = $state<ListHandle | null>(null);
  $effect(() => {
    if (!scope.ready) {
      list = null;
      return;
    }
    const h = c.cache.watchList({
      q: joinQuery(scope.apiBase, query),
      tree: false,
      store: scope.store,
    });
    list = h;
    return () => h.dispose();
  });
  // The tasks the default filter leaves out, to say how many.
  $effect(() => {
    if (!scope.ready || !hasTerm(query, NO_CANCELED)) {
      canceledList = null;
      return;
    }
    const q = joinQuery(scope.apiBase, `${withoutTerm(query, NO_CANCELED)} status:canceled`);
    const h = c.cache.watchList({ q, tree: false, store: scope.store });
    canceledList = h;
    return () => h.dispose();
  });

  const tasks = $derived.by(() => {
    cacheRev.current;
    return (list?.ulids() ?? [])
      .map((u) => c.cache.task(u))
      .filter((t): t is TaskView => t !== undefined);
  });
  const loading = $derived.by(() => {
    cacheRev.current;
    return !scope.ready || (list?.loading() ?? true);
  });
  const listError = $derived.by(() => {
    cacheRev.current;
    return list?.error() ?? null;
  });
  const canceledCount = $derived.by(() => {
    cacheRev.current;
    return canceledList ? canceledList.ulids().length : null;
  });

  // --- the schemas -----------------------------------------------------------------------
  const storeNames = $derived.by(() => {
    storesRev.current;
    return c.stores.stores().map((s) => s.name);
  });
  // The tasks of more than one store are told apart by their store.
  const showStore = $derived(view.kind === "all" && storeNames.length >= 2);
  const taskStores = $derived([...new Set(tasks.map((t) => t.store))]);
  // The columns on offer are the properties of the stores the screen shows; with no task to tell,
  // every store.
  const shownStores = $derived(
    scope.store ? [scope.store] : taskStores.length > 0 ? taskStores : storeNames,
  );
  const storeKey = $derived([...new Set([...taskStores, ...shownStores])].join("\n"));
  let schemaFailed = $state<string[]>([]);
  $effect(() => {
    for (const store of storeKey.split("\n")) {
      if (store === "") continue;
      c.schema.load(store).then(
        () => (schemaFailed = schemaFailed.filter((s) => s !== store)),
        () =>
          (schemaFailed = schemaFailed.includes(store) ? schemaFailed : [...schemaFailed, store]),
      );
    }
  });
  const schemas = $derived.by(() => {
    schemaRev.current;
    return scope.schemas(shownStores);
  });

  // --- the table -----------------------------------------------------------------------------
  const ctx = $derived.by<SortCtx>(() => {
    cacheRev.current;
    schemaRev.current;
    return {
      schemaOf: (store) => c.schema.get(store),
      taskOf: (ulid) => c.cache.task(ulid),
      storeOrder: storeNames,
    };
  });
  const candidates = $derived(columnCandidates(schemas));
  const shown = $derived(shownColumns(settings.columns, candidates));
  const specs = $derived<ColumnSpec[]>(showStore ? [STORE_COLUMN, ...shown] : shown);

  const features = tableFeatures({ rowSortingFeature, sortedRowModel: createSortedRowModel() });
  const sortFn = constructSortFn({ sort: compareKeys });
  // The accessor of a column gives the value the column sorts by; the cells are drawn from the task.
  const columnDefs = $derived(
    specs.map((spec) => ({
      id: spec.id,
      header: spec.label,
      accessorFn: (t: TaskView) => sortKey(t, spec.id, ctx),
      sortFn,
      sortUndefined: "last" as const,
    })),
  );
  // The order of the headers is not saved: a sort to keep is written in the filter as sort:.
  let sorting = $state<SortingState>([]);
  const table = createTable({
    features,
    // A click sorts ascending first, then descending, then back to the order of the query.
    sortDescFirst: false,
    enableMultiSort: false,
    getRowId: (t: TaskView) => t.ulid,
    get columns() {
      return columnDefs;
    },
    get data() {
      return tasks;
    },
    state: {
      get sorting() {
        return sorting;
      },
    },
    onSortingChange: (u) => {
      sorting = typeof u === "function" ? u(sorting) : u;
    },
  });
  const headers = $derived(table.getHeaderGroups()[0]?.headers ?? []);
  const sorted = $derived(table.getRowModel().rows.map((r) => r.original));

  // The rows and the headings of the groups, flat.
  const items = $derived(groupTasks(sorted, settings.groupBy, ctx));
  const GUTTER = 1.75; // rem, the mark of a save on its way or failed
  const template = $derived(
    `${GUTTER}rem ${specs.map((s) => (s.id === "title" ? `minmax(${s.width}rem, 1fr)` : `${s.width}rem`)).join(" ")}`,
  );
  const minWidth = $derived(GUTTER + specs.reduce((sum, s) => sum + s.width, 0));

  // --- editing a cell --------------------------------------------------------------------------
  function edit(t: TaskView, field: PatchField, value: unknown) {
    c.save.edit(t.ulid, t.store, field, value);
  }
  const commit = (t: TaskView) => void c.save.flush(t.ulid);

  // --- virtual scrolling -------------------------------------------------------------------
  const virtualized = $derived(items.length > VIRTUALIZE_AT);
  let scroller = $state<HTMLDivElement | null>(null);
  type Virtualizer = {
    getVirtualItems(): { index: number; start: number }[];
    getTotalSize(): number;
    setOptions(o: { count: number; getScrollElement: () => HTMLDivElement | null }): void;
  };
  let vinst = $state.raw<Virtualizer | null>(null);
  let windowed = $state<{ index: number; start: number }[]>([]);
  let totalSize = $state(0);
  // The key of an item: the rows keep their elements when the window moves or the list changes.
  function itemKey(i: number): string {
    const it = items[i];
    return it ? (it.type === "row" ? it.key : `#${it.key}`) : `#${i}`;
  }
  const virtualizer = createVirtualizer<HTMLDivElement, HTMLDivElement>({
    count: 0,
    getScrollElement: () => scroller,
    estimateSize: (i) => (items[i]?.type === "group" ? GROUP_HEIGHT : ROW_HEIGHT),
    getItemKey: itemKey,
    overscan: 12,
  });
  $effect(() =>
    virtualizer.subscribe((v) => {
      vinst = v as unknown as Virtualizer;
      windowed = v.getVirtualItems();
      totalSize = v.getTotalSize();
    }),
  );
  // The window is only asked for while the list is long.
  $effect(() => {
    vinst?.setOptions({
      count: virtualized ? items.length : 0,
      getScrollElement: () => scroller,
    });
  });

  // --- the controls ------------------------------------------------------------------------------
  const GROUPS: { value: GroupBy; label: string }[] = [
    { value: "none", label: "なし" },
    { value: "parent", label: "親" },
    { value: "status", label: "ステータス" },
    { value: "assignee", label: "担当者" },
    { value: "project", label: "プロジェクト" },
  ];
  const groupLabel = $derived(GROUPS.find((g) => g.value === settings.groupBy)?.label ?? "なし");
  let menu = $state<{ which: "group" | "columns"; anchor: HTMLElement } | null>(null);
  const shownIds = $derived(new Set(shown.map((s) => s.id)));
  const columnItems = $derived<MenuItem[]>(
    candidates.map((s) => {
      const last = shownIds.size === 1 && shownIds.has(s.id);
      return {
        value: s.id,
        label: s.label,
        disabled: last,
        hint: last ? "最後の 1 列は外せません" : undefined,
      };
    }),
  );
  function pickColumn(id: string) {
    onsettings({ ...settings, columns: toggleColumn(settings.columns, id, !shownIds.has(id)) });
  }
  function pickGroup(value: string) {
    menu = null;
    onsettings({ ...settings, groupBy: value as GroupBy });
  }

  // --- reading the screen -----------------------------------------------------------------------
  const lsCli = $derived(
    cliLs(
      joinQuery(scope.cliBase, query),
      false,
      view.kind === "workspace" ? view.store : undefined,
    ),
  );
  // Where the settings of the table would be saved as a view; the name is the user's to give.
  const saveCli = $derived(
    cliViewSave({
      name: new Placeholder("名前"),
      body: {
        layout: "table",
        query: joinQuery(scope.saveBase, query),
        columns: settings.columns,
        group_by: settings.groupBy,
      },
    }),
  );
  const noTasks = $derived(
    withoutTerm(query, NO_CANCELED).trim() === "" && !(canceledCount && canceledCount > 0),
  );
  const crumbTitle = $derived(
    view.kind === "project" ? (scope.project?.title ?? "") : (scope.workspace?.name ?? ""),
  );
  // Spread, because Biome reads a prop called scope as the HTML attribute.
  const emptyProps = $derived({
    scope: view.kind === "workspace" ? ("workspace" as const) : ("project" as const),
    name: crumbTitle,
  });
  const notFoundProps = $derived({
    scope: view.kind === "workspace" ? ("workspace" as const) : ("project" as const),
    name: view.kind === "workspace" ? view.ulid : "",
  });
  const labelOf = (id: string) => specs.find((s) => s.id === id)?.label ?? id;
  const groupMeta = (g: Extract<TableItem, { type: "group" }>) =>
    g.progress ? `${g.progress.done}/${g.progress.total} 完了` : "";
</script>

{#snippet controls()}
  <div class="controls">
    <Button
      size="sm"
      aria-haspopup="menu"
      aria-expanded={menu?.which === "group"}
      onclick={(e) =>
        (menu = menu?.which === "group" ? null : { which: "group", anchor: e.currentTarget })}
    >
      グループ: {groupLabel}
    </Button>
    <Button
      size="sm"
      aria-haspopup="menu"
      aria-expanded={menu?.which === "columns"}
      onclick={(e) =>
        (menu = menu?.which === "columns" ? null : { which: "columns", anchor: e.currentTarget })}
    >
      列 {shown.length}
    </Button>
    <!-- The name is not given yet, so the line is not complete to copy. -->
    <CliLine command={saveCli} disabled />
  </div>
{/snippet}

{#snippet columnItem(it: MenuItem)}
  <span class="check" class:on={shownIds.has(it.value)} aria-hidden="true">
    {#if shownIds.has(it.value)}
      <Icon name="check" size={12} />
    {/if}
  </span>
  <span class="item-label">{it.label}</span>
  <span class="sr">{shownIds.has(it.value) ? "表示中" : "非表示"}</span>
{/snippet}

{#snippet entry(item: TableItem, start?: number)}
  {#if item.type === "group"}
    <tr
      class="group"
      class:placed={start !== undefined}
      style:transform={start === undefined ? undefined : `translateY(${start}px)`}
    >
      <td class="ghead" colspan={specs.length + 1}>
        <span class="gname">{item.label}</span>
        {#if item.id}
          <span class="gid">{item.id}</span>
        {/if}
        {#if groupMeta(item)}
          <span class="gmeta">{groupMeta(item)}</span>
        {/if}
        <span class="gmeta">{item.count} 件</span>
      </td>
    </tr>
  {:else}
    {@const t = item.task}
    {@const st = saves.current.state(t.ulid)}
    <tr
      class="row"
      class:placed={start !== undefined}
      style:transform={start === undefined ? undefined : `translateY(${start}px)`}
      data-ulid={t.ulid}
    >
      <td class="mark">
        {#if st.kind === "saving"}
          <span role="img" aria-label="保存中" title="保存中…">…</span>
        {:else if st.kind === "error"}
          <button
            type="button"
            class="retry"
            aria-label="保存に失敗しました。やり直す: {st.message}"
            title={st.message}
            onclick={() => commit(t)}
          >
            <Icon name="warning" size={14} />
          </button>
        {/if}
      </td>
      {#each specs as spec (spec.id)}
        {@const field = patchFieldOf(spec)}
        <td class="cell" class:link={spec.id === "id" || spec.id === "parent"}>
          <TableCell
            task={t}
            col={spec}
            schema={schemaOf(t.store)}
            tone={c.stores.tone(t.store)}
            pending={field ? saves.current.editing(t.ulid, field) : false}
            {taskOf}
            onedit={(f, v) => edit(t, f, v)}
            oncommit={() => commit(t)}
          />
        </td>
      {/each}
    </tr>
  {/if}
{/snippet}

<section class="table" aria-label="テーブル">
  <ListBar
    base={scope.cliBase}
    applied={query}
    count={countLabel(tasks.length, canceledCount)}
    cli={cli ?? lsCli}
    error={listError}
    onchange={onquery}
    {controls}
    {actions}
  />

  <div class="body">
    <div class="scroller" bind:this={scroller}>
      <div class="inner" style:--cols={template} style:min-width="{minWidth}rem">
        <!-- The table elements are laid out as a grid, so that the columns line up and the
          virtualizer can place a row; they keep their roles. -->
        <table aria-label="タスクの表" aria-colcount={specs.length + 1}>
          <thead>
            <tr class="head">
              <th class="hcell" scope="col"><span class="sr">保存の状態</span></th>
              {#each headers as h (h.id)}
                {@const dir = h.column.getIsSorted()}
                <th
                  class="hcell"
                  scope="col"
                  aria-sort={dir === "asc" ? "ascending" : dir === "desc" ? "descending" : "none"}
                >
                  <button type="button" class="sort" onclick={h.column.getToggleSortingHandler()}>
                    {labelOf(h.column.id)}
                    <span class="dir" aria-hidden="true">
                      {dir === "asc" ? "↑" : dir === "desc" ? "↓" : ""}
                    </span>
                  </button>
                </th>
              {/each}
            </tr>
          </thead>
          {#if virtualized}
            <tbody class="vbox" style:height="{totalSize}px">
              {#each windowed as v (itemKey(v.index))}
                {@const item = items[v.index]}
                {#if item}
                  {@render entry(item, v.start)}
                {/if}
              {/each}
            </tbody>
          {:else}
            <tbody>
              {#each items as item (item.type === "row" ? item.key : `#${item.key}`)}
                {@render entry(item)}
              {/each}
            </tbody>
          {/if}
        </table>

        {#if scope.notFound}
          <Empty
            variant="notfound"
            {...notFoundProps}
            onHome={() => router.go(router.href({ name: "home" }))}
          />
        {:else if loading && items.length === 0}
          <p class="note">読み込み中…</p>
        {:else if tasks.length === 0 && !listError}
          {#if view.kind !== "all" && noTasks}
            <Empty
              variant="empty"
              {...emptyProps}
              onCreate={() =>
                openCreate({
                  kind: "task",
                  parent: view.kind === "project" ? view.ulid : undefined,
                })}
            />
          {:else}
            <Empty
              variant="nomatch"
              note={hasTerm(query, NO_CANCELED) ? "中止を除外しています" : undefined}
              onClear={() => onquery("")}
            />
          {/if}
        {/if}
        {#each schemaFailed as store (store)}
          <p class="note" role="alert">{store} の schema.yaml を読み込めていません</p>
        {/each}
      </div>
    </div>
  </div>

  <footer class="foot">
    <span
      >セルは直接編集できます。入力が 2 秒止まるか、フォーカスが外れたときに 1 操作 1
      コミットで保存します。</span
    >
    <span
      >見出しのクリックの並び替えは保存されません。保存したい並びはフィルタ欄の sort:
      に書きます。</span
    >
  </footer>
</section>

{#if menu?.which === "group"}
  <Menu
    anchor={menu.anchor}
    items={GROUPS}
    label="グループ"
    selected={settings.groupBy}
    onselect={pickGroup}
    onclose={() => (menu = null)}
  />
{:else if menu?.which === "columns"}
  <Menu
    anchor={menu.anchor}
    items={columnItems}
    label="列"
    item={columnItem}
    onselect={pickColumn}
    onclose={() => (menu = null)}
  />
{/if}

<style>
  .table {
    box-sizing: border-box;
    display: flex;
    flex-direction: column;
    height: 100%;
    min-width: 0;
    min-height: 0;
  }
  .controls {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-8);
    padding: 0 var(--gutter) var(--space-8);
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
    padding: 0 var(--space-16);
  }
  .inner {
    position: relative;
  }
  /* The table is a grid of rows; every row has the same columns (--cols). */
  table,
  thead,
  tbody {
    display: grid;
    margin: 0;
    padding: 0;
    border-collapse: collapse;
  }
  thead {
    position: sticky;
    top: 0;
    z-index: 4;
    background: var(--surface);
  }
  tr {
    display: grid;
    grid-template-columns: var(--cols);
    align-items: center;
  }
  .head {
    height: var(--control-h);
    border-bottom: var(--hairline) solid var(--line);
  }
  .hcell {
    min-width: 0;
    padding: 0 var(--space-4);
    font-weight: normal;
    text-align: left;
  }
  .sort {
    display: inline-flex;
    align-items: center;
    gap: var(--space-4);
    max-width: 100%;
    padding: 0 var(--space-8);
    border: 0;
    border-radius: var(--radius-4);
    background: transparent;
    color: var(--muted);
    font: inherit;
    font-size: var(--fs-12);
    font-weight: 600;
    cursor: pointer;
  }
  .sort:hover {
    background: var(--hover);
    color: var(--ink);
  }
  .sort:focus-visible {
    outline: 2px solid var(--accent);
  }
  .dir {
    color: var(--accent);
  }
  .vbox {
    position: relative;
    width: 100%;
  }
  /* The virtualizer places a row by its offset. */
  .placed {
    position: absolute;
    top: 0;
    left: 0;
    width: 100%;
  }
  .row {
    height: var(--row-h);
    border-bottom: var(--hairline) solid var(--line-soft);
  }
  .row:hover {
    background: var(--sunken);
  }
  /* The ID pops its ULID over the rows below it. */
  .row:hover,
  .row:focus-within {
    position: relative;
    z-index: 3;
  }
  .row.placed:hover,
  .row.placed:focus-within {
    position: absolute;
  }
  /* A cell keeps to its row: chips that do not fit are cut off until the cell has the focus. The
    ID and the parent pop their ULID out of the cell. */
  .cell {
    display: flex;
    align-items: center;
    min-width: 0;
    height: var(--row-h);
    padding: 0 var(--space-4);
    overflow: hidden;
  }
  .cell.link,
  .cell:focus-within {
    overflow: visible;
  }
  .cell > :global(*) {
    min-width: 0;
  }
  /* The chips of a cell stay on one line, so that the row keeps its height. */
  .cell :global(.chips) {
    flex-wrap: nowrap;
  }
  .mark {
    display: flex;
    justify-content: center;
    color: var(--muted);
  }
  .retry {
    display: inline-flex;
    padding: 0;
    border: 0;
    background: transparent;
    color: var(--danger);
    cursor: pointer;
  }
  .group {
    display: flex;
    align-items: center;
    height: var(--control-h);
    background: var(--sunken);
    border-bottom: var(--hairline) solid var(--line-soft);
  }
  .ghead {
    display: flex;
    align-items: baseline;
    gap: var(--space-8);
    padding: 0 var(--space-12);
    font-size: var(--fs-13);
  }
  .gname {
    font-weight: 600;
    color: var(--ink);
  }
  .gid {
    font-family: var(--font-mono);
    font-size: var(--fs-mono);
    color: var(--muted);
  }
  .gmeta {
    font-size: var(--fs-12);
    color: var(--muted);
  }
  .note {
    margin: var(--space-16) var(--space-12);
    color: var(--muted);
  }
  .foot {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-4) var(--space-16);
    padding: var(--space-10) var(--gutter);
    border-top: var(--hairline) solid var(--line);
    color: var(--muted);
    font-size: var(--fs-12);
  }
  .check {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 16px;
    height: 16px;
    border: 1px solid var(--line-input);
    border-radius: var(--radius-4);
    background: var(--surface);
    color: var(--accent-on);
  }
  .check.on {
    border-color: var(--accent);
    background: var(--accent);
  }
  .item-label {
    flex: 1;
  }
  .sr {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip: rect(0 0 0 0);
    white-space: nowrap;
  }
</style>
