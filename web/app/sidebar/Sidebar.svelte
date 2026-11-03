<script lang="ts">
  import { onMount } from "svelte";
  import type { WorkspaceEntry } from "../../core/stores";
  import { core } from "../context";
  import { openCreate } from "../create-state.svelte";
  import Icon from "../icons/Icon.svelte";
  import KindIcon from "../icons/KindIcon.svelte";
  import { kbdLabel, registerShortcut } from "../keys";
  import { MY_QUERY } from "../my/groups";
  import { openPalette } from "../palette-state.svelte";
  import { hrefOf, onLinkClick, router } from "../router";
  import { useSubscribe } from "../svelte-store";
  import Menu from "../ui/Menu.svelte";
  import StoreBadge from "../ui/StoreBadge.svelte";
  import { openWorkspaceDelete } from "../workspace/dialogs-state.svelte";
  import SyncCard from "./SyncCard.svelte";

  const { stores, cache, views } = core();
  const workspaces = useSubscribe(stores, () => stores.workspaces());
  const savedViews = useSubscribe(views, () => views.list());
  const cacheRev = useSubscribe(cache, () => ({}));
  const route = $derived(router.route);
  // /settings names no Store: the settings go to the first one.
  const settingsHref = hrefOf({ name: "settings", page: "workspaces", store: "" });
  const myHref = hrefOf({ name: "my" });

  // The count beside "マイタスク" is the number the screen shows with its own query (細部 55). It
  // is left out while the list is read, and when the query does not run (no account).
  let myList = $state.raw<ReturnType<typeof cache.watchList> | null>(null);
  $effect(() => {
    const w = cache.watchList({ q: MY_QUERY, tree: false });
    myList = w;
    return () => w.dispose();
  });
  const myCount = $derived.by(() => {
    cacheRev.current;
    if (!myList || myList.loading() || myList.error()) return null;
    return myList.ulids().length;
  });

  onMount(() =>
    registerShortcut("mod+k", () => {
      // Another dialog is open: leave it alone. The palette itself is not one.
      if (document.querySelector('[aria-modal="true"]:not([data-palette])')) return;
      openPalette();
    }),
  );

  // The menu of the "…" of a workspace row: the workspace, and the button it opens from.
  let menu = $state<{ entry: WorkspaceEntry; anchor: HTMLElement } | null>(null);
  const menuItems = [
    { value: "settings", label: "設定を開く" },
    { value: "delete", label: "削除…" },
  ];

  function pickMenu(value: string) {
    const m = menu;
    menu = null;
    if (!m) return;
    if (value === "settings") {
      router.go(
        hrefOf({ name: "workspace", store: m.entry.store, ulid: m.entry.ulid, tab: "settings" }),
      );
    } else {
      openWorkspaceDelete(m.entry);
    }
  }

  function workspaceActive(store: string, ulid: string, index: number): boolean {
    const r = route;
    if (r.name === "workspace") return r.store === store && r.ulid === ulid;
    return r.name === "home" && index === 0;
  }
</script>

