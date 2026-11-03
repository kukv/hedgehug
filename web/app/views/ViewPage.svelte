<script lang="ts">
  import { cliViewSave, cliViewShow } from "../../core/cli";
  import { errorMessage } from "../../core/messages";
  import {
    draftOf,
    type KanbanSettings,
    sameDraft,
    saveBodyOf,
    type TableSettings,
    type ViewDraft,
  } from "../../core/views";
  import { core } from "../context";
  import Icon from "../icons/Icon.svelte";
  import KanbanView from "../kanban/KanbanView.svelte";
  import type { ListScope } from "../list/scope";
  import { type Layout, router } from "../router";
  import Empty from "../states/Empty.svelte";
  import { useSubscribe } from "../svelte-store";
  import TableView from "../table/TableView.svelte";
  import { toast } from "../toast.svelte";
  import TreeView from "../tree/TreeView.svelte";
  import Button from "../ui/Button.svelte";
  import CliLine from "../ui/CliLine.svelte";
  import Menu from "../ui/Menu.svelte";
  import StoreBadge from "../ui/StoreBadge.svelte";
  import Tabs from "../ui/Tabs.svelte";
  import { openViewDelete, openViewRename, takeCarriedDraft } from "./dialogs-state.svelte";
  import { LAYOUTS, treeOrderProblem } from "./draft";

  // A saved view (細部 50): it opens in the layout it was saved with, and the tabs change that
  // layout in place (a PUT of the layout only).
  type Props = { store: string; name: string };
  let { store, name }: Props = $props();

  const { views } = core();
  const found = useSubscribe(views, () => ({
    entry: views.get(store, name),
    loaded: views.loaded(),
  }));
  const entry = $derived(found.current.entry);
  // Spread, because Biome reads a prop called scope as the HTML attribute.
  const notFoundProps = $derived({ scope: "view" as const, name });

  // The view of every Store, one object, so the lists below do not start over at each render.
  const everywhere: ListScope = { kind: "all" };

  // What is saved, and what the screen holds on top of it (the filter, the columns, the swimlane,
  // the group). local is null while the screen is as saved. The layout is always the saved one:
  // a tab changes it in the file at once, and an edit that is not saved does not go with it (D6).
  const saved = $derived(entry ? draftOf(entry.view) : null);
  // It starts with the change a rename carried over; the page is keyed by store and name, so the
  // first values are the ones to ask for.
  // svelte-ignore state_referenced_locally
  let local = $state.raw<ViewDraft | null>(takeCarriedDraft(store, name));
  const draft = $derived(saved ? { ...(local ?? saved), layout: saved.layout } : null);
  const dirty = $derived(local !== null && saved !== null && !sameDraft(local, saved));
  const problem = $derived(draft ? treeOrderProblem(draft) : null);

  let busy = $state(false);

  function edit(part: Partial<Pick<ViewDraft, "query" | "kanban" | "table">>) {
    if (draft) local = { ...draft, ...part };
  }

  async function write(body: Parameters<typeof views.update>[2]): Promise<boolean> {
    busy = true;
    try {
      await views.update(store, name, body);
      return true;
    } catch (e) {
      toast({ message: errorMessage(e) });
      return false;
    } finally {
      busy = false;
    }
  }

  async function switchLayout(layout: Layout) {
    if (busy || !saved || layout === saved.layout) return;
    await write({ layout });
  }

  async function update() {
    if (busy || !dirty || !draft || problem !== null) return;
    const sent = local;
    if (await write(saveBodyOf(draft))) {
      // An edit made while the write ran is not lost.
      if (local === sent) local = null;
      toast({ message: `ビュー ${name} を更新しました` });
    }
  }

  // The server refuses the tree for a saved query that has a sort: or a group: (view.tree_order).
  const treeBlock = $derived(
    saved && saved.layout !== "tree"
      ? treeOrderProblem({ layout: "tree", query: saved.query })
      : null,
  );
  const items = $derived(
    LAYOUTS.map((t) => ({
      label: t.label,
      href: router.href({ name: "view", store, view: name }),
      current: t.layout === saved?.layout,
      onselect: () => void switchLayout(t.layout),
      disabled: t.layout === "tree" ? (treeBlock ?? undefined) : undefined,
    })),
  );

  let more = $state<HTMLElement | null>(null);
  const moreItems = [
    { value: "rename", label: "改名…" },
    { value: "delete", label: "削除…" },
  ];
  function pickMore(value: string) {
    more = null;
    if (!entry) return;
    if (value === "rename") openViewRename(entry, dirty ? local : null);
    else openViewDelete(entry);
  }

  const showCli = $derived(cliViewShow(name, store));
  // The commands of what the buttons do (Foundations: the same operation as a command).
  const updateCli = $derived(draft ? cliViewSave({ name, store, body: saveBodyOf(draft) }) : "");
  const switchCli = $derived(
    saved ? cliViewSave({ name, store, body: { layout: saved.layout } }) : "",
  );
