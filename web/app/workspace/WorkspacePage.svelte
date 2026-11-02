<script lang="ts">
  import { core } from "../context";
  import { router, type WorkspaceTab } from "../router";
  import Empty from "../states/Empty.svelte";
  import { useSubscribe } from "../svelte-store";
  import TreeView from "../tree/TreeView.svelte";
  import WorkspaceHeader from "./WorkspaceHeader.svelte";
  import WorkspaceOverview from "./WorkspaceOverview.svelte";
  import WorkspaceSettings from "./WorkspaceSettings.svelte";

  type Props = { store: string; ulid: string; tab: WorkspaceTab };
  let { store, ulid, tab }: Props = $props();

  const c = core();
  const found = useSubscribe(c.stores, () => ({
    entry: c.stores.workspace(store, ulid),
    loaded: c.stores.loaded(),
  }));
  const entry = $derived(found.current.entry);
  // Spread, because Biome reads a prop called scope as the HTML attribute.
  const notFoundProps = $derived({ scope: "workspace" as const, name: ulid });
</script>

{#if entry}
  <div class="page">
    {#if tab === "overview"}
      <!-- The overview draws its own heading, which has the buttons that make a task or a project. -->
      <WorkspaceOverview {entry} />
    {:else}
      <WorkspaceHeader {entry} {tab} />
      <div class="body">
        {#if tab === "tree"}
          <TreeView view={{ kind: "workspace", store, ulid }} />
        {:else}
          <WorkspaceSettings {entry} />
        {/if}
      </div>
    {/if}
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
  /* The tree scrolls in its own box, so the page is the height of the window and the body takes
     what the heading leaves. */
  .page {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
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