<nav class="side" aria-label="メインナビゲーション">
  <div class="top">
    <span class="logo" aria-hidden="true">hh</span>
    <span class="host">{location.host}</span>
    <a
      class="gear"
      href={settingsHref}
      aria-label="設定"
      aria-current={route.name === "settings" ? "page" : undefined}
      onclick={(e) => onLinkClick(e, settingsHref)}
    >
      <Icon name="settings" size={16} />
    </a>
  </div>

  <button type="button" class="search" onclick={openPalette}>
    <Icon name="search" size={15} />
    <span class="ph">ID・検索・コマンド</span>
    <kbd>{kbdLabel("mod+k")}</kbd>
  </button>

  <div class="places">
    <a
      href={myHref}
      class="nav-link"
      class:active={route.name === "my"}
      aria-current={route.name === "my" ? "page" : undefined}
      onclick={(e) => onLinkClick(e, myHref)}
    >
      <Icon name="checklist" size={14} />
      <span class="nav-text">マイタスク</span>
      {#if myCount !== null}
        <span class="count">{myCount}</span>
      {/if}
    </a>
  </div>

  {#if savedViews.current.length > 0}
    <section class="views" aria-label="保存済みビュー">
      <div class="ws-head">
        <span class="ws-title">保存済みビュー</span>
      </div>
      {#each savedViews.current as v (`${v.store}/${v.view.name}`)}
        {@const vhref = hrefOf({ name: "view", store: v.store, view: v.view.name })}
        {@const vhere =
          route.name === "view" && route.store === v.store && route.view === v.view.name}
        <a
          href={vhref}
          class="view"
          class:active={vhere}
          aria-current={vhere ? "page" : undefined}
          title="{v.store} の views/ に保存"
          onclick={(e) => onLinkClick(e, vhref)}
        >
          <span class="dot" style:background="var(--store-{v.tone}-dot)" aria-hidden="true"></span>
          <span class="p-name">{v.view.name}</span>
        </a>
      {/each}
    </section>
  {/if}

  <section class="ws-section" aria-label="ワークスペース">
    <div class="ws-head">
      <span class="ws-title">ワークスペース</span>
      <button
        type="button"
        class="add"
        aria-label="プロジェクトを追加"
        onclick={() => openCreate({ kind: "project" })}
      >
        <Icon name="folder-plus" size={14} />
      </button>
    </div>

    {#each workspaces.current as w, i (`${w.store}/${w.ulid}`)}
      {@const href = hrefOf({ name: "workspace", store: w.store, ulid: w.ulid, tab: "overview" })}
      {@const wsHere = workspaceActive(w.store, w.ulid, i)}
      <div class="ws">
        <div
          class="ws-row"
          class:open={menu?.entry.store === w.store && menu?.entry.ulid === w.ulid}
        >
          <a
            {href}
            class="ws-link"
            class:active={wsHere}
            aria-current={wsHere ? "page" : undefined}
            onclick={(e) => onLinkClick(e, href)}
          >
            <Icon name="workspace" size={13} />
            <span class="ws-text">
              <span class="ws-name">{w.name}</span>
              <span class="ws-meta">
                <StoreBadge store={w.store} tone={w.tone} compact />
                <span class="prefix" title="ID の接頭辞">{w.prefix}</span>
              </span>
            </span>
          </a>
          <button
            type="button"
            class="more"
            aria-label="{w.name} の操作（設定を開く・削除）"
            aria-haspopup="menu"
            onclick={(e) => (menu = { entry: w, anchor: e.currentTarget })}
          >
            <Icon name="more" size={14} />
          </button>
        </div>
        <div class="projects">
          {#each w.projects as p (p.ulid)}
            {@const phref = hrefOf({ name: "project", ulid: p.ulid, tab: "tree" })}
            {@const here = route.name === "project" && route.ulid === p.ulid}
            <a
              href={phref}
              class="project"
              class:active={here}
              aria-current={here ? "page" : undefined}
              onclick={(e) => onLinkClick(e, phref)}
            >
              <KindIcon kind="project" size={14} />
              <span class="p-name">{p.title}</span>
              {#if p.progress}
                <span class="progress">{p.progress.done}/{p.progress.total}</span>
              {/if}
            </a>
          {/each}
          {#if w.projects.length === 0}
            <span class="none">プロジェクトなし</span>
          {/if}
        </div>
      </div>
    {/each}
  </section>

  <div class="sync"><SyncCard /></div>
</nav>

{#if menu}
  <Menu
    anchor={menu.anchor}
    label="ワークスペースの操作"
    items={menuItems}
    onselect={pickMenu}
    onclose={() => (menu = null)}
  />
{/if}

<style>
  .side {
    display: flex;
    flex-direction: column;
    gap: var(--space-16);
    min-height: 100%;
    padding: 14px var(--space-10);
  }
  .top {
    display: flex;
    align-items: center;
    gap: var(--space-8);
    padding: 2px var(--space-4) 2px var(--space-8);
  }
  .logo {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 24px;
    height: 24px;
    border-radius: var(--radius-6);
    background: var(--ink);
    color: var(--surface);
    font-family: var(--font-mono);
    font-size: var(--fs-12);
    font-weight: 500;
  }
  .host {
    color: var(--label);
    font-family: var(--font-mono);
    font-size: var(--fs-11);
  }
  .gear {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: var(--control-h-sm);
    height: var(--control-h-sm);
    margin-left: auto;
    border-radius: var(--radius-6);
    color: var(--label);
  }
  .gear:hover {
    background: var(--hover);
  }
  .search {
    display: flex;
    align-items: center;
    gap: var(--space-8);
    width: 100%;
    min-height: var(--control-h);
    padding: 0 var(--space-10);
    border: 1px solid var(--line-input);
    border-radius: var(--radius-8);
    background: var(--surface);
    color: var(--label);
    text-align: left;
    cursor: pointer;
  }
  .ph {
    flex: 1;
  }
  kbd {
    font-size: var(--fs-11);
  }
  .ws-section {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .ws-head {
    display: flex;
    align-items: center;
    padding: 0 var(--space-4) var(--space-4) var(--space-10);
  }
  .ws-title {
    flex: 1;
    color: var(--label);
    font-size: var(--fs-11);
    font-weight: 600;
    letter-spacing: 0.04em;
  }
  .add {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: var(--control-h-sm);
    height: var(--control-h-sm);
    border: 0;
    border-radius: var(--radius-6);
    background: transparent;
    color: var(--label);
    cursor: pointer;
  }
  .add:hover {
    background: var(--hover);
  }
  .ws {
    display: flex;
    flex-direction: column;
    gap: 1px;
  }
  .ws-row {
    display: flex;
    align-items: center;
  }
  /* The "…" shows when the row is pointed at or has the focus, or while its menu is open. */
  .more {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    flex: 0 0 auto;
    width: var(--control-h-sm);
    height: var(--control-h-sm);
    margin-left: calc(var(--control-h-sm) * -1);
    border: 0;
    border-radius: var(--radius-6);
    background: transparent;
    color: var(--label);
    opacity: 0;
    cursor: pointer;
  }
  .ws-row:hover .more,
  .ws-row:focus-within .more,
  .ws-row.open .more {
    opacity: 1;
  }
  .more:hover {
    background: var(--hover);
  }
  .ws-link,
  .project,
  .nav-link,
  .view {
    display: flex;
    align-items: center;
    gap: var(--space-8);
    padding: 0 var(--space-8);
    border-radius: var(--radius-6);
    color: var(--ink-2);
    text-decoration: none;
  }
  .nav-link {
    min-height: var(--row-h);
  }
  .nav-text {
    flex: 1;
  }
  .count {
    color: var(--label);
    font-family: var(--font-mono);
    font-size: var(--fs-11);
  }
  .places,
  .views {
    display: flex;
    flex-direction: column;
    gap: 1px;
  }
  .dot {
    flex: 0 0 auto;
    width: 8px;
    height: 8px;
    border-radius: 2px;
  }
  .ws-link {
    flex: 1;
    min-width: 0;
    min-height: var(--row-h);
    padding-right: var(--control-h-sm);
  }
  .project,
  .view {
    min-height: 30px;
  }
  .ws-link:hover,
  .project:hover,
  .nav-link:hover,
  .view:hover {
    background: var(--hover);
  }
  .active,
  .active:hover {
    background: var(--accent-weak);
    color: var(--accent-ink);
    font-weight: 600;
  }
  .ws-text {
    display: flex;
    flex: 1;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
    line-height: 1.3;
  }
  .ws-name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-weight: 500;
  }
  .ws-meta {
    display: flex;
    align-items: center;
    gap: var(--space-6);
  }
  .prefix {
    color: var(--label);
    font-family: var(--font-mono);
    font-size: 10.5px;
  }
  .projects {
    display: flex;
    flex-direction: column;
    gap: 1px;
    margin-left: var(--space-16);
    padding-left: var(--space-6);
    border-left: 1px solid var(--line-tree);
  }
  .p-name {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .progress {
    color: var(--label);
    font-family: var(--font-mono);
    font-size: var(--fs-11);
  }
  .none {
    padding: var(--space-4) var(--space-8);
    color: var(--muted);
    font-size: var(--fs-12);
  }
  .sync {
    margin-top: auto;
  }
</style>