</script>

{#if entry && draft && saved}
  <div class="page">
    <header class="head">
      <div class="title">
        <h1>{entry.view.name}</h1>
        <StoreBadge store={entry.store} tone={entry.tone} />
        <span class="eyebrow">保存済みビュー</span>
        <span class="path">{entry.store} の views/{entry.view.name}.yaml</span>
      </div>
      <div class="actions">
        {#if dirty}
          <span class="dirty" role="status">未保存の変更</span>
          <Button
            variant="primary"
            size="sm"
            disabled={busy || problem !== null}
            onclick={() => void update()}
            >ビューを更新</Button
          >
        {/if}
        <Button
          size="sm"
          aria-label="ビューの操作（改名・削除）"
          aria-haspopup="menu"
          aria-expanded={more !== null}
          onclick={(e) => (more = e.currentTarget)}
        >
          <Icon name="more" size={14} />
        </Button>
        {#if more}
          <Menu
            anchor={more}
            items={moreItems}
            label="ビューの操作"
            onselect={pickMore}
            onclose={() => (more = null)}
          />
        {/if}
      </div>
    </header>
    {#if dirty}
      <div class="update">
        {#if problem}
          <p class="problem" role="alert">{problem}</p>
        {:else}
          <span class="hint">今の設定で保存し直します（1 コミット）</span>
        {/if}
        <CliLine command={updateCli} disabled={problem !== null} />
      </div>
    {/if}
    <Tabs {items} label="レイアウトの切り替え" />
    {#if treeBlock}
      <p class="problem blocked">ツリーには切り替えられません。{treeBlock}</p>
    {/if}
    <div class="switch">
      <span class="hint">切り替えは保存したビューの layout を変えます（1 コミット）</span>
      <CliLine command={switchCli} />
    </div>
    <div class="body">
      {#if saved.layout === "kanban"}
        <KanbanView
          view={everywhere}
          query={draft.query}
          onquery={(q) => edit({ query: q })}
          settings={draft.kanban}
          onsettings={(s: KanbanSettings) => edit({ kanban: s })}
          cli={showCli}
          saved={{ name, store }}
        />
      {:else if saved.layout === "table"}
        <TableView
          view={everywhere}
          query={draft.query}
          onquery={(q) => edit({ query: q })}
          settings={draft.table}
          onsettings={(s: TableSettings) => edit({ table: s })}
          cli={showCli}
          saved={{ name, store }}
        />
      {:else}
        <TreeView
          view={everywhere}
          query={draft.query}
          onquery={(q) => edit({ query: q })}
          cli={showCli}
        />
      {/if}
    </div>
  </div>
{:else if found.current.loaded}
  <Empty
    variant="notfound"
    {...notFoundProps}
    onHome={() => router.go(router.href({ name: "home" }))}
  />
{:else}
  <p class="note">読み込み中…</p>
{/if}

<style>
  .page {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
  }
  .head {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-12);
    padding: var(--space-16) var(--gutter) 0;
  }
  .title {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-10);
    min-width: 0;
  }
  h1 {
    margin: 0;
    font-size: var(--fs-19);
    font-weight: 600;
  }
  .eyebrow {
    color: var(--label);
    font-size: var(--fs-11);
    font-weight: 600;
    letter-spacing: 0.04em;
  }
  .path {
    color: var(--muted);
    font-family: var(--font-mono);
    font-size: var(--fs-12);
  }
  .actions {
    display: flex;
    align-items: center;
    gap: var(--space-8);
  }
  .dirty {
    color: var(--warning);
    font-size: var(--fs-12);
    font-weight: 600;
  }
  .update,
  .switch {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-10);
    padding: var(--space-8) var(--gutter) 0;
  }
  .hint {
    color: var(--muted);
    font-size: var(--fs-12);
  }
  .problem {
    margin: 0;
    padding: var(--space-6) var(--space-10);
    border-radius: var(--radius-8);
    background: var(--warning-weak);
    font-size: var(--fs-13);
  }
  .blocked {
    margin: var(--space-8) var(--gutter) 0;
  }
  .body {
    flex: 1 1 auto;
    min-height: 0;
  }
  .note {
    margin: var(--space-16) var(--space-12);
    color: var(--muted);
  }
</style>
