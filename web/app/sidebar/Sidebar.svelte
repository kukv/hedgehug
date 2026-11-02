<script lang="ts">
  import { onMount } from "svelte";
  import { core } from "../context";
  import { openCreate } from "../create-state.svelte";
  import Icon from "../icons/Icon.svelte";
  import KindIcon from "../icons/KindIcon.svelte";
  import { kbdLabel, registerShortcut } from "../keys";
  import { openPalette } from "../palette-state.svelte";
  import { hrefOf, onLinkClick, router } from "../router";
  import { useSubscribe } from "../svelte-store";
  import StoreBadge from "../ui/StoreBadge.svelte";
  import SyncCard from "./SyncCard.svelte";

  const { stores } = core();
  const workspaces = useSubscribe(stores, () => stores.workspaces());
  const route = $derived(router.route);

  onMount(() =>
    registerShortcut("mod+k", () => {
      // Another dialog is open: leave it alone. The palette itself is not one.
      if (document.querySelector('[aria-modal="true"]:not([data-palette])')) return;
      openPalette();
    }),
  );

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
  </div>

  <button type="button" class="search" onclick={openPalette}>
    <Icon name="search" size={15} />
    <span class="ph">ID・検索・コマンド</span>
    <kbd>{kbdLabel("mod+k")}</kbd>
  </button>

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
      {@const href = hrefOf({ name: "workspace", store: w.store, ulid: w.ulid })}
      {@const wsHere = workspaceActive(w.store, w.ulid, i)}
      <div class="ws">
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
        <div class="projects">
          {#each w.projects as p (p.ulid)}
            {@const phref = hrefOf({ name: "project", ulid: p.ulid })}
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
  .ws-link,
  .project {
    display: flex;
    align-items: center;
    gap: var(--space-8);
    padding: 0 var(--space-8);
    border-radius: var(--radius-6);
    color: var(--ink-2);
    text-decoration: none;
  }
  .ws-link {
    min-height: var(--row-h);
  }
  .project {
    min-height: 30px;
  }
  .ws-link:hover,
  .project:hover {
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
