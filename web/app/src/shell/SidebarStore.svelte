<script lang="ts">
  import { projectCategory, type Resolved, type Route, type StoreNode } from "@hedgehug/core";
  import { StatusDot, StoreBadge } from "@hedgehug/ui";
  import type { Links } from "../links";
  import StoreMenu from "./StoreMenu.svelte";
  import WorkspaceMenu from "./WorkspaceMenu.svelte";

  // サイドバーの Store ごとのブロック(04「Store ごとのブロック」)。
  interface Props {
    store: StoreNode;
    current: Resolved | null;
    links: Links;
    // collapsed は、プロジェクトの行を隠しているワークスペースの内部 ID。
    collapsed: Set<string>;
    ontoggle: (workspaceId: string) => void;
  }

  let { store, current, links, collapsed, ontoggle }: Props = $props();

  const wsRoute = (prefix: string): Route => ({
    name: "workspace",
    store: store.name,
    prefix,
    tab: "overview",
  });
  const projectRoute = (displayId: string): Route => ({
    name: "project",
    store: store.name,
    project: displayId,
    tab: "overview",
  });
  const isWs = (id: string) =>
    current?.name === "workspace" && current.storeId === store.id && current.workspaceId === id;
  const isProject = (id: string) =>
    current?.name === "project" && current.storeId === store.id && current.projectId === id;
</script>

<section class="side-sec" aria-label={store.name}>
  <div class="side-head">
    <StoreBadge name={store.name} color={store.color} /><StoreMenu {store} />
  </div>
  {#each store.workspaces as w (w.id)}
    <div class="ws-row">
      <button
        type="button"
        class="twisty"
        class:open={!collapsed.has(w.id)}
        aria-label="開閉"
        aria-expanded={!collapsed.has(w.id)}
        onclick={() => ontoggle(w.id)}
      >
        ›
      </button>
      <a
        class="ws-name"
        class:on={isWs(w.id)}
        aria-current={isWs(w.id) ? "page" : undefined}
        href={links.href(wsRoute(w.prefix))}
        onclick={(e) => links.click(e, wsRoute(w.prefix))}
        onauxclick={(e) => links.click(e, wsRoute(w.prefix))}
        >{w.name}</a
      >
      <code class="prefix">{w.prefix}</code>
      <WorkspaceMenu {store} workspace={w} />
    </div>
    {#if !collapsed.has(w.id)}
      {#each w.projects as p (p.id)}
        <a
          class="proj-row"
          class:on={isProject(p.id)}
          aria-current={isProject(p.id) ? "page" : undefined}
          href={links.href(projectRoute(p.displayId))}
          onclick={(e) => links.click(e, projectRoute(p.displayId))}
          onauxclick={(e) => links.click(e, projectRoute(p.displayId))}
        >
          <StatusDot category={projectCategory(p.status)} label={p.status} />
          <span>{p.title}</span>
        </a>
      {/each}
    {/if}
  {:else}
    <p class="side-empty">ワークスペースはまだありません</p>
  {/each}
</section>

<style>
  .side-sec {
    display: flex;
    flex-direction: column;
    gap: 1px;
    padding-top: 8px;
  }
  .side-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 4px 4px;
  }
  .ws-row {
    display: flex;
    align-items: center;
    gap: 4px;
    min-height: 28px;
  }
  .twisty {
    width: 18px;
    height: 18px;
    border: 0;
    background: none;
    color: var(--muted);
    cursor: pointer;
    transition: transform 0.12s;
  }
  .twisty.open {
    transform: rotate(90deg);
  }
  .ws-name,
  .proj-row {
    color: var(--ink);
    text-decoration: none;
    border-radius: 6px;
  }
  .ws-name {
    flex: 1;
    min-width: 0;
    font-weight: 500;
    padding: 3px 4px;
  }
  .proj-row {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 4px 6px 4px 26px;
    min-height: 28px;
  }
  .ws-name:hover,
  .proj-row:hover {
    background: var(--hover);
  }
  .on {
    background: var(--accent-faint);
    color: var(--accent-ink);
  }
  .prefix {
    font-family: var(--font-mono);
    font-size: 11px;
    color: var(--muted);
  }
  .side-empty {
    color: var(--muted);
    font-size: 12px;
    margin: 4px 6px;
  }
</style>
